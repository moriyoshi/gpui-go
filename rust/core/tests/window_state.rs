use gpui_go_core::{Backend, Headless, Session, View, WindowState, pb::*, validate_snapshot};

fn text(id: u64) -> Element {
    Element {
        id,
        kind: Some(element::Kind::Text(Text {
            content: "hello".into(),
        })),
        ..Default::default()
    }
}
fn open_with(s: &mut Session, root: Element, backend: &mut impl Backend) -> Response {
    s.apply(
        Request {
            request_id: 2,
            session_id: 9,
            body: Some(request::Body::OpenWindow(OpenWindow {
                options: Some(WindowOptions {
                    title: "window state".into(),
                    size: Some(Size {
                        width: 800.0,
                        height: 600.0,
                    }),
                }),
                initial_view: Some(ViewSnapshot {
                    revision: 1,
                    root: Some(root),
                }),
            })),
        },
        backend,
    )
    .0
}
fn opened(r: Response) -> WindowOpened {
    match r.body.unwrap() {
        response::Body::WindowOpened(v) => v,
        other => panic!("open failed: {other:?}"),
    }
}
fn size(width: f32, height: f32) -> Size {
    Size { width, height }
}
fn resize(s: &mut Session, window: Handle, width: f32, height: f32) -> (Response, Vec<Event>) {
    s.apply(
        Request {
            request_id: 3,
            session_id: 9,
            body: Some(request::Body::Command(Command {
                kind: Some(command::Kind::Window(WindowCommand {
                    window: Some(window),
                    operation: WindowOperation::Resize as i32,
                    size: Some(size(width, height)),
                    ..Default::default()
                })),
            })),
        },
        &mut Headless,
    )
}
fn resized(events: &[Event]) -> Vec<Size> {
    events
        .iter()
        .filter_map(|e| match &e.kind {
            Some(event::Kind::WindowResized(r)) => r.viewport.clone(),
            _ => None,
        })
        .collect()
}

#[test]
fn capabilities_advertise_window_state_and_conditional_styles() {
    let caps = Session::new(9).capabilities();
    assert_eq!((caps.major, caps.minor), (1, 4));
    for feature in ["conditional-styles", "window-size", "window-color-scheme"] {
        assert!(caps.features.iter().any(|f| f == feature), "{feature}");
    }
}

#[test]
fn headless_windows_report_their_requested_size_and_resizes() {
    let mut s = Session::new(9);
    let window = opened(open_with(&mut s, text(1), &mut Headless));
    assert_eq!(window.viewport, Some(size(800., 600.)));
    assert_eq!(window.color_scheme, ColorScheme::Light as i32);
    let handle = window.window.unwrap();
    let (response, events) = resize(&mut s, handle.clone(), 400., 300.);
    assert!(matches!(response.body, Some(response::Body::Result(_))));
    assert_eq!(resized(&events), vec![size(400., 300.)]);
    assert_eq!(
        events.len(),
        1,
        "a resize does not report a color scheme change"
    );
    assert_eq!(events[0].window, Some(handle.clone()));
    // An unchanged size reports nothing.
    assert!(resized(&resize(&mut s, handle, 400., 300.).1).is_empty());
}

#[test]
fn size_and_color_scheme_changes_are_separate_deduplicated_events() {
    let mut s = Session::new(9);
    let window = opened(open_with(&mut s, text(1), &mut Headless))
        .window
        .unwrap();
    assert!(s.window_resized(&window, size(800., 600.)).is_none());
    assert!(
        s.color_scheme_changed(&window, ColorScheme::Light)
            .is_none()
    );
    let event = s.color_scheme_changed(&window, ColorScheme::Dark).unwrap();
    assert_eq!(event.window, Some(window.clone()));
    assert_eq!(
        event.kind,
        Some(event::Kind::ColorSchemeChanged(ColorSchemeChanged {
            color_scheme: ColorScheme::Dark as i32
        }))
    );
    assert!(s.color_scheme_changed(&window, ColorScheme::Dark).is_none());
    // A color scheme change leaves the recorded size alone.
    assert!(s.window_resized(&window, size(800., 600.)).is_none());
    assert!(s.window_resized(&window, size(f32::NAN, 1.)).is_none());
    assert!(s.window_resized(&window, size(-1., 1.)).is_none());
    assert!(
        s.color_scheme_changed(&window, ColorScheme::Unspecified)
            .is_none()
    );
    let unknown = Handle {
        id: 999,
        generation: 1,
    };
    assert!(s.window_resized(&unknown, size(1., 1.)).is_none());
    assert!(
        s.color_scheme_changed(&unknown, ColorScheme::Dark)
            .is_none()
    );
}

struct Measured(WindowState);
impl Backend for Measured {
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
    fn window_state(&mut self, _: &View) -> Option<WindowState> {
        Some(self.0.clone())
    }
}

#[test]
fn backends_with_windows_report_their_measured_state_on_open() {
    let mut s = Session::new(9);
    let window = opened(open_with(
        &mut s,
        text(1),
        &mut Measured(WindowState {
            viewport: size(812., 590.),
            color_scheme: ColorScheme::Dark,
        }),
    ));
    assert_eq!(window.viewport, Some(size(812., 590.)));
    assert_eq!(window.color_scheme, ColorScheme::Dark as i32);
}

fn conditional(condition: ConditionalStyle) -> ViewSnapshot {
    ViewSnapshot {
        revision: 1,
        root: Some(Element {
            conditional_styles: vec![condition],
            ..text(1)
        }),
    }
}
fn padded() -> Option<Style> {
    Some(Style {
        opacity: Some(0.5),
        ..Default::default()
    })
}

#[test]
fn conditional_styles_are_validated() {
    let ok = [
        ConditionalStyle {
            min_width: Some(768.),
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            max_width: Some(640.),
            hover_style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            min_width: Some(640.),
            max_width: Some(1024.),
            color_scheme: ColorScheme::Dark as i32,
            style: padded(),
            ..Default::default()
        },
    ];
    for condition in ok {
        assert!(
            validate_snapshot(&conditional(condition.clone())).is_ok(),
            "{condition:?}"
        );
    }
    let bad = [
        ConditionalStyle {
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            min_width: Some(768.),
            ..Default::default()
        },
        ConditionalStyle {
            min_width: Some(1024.),
            max_width: Some(640.),
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            min_width: Some(f32::INFINITY),
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            max_width: Some(-1.),
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            color_scheme: 9,
            style: padded(),
            ..Default::default()
        },
        ConditionalStyle {
            min_width: Some(1.),
            style: Some(Style {
                opacity: Some(2.),
                ..Default::default()
            }),
            ..Default::default()
        },
    ];
    for condition in bad {
        assert!(
            validate_snapshot(&conditional(condition.clone())).is_err(),
            "{condition:?}"
        );
    }
    let many = ViewSnapshot {
        revision: 1,
        root: Some(Element {
            conditional_styles: vec![
                ConditionalStyle {
                    min_width: Some(1.),
                    style: padded(),
                    ..Default::default()
                };
                33
            ],
            ..text(1)
        }),
    };
    assert!(validate_snapshot(&many).is_err());
}
