//! Display-independent protocol validation and UI ownership model.
use std::collections::{HashMap, HashSet};

pub mod pb {
    include!(concat!(env!("OUT_DIR"), "/gpui.v1.rs"));
}
use pb::*;
pub mod features;
pub mod text;

pub const MAX_MESSAGE_BYTES: usize = 4 * 1024 * 1024;
// Keep ample room below prost's recursion limit for envelopes and style fields.
pub const MAX_TREE_DEPTH: usize = 32;
pub const MAX_ELEMENTS: usize = 10_000;

pub fn error(code: ErrorCode, message: impl Into<String>) -> ProtocolError {
    ProtocolError {
        code: code as i32,
        message: message.into(),
    }
}
type Result<T> = std::result::Result<T, ProtocolError>;
fn invalid(message: impl Into<String>) -> ProtocolError {
    error(ErrorCode::InvalidArgument, message)
}

#[derive(Clone)]
pub struct View {
    pub window: Handle,
    pub entity: Handle,
    pub options: WindowOptions,
    pub snapshot: ViewSnapshot,
}

/// Implementations are called only on their UI thread. No GPUI values cross
/// this boundary. A backend must return an error before committing any effect.
pub trait Backend {
    fn open(&mut self, view: &View) -> std::result::Result<(), String>;
    fn commit(&mut self, view: &View) -> std::result::Result<(), String>;
    fn close(&mut self, view: &View) -> std::result::Result<(), String>;
    fn quit(&mut self);
    fn resource(&mut self, _: &Resource, _: bool) -> std::result::Result<(), String> {
        Ok(())
    }
    fn features(&self) -> Vec<String> {
        Vec::new()
    }
    fn automate(&mut self, _: &View, _: &AutomationRequest) -> Result<AutomationResult> {
        Err(error(
            ErrorCode::Unsupported,
            "test automation is not enabled",
        ))
    }
    fn model(&mut self, _: &ModelState, _: bool) -> std::result::Result<(), String> {
        Ok(())
    }
    fn command(&mut self, _: Option<&View>, command: &Command) -> Result<CommandResult> {
        match command.kind {
            Some(command::Kind::Focus(_) | command::Kind::ScrollTo(_)) => {
                Ok(CommandResult::default())
            }
            _ => Err(error(
                ErrorCode::Unsupported,
                "platform service is not available in this backend",
            )),
        }
    }
}

#[derive(Default)]
pub struct Headless;
impl Backend for Headless {
    fn open(&mut self, _: &View) -> std::result::Result<(), String> {
        Ok(())
    }
    fn commit(&mut self, _: &View) -> std::result::Result<(), String> {
        Ok(())
    }
    fn close(&mut self, _: &View) -> std::result::Result<(), String> {
        Ok(())
    }
    fn quit(&mut self) {}
}

pub struct Session {
    pub id: u64,
    next_id: u64,
    views: HashMap<u64, View>,
    closed: bool,
    resources: HashMap<u64, Resource>,
    models: HashMap<u64, (ModelState, HashSet<u64>)>,
}

impl Session {
    pub fn new(id: u64) -> Self {
        assert_ne!(id, 0);
        Self {
            id,
            next_id: 1,
            views: HashMap::new(),
            closed: false,
            resources: HashMap::new(),
            models: HashMap::new(),
        }
    }

    pub fn capabilities(&self) -> Capabilities {
        Capabilities {
            major: 1,
            minor: 3,
            session_id: self.id,
            features: [
                "container",
                "text",
                "flex",
                "click",
                "view-snapshots",
                "event-cursors",
                "hover",
                "scroll",
                "positioning",
                "font-weight",
                "text-input",
                "resources",
                "drawing",
                "uniform-list",
                "interaction",
                "actions",
                "focus",
                "accessibility",
                "extended-style",
                "animation",
                "models",
            ]
            .map(String::from)
            .to_vec(),
            max_message_bytes: MAX_MESSAGE_BYTES as u32,
            max_tree_depth: MAX_TREE_DEPTH as u32,
            max_elements: MAX_ELEMENTS as u32,
        }
    }

    fn allocate(&mut self) -> Handle {
        let handle = Handle {
            id: self.next_id,
            generation: 1,
        };
        self.next_id = self.next_id.checked_add(1).expect("handle space exhausted");
        handle
    }

    pub fn is_closed(&self) -> bool {
        self.closed
    }

    /// Returns events only after the corresponding backend operation succeeds.
    pub fn apply(
        &mut self,
        request: Request,
        backend: &mut impl Backend,
    ) -> (Response, Vec<Event>) {
        let request_id = request.request_id;
        let mut events = Vec::new();
        let body = self
            .apply_inner(request, backend, &mut events)
            .unwrap_or_else(response::Body::Error);
        (
            Response {
                request_id,
                body: Some(body),
            },
            events,
        )
    }

    fn apply_inner(
        &mut self,
        request: Request,
        backend: &mut impl Backend,
        events: &mut Vec<Event>,
    ) -> Result<response::Body> {
        if request.request_id == 0 {
            return Err(invalid("request_id must be nonzero"));
        }
        if self.closed {
            return Err(error(ErrorCode::Closed, "application is closed"));
        }
        let body = request
            .body
            .ok_or_else(|| invalid("missing or unknown request body"))?;
        if let request::Body::Hello(hello) = body {
            if request.session_id != 0 {
                return Err(invalid("Hello must use session_id zero"));
            }
            if hello.major != 1 {
                return Err(error(ErrorCode::Unsupported, "unsupported protocol major"));
            }
            let mut capabilities = self.capabilities();
            capabilities.features.extend(backend.features());
            return Ok(response::Body::Capabilities(capabilities));
        }
        if request.session_id != self.id {
            return Err(error(ErrorCode::WrongSession, "wrong application session"));
        }
        match body {
            request::Body::OpenWindow(open) => {
                if self.views.len() >= 64 {
                    return Err(error(ErrorCode::Backpressure, "window limit reached"));
                }
                let options = open
                    .options
                    .ok_or_else(|| invalid("missing window options"))?;
                let size = options
                    .size
                    .as_ref()
                    .ok_or_else(|| invalid("missing window size"))?;
                if !size.width.is_finite()
                    || !size.height.is_finite()
                    || size.width < 1.
                    || size.height < 1.
                    || size.width > 16384.
                    || size.height > 16384.
                {
                    return Err(invalid(
                        "window dimensions must be finite and in [1, 16384]",
                    ));
                }
                if options.title.len() > 4096 || options.title.contains('\0') {
                    return Err(invalid("invalid window title"));
                }
                let snapshot = open
                    .initial_view
                    .ok_or_else(|| invalid("missing initial view"))?;
                validate_snapshot(&snapshot)?;
                self.validate_references(&snapshot)?;
                features::validate_embedded(&snapshot, &*backend)?;
                let view = View {
                    window: self.allocate(),
                    entity: self.allocate(),
                    options,
                    snapshot,
                };
                backend
                    .open(&view)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                let opened = WindowOpened {
                    window: Some(view.window.clone()),
                    view: Some(view.entity.clone()),
                    revision: view.snapshot.revision,
                };
                self.views.insert(view.entity.id, view);
                Ok(response::Body::WindowOpened(opened))
            }
            request::Body::CommitView(commit) => {
                let handle = commit.view.ok_or_else(|| invalid("missing view handle"))?;
                let old = self.view(&handle)?;
                let snapshot = commit
                    .snapshot
                    .ok_or_else(|| invalid("missing view snapshot"))?;
                if snapshot.revision <= old.snapshot.revision {
                    return Err(error(
                        ErrorCode::StaleRevision,
                        "view revision must increase",
                    ));
                }
                validate_snapshot(&snapshot)?;
                self.validate_references(&snapshot)?;
                features::validate_embedded(&snapshot, &*backend)?;
                let next = View {
                    snapshot,
                    ..old.clone()
                };
                backend
                    .commit(&next)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                events.push(Self::event(
                    &next,
                    event::Kind::ViewUpdated(ViewUpdated::default()),
                ));
                let committed = ViewCommitted {
                    view: Some(handle),
                    revision: next.snapshot.revision,
                };
                self.views.insert(next.entity.id, next);
                Ok(response::Body::ViewCommitted(committed))
            }
            request::Body::CloseWindow(close) => {
                let handle = close
                    .window
                    .ok_or_else(|| invalid("missing window handle"))?;
                let view = self
                    .views
                    .values()
                    .find(|v| v.window == handle)
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "window is closed or unknown"))?;
                backend
                    .close(view)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                events.push(Self::event(
                    view,
                    event::Kind::WindowClosed(WindowClosed::default()),
                ));
                self.views.remove(&view.entity.id.clone());
                Ok(response::Body::Ack(Ack::default()))
            }
            request::Body::Quit(_) => {
                backend.quit();
                self.closed = true;
                self.views.clear();
                Ok(response::Body::Ack(Ack::default()))
            }
            request::Body::Command(command) => self
                .apply_command(command, backend, events)
                .map(response::Body::Result),
            request::Body::Automation(request) => {
                if request.version != 1 || request.operation.is_none() {
                    return Err(invalid("invalid automation version or operation"));
                }
                let view = self.view(
                    request
                        .view
                        .as_ref()
                        .ok_or_else(|| invalid("missing automation view"))?,
                )?;
                if features::find_element(view.snapshot.root.as_ref().unwrap(), request.element_id)
                    .is_none()
                {
                    return Err(invalid("unknown automation element"));
                }
                backend
                    .automate(view, &request)
                    .map(response::Body::Automation)
            }
            request::Body::NextEvents(_) => {
                Err(invalid("NextEvents must use the event transport operation"))
            }
            request::Body::Hello(_) => unreachable!(),
        }
    }

    fn view(&self, handle: &Handle) -> Result<&View> {
        self.views
            .get(&handle.id)
            .filter(|v| v.entity == *handle)
            .ok_or_else(|| error(ErrorCode::StaleHandle, "view is closed or unknown"))
    }

    fn event(view: &View, kind: event::Kind) -> Event {
        Event {
            window: Some(view.window.clone()),
            view: Some(view.entity.clone()),
            revision: view.snapshot.revision,
            kind: Some(kind),
            ..Default::default()
        }
    }

    pub fn window_closed(&mut self, window: &Handle) -> Option<Event> {
        let id = self.views.values().find(|v| v.window == *window)?.entity.id;
        let view = self.views.remove(&id)?;
        Some(Self::event(
            &view,
            event::Kind::WindowClosed(WindowClosed::default()),
        ))
    }

    /// Ignore input captured from an obsolete render tree. Handler IDs cannot
    /// be repurposed by an older frame after the next snapshot is installed.
    pub fn click(
        &self,
        entity: &Handle,
        revision: u64,
        element_id: u64,
        click: Click,
    ) -> Option<Event> {
        let view = self.view(entity).ok()?;
        if view.snapshot.revision != revision {
            return None;
        }
        let binding = features::find_element(view.snapshot.root.as_ref()?, element_id)?
            .on_click
            .as_ref()?;
        let mut event = Self::event(view, event::Kind::Click(click));
        event.element_id = element_id;
        event.handler_id = binding.handler_id;
        Some(event)
    }

    pub fn text_changed(
        &self,
        entity: &Handle,
        element_id: u64,
        change: TextChanged,
    ) -> Option<Event> {
        let view = self.view(entity).ok()?;
        let element = features::find_element(view.snapshot.root.as_ref()?, element_id)?;
        let Some(element::Kind::TextInput(input)) = &element.kind else {
            return None;
        };
        if input.disabled
            || change.epoch != input.epoch
            || change.edit_revision <= input.edit_revision
            || change.value.len() > text::MAX_INPUT_BYTES
        {
            return None;
        }
        let binding = input.on_change.as_ref()?;
        let mut event = Self::event(view, event::Kind::TextChanged(change));
        event.element_id = element_id;
        event.handler_id = binding.handler_id;
        Some(event)
    }
}

pub fn validate_snapshot(snapshot: &ViewSnapshot) -> Result<()> {
    if snapshot.revision == 0 {
        return Err(invalid("revision must be nonzero"));
    }
    let root = snapshot
        .root
        .as_ref()
        .ok_or_else(|| invalid("missing root element"))?;
    let mut seen = HashSet::new();
    validate_element(root, 1, &mut seen)
}

fn validate_element(element: &Element, depth: usize, seen: &mut HashSet<u64>) -> Result<()> {
    if depth > MAX_TREE_DEPTH {
        return Err(invalid("element tree exceeds depth limit"));
    }
    if element.id == 0 || !seen.insert(element.id) {
        return Err(invalid("element IDs must be nonzero and unique"));
    }
    if seen.len() > MAX_ELEMENTS {
        return Err(invalid("element count exceeds limit"));
    }
    if let Some(binding) = &element.on_click
        && binding.handler_id == 0
    {
        return Err(invalid("click handler ID must be nonzero"));
    }
    features::validate_element_features(element)?;
    if let Some(style) = &element.style {
        validate_style(style)?;
    }
    if let Some(style) = &element.hover_style {
        validate_style(style)?;
    }
    match element
        .kind
        .as_ref()
        .ok_or_else(|| invalid("missing or unknown element kind"))?
    {
        element::Kind::Container(container) => {
            for child in &container.children {
                validate_element(child, depth + 1, seen)?;
            }
        }
        element::Kind::Text(_) => {}
        element::Kind::TextInput(input) => {
            if input.value.len() > text::MAX_INPUT_BYTES || input.placeholder.len() > 4096 {
                return Err(invalid("text input exceeds value or placeholder limit"));
            }
            if input.value != text::single_line(&input.value)
                || input.placeholder != text::single_line(&input.placeholder)
            {
                return Err(invalid("text inputs are single-line"));
            }
            if input
                .on_change
                .as_ref()
                .is_some_and(|binding| binding.handler_id == 0)
            {
                return Err(invalid("text change handler ID must be nonzero"));
            }
        }
        element::Kind::Image(image) => {
            if image
                .resource
                .as_ref()
                .is_none_or(|h| h.id == 0 || h.generation == 0)
            {
                return Err(invalid("missing image resource"));
            }
        }
        element::Kind::Drawing(drawing) => features::validate_drawing(drawing)?,
        element::Kind::Embedded(embedded) => {
            if embedded.instance == 0 {
                return Err(invalid("embedded instance must be nonzero"));
            }
        }
        element::Kind::UniformList(list) => {
            if !list.row_height.is_finite() || !(1.0..=16384.0).contains(&list.row_height) {
                return Err(invalid("invalid list row height"));
            }
            for child in &list.items {
                validate_element(child, depth + 1, seen)?;
            }
        }
    }
    Ok(())
}

fn validate_length(length: &Length, negative: bool) -> Result<()> {
    let value = match length.value {
        Some(length::Value::Pixels(n) | length::Value::Rems(n)) => n,
        Some(length::Value::Percent(n)) if (0. ..=100.).contains(&n) => n,
        Some(length::Value::Auto(true)) => return Ok(()),
        _ => return Err(invalid("invalid length")),
    };
    if !value.is_finite() || (!negative && value < 0.) || value.abs() > 1_000_000. {
        return Err(invalid("length out of range"));
    }
    Ok(())
}

fn validate_style(style: &Style) -> Result<()> {
    features::validate_style_features(style)?;
    for length in [
        &style.width,
        &style.height,
        &style.min_width,
        &style.min_height,
        &style.gap,
    ]
    .into_iter()
    .flatten()
    {
        validate_length(length, false)?;
    }
    for (edges, negative) in [(&style.padding, false), (&style.margin, true)] {
        if let Some(edges) = edges {
            for length in [&edges.top, &edges.right, &edges.bottom, &edges.left]
                .into_iter()
                .flatten()
            {
                validate_length(length, negative)?;
            }
        }
    }
    // Auto and percentages are not valid for every GPUI style slot.
    for edges in [&style.padding, &style.margin].into_iter().flatten() {
        for length in [&edges.top, &edges.right, &edges.bottom, &edges.left]
            .into_iter()
            .flatten()
        {
            if matches!(length.value, Some(length::Value::Auto(_))) {
                return Err(error(
                    ErrorCode::Unsupported,
                    "auto edges are not supported",
                ));
            }
        }
    }
    if let Some(gap) = &style.gap
        && !matches!(
            gap.value,
            Some(length::Value::Pixels(_) | length::Value::Rems(_))
        )
    {
        return Err(error(ErrorCode::Unsupported, "gap requires pixels or rems"));
    }
    for color in [&style.background, &style.text_color, &style.border_color]
        .into_iter()
        .flatten()
    {
        if [color.red, color.green, color.blue, color.alpha]
            .iter()
            .any(|n| !n.is_finite() || !(0. ..=1.).contains(n))
        {
            return Err(invalid("color channels must be finite and in [0, 1]"));
        }
    }
    for number in [
        style.font_size,
        style.border_width,
        style.corner_radius,
        style.flex_grow,
        style.flex_shrink,
    ]
    .into_iter()
    .flatten()
    {
        if !number.is_finite() || !(0. ..=1_000_000.).contains(&number) {
            return Err(invalid("invalid style number"));
        }
    }
    if style.flex_direction.is_some_and(|n| {
        !matches!(
            FlexDirection::try_from(n),
            Ok(FlexDirection::Row | FlexDirection::Column)
        )
    }) {
        return Err(invalid("unknown flex direction"));
    }
    if style.align_items.is_some_and(|n| {
        !matches!(
            Alignment::try_from(n),
            Ok(Alignment::Start | Alignment::Center | Alignment::End | Alignment::Stretch)
        )
    }) {
        return Err(invalid("unknown alignment"));
    }
    if style.justify_content.is_some_and(|n| {
        !matches!(
            Justification::try_from(n),
            Ok(Justification::Start
                | Justification::Center
                | Justification::End
                | Justification::SpaceBetween)
        )
    }) {
        return Err(invalid("unknown justification"));
    }
    for overflow in [style.overflow_x, style.overflow_y].into_iter().flatten() {
        if !matches!(
            Overflow::try_from(overflow),
            Ok(Overflow::Visible | Overflow::Hidden | Overflow::Scroll)
        ) {
            return Err(invalid("unknown overflow"));
        }
    }
    if style
        .cursor
        .is_some_and(|n| !matches!(Cursor::try_from(n), Ok(Cursor::Arrow | Cursor::Pointer)))
    {
        return Err(invalid("unknown cursor"));
    }
    if style.position.is_some_and(|n| {
        !matches!(
            Position::try_from(n),
            Ok(Position::Relative | Position::Absolute)
        )
    }) {
        return Err(invalid("unknown position"));
    }
    if style.font_weight.is_some_and(|n| !(100..=900).contains(&n)) {
        return Err(invalid("font_weight must be in [100, 900]"));
    }
    if let Some(inset) = &style.inset {
        for length in [&inset.top, &inset.right, &inset.bottom, &inset.left]
            .into_iter()
            .flatten()
        {
            validate_length(length, true)?;
        }
    }
    Ok(())
}
