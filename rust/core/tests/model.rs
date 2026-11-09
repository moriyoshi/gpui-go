use gpui_go_core::{Backend, Headless, Session, View, pb::*, validate_snapshot};
use prost::Message;

fn snapshot(revision: u64) -> ViewSnapshot {
    ViewSnapshot {
        revision,
        root: Some(Element {
            id: 1,
            on_click: Some(EventBinding {
                handler_id: 77,
                stop_propagation: true,
            }),
            kind: Some(element::Kind::Text(Text {
                content: "こんにちは GPUI".into(),
            })),
            ..Default::default()
        }),
    }
}
fn request(body: request::Body) -> Request {
    Request {
        request_id: 7,
        session_id: 42,
        body: Some(body),
    }
}
fn open() -> Request {
    request(request::Body::OpenWindow(OpenWindow {
        options: Some(WindowOptions {
            title: "Protocol test".into(),
            size: Some(Size {
                width: 560.,
                height: 420.,
            }),
        }),
        initial_view: Some(snapshot(1)),
    }))
}
fn failure(response: Response) -> ErrorCode {
    match response.body.unwrap() {
        response::Body::Error(e) => ErrorCode::try_from(e.code).unwrap(),
        other => panic!("expected error, got {other:?}"),
    }
}
fn opened(session: &mut Session) -> WindowOpened {
    match session.apply(open(), &mut Headless).0.body.unwrap() {
        response::Body::WindowOpened(window) => window,
        other => panic!("{other:?}"),
    }
}

#[test]
fn session_and_protocol_version_are_checked() {
    let mut session = Session::new(42);
    let mut hello = request(request::Body::Hello(Hello { major: 2, minor: 0 }));
    hello.session_id = 0;
    assert_eq!(
        failure(session.apply(hello, &mut Headless).0),
        ErrorCode::Unsupported
    );
    let mut wrong = open();
    wrong.session_id = 41;
    assert_eq!(
        failure(session.apply(wrong, &mut Headless).0),
        ErrorCode::WrongSession
    );
}

#[test]
fn revisions_and_closed_handles_are_enforced() {
    let mut session = Session::new(42);
    let window = opened(&mut session);
    let entity = window.view.unwrap();
    let update = |revision| {
        request(request::Body::CommitView(CommitView {
            view: Some(entity.clone()),
            snapshot: Some(snapshot(revision)),
        }))
    };
    let (response, events) = session.apply(update(2), &mut Headless);
    assert!(matches!(
        response.body,
        Some(response::Body::ViewCommitted(_))
    ));
    assert_eq!(events[0].revision, 2);
    assert_eq!(
        failure(session.apply(update(2), &mut Headless).0),
        ErrorCode::StaleRevision
    );
    let mut stale = entity.clone();
    stale.generation += 1;
    assert_eq!(
        failure(
            session
                .apply(
                    request(request::Body::CommitView(CommitView {
                        view: Some(stale),
                        snapshot: Some(snapshot(3))
                    })),
                    &mut Headless
                )
                .0
        ),
        ErrorCode::StaleHandle
    );
    let (_, events) = session.apply(
        request(request::Body::CloseWindow(CloseWindow {
            window: window.window,
        })),
        &mut Headless,
    );
    assert!(matches!(events[0].kind, Some(event::Kind::WindowClosed(_))));
    assert_eq!(
        failure(session.apply(update(3), &mut Headless).0),
        ErrorCode::StaleHandle
    );
}

#[test]
fn input_from_obsolete_frames_does_not_invoke_handlers() {
    let mut session = Session::new(42);
    let entity = opened(&mut session).view.unwrap();
    let click = session.click(&entity, 1, 1, Click::default()).unwrap();
    assert_eq!(click.handler_id, 77);
    session.apply(
        request(request::Body::CommitView(CommitView {
            view: Some(entity.clone()),
            snapshot: Some(snapshot(2)),
        })),
        &mut Headless,
    );
    assert!(session.click(&entity, 1, 1, Click::default()).is_none());
    assert!(session.click(&entity, 2, 999, Click::default()).is_none());
}

#[test]
fn invalid_replacement_is_atomic() {
    let mut session = Session::new(42);
    let entity = opened(&mut session).view.unwrap();
    let mut bad = snapshot(2);
    let child = bad.root.clone().unwrap();
    bad.root.as_mut().unwrap().kind = Some(element::Kind::Container(Container {
        children: vec![child],
    }));
    assert_eq!(
        failure(
            session
                .apply(
                    request(request::Body::CommitView(CommitView {
                        view: Some(entity.clone()),
                        snapshot: Some(bad)
                    })),
                    &mut Headless
                )
                .0
        ),
        ErrorCode::InvalidArgument
    );
    assert!(session.click(&entity, 1, 1, Click::default()).is_some());
}

#[test]
fn numeric_limits_depth_and_missing_resource_handles_are_rejected() {
    let mut bad = snapshot(1);
    bad.root.as_mut().unwrap().style = Some(Style {
        font_size: Some(f32::NAN),
        ..Default::default()
    });
    assert!(validate_snapshot(&bad).is_err());
    bad.root.as_mut().unwrap().style = Some(Style {
        align_items: Some(999),
        ..Default::default()
    });
    assert!(validate_snapshot(&bad).is_err());
    bad.root.as_mut().unwrap().style = None;
    bad.root.as_mut().unwrap().kind = Some(element::Kind::Image(Image::default()));
    assert_eq!(
        validate_snapshot(&bad).unwrap_err().code,
        ErrorCode::InvalidArgument as i32
    );
    let mut root = snapshot(1).root.unwrap();
    for id in 2..=60 {
        root = Element {
            id,
            kind: Some(element::Kind::Container(Container {
                children: vec![root],
            })),
            ..Default::default()
        };
    }
    assert!(
        validate_snapshot(&ViewSnapshot {
            revision: 1,
            root: Some(root)
        })
        .is_err()
    );
}

#[test]
fn backend_failure_does_not_install_a_view() {
    struct Failing;
    impl Backend for Failing {
        fn open(&mut self, _: &View) -> Result<(), String> {
            Err("no display".into())
        }
        fn commit(&mut self, _: &View) -> Result<(), String> {
            unreachable!()
        }
        fn close(&mut self, _: &View) -> Result<(), String> {
            unreachable!()
        }
        fn quit(&mut self) {}
    }
    let mut session = Session::new(42);
    assert_eq!(
        failure(session.apply(open(), &mut Failing).0),
        ErrorCode::Internal
    );
    assert!(
        session
            .click(
                &Handle {
                    id: 2,
                    generation: 1
                },
                1,
                1,
                Click::default()
            )
            .is_none()
    );
}

#[test]
fn shutdown_rejects_new_work() {
    let mut session = Session::new(42);
    session.apply(request(request::Body::Quit(Quit::default())), &mut Headless);
    assert_eq!(
        failure(session.apply(open(), &mut Headless).0),
        ErrorCode::Closed
    );
}

#[test]
fn advertised_tree_depth_round_trips_through_the_protobuf_envelope() {
    let mut root = snapshot(1).root.unwrap();
    root.style = Some(Style {
        padding: Some(Edges {
            top: Some(Length {
                value: Some(length::Value::Pixels(12.)),
            }),
            ..Default::default()
        }),
        ..Default::default()
    });
    for id in 2..=gpui_go_core::MAX_TREE_DEPTH as u64 {
        root = Element {
            id,
            kind: Some(element::Kind::Container(Container {
                children: vec![root],
            })),
            ..Default::default()
        };
    }
    let mut request = open();
    let Some(request::Body::OpenWindow(window)) = &mut request.body else {
        unreachable!()
    };
    window.initial_view.as_mut().unwrap().root = Some(root);
    let decoded = Request::decode(request.encode_to_vec().as_slice()).unwrap();
    let response = Session::new(42).apply(decoded, &mut Headless).0;
    assert!(matches!(
        response.body,
        Some(response::Body::WindowOpened(_))
    ));
}

#[test]
fn showcase_styles_validate_and_invalid_hover_is_rejected_atomically() {
    let mut session = Session::new(42);
    let entity = opened(&mut session).view.unwrap();
    let mut next = snapshot(2);
    let root = next.root.as_mut().unwrap();
    root.style = Some(Style {
        overflow_y: Some(Overflow::Scroll as i32),
        overflow_x: Some(Overflow::Hidden as i32),
        cursor: Some(Cursor::Pointer as i32),
        font_weight: Some(600),
        flex_shrink: Some(0.),
        position: Some(Position::Absolute as i32),
        inset: Some(Edges {
            top: Some(Length {
                value: Some(length::Value::Pixels(-4.)),
            }),
            ..Default::default()
        }),
        ..Default::default()
    });
    root.hover_style = Some(Style {
        background: Some(Color {
            red: 0.2,
            green: 0.4,
            blue: 0.6,
            alpha: 0.5,
        }),
        ..Default::default()
    });
    let send = |snapshot| {
        request(request::Body::CommitView(CommitView {
            view: Some(entity.clone()),
            snapshot: Some(snapshot),
        }))
    };
    assert!(matches!(
        session.apply(send(next.clone()), &mut Headless).0.body,
        Some(response::Body::ViewCommitted(_))
    ));
    next.revision = 3;
    next.root
        .as_mut()
        .unwrap()
        .hover_style
        .as_mut()
        .unwrap()
        .font_weight = Some(999);
    assert_eq!(
        failure(session.apply(send(next.clone()), &mut Headless).0),
        ErrorCode::InvalidArgument
    );
    assert!(session.click(&entity, 2, 1, Click::default()).is_some());
    next.root.as_mut().unwrap().hover_style = None;
    next.root
        .as_mut()
        .unwrap()
        .style
        .as_mut()
        .unwrap()
        .overflow_y = Some(999);
    assert_eq!(
        failure(session.apply(send(next), &mut Headless).0),
        ErrorCode::InvalidArgument
    );
}

#[test]
fn input_changes_respect_epochs_acknowledgements_and_disabled_fields() {
    let mut session = Session::new(42);
    let entity = opened(&mut session).view.unwrap();
    let mut next = snapshot(2);
    next.root.as_mut().unwrap().kind = Some(element::Kind::TextInput(TextInput {
        value: "A🙂Z".into(),
        epoch: 2,
        edit_revision: 4,
        on_change: Some(EventBinding {
            handler_id: 99,
            stop_propagation: false,
        }),
        ..Default::default()
    }));
    let send = |snapshot| {
        request(request::Body::CommitView(CommitView {
            view: Some(entity.clone()),
            snapshot: Some(snapshot),
        }))
    };
    session.apply(send(next.clone()), &mut Headless);
    let edit = |epoch, edit_revision| TextChanged {
        value: "日本語🙂".into(),
        epoch,
        edit_revision,
        composing: true,
    };
    assert!(session.text_changed(&entity, 1, edit(1, 99)).is_none());
    assert!(session.text_changed(&entity, 1, edit(2, 4)).is_none());
    let event = session.text_changed(&entity, 1, edit(2, 5)).unwrap();
    assert_eq!(event.handler_id, 99);
    assert_eq!(event.revision, 2);
    next.revision = 3;
    let Some(element::Kind::TextInput(input)) = &mut next.root.as_mut().unwrap().kind else {
        unreachable!()
    };
    input.disabled = true;
    session.apply(send(next), &mut Headless);
    assert!(session.text_changed(&entity, 1, edit(2, 5)).is_none());
    let bad = ViewSnapshot {
        revision: 1,
        root: Some(Element {
            id: 1,
            kind: Some(element::Kind::TextInput(TextInput {
                value: "multiple\nlines".into(),
                ..Default::default()
            })),
            ..Default::default()
        }),
    };
    assert!(validate_snapshot(&bad).is_err());
}
