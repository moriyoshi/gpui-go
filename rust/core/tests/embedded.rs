use gpui_go_core::{Backend, Headless, Session, View, pb::*};

struct Capable;
impl Backend for Capable {
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
    fn features(&self) -> Vec<String> {
        vec!["embedded-views".into()]
    }
}

fn embedded(id: u64, instance: u64) -> Element {
    Element {
        id,
        kind: Some(element::Kind::Embedded(Embedded { instance })),
        ..Default::default()
    }
}
fn container(children: Vec<Element>) -> ViewSnapshot {
    ViewSnapshot {
        revision: 2,
        root: Some(Element {
            id: 1,
            kind: Some(element::Kind::Container(Container { children })),
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
fn open(session: &mut Session, backend: &mut impl Backend) -> Handle {
    let initial = ViewSnapshot {
        revision: 1,
        ..container(Vec::new())
    };
    let opened = session.apply(
        request(request::Body::OpenWindow(OpenWindow {
            options: Some(WindowOptions {
                title: "Embedded".into(),
                size: Some(Size {
                    width: 560.,
                    height: 420.,
                }),
            }),
            initial_view: Some(initial),
        })),
        backend,
    );
    match opened.0.body.unwrap() {
        response::Body::WindowOpened(window) => window.view.unwrap(),
        other => panic!("{other:?}"),
    }
}
fn commit(
    session: &mut Session,
    backend: &mut impl Backend,
    view: &Handle,
    snapshot: ViewSnapshot,
) -> Option<ErrorCode> {
    let response = session.apply(
        request(request::Body::CommitView(CommitView {
            view: Some(view.clone()),
            snapshot: Some(snapshot),
        })),
        backend,
    );
    match response.0.body.unwrap() {
        response::Body::Error(e) => Some(ErrorCode::try_from(e.code).unwrap()),
        _ => None,
    }
}

#[test]
fn a_backend_without_the_capability_rejects_embedded_views() {
    let mut session = Session::new(42);
    let view = open(&mut session, &mut Headless);
    assert_eq!(
        commit(
            &mut session,
            &mut Headless,
            &view,
            container(vec![embedded(2, 5)])
        ),
        Some(ErrorCode::Unsupported)
    );
}

#[test]
fn embedded_instances_are_nonzero_and_unique() {
    let mut session = Session::new(42);
    let view = open(&mut session, &mut Capable);
    assert_eq!(
        commit(
            &mut session,
            &mut Capable,
            &view,
            container(vec![embedded(2, 0)])
        ),
        Some(ErrorCode::InvalidArgument)
    );
    assert_eq!(
        commit(
            &mut session,
            &mut Capable,
            &view,
            container(vec![embedded(2, 5), embedded(3, 5)])
        ),
        Some(ErrorCode::InvalidArgument)
    );
    assert_eq!(
        commit(
            &mut session,
            &mut Capable,
            &view,
            container(vec![embedded(2, 5), embedded(3, 6)])
        ),
        None
    );
}
