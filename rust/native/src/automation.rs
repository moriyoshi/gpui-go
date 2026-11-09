//! Explicitly opted-in E2E instrumentation. All input goes through native GPUI.
use crate::*;
use gpui::{
    AnyElement, Element, ElementId, EntityInputHandler, GlobalElementId, InputEvent,
    InspectorElementId, LayoutId, Pixels,
};

pub(crate) type RenderedBounds = Rc<RefCell<HashMap<u64, Bounds<Pixels>>>>;

struct Observed {
    inner: AnyElement,
    element: u64,
    bounds: RenderedBounds,
}
pub(crate) fn observe(inner: AnyElement, element: u64, bounds: RenderedBounds) -> AnyElement {
    Observed {
        inner,
        element,
        bounds,
    }
    .into_any_element()
}
impl IntoElement for Observed {
    type Element = Self;
    fn into_element(self) -> Self {
        self
    }
}
impl Element for Observed {
    type RequestLayoutState = ();
    type PrepaintState = ();
    fn id(&self) -> Option<ElementId> {
        None
    }
    fn source_location(&self) -> Option<&'static std::panic::Location<'static>> {
        None
    }
    fn request_layout(
        &mut self,
        _: Option<&GlobalElementId>,
        _: Option<&InspectorElementId>,
        window: &mut Window,
        cx: &mut App,
    ) -> (LayoutId, ()) {
        (self.inner.request_layout(window, cx), ())
    }
    fn prepaint(
        &mut self,
        _: Option<&GlobalElementId>,
        _: Option<&InspectorElementId>,
        bounds: Bounds<Pixels>,
        _: &mut (),
        window: &mut Window,
        cx: &mut App,
    ) {
        self.bounds.borrow_mut().insert(
            self.element,
            bounds.intersect(&window.content_mask().bounds),
        );
        self.inner.prepaint(window, cx);
    }
    fn paint(
        &mut self,
        _: Option<&GlobalElementId>,
        _: Option<&InspectorElementId>,
        _: Bounds<Pixels>,
        _: &mut (),
        _: &mut (),
        window: &mut Window,
        cx: &mut App,
    ) {
        self.inner.paint(window, cx);
    }
}
fn invalid(message: impl Into<String>) -> pb::ProtocolError {
    core::error(pb::ErrorCode::InvalidArgument, message)
}
fn mods(value: Option<&pb::Modifiers>) -> gpui::Modifiers {
    value
        .map(|m| gpui::Modifiers {
            control: m.control,
            alt: m.alt,
            shift: m.shift,
            platform: m.platform,
            ..Default::default()
        })
        .unwrap_or_default()
}
fn rendered(bounds: &Bounds<Pixels>) -> bool {
    bounds.size.width > px(0.) && bounds.size.height > px(0.)
}

fn role(element: &pb::Element) -> i32 {
    element
        .accessibility
        .as_ref()
        .map(|a| a.role)
        .unwrap_or_else(|| match element.kind {
            Some(pb::element::Kind::TextInput(_)) => pb::AccessibleRole::TextInput as i32,
            Some(pb::element::Kind::Text(_)) => pb::AccessibleRole::Text as i32,
            Some(pb::element::Kind::Image(_)) => pb::AccessibleRole::Image as i32,
            Some(pb::element::Kind::UniformList(_)) => pb::AccessibleRole::List as i32,
            _ => pb::AccessibleRole::Generic as i32,
        })
}
fn name(element: &pb::Element) -> String {
    if let Some(label) = &element.accessibility_label
        && !label.is_empty()
    {
        return label.clone();
    }
    if matches!(element.kind, Some(pb::element::Kind::Text(_)))
        || matches!(
            pb::AccessibleRole::try_from(role(element)),
            Ok(pb::AccessibleRole::Button
                | pb::AccessibleRole::Checkbox
                | pb::AccessibleRole::Link)
        )
    {
        text_content(element)
    } else {
        String::new()
    }
}
fn text_content(element: &pb::Element) -> String {
    match &element.kind {
        Some(pb::element::Kind::Text(text)) => text.content.clone(),
        _ => core::features::children(element)
            .iter()
            .map(text_content)
            .collect::<Vec<_>>()
            .join(""),
    }
}

fn normalize(text: &str) -> String {
    text.split_whitespace().collect::<Vec<_>>().join(" ")
}
fn query_matches(
    element: &pb::Element,
    query: &pb::AutomationQuery,
    found: &mut Vec<pb::AutomationMatch>,
) {
    let matches_text = |actual: &str, expected: &str| {
        let actual = normalize(actual);
        let expected = normalize(expected);
        if query.exact {
            actual == expected
        } else {
            actual.contains(&expected)
        }
    };
    if (query.element_id == 0 || query.element_id == element.id)
        && (query.role == 0 || query.role == role(element))
        && (query.name.is_empty() || matches_text(&name(element), &query.name))
        && query.text.as_ref().is_none_or(|expected| matches!(&element.kind, Some(pb::element::Kind::Text(t)) if matches_text(&t.content, expected))) {
        found.push(pb::AutomationMatch { element_id: element.id });
    }
    for child in core::features::children(element) {
        query_matches(child, query, found);
    }
}
fn reveal_rows(element: &pb::Element, target: u64, rows: &mut Vec<(u64, usize)>) {
    if let Some(pb::element::Kind::UniformList(list)) = &element.kind
        && let Some(index) = list
            .items
            .iter()
            .position(|row| core::features::find_element(row, target).is_some())
    {
        rows.push((element.id, index));
    }
    for child in core::features::children(element) {
        if core::features::find_element(child, target).is_some() {
            reveal_rows(child, target, rows);
            break;
        }
    }
}

pub(crate) fn execute(
    windows: &mut Windows,
    cx: &mut App,
    hub: &Arc<Hub>,
    view: &View,
    request: &pb::AutomationRequest,
) -> Result<pb::AutomationResult, pb::ProtocolError> {
    use pb::automation_request::Operation;
    let os = cx.platform_info().os;
    // Parse/validate before drawing or dispatching anything.
    let key = match request.operation.as_ref().unwrap() {
        Operation::Key(k) => {
            if k.keystroke.is_empty()
                || k.keystroke.len() > 128
                || k.keystroke.contains(char::is_whitespace)
            {
                return Err(invalid("expected one keystroke"));
            }
            Some(
                gpui::Keystroke::parse_with_os(&k.keystroke, os)
                    .map_err(|e| invalid(e.to_string()))?,
            )
        }
        Operation::Text(t) if t.text.len() > core::text::MAX_INPUT_BYTES => {
            return Err(invalid("text exceeds input limit"));
        }
        Operation::Composition(t) if t.text.len() > core::text::MAX_INPUT_BYTES => {
            return Err(invalid("composition exceeds input limit"));
        }
        Operation::Query(q)
            if q.name.len() > 4096
                || q.text.as_ref().is_some_and(|s| s.len() > 65536)
                || pb::AccessibleRole::try_from(q.role).is_err() =>
        {
            return Err(invalid("invalid locator query"));
        }
        Operation::Pointer(p) => {
            if !matches!(
                pb::automation_pointer::Phase::try_from(p.phase),
                Ok(pb::automation_pointer::Phase::Click
                    | pb::automation_pointer::Phase::Down
                    | pb::automation_pointer::Phase::Up
                    | pb::automation_pointer::Phase::Move)
            ) || p.position.as_ref().is_some_and(|p| {
                !p.x.is_finite()
                    || !p.y.is_finite()
                    || p.x.abs() > 1_000_000.
                    || p.y.abs() > 1_000_000.
            }) {
                return Err(invalid("invalid pointer input"));
            }
            None
        }
        _ => None,
    };
    let window = windows
        .views
        .get(&view.entity.id)
        .ok_or_else(|| invalid("native view is closed"))?
        .1;
    let any: gpui::AnyWindowHandle = window.into();
    any.update(cx, |_, window, cx| window.draw(cx).clear(cx))
        .map_err(|e| invalid(e.to_string()))?;
    let inputs = window
        .read(cx)
        .map_err(|e| invalid(e.to_string()))?
        .inputs
        .clone();
    let visible_bounds = window
        .read(cx)
        .map_err(|e| invalid(e.to_string()))?
        .rendered
        .clone();
    let lists = window
        .read(cx)
        .map_err(|e| invalid(e.to_string()))?
        .lists
        .clone();
    let mut rows = Vec::new();
    if matches!(request.operation, Some(Operation::Reveal(_))) {
        reveal_rows(
            view.snapshot.root.as_ref().unwrap(),
            request.element_id,
            &mut rows,
        );
    }
    // dispatch_event may itself draw. Never lease the root view across input.
    any.update(cx, |_, window, cx| {
        match request.operation.as_ref().unwrap() {
            Operation::Inspect(_) | Operation::Query(_) => {}
            Operation::Reveal(_) => {
                for (id, index) in rows {
                    if let Some(handle) = lists.get(&id) {
                        handle.scroll_to_item(index, gpui::ScrollStrategy::Top);
                    }
                }
            }
            Operation::Key(_) => {
                let key = key.unwrap();
                window.dispatch_keystroke(key.clone(), cx);
                window.dispatch_event(gpui::KeyUpEvent { keystroke: key }.to_platform_input(), cx);
            }
            Operation::Text(t) => {
                if !inputs
                    .values()
                    .any(|input| input.focus_handle(cx).is_focused(window))
                {
                    return Err(invalid("text input requires a focused native field"));
                }
                // Use GPUI's focused platform input handler, as dispatch_keystroke
                // does for printable keys. One payload preserves combining text.
                window.dispatch_keystroke(
                    gpui::Keystroke {
                        key: "text".into(),
                        key_char: Some(t.text.clone()),
                        modifiers: Default::default(),
                    },
                    cx,
                );
            }
            Operation::Composition(t) => {
                let input = inputs
                    .get(&request.element_id)
                    .ok_or_else(|| invalid("composition target is not a native field"))?
                    .clone();
                if !input.focus_handle(cx).is_focused(window) {
                    return Err(invalid("composition target is not focused"));
                }
                input.update(cx, |input, cx| {
                    if t.commit {
                        input.replace_text_in_range(None, &t.text, window, cx);
                    } else {
                        input.replace_and_mark_text_in_range(None, &t.text, None, window, cx);
                    }
                });
            }
            Operation::Pointer(p) => {
                let position = if let Some(p) = &p.position {
                    gpui::point(px(p.x), px(p.y))
                } else {
                    let bounds = visible_bounds
                        .borrow()
                        .get(&request.element_id)
                        .copied()
                        .filter(rendered)
                        .ok_or_else(|| invalid("pointer target is not rendered"))?;
                    bounds.center()
                };
                let modifiers = mods(p.modifiers.as_ref());
                let phase = pb::automation_pointer::Phase::try_from(p.phase).unwrap();
                if matches!(
                    phase,
                    pb::automation_pointer::Phase::Move | pb::automation_pointer::Phase::Click
                ) {
                    window.dispatch_event(
                        gpui::MouseMoveEvent {
                            position,
                            modifiers,
                            pressed_button: p.pressed.then_some(gpui::MouseButton::Left),
                        }
                        .to_platform_input(),
                        cx,
                    );
                }
                if matches!(
                    phase,
                    pb::automation_pointer::Phase::Down | pb::automation_pointer::Phase::Click
                ) {
                    window.dispatch_event(
                        gpui::MouseDownEvent {
                            position,
                            modifiers,
                            button: gpui::MouseButton::Left,
                            click_count: 1,
                            first_mouse: false,
                        }
                        .to_platform_input(),
                        cx,
                    );
                }
                if matches!(
                    phase,
                    pb::automation_pointer::Phase::Up | pb::automation_pointer::Phase::Click
                ) {
                    window.dispatch_event(
                        gpui::MouseUpEvent {
                            position,
                            modifiers,
                            button: gpui::MouseButton::Left,
                            click_count: 1,
                        }
                        .to_platform_input(),
                        cx,
                    );
                }
            }
        }
        window.refresh();
        Ok(())
    })
    .map_err(|e| invalid(e.to_string()))??;
    // Drawing cannot occur while the root GoView entity is mutably borrowed.
    any.update(cx, |_, window, cx| window.draw(cx).clear(cx))
        .map_err(|e| invalid(e.to_string()))?;
    window
        .update(cx, |entity, window, cx| {
            // Input changes stay in persistent entities even before Go echoes them.
            let bounds = entity.rendered.borrow().get(&request.element_id).copied();
            let element = core::features::find_element(
                entity.view.snapshot.root.as_ref().unwrap(),
                request.element_id,
            )
            .unwrap();
            let mut matches = Vec::new();
            if let Some(Operation::Query(query)) = &request.operation { query_matches(element, query, &mut matches); }
            Ok(pb::AutomationResult {
                version: 1,
                revision: entity.view.snapshot.revision,
                event_sequence: hub.event_sequence(),
                focused_element_id: entity
                    .focus
                    .iter()
                    .find(|(_, (f, _))| f.is_focused(window))
                    .map(|(id, _)| *id)
                    .unwrap_or(0),
                element_id: request.element_id,
                matches,
                enabled: !matches!(&element.kind, Some(pb::element::Kind::TextInput(t)) if t.disabled),
                role: role(element),
                checked: element.accessibility.as_ref().and_then(|a| a.checked),
                name: name(element),
                rendered: bounds.as_ref().is_some_and(rendered),
                bounds: bounds.map(|b| pb::Bounds {
                    origin: Some(pb::Point {
                        x: b.origin.x.into(),
                        y: b.origin.y.into(),
                    }),
                    size: Some(pb::Size {
                        width: b.size.width.into(),
                        height: b.size.height.into(),
                    }),
                }),
                input: entity
                    .inputs
                    .get(&request.element_id)
                    .map(|input| input.read(cx).automation_state()),
                text: Some(text_content(element)),
            })
        })
        .map_err(|e| invalid(e.to_string()))?
}

#[cfg(test)]
mod selector_tests {
    use super::*;
    #[test]
    fn semantic_queries_use_roles_names_and_normalized_text_without_input_values() {
        let root = pb::Element {
            id: 1,
            kind: Some(pb::element::Kind::Container(pb::Container {
                children: vec![
                    pb::Element {
                        id: 2,
                        kind: Some(pb::element::Kind::Text(pb::Text {
                            content: "Save   日本語".into(),
                        })),
                        accessibility_label: Some("Save document".into()),
                        accessibility: Some(pb::Accessibility {
                            role: pb::AccessibleRole::Button as i32,
                            ..Default::default()
                        }),
                        ..Default::default()
                    },
                    pb::Element {
                        id: 3,
                        kind: Some(pb::element::Kind::TextInput(pb::TextInput {
                            value: "secret input value".into(),
                            ..Default::default()
                        })),
                        accessibility_label: Some("Name".into()),
                        ..Default::default()
                    },
                    pb::Element {
                        id: 4,
                        kind: Some(pb::element::Kind::Container(pb::Container {
                            children: vec![pb::Element {
                                id: 5,
                                kind: Some(pb::element::Kind::Text(pb::Text {
                                    content: "Run".into(),
                                })),
                                ..Default::default()
                            }],
                        })),
                        accessibility: Some(pb::Accessibility {
                            role: pb::AccessibleRole::Button as i32,
                            ..Default::default()
                        }),
                        ..Default::default()
                    },
                ],
            })),
            ..Default::default()
        };
        let query = |q: pb::AutomationQuery| {
            let mut found = Vec::new();
            query_matches(&root, &q, &mut found);
            found.into_iter().map(|m| m.element_id).collect::<Vec<_>>()
        };
        assert_eq!(
            query(pb::AutomationQuery {
                role: pb::AccessibleRole::Button as i32,
                name: "Save document".into(),
                exact: true,
                ..Default::default()
            }),
            vec![2]
        );
        assert!(
            query(pb::AutomationQuery {
                role: pb::AccessibleRole::Button as i32,
                name: "Save".into(),
                exact: true,
                ..Default::default()
            })
            .is_empty()
        );
        assert_eq!(
            query(pb::AutomationQuery {
                text: Some("Save 日本語".into()),
                exact: true,
                ..Default::default()
            }),
            vec![2]
        );
        assert_eq!(
            query(pb::AutomationQuery {
                role: pb::AccessibleRole::TextInput as i32,
                name: "Name".into(),
                ..Default::default()
            }),
            vec![3]
        );
        assert!(
            query(pb::AutomationQuery {
                text: Some("secret input value".into()),
                ..Default::default()
            })
            .is_empty()
        );
        assert_eq!(
            query(pb::AutomationQuery {
                role: pb::AccessibleRole::Button as i32,
                name: "Run".into(),
                exact: true,
                ..Default::default()
            }),
            vec![4]
        );
        assert_eq!(
            query(pb::AutomationQuery {
                text: Some("Run".into()),
                exact: true,
                ..Default::default()
            }),
            vec![5]
        );
        assert_eq!(text_content(&root), "Save   日本語Run");
        assert_eq!(name(&root), "");
    }
}
