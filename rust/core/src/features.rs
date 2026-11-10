//! Validation and ownership for serializable GPUI features.
use super::*;
pub const MAX_RESOURCE_BYTES: usize = 2 * 1024 * 1024;
const RESOURCE_BUDGET: usize = 32 * 1024 * 1024;

/// Enumerate child nodes uniformly, including virtual-list row snapshots.
pub fn children(element: &Element) -> &[Element] {
    match &element.kind {
        Some(element::Kind::Container(v)) => &v.children,
        Some(element::Kind::UniformList(v)) => &v.items,
        _ => &[],
    }
}
/// Find a stable element ID in a snapshot tree.
pub fn find_element(root: &Element, id: u64) -> Option<&Element> {
    if root.id == id {
        Some(root)
    } else {
        children(root).iter().find_map(|e| find_element(e, id))
    }
}
fn walk(element: &Element, f: &mut impl FnMut(&Element) -> Result<()>) -> Result<()> {
    f(element)?;
    for child in children(element) {
        walk(child, f)?;
    }
    Ok(())
}
fn bounded(text: &str, max: usize) -> bool {
    text.len() <= max && !text.contains('\0')
}
fn number(v: f32) -> bool {
    v.is_finite() && v.abs() <= 1_000_000.0
}
fn valid_color(c: &Color) -> bool {
    [c.red, c.green, c.blue, c.alpha]
        .iter()
        .all(|v| v.is_finite() && (0.0..=1.0).contains(v))
}
fn point(p: &Point) -> bool {
    number(p.x) && number(p.y)
}
fn paint_color(c: Option<&Color>) -> Result<()> {
    if c.is_some_and(|v| !valid_color(v)) {
        return Err(invalid("invalid paint color"));
    }
    Ok(())
}
/// Rejects a repeated embedded instance, and any on a backend without them.
/// One native view cannot appear twice in a tree.
pub(crate) fn validate_embedded(snapshot: &ViewSnapshot, backend: &dyn Backend) -> Result<()> {
    fn collect(element: &Element, found: &mut Vec<u64>) {
        if let Some(element::Kind::Embedded(embedded)) = &element.kind {
            found.push(embedded.instance);
        }
        for child in children(element) {
            collect(child, found);
        }
    }
    let mut found = Vec::new();
    if let Some(root) = &snapshot.root {
        collect(root, &mut found);
    }
    if found.is_empty() {
        return Ok(());
    }
    if !backend.features().iter().any(|f| f == "embedded-views") {
        return Err(error(
            ErrorCode::Unsupported,
            "this backend has no embedded views",
        ));
    }
    found.sort_unstable();
    if found.windows(2).any(|pair| pair[0] == pair[1]) {
        return Err(invalid("embedded instance appears more than once"));
    }
    Ok(())
}

pub(crate) fn validate_element_features(e: &Element) -> Result<()> {
    if e.accessibility_label
        .as_ref()
        .is_some_and(|v| !bounded(v, 4096))
    {
        return Err(invalid("invalid accessibility label"));
    }
    if let Some(a) = &e.accessibility
        && (!matches!(AccessibleRole::try_from(a.role), Ok(v) if v != AccessibleRole::Unspecified)
            || !bounded(&a.description, 4096)
            || a.value.as_ref().is_some_and(|v| !bounded(v, 4096)))
    {
        return Err(invalid("invalid accessibility metadata"));
    }
    if let Some(i) = &e.interaction {
        if !bounded(&i.accept_drop_kind, 256)
            || i.drag.as_ref().is_some_and(|d| {
                d.kind.is_empty()
                    || !bounded(&d.kind, 256)
                    || d.payload.len() > 65536
                    || !bounded(&d.preview, 4096)
            })
        {
            return Err(invalid("invalid drag metadata"));
        }
        if !bounded(&i.key_context, 256) || i.tab_index < -1 || i.tab_index > 32767 {
            return Err(invalid("invalid interaction configuration"));
        }
        if (i.key_down != 0 || i.key_up != 0 || i.focus_changed != 0 || !e.key_bindings.is_empty())
            && !i.focusable
            && !matches!(e.kind, Some(element::Kind::TextInput(_)))
        {
            return Err(invalid("keyboard handlers require a focusable node"));
        }
    } else if !e.key_bindings.is_empty() {
        return Err(invalid("key bindings require interaction configuration"));
    }
    if e.key_bindings.len() > 128 {
        return Err(invalid("too many key bindings"));
    }
    let mut keys = HashSet::new();
    let mut actions = HashMap::new();
    for b in &e.key_bindings {
        if b.handler_id == 0
            || !bounded(&b.keystroke, 128)
            || (!b.keystroke.is_empty() && !keys.insert((&b.keystroke, &b.context)))
            || b.action.as_ref().is_none_or(|a| {
                a.name.is_empty() || !bounded(&a.name, 256) || a.payload.len() > 65536
            })
        {
            return Err(invalid("invalid key binding"));
        }
        let action = b.action.as_ref().unwrap();
        if actions
            .insert(&action.name, b.handler_id)
            .is_some_and(|old| old != b.handler_id)
        {
            return Err(invalid("one logical action must use one handler"));
        }
        if !bounded(&b.context, 1024) {
            return Err(invalid("key context predicate too large"));
        }
    }
    if let Some(a) = &e.animation
        && (a.duration_ms == 0
            || a.duration_ms > 600000
            || ![a.from, a.to]
                .iter()
                .all(|v| v.is_finite() && (0.0..=1.0).contains(v)))
    {
        return Err(invalid("invalid opacity animation"));
    }
    Ok(())
}
pub(crate) fn validate_style_features(s: &Style) -> Result<()> {
    for length in [&s.max_width, &s.max_height].into_iter().flatten() {
        validate_length(length, false)?;
    }
    if s.display.is_some_and(|v| {
        !matches!(
            Display::try_from(v),
            Ok(Display::Flex | Display::Grid | Display::None)
        )
    }) || s.grid_columns.is_some_and(|v| v == 0 || v > 1024)
        || s.opacity
            .is_some_and(|v| !v.is_finite() || !(0.0..=1.0).contains(&v))
        || s.font_family
            .as_ref()
            .is_some_and(|v| v.is_empty() || !bounded(v, 256))
        || s.text_align.is_some_and(|v| {
            !matches!(
                TextAlignment::try_from(v),
                Ok(TextAlignment::Left | TextAlignment::Center | TextAlignment::Right)
            )
        })
    {
        return Err(invalid("invalid extended style"));
    }
    if s.shadows.len() > 8 {
        return Err(invalid("too many shadows"));
    }
    for shadow in &s.shadows {
        paint_color(shadow.color.as_ref())?;
        if shadow.offset.as_ref().is_none_or(|p| !point(p))
            || !number(shadow.blur)
            || shadow.blur < 0.0
            || !number(shadow.spread)
        {
            return Err(invalid("invalid shadow"));
        }
    }
    Ok(())
}
pub(crate) fn validate_drawing(d: &Drawing) -> Result<()> {
    if d.commands.len() > 4096 {
        return Err(invalid("too many draw commands"));
    }
    let mut steps = 0;
    for command in &d.commands {
        match &command.kind {
            Some(draw_command::Kind::Rect(r)) => {
                let b = r
                    .bounds
                    .as_ref()
                    .ok_or_else(|| invalid("missing rectangle bounds"))?;
                if b.origin.as_ref().is_none_or(|p| !point(p))
                    || b.size.as_ref().is_none_or(|s| {
                        !number(s.width) || !number(s.height) || s.width < 0.0 || s.height < 0.0
                    })
                    || !number(r.border_width)
                    || r.border_width < 0.0
                    || !number(r.radius)
                    || r.radius < 0.0
                {
                    return Err(invalid("invalid rectangle"));
                }
                paint_color(r.fill.as_ref())?;
                paint_color(r.border.as_ref())?;
            }
            Some(draw_command::Kind::Path(p)) => {
                steps += p.steps.len();
                if steps > 16384
                    || p.steps.is_empty()
                    || !number(p.stroke_width)
                    || p.stroke_width < 0.0
                    || p.steps[0].verb != PathVerb::Move as i32
                {
                    return Err(invalid("invalid drawing path"));
                }
                paint_color(p.color.as_ref())?;
                for s in &p.steps {
                    let verb =
                        PathVerb::try_from(s.verb).map_err(|_| invalid("unknown path verb"))?;
                    if verb == PathVerb::Unspecified
                        || (verb != PathVerb::Close && s.point.as_ref().is_none_or(|p| !point(p)))
                        || (matches!(verb, PathVerb::Quadratic | PathVerb::Cubic)
                            && s.control1.as_ref().is_none_or(|p| !point(p)))
                        || (verb == PathVerb::Cubic
                            && s.control2.as_ref().is_none_or(|p| !point(p)))
                    {
                        return Err(invalid("invalid path step"));
                    }
                }
            }
            None => return Err(invalid("missing drawing command")),
        }
    }
    Ok(())
}
impl Session {
    pub(crate) fn validate_references(&self, s: &ViewSnapshot) -> Result<()> {
        walk(s.root.as_ref().unwrap(), &mut |e| {
            if let Some(element::Kind::Image(image)) = &e.kind {
                let handle = image.resource.as_ref().unwrap();
                let resource = self
                    .resources
                    .get(&handle.id)
                    .filter(|r| r.handle.as_ref() == Some(handle))
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown resource"))?;
                if resource.kind != ResourceKind::Image as i32
                    && resource.kind != ResourceKind::Svg as i32
                {
                    return Err(invalid("resource is not an image"));
                }
            }
            Ok(())
        })
    }
    pub(crate) fn apply_command(
        &mut self,
        command: Command,
        backend: &mut impl Backend,
        events: &mut Vec<Event>,
    ) -> Result<CommandResult> {
        let kind = command
            .kind
            .as_ref()
            .ok_or_else(|| invalid("missing command kind"))?;
        match kind {
            command::Kind::SetMenus(menus) => {
                if menus.menus.len() > 16 {
                    return Err(invalid("too many menus"));
                }
                let mut count = 0;
                for menu in &menus.menus {
                    if menu.label.is_empty() || !bounded(&menu.label, 256) {
                        return Err(invalid("invalid menu label"));
                    }
                    self.validate_menu(&menu.items, 1, &mut count)?;
                }
                backend.command(None, &command)
            }
            command::Kind::PromptPaths(prompt) => {
                if prompt.handler_id == 0
                    || (!prompt.files && !prompt.directories)
                    || !bounded(&prompt.prompt, 4096)
                {
                    return Err(invalid("invalid path prompt"));
                }
                backend.command(None, &command)
            }
            command::Kind::PromptSavePath(prompt) => {
                if prompt.handler_id == 0
                    || prompt.directory.is_empty()
                    || !bounded(&prompt.directory, 4096)
                    || !bounded(&prompt.suggested_name, 1024)
                    || prompt.suggested_name.contains('/')
                    || prompt.suggested_name.contains('\\')
                {
                    return Err(invalid("invalid save prompt"));
                }
                backend.command(None, &command)
            }
            command::Kind::CreateModel(create) => {
                if create.value.len() > 65536 {
                    return Err(invalid("model value exceeds 64 KiB"));
                }
                if self.models.len() >= 1024 {
                    return Err(error(ErrorCode::Backpressure, "model limit reached"));
                }
                let state = ModelState {
                    entity: Some(self.allocate()),
                    revision: 1,
                    value: create.value.clone(),
                };
                backend
                    .model(&state, true)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                self.models.insert(
                    state.entity.as_ref().unwrap().id,
                    (state.clone(), HashSet::new()),
                );
                Ok(CommandResult {
                    model: Some(state),
                    ..Default::default()
                })
            }
            command::Kind::UpdateModel(update) => {
                if update.value.len() > 65536 {
                    return Err(invalid("model value exceeds 64 KiB"));
                }
                let h = update
                    .entity
                    .as_ref()
                    .ok_or_else(|| invalid("missing model handle"))?;
                let (old, observers) = self
                    .models
                    .get(&h.id)
                    .filter(|(s, _)| s.entity.as_ref() == Some(h))
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown model"))?;
                if old.revision != update.expected_revision {
                    return Err(error(ErrorCode::StaleRevision, "model revision conflict"));
                }
                let next = ModelState {
                    revision: old
                        .revision
                        .checked_add(1)
                        .ok_or_else(|| invalid("model revision exhausted"))?,
                    value: update.value.clone(),
                    ..old.clone()
                };
                backend
                    .model(&next, true)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                for id in observers {
                    events.push(Event {
                        handler_id: *id,
                        kind: Some(event::Kind::ModelChanged(next.clone())),
                        ..Default::default()
                    });
                }
                self.models.get_mut(&h.id).unwrap().0 = next.clone();
                Ok(CommandResult {
                    model: Some(next),
                    ..Default::default()
                })
            }
            command::Kind::ReadModel(target) | command::Kind::ReleaseModel(target) => {
                let h = target
                    .entity
                    .as_ref()
                    .ok_or_else(|| invalid("missing model handle"))?;
                let (state, _) = self
                    .models
                    .get(&h.id)
                    .filter(|(s, _)| s.entity.as_ref() == Some(h))
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown model"))?;
                if matches!(kind, command::Kind::ReleaseModel(_)) {
                    backend
                        .model(state, false)
                        .map_err(|e| error(ErrorCode::Internal, e))?;
                    self.models.remove(&h.id);
                    Ok(CommandResult::default())
                } else {
                    Ok(CommandResult {
                        model: Some(state.clone()),
                        ..Default::default()
                    })
                }
            }
            command::Kind::ObserveModel(observe) | command::Kind::UnobserveModel(observe) => {
                let h = observe
                    .entity
                    .as_ref()
                    .ok_or_else(|| invalid("missing model handle"))?;
                let (_, observers) = self
                    .models
                    .get_mut(&h.id)
                    .filter(|(s, _)| s.entity.as_ref() == Some(h))
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown model"))?;
                if observe.handler_id == 0 {
                    return Err(invalid("missing observer handler"));
                }
                if matches!(kind, command::Kind::ObserveModel(_)) {
                    if observers.len() >= 128 && !observers.contains(&observe.handler_id) {
                        return Err(error(ErrorCode::Backpressure, "observer limit reached"));
                    }
                    observers.insert(observe.handler_id);
                } else {
                    observers.remove(&observe.handler_id);
                }
                Ok(CommandResult::default())
            }
            command::Kind::UploadResource(upload) => {
                if !matches!(
                    ResourceKind::try_from(upload.kind),
                    Ok(ResourceKind::Image | ResourceKind::Svg | ResourceKind::Font)
                ) || upload.data.is_empty()
                    || upload.data.len() > MAX_RESOURCE_BYTES
                    || !bounded(&upload.media_type, 128)
                {
                    return Err(invalid("invalid resource"));
                }
                if self.resources.len() >= 128
                    || self.resources.values().map(|r| r.data.len()).sum::<usize>()
                        + upload.data.len()
                        > RESOURCE_BUDGET
                {
                    return Err(error(ErrorCode::Backpressure, "resource budget exceeded"));
                }
                let handle = self.allocate();
                let resource = Resource {
                    handle: Some(handle.clone()),
                    kind: upload.kind,
                    media_type: upload.media_type.clone(),
                    data: upload.data.clone(),
                };
                backend.resource(&resource, true).map_err(invalid)?;
                self.resources.insert(handle.id, resource);
                Ok(CommandResult {
                    resource: Some(handle),
                    ..Default::default()
                })
            }
            command::Kind::ReleaseResource(release) => {
                let handle = release
                    .resource
                    .as_ref()
                    .ok_or_else(|| invalid("missing resource handle"))?;
                let resource = self
                    .resources
                    .get(&handle.id)
                    .filter(|r| r.handle.as_ref() == Some(handle))
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown resource"))?;
                for view in self.views.values() {
                    walk(view.snapshot.root.as_ref().unwrap(), &mut |e| {
                        if matches!(&e.kind,Some(element::Kind::Image(i)) if i.resource.as_ref() == Some(handle))
                        {
                            return Err(invalid("resource is still referenced by a view"));
                        }
                        Ok(())
                    })?;
                }
                backend
                    .resource(resource, false)
                    .map_err(|e| error(ErrorCode::Internal, e))?;
                self.resources.remove(&handle.id);
                Ok(CommandResult::default())
            }
            command::Kind::Focus(target)
            | command::Kind::ScrollTo(ScrollTo {
                target: Some(target),
                ..
            })
            | command::Kind::DispatchAction(DispatchAction {
                target: Some(target),
                ..
            }) => {
                let view = self.view(
                    target
                        .view
                        .as_ref()
                        .ok_or_else(|| invalid("missing view"))?,
                )?;
                let element = find_element(view.snapshot.root.as_ref().unwrap(), target.element_id)
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown element"))?;
                if let command::Kind::DispatchAction(dispatch) = kind {
                    let action = dispatch
                        .action
                        .as_ref()
                        .ok_or_else(|| invalid("missing action"))?;
                    if !bounded(&action.name, 256) || action.payload.len() > 65536 {
                        return Err(invalid("action exceeds limits"));
                    }
                    let event = self
                        .interaction(
                            &view.entity,
                            view.snapshot.revision,
                            element.id,
                            event::Kind::Action(action.clone()),
                        )
                        .ok_or_else(|| invalid("action is not bound on the element"))?;
                    events.push(event);
                    return Ok(CommandResult::default());
                }
                if let command::Kind::ScrollTo(scroll) = kind {
                    let Some(element::Kind::UniformList(list)) = &element.kind else {
                        return Err(invalid("target is not a uniform list"));
                    };
                    if scroll.item as usize >= list.items.len() {
                        return Err(invalid("list item out of range"));
                    }
                } else if element.interaction.as_ref().is_none_or(|i| !i.focusable)
                    && !matches!(element.kind, Some(element::Kind::TextInput(_)))
                {
                    return Err(invalid("target is not focusable"));
                }
                backend.command(Some(view), &command)
            }
            command::Kind::ScrollTo(_) | command::Kind::DispatchAction(_) => {
                Err(invalid("missing command target"))
            }
            command::Kind::Window(w) => {
                let view = self
                    .views
                    .values()
                    .find(|v| Some(&v.window) == w.window.as_ref())
                    .ok_or_else(|| error(ErrorCode::StaleHandle, "unknown window"))?;
                if !matches!(WindowOperation::try_from(w.operation),Ok(v) if v != WindowOperation::Unspecified)
                    || !bounded(&w.title, 4096)
                {
                    return Err(invalid("invalid window operation"));
                }
                if w.operation == WindowOperation::Resize as i32
                    && w.size.as_ref().is_none_or(|s| {
                        !s.width.is_finite()
                            || !s.height.is_finite()
                            || !(1.0..=16384.0).contains(&s.width)
                            || !(1.0..=16384.0).contains(&s.height)
                    })
                {
                    return Err(invalid("invalid window size"));
                }
                let result = backend.command(Some(view), &command)?;
                // A backend without real windows takes the requested size at once;
                // native backends report the size their window actually gets.
                if w.operation == WindowOperation::Resize as i32 {
                    let view = view.clone();
                    if backend.window_state(&view).is_none() {
                        events.extend(self.window_resized(&view.window, w.size.clone().unwrap()));
                    }
                }
                Ok(result)
            }
            command::Kind::ClipboardWrite(w) => {
                if w.text.len() > MAX_RESOURCE_BYTES {
                    return Err(invalid("clipboard text too large"));
                }
                backend.command(None, &command)
            }
            command::Kind::ClipboardRead(_) => backend.command(None, &command),
            command::Kind::OpenUrl(url) => {
                if !bounded(&url.url, 8192)
                    || url.url.bytes().any(|b| b.is_ascii_control())
                    || !(url.url.starts_with("https://")
                        || url.url.starts_with("http://")
                        || url.url.starts_with("mailto:"))
                {
                    return Err(invalid("unsupported URL scheme or invalid URL"));
                }
                backend.command(None, &command)
            }
        }
    }
    fn validate_menu(&self, items: &[MenuItem], depth: usize, count: &mut usize) -> Result<()> {
        if depth > 8 {
            return Err(invalid("menu depth limit exceeded"));
        }
        for item in items {
            *count += 1;
            if *count > 256 || !bounded(&item.label, 256) {
                return Err(invalid("menu item limit or label exceeded"));
            }
            if item.separator {
                if !item.children.is_empty() || item.action.is_some() || item.target.is_some() {
                    return Err(invalid("separator cannot contain actions"));
                }
                continue;
            }
            if item.label.is_empty() {
                return Err(invalid("missing menu item label"));
            }
            if !item.children.is_empty() {
                if item.action.is_some() || item.target.is_some() {
                    return Err(invalid("submenu cannot contain an action"));
                }
                self.validate_menu(&item.children, depth + 1, count)?;
            } else {
                let target = item
                    .target
                    .as_ref()
                    .ok_or_else(|| invalid("missing menu target"))?;
                let view = self.view(
                    target
                        .view
                        .as_ref()
                        .ok_or_else(|| invalid("missing menu view"))?,
                )?;
                let action = item
                    .action
                    .as_ref()
                    .ok_or_else(|| invalid("missing menu action"))?;
                if action.payload.len() > 65536
                    || self
                        .interaction(
                            &view.entity,
                            view.snapshot.revision,
                            target.element_id,
                            event::Kind::Action(action.clone()),
                        )
                        .is_none()
                {
                    return Err(invalid("menu action is not bound"));
                }
            }
        }
        Ok(())
    }
    /// Menus refer to a logical action on the current node, not an old frame.
    pub fn native_action(&self, entity: &Handle, element: u64, action: Action) -> Option<Event> {
        let view = self.view(entity).ok()?;
        self.interaction(
            entity,
            view.snapshot.revision,
            element,
            event::Kind::Action(action),
        )
    }
    /// Route a native interaction against the current snapshot, never a stale callback.
    pub fn interaction(
        &self,
        entity: &Handle,
        revision: u64,
        element: u64,
        kind: event::Kind,
    ) -> Option<Event> {
        let view = self.view(entity).ok()?;
        if view.snapshot.revision != revision {
            return None;
        }
        let e = find_element(view.snapshot.root.as_ref()?, element)?;
        let handler = if let event::Kind::Action(a) = &kind {
            e.key_bindings
                .iter()
                .find(|b| b.action.as_ref().is_some_and(|bound| bound.name == a.name))?
                .handler_id
        } else {
            let i = e.interaction.as_ref()?;
            match &kind {
                event::Kind::KeyDown(_) => i.key_down,
                event::Kind::KeyUp(_) => i.key_up,
                event::Kind::MouseDown(_) => i.mouse_down,
                event::Kind::MouseUp(_) => i.mouse_up,
                event::Kind::MouseMove(_) => i.mouse_move,
                event::Kind::Scroll(_) => i.scroll,
                event::Kind::FocusChanged(_) => i.focus_changed,
                event::Kind::FileDrop(_) => i.file_drop,
                event::Kind::DataDrop(d) => {
                    if !i.accept_drop_kind.is_empty() && i.accept_drop_kind != d.kind {
                        return None;
                    }
                    i.data_drop
                }
                _ => return None,
            }
        };
        if handler == 0 {
            return None;
        }
        let mut event = Self::event(view, kind);
        event.handler_id = handler;
        event.element_id = element;
        Some(event)
    }
}
