use gpui_go_core::{Backend, Headless, Session, View, pb::*, validate_snapshot};
fn req(kind: command::Kind) -> Request {
    Request {
        request_id: 1,
        session_id: 9,
        body: Some(request::Body::Command(Command { kind: Some(kind) })),
    }
}
fn command(s: &mut Session, kind: command::Kind) -> (Response, Vec<Event>) {
    s.apply(req(kind), &mut Headless)
}
fn good(r: Response) -> CommandResult {
    match r.body.unwrap() {
        response::Body::Result(v) => v,
        other => panic!("expected result, got {other:?}"),
    }
}
fn code(r: Response) -> ErrorCode {
    match r.body.unwrap() {
        response::Body::Error(v) => ErrorCode::try_from(v.code).unwrap(),
        other => panic!("expected failure, got {other:?}"),
    }
}
fn snapshot(root: Element) -> ViewSnapshot {
    ViewSnapshot {
        revision: 1,
        root: Some(root),
    }
}
fn text(id: u64) -> Element {
    Element {
        id,
        kind: Some(element::Kind::Text(Text {
            content: "hello".into(),
        })),
        ..Default::default()
    }
}
fn open(s: &mut Session, root: Element) -> WindowOpened {
    let (r, _) = s.apply(
        Request {
            request_id: 2,
            session_id: 9,
            body: Some(request::Body::OpenWindow(OpenWindow {
                options: Some(WindowOptions {
                    title: "features".into(),
                    size: Some(Size {
                        width: 800.0,
                        height: 600.0,
                    }),
                }),
                initial_view: Some(snapshot(root)),
            })),
        },
        &mut Headless,
    );
    match r.body.unwrap() {
        response::Body::WindowOpened(v) => v,
        other => panic!("open failed: {other:?}"),
    }
}
#[test]
fn resource_lifetime_is_session_scoped_and_live_references_block_release() {
    let mut s = Session::new(9);
    let h = good(
        command(
            &mut s,
            command::Kind::UploadResource(UploadResource {
                kind: ResourceKind::Svg as i32,
                media_type: "image/svg+xml".into(),
                data: b"<svg/>".to_vec(),
            }),
        )
        .0,
    )
    .resource
    .unwrap();
    let root = Element {
        id: 1,
        kind: Some(element::Kind::Image(Image {
            resource: Some(h.clone()),
        })),
        ..Default::default()
    };
    let w = open(&mut s, root.clone());
    assert_eq!(
        code(
            command(
                &mut s,
                command::Kind::ReleaseResource(ReleaseResource {
                    resource: Some(h.clone())
                })
            )
            .0
        ),
        ErrorCode::InvalidArgument
    );
    s.apply(
        Request {
            request_id: 3,
            session_id: 9,
            body: Some(request::Body::CloseWindow(CloseWindow { window: w.window })),
        },
        &mut Headless,
    );
    good(
        command(
            &mut s,
            command::Kind::ReleaseResource(ReleaseResource {
                resource: Some(h.clone()),
            }),
        )
        .0,
    );
    assert_eq!(
        code(
            command(
                &mut s,
                command::Kind::ReleaseResource(ReleaseResource { resource: Some(h) })
            )
            .0
        ),
        ErrorCode::StaleHandle
    );
    let (r, _) = s.apply(
        Request {
            request_id: 4,
            session_id: 9,
            body: Some(request::Body::OpenWindow(OpenWindow {
                options: Some(WindowOptions {
                    title: "stale".into(),
                    size: Some(Size {
                        width: 10.0,
                        height: 10.0,
                    }),
                }),
                initial_view: Some(snapshot(root)),
            })),
        },
        &mut Headless,
    );
    assert_eq!(code(r), ErrorCode::StaleHandle);
}
#[test]
fn virtual_rows_share_id_validation_and_actions_reject_obsolete_frames() {
    let mut s = Session::new(9);
    let mut row = text(2);
    row.interaction = Some(Interaction {
        focusable: true,
        ..Default::default()
    });
    row.key_bindings.push(KeyBinding {
        keystroke: "cmd-k".into(),
        action: Some(Action {
            name: "test".into(),
            payload: vec![],
        }),
        handler_id: 91,
        ..Default::default()
    });
    let root = Element {
        id: 1,
        kind: Some(element::Kind::UniformList(UniformList {
            items: vec![row.clone()],
            row_height: 30.0,
        })),
        ..Default::default()
    };
    let view = open(&mut s, root.clone()).view.unwrap();
    let action = Action {
        name: "test".into(),
        payload: vec![1, 2],
    };
    let e = s
        .interaction(&view, 1, 2, event::Kind::Action(action.clone()))
        .unwrap();
    assert_eq!(e.handler_id, 91);
    assert_eq!(e.kind, Some(event::Kind::Action(action)));
    assert!(
        s.interaction(
            &view,
            99,
            2,
            event::Kind::Action(Action {
                name: "test".into(),
                payload: vec![]
            })
        )
        .is_none()
    );
    assert!(
        s.interaction(
            &view,
            1,
            2,
            event::Kind::Action(Action {
                name: "unbound".into(),
                payload: vec![]
            })
        )
        .is_none()
    );
    let mut invalid = root;
    let Some(element::Kind::UniformList(list)) = &mut invalid.kind else {
        unreachable!()
    };
    list.items.push(row);
    assert!(validate_snapshot(&snapshot(invalid)).is_err());
}
#[test]
fn model_compare_and_swap_and_unsubscription_preserve_state() {
    let mut s = Session::new(9);
    let initial = good(
        command(
            &mut s,
            command::Kind::CreateModel(CreateModel {
                value: b"one".to_vec(),
            }),
        )
        .0,
    )
    .model
    .unwrap();
    let h = initial.entity.clone();
    good(
        command(
            &mut s,
            command::Kind::ObserveModel(ObserveModel {
                entity: h.clone(),
                handler_id: 55,
            }),
        )
        .0,
    );
    let (r, events) = command(
        &mut s,
        command::Kind::UpdateModel(UpdateModel {
            entity: h.clone(),
            expected_revision: 1,
            value: b"two".to_vec(),
        }),
    );
    assert_eq!(good(r).model.unwrap().revision, 2);
    assert_eq!(events.len(), 1);
    assert_eq!(events[0].handler_id, 55);
    let (r, events) = command(
        &mut s,
        command::Kind::UpdateModel(UpdateModel {
            entity: h.clone(),
            expected_revision: 1,
            value: b"lost".to_vec(),
        }),
    );
    assert_eq!(code(r), ErrorCode::StaleRevision);
    assert!(events.is_empty());
    assert_eq!(
        good(
            command(
                &mut s,
                command::Kind::ReadModel(ModelTarget { entity: h.clone() })
            )
            .0
        )
        .model
        .unwrap()
        .value,
        b"two"
    );
    good(
        command(
            &mut s,
            command::Kind::UnobserveModel(ObserveModel {
                entity: h.clone(),
                handler_id: 55,
            }),
        )
        .0,
    );
    assert!(
        command(
            &mut s,
            command::Kind::UpdateModel(UpdateModel {
                entity: h.clone(),
                expected_revision: 2,
                value: b"three".to_vec()
            })
        )
        .1
        .is_empty()
    );
    good(
        command(
            &mut s,
            command::Kind::ReleaseModel(ModelTarget { entity: h.clone() }),
        )
        .0,
    );
    assert_eq!(
        code(command(&mut s, command::Kind::ReadModel(ModelTarget { entity: h })).0),
        ErrorCode::StaleHandle
    );
}
#[test]
fn invalid_paint_and_interaction_metadata_is_rejected() {
    let mut e = text(1);
    e.interaction = Some(Interaction {
        key_down: 5,
        ..Default::default()
    });
    assert!(validate_snapshot(&snapshot(e.clone())).is_err());
    e.interaction = None;
    e.accessibility_label = Some("accepted label".into());
    assert!(validate_snapshot(&snapshot(e.clone())).is_ok());
    e.style = Some(Style {
        opacity: Some(f32::NAN),
        ..Default::default()
    });
    assert!(validate_snapshot(&snapshot(e)).is_err());
    let path = DrawPath {
        stroke_width: 1.0,
        steps: vec![PathStep {
            verb: PathVerb::Line as i32,
            point: Some(Point { x: 1.0, y: 2.0 }),
            ..Default::default()
        }],
        ..Default::default()
    };
    let e = Element {
        id: 1,
        kind: Some(element::Kind::Drawing(Drawing {
            commands: vec![DrawCommand {
                kind: Some(draw_command::Kind::Path(path)),
            }],
        })),
        ..Default::default()
    };
    assert!(validate_snapshot(&snapshot(e)).is_err());
}
#[test]
fn unavailable_headless_platform_service_is_explicit() {
    let mut s = Session::new(9);
    let window = open(&mut s, text(1)).window;
    assert_eq!(
        code(
            command(
                &mut s,
                command::Kind::Window(WindowCommand {
                    window,
                    operation: WindowOperation::SetTitle as i32,
                    title: "native only".into(),
                    ..Default::default()
                })
            )
            .0
        ),
        ErrorCode::Unsupported
    );
    assert_eq!(
        code(command(&mut s, command::Kind::ClipboardRead(ClipboardRead {})).0),
        ErrorCode::Unsupported
    );
    assert_eq!(
        code(
            command(
                &mut s,
                command::Kind::OpenUrl(OpenUrl {
                    url: "javascript:alert(1)".into()
                })
            )
            .0
        ),
        ErrorCode::InvalidArgument
    );
}
struct FailedBackend;
impl Backend for FailedBackend {
    fn open(&mut self, _: &View) -> Result<(), String> {
        Ok(())
    }
    fn commit(&mut self, _: &View) -> Result<(), String> {
        Ok(())
    }
    fn close(&mut self, _: &View) -> Result<(), String> {
        Ok(())
    }
    fn quit(&mut self) {}
    fn model(&mut self, _: &ModelState, _: bool) -> Result<(), String> {
        Err("not installed".into())
    }
}
#[test]
fn failed_model_creation_is_not_published_or_readable() {
    let mut s = Session::new(9);
    let (r, events) = s.apply(
        req(command::Kind::CreateModel(CreateModel { value: vec![1] })),
        &mut FailedBackend,
    );
    assert_eq!(code(r), ErrorCode::Internal);
    assert!(events.is_empty());
    assert_eq!(
        code(
            command(
                &mut s,
                command::Kind::ReadModel(ModelTarget {
                    entity: Some(Handle {
                        id: 1,
                        generation: 1
                    })
                })
            )
            .0
        ),
        ErrorCode::StaleHandle
    );
}
