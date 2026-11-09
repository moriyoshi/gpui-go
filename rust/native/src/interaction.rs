//! Native dispatch policies run synchronously; Go receives immutable events.
use crate::*;
use gpui::{Div, InteractiveElement, Stateful};

pub(crate) fn modifiers(m: gpui::Modifiers) -> pb::Modifiers {
    pb::Modifiers {
        control: m.control,
        alt: m.alt,
        shift: m.shift,
        platform: m.platform,
    }
}
fn button(b: gpui::MouseButton) -> i32 {
    match b {
        gpui::MouseButton::Left => 1,
        gpui::MouseButton::Right => 2,
        gpui::MouseButton::Middle => 3,
        _ => 0,
    }
}
fn pointer(
    position: gpui::Point<gpui::Pixels>,
    b: Option<gpui::MouseButton>,
    m: gpui::Modifiers,
) -> pb::PointerEvent {
    pb::PointerEvent {
        position: Some(pb::Point {
            x: f32::from(position.x),
            y: f32::from(position.y),
        }),
        button: b.map(button).unwrap_or(0),
        modifiers: Some(modifiers(m)),
        ..Default::default()
    }
}
pub(crate) fn emit(hub: &Arc<Hub>, view: &View, id: u64, kind: pb::event::Kind, cx: &mut App) {
    if let Err(failure) = hub.input(Message::Interaction {
        entity: view.entity.clone(),
        revision: view.snapshot.revision,
        element: id,
        kind,
    }) {
        hub.finish(failure);
        cx.quit();
    }
}
pub(crate) fn validate(root: &pb::Element, cx: &App) -> Result<(), String> {
    if let Some(i) = &root.interaction
        && !i.key_context.is_empty()
    {
        gpui::KeyContext::try_from(i.key_context.as_str()).map_err(|e| e.to_string())?;
    }
    for binding in &root.key_bindings {
        if !binding.keystroke.is_empty() {
            RegisteredKey {
                target: pb::FocusTarget {
                    view: Some(pb::Handle {
                        id: 1,
                        generation: 1,
                    }),
                    element_id: root.id,
                },
                binding: binding.clone(),
            }
            .load(cx)?;
        }
    }
    for child in core::features::children(root) {
        validate(child, cx)?;
    }
    Ok(())
}
pub(crate) fn attach(
    mut node: Stateful<Div>,
    element: &pb::Element,
    view: &View,
    hub: &Arc<Hub>,
    focus: Option<&gpui::FocusHandle>,
    _cx: &App,
) -> Stateful<Div> {
    if let Some(label) = &element.accessibility_label {
        node = node
            .role(gpui::Role::GenericContainer)
            .aria_label(label.clone());
    }
    if let Some(a) = &element.accessibility {
        let role = match pb::AccessibleRole::try_from(a.role).unwrap() {
            pb::AccessibleRole::Button => gpui::Role::Button,
            pb::AccessibleRole::Checkbox => gpui::Role::CheckBox,
            pb::AccessibleRole::TextInput => gpui::Role::TextInput,
            pb::AccessibleRole::Text => gpui::Role::Label,
            pb::AccessibleRole::List => gpui::Role::List,
            pb::AccessibleRole::ListItem => gpui::Role::ListItem,
            pb::AccessibleRole::Link => gpui::Role::Link,
            pb::AccessibleRole::Image => gpui::Role::Image,
            pb::AccessibleRole::Slider => gpui::Role::Slider,
            pb::AccessibleRole::Dialog => gpui::Role::Dialog,
            _ => gpui::Role::GenericContainer,
        };
        node = node.role(role).aria_description(a.description.clone());
        if let Some(v) = a.selected {
            node = node.aria_selected(v);
        }
        if let Some(v) = a.expanded {
            node = node.aria_expanded(v);
        }
        if let Some(v) = a.checked {
            node = node.aria_toggled(if v {
                gpui::Toggled::True
            } else {
                gpui::Toggled::False
            });
        }
        if let Some(v) = &a.value {
            node = node.aria_value(v.clone());
        }
    }
    let Some(i) = &element.interaction else {
        return node;
    };
    node = node
        .key_context(format!("{} {}", scope(view.entity.id, element.id), i.key_context).as_str());
    if let Some(focus) = focus {
        // track_focus uses the handle's navigation flags; setting tab_index on
        // the Div alone does not configure an explicitly tracked handle.
        let focus = focus
            .clone()
            .tab_index(i.tab_index as isize)
            .tab_stop(i.tab_index >= 0);
        if !matches!(element.kind, Some(pb::element::Kind::TextInput(_))) {
            node = node.track_focus(&focus);
        }
        // GPUI's tracked-focus listener handles clicks and prevents ancestors
        // from stealing a descendant's focus. Do not add a second listener.
        let f = focus.clone();
        node = node.on_a11y_action(gpui::AccessibleAction::Focus, move |_, window, cx| {
            window.focus(&f, cx)
        });
    }
    let id = element.id;
    let stop = i.stop_propagation;
    if i.key_down != 0 {
        let view = view.clone();
        let hub = hub.clone();
        node = node.on_key_down(move |event, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            emit(
                &hub,
                &view,
                id,
                pb::event::Kind::KeyDown(pb::KeyEvent {
                    key: event.keystroke.key.clone(),
                    modifiers: Some(modifiers(event.keystroke.modifiers)),
                    held: event.is_held,
                }),
                cx,
            );
        });
    }
    if i.key_up != 0 {
        let view = view.clone();
        let hub = hub.clone();
        node = node.on_key_up(move |event, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            emit(
                &hub,
                &view,
                id,
                pb::event::Kind::KeyUp(pb::KeyEvent {
                    key: event.keystroke.key.clone(),
                    modifiers: Some(modifiers(event.keystroke.modifiers)),
                    held: false,
                }),
                cx,
            );
        });
    }
    for b in [
        gpui::MouseButton::Left,
        gpui::MouseButton::Right,
        gpui::MouseButton::Middle,
    ] {
        if i.mouse_down != 0 {
            let view = view.clone();
            let hub = hub.clone();
            node = node.on_mouse_down(b, move |event, _, cx| {
                if stop {
                    cx.stop_propagation();
                }
                emit(
                    &hub,
                    &view,
                    id,
                    pb::event::Kind::MouseDown(pointer(
                        event.position,
                        Some(event.button),
                        event.modifiers,
                    )),
                    cx,
                );
            });
        }
        if i.mouse_up != 0 {
            let view = view.clone();
            let hub = hub.clone();
            node = node.on_mouse_up(b, move |event, _, cx| {
                if stop {
                    cx.stop_propagation();
                }
                emit(
                    &hub,
                    &view,
                    id,
                    pb::event::Kind::MouseUp(pointer(
                        event.position,
                        Some(event.button),
                        event.modifiers,
                    )),
                    cx,
                );
            });
        }
    }
    if i.mouse_move != 0 {
        let view = view.clone();
        let hub = hub.clone();
        node = node.on_mouse_move(move |event, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            let mut p = pointer(event.position, event.pressed_button, event.modifiers);
            p.dragging = event.dragging();
            emit(&hub, &view, id, pb::event::Kind::MouseMove(p), cx);
        });
    }
    if i.scroll != 0 {
        let view = view.clone();
        let hub = hub.clone();
        node = node.on_scroll(move |event, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            let mut p = pointer(event.position, None, event.modifiers);
            let delta = event.delta.pixel_delta(px(16.0));
            p.delta = Some(pb::Point {
                x: delta.x.into(),
                y: delta.y.into(),
            });
            emit(&hub, &view, id, pb::event::Kind::Scroll(p), cx);
        });
    }
    if i.file_drop != 0 {
        let view = view.clone();
        let hub = hub.clone();
        node = node.on_drop::<gpui::ExternalPaths>(move |paths, _, cx| {
            if stop {
                cx.stop_propagation();
            }
            emit(
                &hub,
                &view,
                id,
                pb::event::Kind::FileDrop(pb::FileDrop {
                    paths: paths
                        .0
                        .iter()
                        .map(|p| p.to_string_lossy().into_owned())
                        .collect(),
                }),
                cx,
            );
        });
    }
    if let Some(drag) = &i.drag {
        node = node.on_drag(drag.clone(), |drag, _, _, cx| {
            cx.new(|_| DragPreview(drag.preview.clone()))
        });
    }
    if i.data_drop != 0 {
        let view = view.clone();
        let hub = hub.clone();
        let accept = i.accept_drop_kind.clone();
        node = node.on_drop::<pb::DragData>(move |drag, _, cx| {
            if !accept.is_empty() && drag.kind != accept {
                cx.propagate();
                return;
            }
            if stop {
                cx.stop_propagation();
            }
            emit(&hub, &view, id, pb::event::Kind::DataDrop(drag.clone()), cx);
        });
    }
    node
}

struct DragPreview(String);
impl Render for DragPreview {
    fn render(&mut self, _: &mut Window, _: &mut Context<Self>) -> impl IntoElement {
        div()
            .p_2()
            .bg(gpui::rgb(0x334155))
            .text_color(gpui::white())
            .child(self.0.clone())
    }
}

fn scope(view: u64, element: u64) -> String {
    format!("GoBridgeV{view}E{element}")
}
#[derive(Clone, PartialEq)]
pub(crate) struct RegisteredKey {
    target: pb::FocusTarget,
    binding: pb::KeyBinding,
}
impl RegisteredKey {
    pub fn load(&self, cx: &App) -> Result<gpui::KeyBinding, String> {
        let base = scope(
            self.target.view.as_ref().unwrap().id,
            self.target.element_id,
        );
        let context = if self.binding.context.is_empty() {
            base
        } else {
            format!("({base}) && ({})", self.binding.context)
        };
        let predicate =
            gpui::KeyBindingContextPredicate::parse(&context).map_err(|e| e.to_string())?;
        gpui::KeyBinding::load(
            &self.binding.keystroke,
            Box::new(crate::platform::MenuAction {
                target: self.target.clone(),
                action: self.binding.action.clone().unwrap(),
                propagate: !self.binding.stop_propagation,
            }),
            Some(Rc::new(predicate)),
            false,
            None,
            cx,
        )
        .map_err(|e| e.to_string())
    }
}
pub(crate) fn key_entries(view: &View) -> Vec<RegisteredKey> {
    fn collect(element: &pb::Element, view: &View, entries: &mut Vec<RegisteredKey>) {
        for binding in &element.key_bindings {
            if !binding.keystroke.is_empty() {
                entries.push(RegisteredKey {
                    target: pb::FocusTarget {
                        view: Some(view.entity.clone()),
                        element_id: element.id,
                    },
                    binding: binding.clone(),
                });
            }
        }
        for child in core::features::children(element) {
            collect(child, view, entries);
        }
    }
    let mut entries = Vec::new();
    collect(view.snapshot.root.as_ref().unwrap(), view, &mut entries);
    entries
}
