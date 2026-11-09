use gpui_go_core::{Headless, Session, pb::*};

fn apply(session: &mut Session, body: request::Body) -> response::Body {
    session
        .apply(
            Request {
                request_id: 1,
                session_id: 9,
                body: Some(body),
            },
            &mut Headless,
        )
        .0
        .body
        .unwrap()
}
fn failure(body: response::Body) -> ErrorCode {
    match body {
        response::Body::Error(error) => ErrorCode::try_from(error.code).unwrap(),
        other => panic!("expected error, got {other:?}"),
    }
}
#[test]
fn automation_is_versioned_scoped_and_unavailable_in_normal_backends() {
    let mut session = Session::new(9);
    let response::Body::WindowOpened(opened) = apply(
        &mut session,
        request::Body::OpenWindow(OpenWindow {
            options: Some(WindowOptions {
                title: "test".into(),
                size: Some(Size {
                    width: 500.,
                    height: 300.,
                }),
            }),
            initial_view: Some(ViewSnapshot {
                revision: 1,
                root: Some(Element {
                    id: 1,
                    kind: Some(element::Kind::Text(Text {
                        content: "test".into(),
                    })),
                    ..Default::default()
                }),
            }),
        }),
    ) else {
        panic!("open failed")
    };
    let mut request = AutomationRequest {
        version: 1,
        view: opened.view,
        element_id: 1,
        operation: Some(automation_request::Operation::Inspect(AutomationInspect {})),
    };
    assert_eq!(
        failure(apply(
            &mut session,
            request::Body::Automation(request.clone())
        )),
        ErrorCode::Unsupported
    );
    request.version = 0;
    assert_eq!(
        failure(apply(
            &mut session,
            request::Body::Automation(request.clone())
        )),
        ErrorCode::InvalidArgument
    );
    request.version = 1;
    request.element_id = 99;
    assert_eq!(
        failure(apply(
            &mut session,
            request::Body::Automation(request.clone())
        )),
        ErrorCode::InvalidArgument
    );
    request.element_id = 1;
    request.view.as_mut().unwrap().generation += 1;
    assert_eq!(
        failure(apply(&mut session, request::Body::Automation(request))),
        ErrorCode::StaleHandle
    );
}
