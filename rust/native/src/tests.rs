//! Exercise actual GPUI dispatch, not just protobuf handler injection.
use super::*;
use gpui::{AnyWindowHandle, AppContext, EntityInputHandler, InputEvent, TestAppContext};
fn root() -> pb::Element {
    pb::Element {
        id: 1,
        kind: Some(pb::element::Kind::Text(pb::Text {
            content: "Keyboard target".into(),
        })),
        interaction: Some(pb::Interaction {
            focusable: true,
            key_down: 50,
            ..Default::default()
        }),
        on_click: Some(pb::EventBinding {
            handler_id: 61,
            ..Default::default()
        }),
        key_bindings: vec![pb::KeyBinding {
            keystroke: "cmd-k".into(),
            action: Some(pb::Action {
                name: "save".into(),
                payload: vec![],
            }),
            handler_id: 51,
            stop_propagation: true,
            ..Default::default()
        }],
        style: Some(pb::Style {
            width: Some(pb::Length {
                value: Some(pb::length::Value::Pixels(200.0)),
            }),
            height: Some(pb::Length {
                value: Some(pb::length::Value::Pixels(80.0)),
            }),
            ..Default::default()
        }),
        ..Default::default()
    }
}
fn setup() -> (TestAppContext, WindowHandle<GoView>, Arc<Hub>) {
    let mut cx = TestAppContext::single();
    let hub = Hub::new(9);
    let create = hub.clone();
    cx.update(|cx| {
        input::bind_keys(cx);
        platform::init(cx, hub.clone());
    });
    let window = cx.add_window(move |window, cx| {
        let view = View {
            window: pb::Handle {
                id: 1,
                generation: 1,
            },
            entity: pb::Handle {
                id: 2,
                generation: 1,
            },
            options: pb::WindowOptions {
                title: "test".into(),
                size: Some(pb::Size {
                    width: 500.0,
                    height: 300.0,
                }),
            },
            snapshot: pb::ViewSnapshot {
                revision: 1,
                root: Some(root()),
            },
        };
        let mut entity = GoView {
            view,
            hub: create,
            inputs: HashMap::new(),
            focus: HashMap::new(),
            lists: HashMap::new(),
            resources: Rc::new(RefCell::new(Assets::new(AssetBytes::default()))),
            reported_size: None,
            reported_scheme: None,
            _scheme_observer: window.observe_window_appearance(|window, _| window.refresh()),
            #[cfg(feature = "automation")]
            rendered: Default::default(),
        };
        entity.sync_inputs(window, cx);
        entity
    });
    window
        .update(&mut cx, |view, window, cx| {
            window.focus(&view.focus[&1].0, cx)
        })
        .unwrap();
    refresh_bindings(&mut cx, window);
    cx.run_until_parked();
    draw(&mut cx, window.into());
    while hub.receiver.try_recv().is_ok() {}
    (cx, window, hub)
}
fn draw(cx: &mut TestAppContext, window: AnyWindowHandle) {
    cx.update_window(window, |_, window, cx| {
        window.draw(cx).clear(cx);
    })
    .unwrap();
}
fn kinds(hub: &Hub) -> Vec<(u64, pb::event::Kind)> {
    let mut values = Vec::new();
    while let Ok(message) = hub.receiver.try_recv() {
        if let Message::Interaction { revision, kind, .. } = message {
            values.push((revision, kind));
        } else if let Message::Action { action, .. } = message {
            values.push((0, pb::event::Kind::Action(action)));
        }
    }
    values
}
#[test]
fn native_key_binding_and_raw_key_use_actual_focus_dispatch() {
    let (mut cx, window, hub) = setup();
    cx.simulate_keystrokes(window.into(), "cmd-k");
    let events = kinds(&hub);
    assert!(
        events
            .iter()
            .any(|(_, kind)| matches!(kind,pb::event::Kind::Action(a) if a.name=="save")),
        "{events:?}"
    );
    assert!(
        !events
            .iter()
            .any(|(_, kind)| matches!(kind, pb::event::Kind::KeyDown(_)))
    );
    cx.simulate_keystrokes(window.into(), "x");
    assert!(
        kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind,pb::event::Kind::KeyDown(k) if k.key=="x"))
    );
}
#[test]
fn native_shortcuts_are_replaced_without_recreating_focus() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, window, cx| {
            view.view.snapshot.revision = 2;
            view.view.snapshot.root.as_mut().unwrap().key_bindings[0].keystroke = "cmd-j".into();
            view.sync_inputs(window, cx);
            cx.notify();
        })
        .unwrap();
    refresh_bindings(&mut cx, window);
    draw(&mut cx, window.into());
    cx.simulate_keystrokes(window.into(), "cmd-k");
    assert!(
        !kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind, pb::event::Kind::Action(_)))
    );
    cx.simulate_keystrokes(window.into(), "cmd-j");
    assert!(
        kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind, pb::event::Kind::Action(_)))
    );
}
#[test]
fn native_keyboard_activation_reaches_click_callback() {
    let (mut cx, window, hub) = setup();
    let os = cx.read(|cx| cx.platform_info().os);
    let keystroke = gpui::Keystroke::parse_with_os("enter", os).unwrap();
    cx.update_window(window.into(), |_, window, cx| {
        window.dispatch_event(
            gpui::KeyDownEvent {
                keystroke: keystroke.clone(),
                is_held: false,
                prefer_character_input: false,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(gpui::KeyUpEvent { keystroke }.to_platform_input(), cx);
    })
    .unwrap();
    let mut clicked = false;
    while let Ok(message) = hub.receiver.try_recv() {
        if matches!(message, Message::Click { element: 1, .. }) {
            clicked = true;
        }
    }
    assert!(
        clicked,
        "Enter must activate the focused native click target"
    );
}

#[test]
fn native_pointer_events_respect_the_element_hitbox() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, _, cx| {
            let input = view
                .view
                .snapshot
                .root
                .as_mut()
                .unwrap()
                .interaction
                .as_mut()
                .unwrap();
            input.mouse_down = 71;
            input.mouse_up = 72;
            input.mouse_move = 73;
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, window.into());
    let position = gpui::point(px(10.0), px(10.0));
    let modifiers = gpui::Modifiers {
        shift: true,
        ..Default::default()
    };
    window
        .update(&mut cx, |_, window, cx| {
            window.dispatch_event(
                gpui::MouseMoveEvent {
                    position,
                    modifiers,
                    pressed_button: None,
                }
                .to_platform_input(),
                cx,
            );
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
        })
        .unwrap();
    let events = kinds(&hub);
    assert!(events.iter().any(|(_, k)| matches!(k, pb::event::Kind::MouseMove(p) if p.position.as_ref().unwrap().x == 10.0)));
    assert!(events.iter().any(|(_, k)| matches!(k, pb::event::Kind::MouseDown(p) if p.button == 1 && p.modifiers.as_ref().unwrap().shift)));
    assert!(
        events
            .iter()
            .any(|(_, k)| matches!(k, pb::event::Kind::MouseUp(p) if p.button == 1))
    );
    window
        .update(&mut cx, |_, window, cx| {
            window.dispatch_event(
                gpui::MouseDownEvent {
                    position: gpui::point(px(300.0), px(200.0)),
                    modifiers,
                    button: gpui::MouseButton::Left,
                    click_count: 1,
                    first_mouse: false,
                }
                .to_platform_input(),
                cx,
            );
        })
        .unwrap();
    assert!(
        !kinds(&hub)
            .iter()
            .any(|(_, k)| matches!(k, pb::event::Kind::MouseDown(_)))
    );
}

#[test]
fn focusable_ancestor_does_not_steal_a_text_inputs_click_focus() {
    let (mut cx, handle, hub) = setup();
    handle
        .update(&mut cx, |view, window, cx| {
            view.view.snapshot.root.as_mut().unwrap().kind =
                Some(pb::element::Kind::Container(pb::Container {
                    children: vec![pb::Element {
                        id: 2,
                        kind: Some(pb::element::Kind::TextInput(pb::TextInput {
                            on_change: Some(pb::EventBinding {
                                handler_id: 81,
                                ..Default::default()
                            }),
                            ..Default::default()
                        })),
                        style: Some(pb::Style {
                            width: Some(pb::Length {
                                value: Some(pb::length::Value::Pixels(190.)),
                            }),
                            height: Some(pb::Length {
                                value: Some(pb::length::Value::Pixels(40.)),
                            }),
                            ..Default::default()
                        }),
                        ..Default::default()
                    }],
                }));
            view.sync_inputs(window, cx);
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, handle.into());
    cx.update_window(handle.into(), |_, window, cx| {
        let position = gpui::point(px(10.), px(10.));
        window.dispatch_event(
            gpui::MouseDownEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
                first_mouse: false,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(
            gpui::MouseUpEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
            }
            .to_platform_input(),
            cx,
        );
    })
    .unwrap();
    handle
        .update(&mut cx, |view, window, cx| {
            assert!(
                view.inputs[&2].focus_handle(cx).is_focused(window),
                "ancestor stole the child's focus"
            )
        })
        .unwrap();
    draw(&mut cx, handle.into());
    cx.simulate_input(handle.into(), "Typed");
    assert!(std::iter::from_fn(|| hub.receiver.try_recv().ok()).any(|message| matches!(message, Message::TextChanged { element: 2, change, .. } if change.value == "Typed")));
}

#[test]
fn unicode_text_uses_native_input_handler_after_kind_change() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, window, cx| {
            view.view.snapshot.revision = 2;
            view.view.snapshot.root = Some(pb::Element {
                id: 1,
                kind: Some(pb::element::Kind::TextInput(pb::TextInput {
                    value: String::new(),
                    epoch: 4,
                    on_change: Some(pb::EventBinding {
                        handler_id: 81,
                        ..Default::default()
                    }),
                    ..Default::default()
                })),
                ..Default::default()
            });
            view.sync_inputs(window, cx);
            window.focus(&view.focus[&1].0, cx);
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, window.into());
    cx.simulate_input(window.into(), "A日本語");
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(view.inputs[&1].read(cx).value(), "A日本語")
        })
        .unwrap();
    let mut changed = false;
    while let Ok(message) = hub.receiver.try_recv() {
        if let Message::TextChanged { change, .. } = message
            && change.value == "A日本語"
        {
            assert_eq!(change.epoch, 4);
            changed = true;
        }
    }
    assert!(changed, "native input did not emit the Unicode change");
}

fn masked_field(value: &str) -> (TestAppContext, WindowHandle<GoView>, Arc<Hub>) {
    let (mut cx, window, hub) = setup();
    let value = value.to_string();
    window
        .update(&mut cx, move |view, window, cx| {
            view.view.snapshot.revision = 2;
            view.view.snapshot.root = Some(pb::Element {
                id: 1,
                kind: Some(pb::element::Kind::TextInput(pb::TextInput {
                    value,
                    masked: true,
                    ..Default::default()
                })),
                ..Default::default()
            });
            view.sync_inputs(window, cx);
            window.focus(&view.focus[&1].0, cx);
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, window.into());
    (cx, window, hub)
}

fn set_masked(cx: &mut TestAppContext, window: WindowHandle<GoView>, masked: bool) {
    window
        .update(cx, |view, window, cx| {
            let root = view.view.snapshot.root.as_mut().unwrap();
            let Some(pb::element::Kind::TextInput(input)) = root.kind.as_mut() else {
                panic!("expected a text input");
            };
            input.masked = masked;
            view.sync_inputs(window, cx);
            cx.notify();
        })
        .unwrap();
    draw(cx, window.into());
}

#[test]
fn masked_offsets_map_whole_graphemes_to_bullets() {
    let content = "e\u{301}\u{1F642}a";
    assert_eq!(input::masked_text(content), "\u{2022}\u{2022}\u{2022}");
    assert_eq!(input::masked_text(""), "");
    let boundaries = [(0, 0), (3, 3), (7, 6), (8, 9)];
    for (real, shown) in boundaries {
        assert_eq!(input::masked_offset(content, real), shown, "real {real}");
        assert_eq!(
            input::unmasked_offset(content, shown),
            real,
            "shown {shown}"
        );
    }
    assert_eq!(input::unmasked_offset(content, 900), content.len());
    assert_eq!(input::unmasked_offset("", 0), 0);
}

#[test]
fn masked_input_shapes_bullets_and_edits_the_real_value() {
    let (mut cx, window, hub) = masked_field("e\u{301}x");
    cx.simulate_input(window.into(), "y");
    draw(&mut cx, window.into());
    window
        .update(&mut cx, |view, _, cx| {
            let input = view.inputs[&1].read(cx);
            assert_eq!(input.value(), "e\u{301}xy");
            assert_eq!(
                input.shaped_text().as_deref(),
                Some("\u{2022}\u{2022}\u{2022}")
            );
        })
        .unwrap();
    let mut changed = false;
    while let Ok(message) = hub.receiver.try_recv() {
        if let Message::TextChanged { change, .. } = message {
            changed |= change.value == "e\u{301}xy";
        }
    }
    assert!(changed, "Go must receive the real value");
    set_masked(&mut cx, window, false);
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(
                view.inputs[&1].read(cx).shaped_text().as_deref(),
                Some("e\u{301}xy")
            );
        })
        .unwrap();
}

#[test]
fn masked_input_withholds_copy_and_cut_but_accepts_paste() {
    let (mut cx, window, _) = masked_field("hunter2");
    cx.write_to_clipboard(gpui::ClipboardItem::new_string("before".into()));
    cx.simulate_keystrokes(window.into(), "cmd-a cmd-c");
    assert_eq!(
        cx.read_from_clipboard().and_then(|item| item.text()),
        Some("before".to_string())
    );
    cx.simulate_keystrokes(window.into(), "cmd-x");
    assert_eq!(
        cx.read_from_clipboard().and_then(|item| item.text()),
        Some("before".to_string())
    );
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(view.inputs[&1].read(cx).value(), "hunter2")
        })
        .unwrap();
    cx.simulate_keystrokes(window.into(), "cmd-v");
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(view.inputs[&1].read(cx).value(), "before")
        })
        .unwrap();
    set_masked(&mut cx, window, false);
    cx.write_to_clipboard(gpui::ClipboardItem::new_string("changed".into()));
    cx.simulate_keystrokes(window.into(), "cmd-a cmd-x");
    assert_eq!(
        cx.read_from_clipboard().and_then(|item| item.text()),
        Some("before".to_string())
    );
}

#[test]
fn masked_input_maps_clicks_to_real_text_offsets() {
    let (mut cx, window, _) = masked_field("abc");
    let x = window
        .update(&mut cx, |view, window, cx| {
            view.inputs[&1].update(cx, |input, cx| {
                let bounds =
                    gpui::Bounds::new(gpui::point(px(0.), px(0.)), gpui::size(px(200.), px(24.)));
                input
                    .bounds_for_range(1..1, bounds, window, cx)
                    .unwrap()
                    .left()
            })
        })
        .unwrap();
    assert!(x > px(0.), "the test font must give bullets a width");
    cx.update_window(window.into(), |_, window, cx| {
        let position = gpui::point(x, px(10.));
        window.dispatch_event(
            gpui::MouseMoveEvent {
                position,
                modifiers: Default::default(),
                pressed_button: None,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(
            gpui::MouseDownEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
                first_mouse: false,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(
            gpui::MouseUpEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
            }
            .to_platform_input(),
            cx,
        );
    })
    .unwrap();
    draw(&mut cx, window.into());
    cx.simulate_input(window.into(), "Z");
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(view.inputs[&1].read(cx).value(), "aZbc")
        })
        .unwrap();
}

#[test]
fn masked_input_places_selection_caret_and_bounds_in_the_shaped_line() {
    let (mut cx, window, _) = masked_field("abc");
    cx.simulate_keystrokes(window.into(), "cmd-a");
    let bullet = "\u{2022}".len();
    window
        .update(&mut cx, |view, _, cx| {
            assert_eq!(
                view.inputs[&1].read(cx).display_selection(),
                (0..3 * bullet, 3 * bullet)
            );
        })
        .unwrap();
    cx.simulate_keystrokes(window.into(), "home right right");
    draw(&mut cx, window.into());
    window
        .update(&mut cx, |view, window, cx| {
            view.inputs[&1].update(cx, |input, cx| {
                assert_eq!(
                    input.display_selection(),
                    (2 * bullet..2 * bullet, 2 * bullet)
                );
                let bounds =
                    gpui::Bounds::new(gpui::point(px(0.), px(0.)), gpui::size(px(200.), px(24.)));
                let left = input
                    .bounds_for_range(2..2, bounds, window, cx)
                    .unwrap()
                    .left();
                assert_eq!(left, input.shaped_x(2 * bullet).unwrap());
                assert_eq!(
                    input.character_index_for_point(gpui::point(left, px(10.)), window, cx),
                    Some(2)
                );
            })
        })
        .unwrap();
}

#[test]
fn masked_composition_shapes_bullets_for_marked_text() {
    let (mut cx, window, _) = masked_field("");
    window
        .update(&mut cx, |view, window, cx| {
            view.inputs[&1].update(cx, |input, cx| {
                input.replace_and_mark_text_in_range(None, "\u{1F642}\u{1F600}", None, window, cx)
            })
        })
        .unwrap();
    draw(&mut cx, window.into());
    window
        .update(&mut cx, |view, _, cx| {
            let input = view.inputs[&1].read(cx);
            assert_eq!(input.value(), "\u{1F642}\u{1F600}");
            assert_eq!(input.shaped_text().as_deref(), Some("\u{2022}\u{2022}"));
        })
        .unwrap();
}

#[test]
fn masked_input_hides_text_from_text_services_and_accessibility() {
    let (mut cx, window, _) = masked_field("ab");
    let read = |cx: &mut TestAppContext| {
        window
            .update(cx, |view, window, cx| {
                view.inputs[&1].update(cx, |input, cx| {
                    let mut actual = None;
                    let text = input.text_for_range(0..2, &mut actual, window, cx);
                    let bounds = gpui::Bounds::new(
                        gpui::point(px(0.), px(0.)),
                        gpui::size(px(200.), px(24.)),
                    );
                    let located = input.bounds_for_range(0..1, bounds, window, cx);
                    (
                        text,
                        located.is_some(),
                        input.accessible_role(),
                        input.accessible_value(),
                    )
                })
            })
            .unwrap()
    };
    assert_eq!(
        read(&mut cx),
        (
            None,
            true,
            gpui::Role::PasswordInput,
            "\u{2022}\u{2022}".to_string()
        )
    );
    set_masked(&mut cx, window, false);
    assert_eq!(
        read(&mut cx),
        (
            Some("ab".to_string()),
            true,
            gpui::Role::TextInput,
            "ab".to_string()
        )
    );
}

#[test]
fn native_menu_action_is_dispatched_by_gpui() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, window, cx| {
            window.dispatch_action(
                Box::new(platform::MenuAction {
                    target: pb::FocusTarget {
                        view: Some(view.view.entity.clone()),
                        element_id: 1,
                    },
                    action: pb::Action {
                        name: "save".into(),
                        payload: vec![7],
                    },
                    propagate: false,
                }),
                cx,
            );
        })
        .unwrap();
    cx.run_until_parked();
    let mut found = false;
    while let Ok(message) = hub.receiver.try_recv() {
        if let Message::Action {
            entity,
            element,
            action,
        } = message
        {
            assert_eq!(entity.id, 2);
            assert_eq!(element, 1);
            assert_eq!(action.payload, vec![7]);
            found = true;
        }
    }
    assert!(
        found,
        "GPUI did not deliver the menu action to the application handler"
    );
}
#[test]
fn path_dialog_completion_and_cancellation_are_asynchronous() {
    let cx = TestAppContext::single();
    let hub = Hub::new(9);
    let mut windows = Windows {
        views: HashMap::new(),
        resources: Rc::new(RefCell::new(Assets::new(AssetBytes::default()))),
        models: HashMap::new(),
        pending_prompts: Rc::new(std::cell::Cell::new(0)),
        base_keys: Vec::new(),
        installed_keys: Vec::new(),
    };
    cx.update(|cx| {
        let command = pb::Command {
            kind: Some(pb::command::Kind::PromptPaths(pb::PathPrompt {
                files: true,
                multiple: true,
                handler_id: 42,
                ..Default::default()
            })),
        };
        assert_eq!(
            platform::command(&command, &mut windows, cx, hub.clone())
                .unwrap()
                .operation_id,
            42
        );
    });
    assert!(cx.did_prompt_for_paths());
    assert!(hub.receiver.try_recv().is_err());
    cx.simulate_path_prompt_response(|options| {
        assert!(options.files && options.multiple);
        Some(vec![std::path::PathBuf::from("/selected/file")])
    });
    cx.run_until_parked();
    match hub.receiver.try_recv().unwrap() {
        Message::PlatformResult { handler_id, result } => {
            assert_eq!(handler_id, 42);
            assert_eq!(result.paths, vec!["/selected/file"]);
            assert!(!result.cancelled);
            assert!(result.error.is_empty());
        }
        _ => panic!("wrong completion"),
    }
    assert_eq!(windows.pending_prompts.get(), 0);
    cx.update(|cx| {
        platform::command(
            &pb::Command {
                kind: Some(pb::command::Kind::PromptPaths(pb::PathPrompt {
                    files: true,
                    handler_id: 43,
                    ..Default::default()
                })),
            },
            &mut windows,
            cx,
            hub.clone(),
        )
        .unwrap();
    });
    cx.simulate_path_prompt_response(|_| None);
    cx.run_until_parked();
    assert!(
        matches!(hub.receiver.try_recv().unwrap(),Message::PlatformResult{handler_id:43,result} if result.cancelled && result.paths.is_empty())
    );
}

fn focus_form() -> (TestAppContext, WindowHandle<GoView>, Arc<Hub>) {
    let (mut cx, handle, hub) = setup();
    handle
        .update(&mut cx, |view, window, cx| {
            let children = (2..=5)
                .map(|id| pb::Element {
                    id,
                    kind: Some(if id == 4 {
                        pb::element::Kind::Text(pb::Text {
                            content: "Button".into(),
                        })
                    } else {
                        pb::element::Kind::TextInput(pb::TextInput {
                            value: if id == 3 {
                                "Locked".into()
                            } else {
                                String::new()
                            },
                            disabled: id == 3,
                            ..Default::default()
                        })
                    }),
                    interaction: (id == 4).then_some(pb::Interaction {
                        focusable: true,
                        ..Default::default()
                    }),
                    style: Some(pb::Style {
                        height: Some(pb::Length {
                            value: Some(pb::length::Value::Pixels(40.)),
                        }),
                        width: Some(pb::Length {
                            value: Some(pb::length::Value::Pixels(200.)),
                        }),
                        ..Default::default()
                    }),
                    ..Default::default()
                })
                .collect();
            let root = view.view.snapshot.root.as_mut().unwrap();
            root.kind = Some(pb::element::Kind::Container(pb::Container { children }));
            root.style.as_mut().unwrap().height = Some(pb::Length {
                value: Some(pb::length::Value::Pixels(200.)),
            });
            root.style.as_mut().unwrap().flex_direction = Some(pb::FlexDirection::Column as i32);
            view.sync_inputs(window, cx);
            window.focus(&view.focus[&2].0, cx);
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, handle.into());
    (cx, handle, hub)
}

#[test]
fn tab_navigation_crosses_mixed_controls_and_skips_disabled_fields() {
    let (mut cx, handle, _) = focus_form();
    for (key, expected) in [("tab", 4), ("tab", 5), ("shift-tab", 4)] {
        cx.simulate_keystrokes(handle.into(), key);
        draw(&mut cx, handle.into());
        handle
            .update(&mut cx, |view, window, _| {
                assert!(
                    view.focus[&expected].0.is_focused(window),
                    "{key} must focus {expected}"
                )
            })
            .unwrap();
    }
}

#[test]
fn clicking_disabled_input_preserves_focus_and_editing_in_another_field() {
    let (mut cx, handle, _) = focus_form();
    handle
        .update(&mut cx, |view, window, cx| {
            window.focus(&view.focus[&5].0, cx)
        })
        .unwrap();
    draw(&mut cx, handle.into());
    cx.update_window(handle.into(), |_, window, cx| {
        let position = gpui::point(px(10.), px(50.));
        window.dispatch_event(
            gpui::MouseMoveEvent {
                position,
                modifiers: Default::default(),
                pressed_button: None,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(
            gpui::MouseDownEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
                first_mouse: false,
            }
            .to_platform_input(),
            cx,
        );
        window.dispatch_event(
            gpui::MouseUpEvent {
                position,
                modifiers: Default::default(),
                button: gpui::MouseButton::Left,
                click_count: 1,
            }
            .to_platform_input(),
            cx,
        );
    })
    .unwrap();
    handle
        .update(&mut cx, |view, window, _| {
            assert!(view.focus[&5].0.is_focused(window))
        })
        .unwrap();
    draw(&mut cx, handle.into());
    cx.simulate_input(handle.into(), "safe");
    handle
        .update(&mut cx, |view, _, cx| {
            assert_eq!(view.inputs[&3].read(cx).value(), "Locked");
            assert_eq!(view.inputs[&5].read(cx).value(), "safe");
        })
        .unwrap();
}

fn refresh_bindings(cx: &mut TestAppContext, window: WindowHandle<GoView>) {
    cx.update(|cx| {
        let entries = interaction::key_entries(&window.read(cx).unwrap().view);
        let bindings = entries
            .iter()
            .map(|e| e.load(cx).unwrap())
            .collect::<Vec<_>>();
        cx.clear_key_bindings();
        input::bind_keys(cx);
        cx.bind_keys(bindings);
    });
}
#[test]
fn native_chords_and_context_predicates_use_gpui_keymap() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, _, cx| {
            let root = view.view.snapshot.root.as_mut().unwrap();
            root.interaction.as_mut().unwrap().key_context = "Features".into();
            root.key_bindings[0].keystroke = "cmd-k cmd-s".into();
            root.key_bindings[0].context = "Features".into();
            cx.notify();
        })
        .unwrap();
    refresh_bindings(&mut cx, window);
    draw(&mut cx, window.into());
    cx.simulate_keystrokes(window.into(), "cmd-k");
    assert!(
        !kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind, pb::event::Kind::Action(_)))
    );
    cx.simulate_keystrokes(window.into(), "cmd-s");
    assert!(
        kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind,pb::event::Kind::Action(a) if a.name=="save"))
    );
    window
        .update(&mut cx, |view, _, cx| {
            view.view.snapshot.root.as_mut().unwrap().key_bindings[0].context =
                "MissingContext".into();
            cx.notify();
        })
        .unwrap();
    refresh_bindings(&mut cx, window);
    draw(&mut cx, window.into());
    cx.simulate_keystrokes(window.into(), "cmd-k cmd-s");
    assert!(
        !kinds(&hub)
            .iter()
            .any(|(_, kind)| matches!(kind, pb::event::Kind::Action(_)))
    );
}

#[test]
fn pending_chord_survives_a_snapshot_commit() {
    let (mut cx, window, hub) = setup();
    let mut windows = Windows {
        views: HashMap::from([(
            2,
            (
                pb::Handle {
                    id: 1,
                    generation: 1,
                },
                window,
            ),
        )]),
        resources: Rc::new(RefCell::new(Assets::new(AssetBytes::default()))),
        models: HashMap::new(),
        pending_prompts: Rc::new(std::cell::Cell::new(0)),
        base_keys: Vec::new(),
        installed_keys: Vec::new(),
    };
    cx.update(|cx| {
        cx.clear_key_bindings();
        input::bind_keys(cx);
        windows.base_keys = cx.key_bindings().borrow().bindings().cloned().collect();
    });
    let mut next = window.read_with(&cx, |view, _| view.view.clone()).unwrap();
    next.snapshot.revision = 2;
    next.snapshot.root.as_mut().unwrap().key_bindings[0].keystroke = "cmd-k cmd-s".into();
    cx.update(|cx| {
        NativeBackend {
            windows: &mut windows,
            cx,
            hub: hub.clone(),
        }
        .commit(&next)
        .unwrap()
    });
    draw(&mut cx, window.into());
    cx.simulate_keystrokes(window.into(), "cmd-k");
    assert!(
        !kinds(&hub)
            .iter()
            .any(|(_, k)| matches!(k, pb::event::Kind::Action(_)))
    );
    next.snapshot.revision = 3;
    next.snapshot.root.as_mut().unwrap().kind = Some(pb::element::Kind::Text(pb::Text {
        content: "Updated while chord is pending".into(),
    }));
    cx.update(|cx| {
        NativeBackend {
            windows: &mut windows,
            cx,
            hub: hub.clone(),
        }
        .commit(&next)
        .unwrap()
    });
    draw(&mut cx, window.into());
    cx.simulate_keystrokes(window.into(), "cmd-s");
    assert!(
        kinds(&hub)
            .iter()
            .any(|(_, k)| matches!(k, pb::event::Kind::Action(a) if a.name == "save"))
    );
    let mut invalid = next.clone();
    invalid.snapshot.revision = 4;
    invalid.snapshot.root.as_mut().unwrap().key_bindings[0].context = "(".into();
    cx.update(|cx| {
        assert!(
            NativeBackend {
                windows: &mut windows,
                cx,
                hub: hub.clone()
            }
            .commit(&invalid)
            .is_err()
        )
    });
    assert_eq!(
        window
            .read_with(&cx, |view, _| view.view.snapshot.revision)
            .unwrap(),
        3
    );
}

fn clicked_at(cx: &mut TestAppContext, window: WindowHandle<GoView>, hub: &Hub, x: f32) -> bool {
    let position = gpui::point(px(x), px(10.0));
    window
        .update(cx, |_, window, cx| {
            for down in [true, false] {
                let event = if down {
                    gpui::MouseDownEvent {
                        position,
                        modifiers: Default::default(),
                        button: gpui::MouseButton::Left,
                        click_count: 1,
                        first_mouse: false,
                    }
                    .to_platform_input()
                } else {
                    gpui::MouseUpEvent {
                        position,
                        modifiers: Default::default(),
                        button: gpui::MouseButton::Left,
                        click_count: 1,
                    }
                    .to_platform_input()
                };
                window.dispatch_event(event, cx);
            }
        })
        .unwrap();
    let mut clicked = false;
    while let Ok(message) = hub.receiver.try_recv() {
        clicked |= matches!(message, Message::Click { element: 1, .. });
    }
    clicked
}

#[test]
fn conditional_styles_follow_the_viewport_and_resizes_are_reported() {
    let (mut cx, window, hub) = setup();
    window
        .update(&mut cx, |view, _, cx| {
            view.view.snapshot.root.as_mut().unwrap().conditional_styles =
                vec![pb::ConditionalStyle {
                    max_width: Some(400.0),
                    style: Some(pb::Style {
                        width: Some(pb::Length {
                            value: Some(pb::length::Value::Pixels(50.0)),
                        }),
                        ..Default::default()
                    }),
                    ..Default::default()
                }];
            cx.notify();
        })
        .unwrap();
    draw(&mut cx, window.into());
    // The default test window is wider than 400px, so the target is 200px wide.
    assert!(clicked_at(&mut cx, window, &hub, 100.0));

    cx.simulate_window_resize(window.into(), gpui::size(px(300.0), px(240.0)));
    cx.run_until_parked();
    draw(&mut cx, window.into());
    let mut reported = Vec::new();
    while let Ok(message) = hub.receiver.try_recv() {
        match message {
            Message::WindowResized(handle, viewport) => {
                assert_eq!(handle.id, 1);
                reported.push(viewport);
            }
            Message::ColorSchemeChanged(..) => panic!("a resize reported a color scheme"),
            _ => {}
        }
    }
    let last = reported.last().expect("a resize reports the new size");
    assert_eq!(
        *last,
        pb::Size {
            width: 300.0,
            height: 240.0
        }
    );
    // Narrower than 400px: the refinement shrinks the target to 50px.
    assert!(!clicked_at(&mut cx, window, &hub, 100.0));
    assert!(clicked_at(&mut cx, window, &hub, 25.0));

    // An unchanged viewport is not reported again.
    draw(&mut cx, window.into());
    assert!(
        std::iter::from_fn(|| hub.receiver.try_recv().ok()).all(|message| !matches!(
            message,
            Message::WindowResized(..) | Message::ColorSchemeChanged(..)
        ))
    );
}

#[test]
fn conditions_compare_width_ranges_and_color_scheme() {
    let dark = pb::ColorScheme::Dark as i32;
    let light = pb::ColorScheme::Light as i32;
    let range = pb::ConditionalStyle {
        min_width: Some(640.0),
        max_width: Some(1024.0),
        ..Default::default()
    };
    assert!(!condition_matches(&range, 639.9, light));
    assert!(condition_matches(&range, 640.0, light));
    assert!(condition_matches(&range, 1023.9, dark));
    assert!(!condition_matches(&range, 1024.0, light));
    let dark_only = pb::ConditionalStyle {
        color_scheme: dark,
        ..Default::default()
    };
    assert!(condition_matches(&dark_only, 10.0, dark));
    assert!(!condition_matches(&dark_only, 10.0, light));
}
