//! GPUI values are created, updated, rendered and destroyed on this thread.
use gpui::{
    App, Bounds, Context, Entity, SharedString, Window, WindowBounds, WindowHandle, WindowOptions,
    div, prelude::*, px, rems, size,
};
#[cfg(feature = "automation")]
mod automation;
mod drawing;
pub mod embed;
mod input;
mod interaction;
mod platform;
mod resources;
use gpui::{AnimationExt, Focusable};
use gpui_go::{
    Hub, Message,
    gpui_go_core::{self as core, Backend, Session, View, pb},
};
use resources::{AssetBytes, Assets, ResourceCache};
use std::{cell::RefCell, rc::Rc};
use std::{
    collections::HashMap,
    panic::{AssertUnwindSafe, catch_unwind},
    sync::Arc,
};

struct GoView {
    view: View,
    hub: Arc<Hub>,
    inputs: HashMap<u64, Entity<input::TextInput>>,
    focus: HashMap<u64, (gpui::FocusHandle, Vec<gpui::Subscription>)>,
    lists: HashMap<u64, gpui::UniformListScrollHandle>,
    resources: ResourceCache,
    #[cfg(feature = "automation")]
    rendered: automation::RenderedBounds,
}

impl Render for GoView {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        self.sync_inputs(window, cx);
        #[cfg(feature = "automation")]
        self.rendered.borrow_mut().clear();
        render_element(
            self.view.snapshot.root.as_ref().expect("validated root"),
            &RenderEnv {
                view: &self.view,
                hub: &self.hub,
                inputs: &self.inputs,
                focus: &self.focus,
                lists: &self.lists,
                resources: &self.resources,
                text_color: gpui::black(),
                #[cfg(feature = "automation")]
                rendered: &self.rendered,
                cx,
            },
        )
    }
}

impl GoView {
    fn sync_inputs(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        fn collect(root: &pb::Element, inputs: &mut HashMap<u64, pb::TextInput>) {
            if let Some(pb::element::Kind::TextInput(props)) = &root.kind {
                inputs.insert(root.id, props.clone());
            }
            for child in core::features::children(root) {
                collect(child, inputs);
            }
        }
        let mut props = HashMap::new();
        if let Some(root) = &self.view.snapshot.root {
            collect(root, &mut props);
        }
        self.inputs.retain(|id, _| props.contains_key(id));
        for (id, props) in props {
            if let Some(entity) = self.inputs.get(&id) {
                entity.update(cx, |input, cx| input.sync(&props, window, cx));
            } else {
                let owner = self.view.entity.clone();
                let hub = self.hub.clone();
                self.inputs.insert(
                    id,
                    cx.new(|cx| input::TextInput::new(&props, owner, id, hub, cx)),
                );
            }
        }
        let root = self.view.snapshot.root.clone().unwrap();
        let mut wanted = Vec::new();
        fn ids(root: &pb::Element, wanted: &mut Vec<(u64, bool)>) {
            if root.interaction.as_ref().is_some_and(|i| i.focusable)
                || matches!(root.kind, Some(pb::element::Kind::TextInput(_)))
            {
                wanted.push((root.id, false));
            }
            if matches!(root.kind, Some(pb::element::Kind::UniformList(_))) {
                wanted.push((root.id, true));
            }
            for child in core::features::children(root) {
                ids(child, wanted);
            }
        }
        ids(&root, &mut wanted);
        self.focus.retain(|id, _| wanted.contains(&(*id, false)));
        self.lists.retain(|id, _| wanted.contains(&(*id, true)));
        for (id, list) in wanted {
            if list {
                self.lists.entry(id).or_default();
                continue;
            }
            if let Some((old, _)) = self.focus.get(&id)
                && self
                    .inputs
                    .get(&id)
                    .is_none_or(|input| input.focus_handle(cx) == *old)
            {
                continue;
            }
            let handle = self
                .inputs
                .get(&id)
                .map(|input| input.focus_handle(cx))
                .unwrap_or_else(|| cx.focus_handle());
            let on = cx.on_focus(&handle, window, move |this, _, cx| {
                interaction::emit(
                    &this.hub,
                    &this.view,
                    id,
                    pb::event::Kind::FocusChanged(pb::FocusChanged { focused: true }),
                    cx,
                )
            });
            let off = cx.on_blur(&handle, window, move |this, _, cx| {
                interaction::emit(
                    &this.hub,
                    &this.view,
                    id,
                    pb::event::Kind::FocusChanged(pb::FocusChanged { focused: false }),
                    cx,
                )
            });
            self.focus.insert(id, (handle, vec![on, off]));
        }
    }
}

#[derive(Clone, Copy)]
struct RenderEnv<'a> {
    view: &'a View,
    hub: &'a Arc<Hub>,
    inputs: &'a HashMap<u64, Entity<input::TextInput>>,
    focus: &'a HashMap<u64, (gpui::FocusHandle, Vec<gpui::Subscription>)>,
    lists: &'a HashMap<u64, gpui::UniformListScrollHandle>,
    resources: &'a ResourceCache,
    // GPUI's SVG element paints only with its own color; it does not inherit one.
    text_color: gpui::Hsla,
    #[cfg(feature = "automation")]
    rendered: &'a automation::RenderedBounds,
    cx: &'a App,
}
fn render_element(element: &pb::Element, env: &RenderEnv<'_>) -> gpui::AnyElement {
    let RenderEnv {
        view,
        hub,
        inputs,
        focus,
        lists,
        resources,
        cx,
        ..
    } = *env;
    let text_color = element
        .style
        .as_ref()
        .and_then(|style| style.text_color.as_ref())
        .map_or(env.text_color, color);
    let env = &RenderEnv { text_color, ..*env };
    // Include the entity ID so identical element IDs in different views cannot
    // alias GPUI state. Numeric IDs themselves remain stable across snapshots.
    let mut node = div().id(SharedString::from(format!(
        "view-{}-element-{}",
        view.entity.id, element.id
    )));
    if view
        .snapshot
        .root
        .as_ref()
        .is_some_and(|root| root.id == element.id)
    {
        node = node
            .tab_group()
            .on_action(|_: &input::FocusNext, window, cx| window.focus_next(cx))
            .on_action(|_: &input::FocusPrevious, window, cx| window.focus_prev(cx));
    }
    if view
        .snapshot
        .root
        .as_ref()
        .is_some_and(|root| root.id == element.id)
    {
        node.style().text.font_family = Some(SharedString::from("Helvetica"));
    }
    if let Some(style) = &element.style {
        apply_style(&mut node, style);
    }
    if let Some(hover) = &element.hover_style {
        node = node.hover(|mut style| {
            apply_style(&mut style, hover);
            style
        });
    }
    match &element.kind {
        Some(pb::element::Kind::Container(container)) => {
            for child in &container.children {
                node = node.child(render_element(child, env));
            }
        }
        Some(pb::element::Kind::Text(text)) => {
            node = node.child(SharedString::from(text.content.clone()));
        }
        Some(pb::element::Kind::TextInput(_)) => {
            node = node.child(
                inputs
                    .get(&element.id)
                    .expect("input entity initialized")
                    .clone(),
            );
        }
        Some(pb::element::Kind::Image(image)) => {
            let asset = resources
                .borrow()
                .items
                .get(&image.resource.as_ref().unwrap().id)
                .cloned()
                .expect("validated resource");
            node = match asset {
                resources::Asset::Raster(image) => node.child(gpui::img(image).size_full()),
                resources::Asset::Svg(path) => {
                    node.child(gpui::svg().path(path).size_full().text_color(text_color))
                }
                _ => unreachable!(),
            };
        }
        Some(pb::element::Kind::Embedded(embedded)) => {
            if let Some(view) = embed::view(embedded.instance, cx) {
                node = node.child(view);
            }
        }
        Some(pb::element::Kind::Drawing(drawing)) => {
            node = node.child(drawing::canvas_element(drawing));
        }
        Some(pb::element::Kind::UniformList(list)) => {
            let list = list.clone();
            let view = view.clone();
            let hub = hub.clone();
            let inputs = inputs.clone();
            let focus: HashMap<_, _> = focus
                .iter()
                .map(|(id, (handle, _))| (*id, (handle.clone(), Vec::new())))
                .collect();
            let handles = lists.clone();
            let resources = resources.clone();
            #[cfg(feature = "automation")]
            let rendered = env.rendered.clone();
            node = node.child(
                gpui::uniform_list(
                    format!("list-{}-{}", view.entity.id, element.id),
                    list.items.len(),
                    move |range, _, cx| {
                        range
                            .map(|index| {
                                div()
                                    .w_full()
                                    .h(px(list.row_height))
                                    .overflow_hidden()
                                    .child(render_element(
                                        &list.items[index],
                                        &RenderEnv {
                                            view: &view,
                                            hub: &hub,
                                            inputs: &inputs,
                                            focus: &focus,
                                            lists: &handles,
                                            resources: &resources,
                                            text_color,
                                            #[cfg(feature = "automation")]
                                            rendered: &rendered,
                                            cx,
                                        },
                                    ))
                            })
                            .collect()
                    },
                )
                .track_scroll(lists.get(&element.id).unwrap())
                .size_full(),
            );
        }
        _ => unreachable!("validated element kind"),
    }
    node = interaction::attach(
        node,
        element,
        view,
        hub,
        focus.get(&element.id).map(|(f, _)| f),
        cx,
    );
    if let Some(binding) = &element.on_click {
        let entity = view.entity.clone();
        let revision = view.snapshot.revision;
        let element = element.id;
        let stop = binding.stop_propagation;
        let hub = hub.clone();
        node = node.on_click(move |event, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            let point = event.position();
            let modifiers = event.modifiers();
            let click = pb::Click {
                position: Some(pb::Point {
                    x: f32::from(point.x),
                    y: f32::from(point.y),
                }),
                button: pb::MouseButton::Left as i32,
                modifiers: Some(pb::Modifiers {
                    control: modifiers.control,
                    alt: modifiers.alt,
                    shift: modifiers.shift,
                    platform: modifiers.platform,
                }),
                click_count: event.click_count() as u32,
            };
            if let Err(failure) = hub.input(Message::Click {
                entity: entity.clone(),
                revision,
                element,
                click,
            }) {
                hub.finish(failure);
                cx.quit();
            }
        });
    }
    let rendered_node = if let Some(animation) = &element.animation {
        let from = animation.from;
        let to = animation.to;
        let mut animation_config = gpui::Animation::new(std::time::Duration::from_millis(
            animation.duration_ms as u64,
        ));
        if animation.repeat {
            animation_config = animation_config.repeat();
        }
        node.with_animation(
            SharedString::from(format!(
                "opacity-{}-{}-{}",
                view.entity.id, element.id, animation.generation
            )),
            animation_config,
            move |node, delta| node.opacity(from + (to - from) * delta),
        )
        .into_any_element()
    } else {
        node.into_any_element()
    };
    #[cfg(feature = "automation")]
    let rendered_node = automation::observe(rendered_node, element.id, env.rendered.clone());
    rendered_node
}

fn definite(length: &pb::Length) -> gpui::DefiniteLength {
    match length.value.as_ref().expect("validated length") {
        pb::length::Value::Pixels(n) => px(*n).into(),
        pb::length::Value::Rems(n) => rems(*n).into(),
        pb::length::Value::Percent(n) => gpui::DefiniteLength::Fraction(n / 100.),
        pb::length::Value::Auto(_) => unreachable!("validated definite length"),
    }
}
fn length(value: &pb::Length) -> gpui::Length {
    if matches!(value.value, Some(pb::length::Value::Auto(true))) {
        gpui::Length::Auto
    } else {
        gpui::Length::Definite(definite(value))
    }
}
fn color(value: &pb::Color) -> gpui::Hsla {
    gpui::Rgba {
        r: value.red,
        g: value.green,
        b: value.blue,
        a: value.alpha,
    }
    .into()
}

fn apply_style(node: &mut impl gpui::Styled, value: &pb::Style) {
    let style = node.style();
    if let Some(v) = &value.max_width {
        style.max_size.width = Some(length(v));
    }
    if let Some(v) = &value.max_height {
        style.max_size.height = Some(length(v));
    }
    if let Some(v) = value.display {
        style.display = Some(match pb::Display::try_from(v).unwrap() {
            pb::Display::Flex => gpui::Display::Flex,
            pb::Display::Grid => gpui::Display::Grid,
            _ => gpui::Display::None,
        });
    }
    if let Some(v) = value.grid_columns {
        style.grid_cols = Some(gpui::GridTemplate {
            repeat: v as u16,
            min_size: gpui::GridTemplateMinSize::Zero,
        });
    }
    if let Some(v) = value.flex_wrap {
        style.flex_wrap = Some(if v {
            gpui::FlexWrap::Wrap
        } else {
            gpui::FlexWrap::NoWrap
        });
    }
    if let Some(v) = value.opacity {
        style.opacity = Some(v);
    }
    if let Some(v) = &value.font_family {
        style.text.font_family = Some(SharedString::from(v.clone()));
    }
    if let Some(v) = value.text_align {
        style.text.text_align = Some(match pb::TextAlignment::try_from(v).unwrap() {
            pb::TextAlignment::Center => gpui::TextAlign::Center,
            pb::TextAlignment::Right => gpui::TextAlign::Right,
            _ => gpui::TextAlign::Left,
        });
    }
    if let Some(v) = value.italic {
        style.text.font_style = Some(if v {
            gpui::FontStyle::Italic
        } else {
            gpui::FontStyle::Normal
        });
    }
    if !value.shadows.is_empty() {
        style.box_shadow = Some(
            value
                .shadows
                .iter()
                .map(|s| gpui::BoxShadow {
                    color: s
                        .color
                        .as_ref()
                        .map(color)
                        .unwrap_or_else(gpui::transparent_black),
                    offset: gpui::point(
                        px(s.offset.as_ref().unwrap().x),
                        px(s.offset.as_ref().unwrap().y),
                    ),
                    blur_radius: px(s.blur),
                    spread_radius: px(s.spread),
                    inset: false,
                })
                .collect(),
        );
    }
    if let Some(v) = &value.width {
        style.size.width = Some(length(v));
    }
    if let Some(v) = &value.height {
        style.size.height = Some(length(v));
    }
    if let Some(v) = &value.min_width {
        style.min_size.width = Some(length(v));
    }
    if let Some(v) = &value.min_height {
        style.min_size.height = Some(length(v));
    }
    if let Some(v) = &value.padding {
        if let Some(v) = &v.top {
            style.padding.top = Some(definite(v));
        }
        if let Some(v) = &v.right {
            style.padding.right = Some(definite(v));
        }
        if let Some(v) = &v.bottom {
            style.padding.bottom = Some(definite(v));
        }
        if let Some(v) = &v.left {
            style.padding.left = Some(definite(v));
        }
    }
    if let Some(v) = &value.margin {
        if let Some(v) = &v.top {
            style.margin.top = Some(length(v));
        }
        if let Some(v) = &v.right {
            style.margin.right = Some(length(v));
        }
        if let Some(v) = &v.bottom {
            style.margin.bottom = Some(length(v));
        }
        if let Some(v) = &v.left {
            style.margin.left = Some(length(v));
        }
    }
    if let Some(v) = &value.gap {
        style.gap.width = Some(definite(v));
        style.gap.height = Some(definite(v));
    }
    if let Some(v) = value.flex_direction {
        style.display = Some(gpui::Display::Flex);
        style.flex_direction = Some(match pb::FlexDirection::try_from(v).unwrap() {
            pb::FlexDirection::Row => gpui::FlexDirection::Row,
            pb::FlexDirection::Column => gpui::FlexDirection::Column,
            _ => unreachable!(),
        });
    }
    if let Some(v) = value.align_items {
        style.align_items = Some(match pb::Alignment::try_from(v).unwrap() {
            pb::Alignment::Start => gpui::AlignItems::FlexStart,
            pb::Alignment::Center => gpui::AlignItems::Center,
            pb::Alignment::End => gpui::AlignItems::FlexEnd,
            pb::Alignment::Stretch => gpui::AlignItems::Stretch,
            _ => unreachable!(),
        });
    }
    if let Some(v) = value.justify_content {
        style.justify_content = Some(match pb::Justification::try_from(v).unwrap() {
            pb::Justification::Start => gpui::JustifyContent::FlexStart,
            pb::Justification::Center => gpui::JustifyContent::Center,
            pb::Justification::End => gpui::JustifyContent::FlexEnd,
            pb::Justification::SpaceBetween => gpui::JustifyContent::SpaceBetween,
            _ => unreachable!(),
        });
    }
    if let Some(v) = &value.background {
        style.background = Some(color(v).into());
    }
    if let Some(v) = &value.text_color {
        style.text.color = Some(color(v));
    }
    if let Some(v) = value.font_size {
        style.text.font_size = Some(px(v).into());
    }
    if let Some(v) = value.border_width {
        let v = Some(px(v).into());
        style.border_widths.top = v;
        style.border_widths.right = v;
        style.border_widths.bottom = v;
        style.border_widths.left = v;
    }
    if let Some(v) = &value.border_color {
        style.border_color = Some(color(v));
    }
    if let Some(v) = value.corner_radius {
        let v = Some(px(v).into());
        style.corner_radii.top_left = v;
        style.corner_radii.top_right = v;
        style.corner_radii.bottom_left = v;
        style.corner_radii.bottom_right = v;
    }
    if let Some(v) = value.flex_grow {
        style.flex_grow = Some(v);
    }
    if let Some(v) = value.flex_shrink {
        style.flex_shrink = Some(v);
    }
    let overflow = |v| match pb::Overflow::try_from(v).unwrap() {
        pb::Overflow::Visible => gpui::Overflow::Visible,
        pb::Overflow::Hidden => gpui::Overflow::Hidden,
        pb::Overflow::Scroll => gpui::Overflow::Scroll,
        _ => unreachable!("validated overflow"),
    };
    if let Some(v) = value.overflow_x {
        style.overflow.x = Some(overflow(v));
    }
    if let Some(v) = value.overflow_y {
        style.overflow.y = Some(overflow(v));
    }
    if let Some(v) = value.cursor {
        style.mouse_cursor = Some(match pb::Cursor::try_from(v).unwrap() {
            pb::Cursor::Arrow => gpui::CursorStyle::Arrow,
            pb::Cursor::Pointer => gpui::CursorStyle::PointingHand,
            _ => unreachable!("validated cursor"),
        });
    }
    if let Some(v) = value.font_weight {
        style.text.font_weight = Some(gpui::FontWeight(v as f32));
    }
    if let Some(v) = value.position {
        style.position = Some(match pb::Position::try_from(v).unwrap() {
            pb::Position::Relative => gpui::Position::Relative,
            pb::Position::Absolute => gpui::Position::Absolute,
            _ => unreachable!("validated position"),
        });
    }
    if let Some(v) = &value.inset {
        if let Some(v) = &v.top {
            style.inset.top = Some(length(v));
        }
        if let Some(v) = &v.right {
            style.inset.right = Some(length(v));
        }
        if let Some(v) = &v.bottom {
            style.inset.bottom = Some(length(v));
        }
        if let Some(v) = &v.left {
            style.inset.left = Some(length(v));
        }
    }
}

struct Windows {
    views: HashMap<u64, (pb::Handle, WindowHandle<GoView>)>,
    resources: ResourceCache,
    models: HashMap<u64, Entity<pb::ModelState>>,
    pending_prompts: Rc<std::cell::Cell<usize>>,
    base_keys: Vec<gpui::KeyBinding>,
    installed_keys: Vec<interaction::RegisteredKey>,
}
impl Windows {
    fn refresh_keys(&mut self, cx: &mut App) {
        let mut views: Vec<_> = self.views.iter().collect();
        views.sort_by_key(|(id, _)| **id);
        let next: Vec<_> = views
            .into_iter()
            .filter_map(|(_, (_, window))| window.read(cx).ok())
            .flat_map(|view| interaction::key_entries(&view.view))
            .collect();
        if self.installed_keys == next {
            return;
        }
        let bindings: Vec<_> = next
            .iter()
            .map(|entry| entry.load(cx).expect("prevalidated binding"))
            .collect();
        cx.clear_key_bindings();
        cx.bind_keys(self.base_keys.iter().cloned().chain(bindings));
        self.installed_keys = next;
    }
}
struct NativeBackend<'a> {
    windows: &'a mut Windows,
    cx: &'a mut App,
    hub: Arc<Hub>,
}

impl Backend for NativeBackend<'_> {
    fn open(&mut self, view: &View) -> Result<(), String> {
        interaction::validate(view.snapshot.root.as_ref().unwrap(), self.cx)?;
        embed::validate(view.snapshot.root.as_ref().unwrap(), None, self.cx)?;
        let resources = self.windows.resources.clone();
        let dimensions = view.options.size.as_ref().unwrap();
        let bounds = Bounds::centered(
            None,
            size(px(dimensions.width), px(dimensions.height)),
            self.cx,
        );
        let initial = view.clone();
        let hub = self.hub.clone();
        let window_id = view.window.clone();
        let window = self
            .cx
            .open_window(
                WindowOptions {
                    window_bounds: Some(WindowBounds::Windowed(bounds)),
                    ..Default::default()
                },
                move |window, cx| {
                    window.set_window_title(&initial.options.title);
                    let closed = hub.clone();
                    window.on_window_should_close(cx, move |_, cx| {
                        if let Err(failure) = closed.input(Message::WindowClosed(window_id.clone()))
                        {
                            closed.finish(failure);
                            cx.quit();
                        }
                        true
                    });
                    cx.new(|cx| {
                        let mut view = GoView {
                            view: initial,
                            hub,
                            inputs: HashMap::new(),
                            focus: HashMap::new(),
                            lists: HashMap::new(),
                            resources,
                            #[cfg(feature = "automation")]
                            rendered: Default::default(),
                        };
                        view.sync_inputs(window, cx);
                        view
                    })
                },
            )
            .map_err(|e| e.to_string())?;
        embed::bind_window(&view.entity, window.into(), self.cx);
        self.windows
            .views
            .insert(view.entity.id, (view.window.clone(), window));
        self.windows.refresh_keys(self.cx);
        self.cx.activate(true);
        Ok(())
    }

    fn commit(&mut self, view: &View) -> Result<(), String> {
        interaction::validate(view.snapshot.root.as_ref().unwrap(), self.cx)?;
        let (_, window) = self
            .windows
            .views
            .get(&view.entity.id)
            .ok_or("unknown native view")?;
        embed::validate(
            view.snapshot.root.as_ref().unwrap(),
            Some((*window).into()),
            self.cx,
        )?;
        window
            .update(self.cx, |entity, window, cx| {
                entity.view = view.clone();
                entity.sync_inputs(window, cx);
                cx.notify();
            })
            .map_err(|e| e.to_string())?;
        self.windows.refresh_keys(self.cx);
        Ok(())
    }

    fn close(&mut self, view: &View) -> Result<(), String> {
        let (_, window) = self
            .windows
            .views
            .get(&view.entity.id)
            .ok_or("unknown native view")?;
        window
            .update(self.cx, |_, window, _| window.remove_window())
            .map_err(|e| e.to_string())?;
        self.windows.views.remove(&view.entity.id);
        embed::forget_window(view.entity.id, self.cx);
        self.windows.refresh_keys(self.cx);
        Ok(())
    }

    fn model(&mut self, state: &pb::ModelState, live: bool) -> Result<(), String> {
        let id = state.entity.as_ref().unwrap().id;
        if !live {
            self.windows.models.remove(&id);
        } else if let Some(model) = self.windows.models.get(&id) {
            model.update(self.cx, |model, cx| {
                *model = state.clone();
                cx.notify();
            });
        } else {
            self.windows
                .models
                .insert(id, self.cx.new(|_| state.clone()));
        }
        Ok(())
    }
    fn features(&self) -> Vec<String> {
        let mut features: Vec<String> = vec![
            "platform-services".into(),
            "menus".into(),
            "path-prompts".into(),
            #[cfg(feature = "automation")]
            "test-automation-v1".into(),
            #[cfg(feature = "automation")]
            "test-locators-v1".into(),
        ];
        if embed::supported(self.cx) {
            features.push("embedded-views".into());
        }
        features
    }
    #[cfg(feature = "automation")]
    fn automate(
        &mut self,
        view: &View,
        request: &pb::AutomationRequest,
    ) -> Result<pb::AutomationResult, pb::ProtocolError> {
        automation::execute(self.windows, self.cx, &self.hub, view, request)
    }
    fn resource(&mut self, resource: &pb::Resource, load: bool) -> Result<(), String> {
        self.windows
            .resources
            .borrow_mut()
            .update(resource, load, self.cx)
    }
    fn command(
        &mut self,
        view: Option<&View>,
        command: &pb::Command,
    ) -> Result<pb::CommandResult, pb::ProtocolError> {
        use pb::command::Kind;
        let fail = |s: String| core::error(pb::ErrorCode::Internal, s);
        if platform::handles(command) {
            return platform::command(command, self.windows, self.cx, self.hub.clone());
        }
        match command.kind.as_ref().unwrap() {
            Kind::ClipboardRead(_) => {
                let text = self
                    .cx
                    .read_from_clipboard()
                    .and_then(|item| item.text())
                    .unwrap_or_default();
                if text.len() > core::features::MAX_RESOURCE_BYTES {
                    return Err(core::error(
                        pb::ErrorCode::InvalidArgument,
                        "clipboard text exceeds limit",
                    ));
                }
                return Ok(pb::CommandResult {
                    text: Some(text),
                    ..Default::default()
                });
            }
            Kind::ClipboardWrite(value) => {
                self.cx
                    .write_to_clipboard(gpui::ClipboardItem::new_string(value.text.clone()));
            }
            Kind::OpenUrl(value) => self.cx.open_url(&value.url),
            kind => {
                let view = view.unwrap();
                let (_, handle) = self
                    .windows
                    .views
                    .get(&view.entity.id)
                    .ok_or_else(|| fail("unknown native view".into()))?;
                handle
                    .update(self.cx, |entity, window, cx| match kind {
                        Kind::Focus(target) => {
                            let focus = &entity.focus.get(&target.element_id).unwrap().0;
                            window.focus(focus, cx);
                        }
                        Kind::ScrollTo(scroll) => {
                            entity
                                .lists
                                .get(&scroll.target.as_ref().unwrap().element_id)
                                .unwrap()
                                .scroll_to_item(scroll.item as usize, gpui::ScrollStrategy::Top);
                            cx.notify();
                        }
                        Kind::Window(command) => {
                            match pb::WindowOperation::try_from(command.operation).unwrap() {
                                pb::WindowOperation::Activate => window.activate_window(),
                                pb::WindowOperation::Minimize => window.minimize_window(),
                                pb::WindowOperation::Zoom => window.zoom_window(),
                                pb::WindowOperation::Fullscreen => window.toggle_fullscreen(),
                                pb::WindowOperation::SetTitle => {
                                    window.set_window_title(&command.title)
                                }
                                pb::WindowOperation::Resize => {
                                    let s = command.size.as_ref().unwrap();
                                    window.resize(size(px(s.width), px(s.height)));
                                }
                                _ => unreachable!(),
                            }
                        }
                        _ => unreachable!(),
                    })
                    .map_err(|e| fail(e.to_string()))?;
            }
        }
        Ok(pb::CommandResult::default())
    }
    fn quit(&mut self) {
        self.cx.quit();
    }
}

#[cfg(target_os = "macos")]
fn is_main_thread() -> bool {
    unsafe extern "C" {
        fn pthread_main_np() -> i32;
    }
    unsafe { pthread_main_np() != 0 }
}
#[cfg(not(target_os = "macos"))]
fn is_main_thread() -> bool {
    true
}

#[unsafe(no_mangle)]
pub extern "C" fn gpui_go_run_native() -> i32 {
    // This entry point intentionally uses ordinary cgo, never asmcgocall.
    catch_unwind(AssertUnwindSafe(|| {
        if !is_main_thread() {
            return -3;
        }
        let Some(hub) = gpui_go::hub() else {
            return -1;
        };
        if !hub.begin() {
            return -1;
        }
        let run_hub = hub.clone();
        let asset_bytes = AssetBytes::default();
        let resource_cache = Rc::new(RefCell::new(Assets::new(asset_bytes.clone())));
        // The Message::WindowClosed path below owns quitting. GPUI's default
        // quits on the last window close outside macOS, racing the close reply.
        gpui_platform::application()
            .with_quit_mode(gpui::QuitMode::Explicit)
            .with_assets(asset_bytes)
            .run(move |cx| {
                embed::run_setup(cx);
                input::bind_keys(cx);
                platform::init(cx, run_hub.clone());
                let hub = run_hub;
                let mut session = Session::new(hub.session_id);
                let mut windows = Windows {
                    views: HashMap::new(),
                    resources: resource_cache,
                    models: HashMap::new(),
                    pending_prompts: Rc::new(std::cell::Cell::new(0)),
                    base_keys: cx.key_bindings().borrow().bindings().cloned().collect(),
                    installed_keys: Vec::new(),
                };
                cx.spawn(async move |cx| {
                    while let Ok(message) = hub.receiver.recv().await {
                        let native_closed = if let Message::WindowClosed(handle) = &message {
                            Some(handle.clone())
                        } else {
                            None
                        };
                        let keep_running = cx.update(|cx| {
                            let mut backend = NativeBackend {
                                windows: &mut windows,
                                cx,
                                hub: hub.clone(),
                            };
                            let keep_running = hub.dispatch(message, &mut session, &mut backend);
                            // Remove native references after an OS window close.
                            if let Some(handle) = &native_closed {
                                let app = &mut *backend.cx;
                                backend.windows.views.retain(|id, (window, _)| {
                                    let keep = window != handle;
                                    if !keep {
                                        embed::forget_window(*id, app);
                                    }
                                    keep
                                });
                                backend.windows.refresh_keys(backend.cx);
                                if backend.windows.views.is_empty() {
                                    backend.quit();
                                    return false;
                                }
                            }
                            keep_running
                        });
                        if !keep_running {
                            break;
                        }
                    }
                    cx.update(|cx| cx.quit());
                })
                .detach();
            });
        hub.finish(core::error(pb::ErrorCode::Closed, "application exited"));
        0
    }))
    .unwrap_or_else(|_| {
        if let Some(hub) = gpui_go::hub() {
            hub.finish(core::error(
                pb::ErrorCode::Internal,
                "GPUI runtime panicked",
            ));
        }
        -2
    })
}

#[cfg(test)]
mod tests;
