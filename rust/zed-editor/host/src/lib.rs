//! Sable/application adapter for the reusable editor component. No demo UI.
use async_channel::{Receiver, Sender};
use gpui::prelude::*;
use gpui::{
    AnyView, AnyWindowHandle, App, Bounds, Context, Entity, Window, WindowBounds, WindowHandle,
    WindowOptions, px, size,
};
use gpui_go_zed_editor::{Change, EditorComponent, EditorOptions, EditorState};
use gpui_go_zed_editor::{
    Clamp as NativeClamp, Position as NativePosition, Selection as NativeSelection,
    WidthTable as NativeWidthTable,
};
use prost::Message;
use std::{
    collections::BTreeMap,
    panic::{AssertUnwindSafe, catch_unwind},
    sync::{
        Arc, OnceLock,
        atomic::{AtomicBool, AtomicU64, Ordering},
    },
};
use tokio::sync::oneshot;

mod pb {
    include!(concat!(env!("OUT_DIR"), "/gpui.editor.v1.rs"));
}
use pb::*;
const OP_COMMAND: u32 = 0x5a454401;
const OP_EVENT: u32 = 0x5a454402;
const MAX_TEXT: usize = 1024 * 1024;
const MAX_EDITORS: usize = 64;
const MAX_REQUEST: usize = MAX_TEXT + 8192;

struct Command {
    request: Request,
    reply: oneshot::Sender<Response>,
}
struct Bridge {
    commands: Sender<Command>,
    receiver: Receiver<Command>,
    events: Sender<Event>,
    event_receiver: Receiver<Event>,
    sequence: AtomicU64,
}
impl Bridge {
    fn new() -> Self {
        let (commands, receiver) = async_channel::bounded(64);
        let (events, event_receiver) = async_channel::bounded(256);
        Self {
            commands,
            receiver,
            events,
            event_receiver,
            sequence: AtomicU64::new(0),
        }
    }
    fn event(&self, editor_id: u64, kind: event::Kind, cx: &mut App) {
        if self
            .events
            .try_send(Event {
                sequence: self.sequence.fetch_add(1, Ordering::Relaxed) + 1,
                editor_id,
                kind: Some(kind),
            })
            .is_err()
        {
            self.stop();
            cx.quit();
        }
    }
    fn stop(&self) {
        self.commands.close();
        self.events.close();
        while let Ok(command) = self.receiver.try_recv() {
            let _ = command.reply.send(failed(
                command.request.request_id,
                command.request.editor_id,
                error(ErrorCode::Closed, "editor host stopped"),
            ));
        }
    }
}
static BRIDGE: OnceLock<Arc<Bridge>> = OnceLock::new();
fn error(code: ErrorCode, message: impl Into<String>) -> Error {
    Error {
        code: code as i32,
        message: message.into(),
    }
}
fn failed(request_id: u64, editor_id: u64, error: Error) -> Response {
    Response {
        request_id,
        editor_id,
        result: Some(response::Result::Error(error)),
    }
}
fn position(value: NativePosition) -> Position {
    Position {
        line: value.line,
        column: value.column,
        character: 0,
        clamp: Clamp::Unspecified as i32,
    }
}
fn native_width_table(value: i32) -> NativeWidthTable {
    match WidthTable::try_from(value).unwrap_or_default() {
        WidthTable::Unspecified => NativeWidthTable::NarrowAmbiguous,
        WidthTable::WideAmbiguous => NativeWidthTable::WideAmbiguous,
        WidthTable::WideSymbols => NativeWidthTable::WideSymbols,
    }
}
fn native_position(value: Option<Position>) -> NativePosition {
    let value = value.unwrap_or_default();
    NativePosition {
        line: value.line,
        column: value.column,
        character: value.character,
        clamp: match value.clamp() {
            Clamp::Unspecified => NativeClamp::None,
            Clamp::Before => NativeClamp::Before,
            Clamp::After => NativeClamp::After,
        },
    }
}
fn state(editor_id: u64, value: EditorState) -> State {
    State {
        editor_id,
        revision: value.revision,
        text: value.text,
        read_only: value.read_only,
        selection: Some(Selection {
            start: Some(position(value.selection.start)),
            end: Some(position(value.selection.end)),
        }),
    }
}
fn bounded_state(editor_id: u64, value: EditorState) -> Result<State, Error> {
    if value.text.len() > MAX_TEXT {
        return Err(error(
            ErrorCode::ResourceExhausted,
            "buffer exceeds 1 MiB bridge limit",
        ));
    }
    Ok(state(editor_id, value))
}

#[unsafe(no_mangle)]
pub extern "C" fn gpui_zed_editor_init() -> i32 {
    catch_unwind(|| {
        let bridge = Arc::new(Bridge::new());
        if BRIDGE.set(bridge.clone()).is_err() {
            return -1;
        }
        let commands = bridge.clone();
        sable::register(OP_COMMAND, move |bytes: Vec<u8>| {
            let bridge = commands.clone();
            async move {
                if bytes.len() > MAX_REQUEST {
                    return Err(b"editor request too large".to_vec());
                }
                let request =
                    Request::decode(bytes.as_slice()).map_err(|e| e.to_string().into_bytes())?;
                if request.request_id == 0 {
                    return Err(b"request_id must be nonzero".to_vec());
                }
                let request_id = request.request_id;
                let editor_id = request.editor_id;
                let (reply, receive) = oneshot::channel();
                if let Err(admission) = bridge.commands.try_send(Command { request, reply }) {
                    let error = match admission {
                        async_channel::TrySendError::Full(_) => {
                            error(ErrorCode::ResourceExhausted, "editor mailbox full")
                        }
                        async_channel::TrySendError::Closed(_) => {
                            error(ErrorCode::Closed, "editor host stopped")
                        }
                    };
                    return Ok(sable::Payload::Bytes(
                        failed(request_id, editor_id, error).encode_to_vec(),
                    ));
                }
                let response = receive.await.unwrap_or_else(|_| {
                    failed(
                        request_id,
                        editor_id,
                        error(ErrorCode::Closed, "editor host stopped"),
                    )
                });
                Ok(sable::Payload::Bytes(response.encode_to_vec()))
            }
        });
        sable::register(OP_EVENT, move |bytes: Vec<u8>| {
            let bridge = bridge.clone();
            async move {
                if bytes.len() > 1024 {
                    return Err(b"invalid event request".to_vec());
                }
                NextEvent::decode(bytes.as_slice()).map_err(|e| e.to_string().into_bytes())?;
                let event = bridge
                    .event_receiver
                    .recv()
                    .await
                    .map_err(|_| b"editor event stream closed".to_vec())?;
                Ok(sable::Payload::Bytes(event.encode_to_vec()))
            }
        });
        0
    })
    .unwrap_or(-2)
}
#[unsafe(no_mangle)]
pub extern "C" fn gpui_zed_editor_stop() {
    let _ = catch_unwind(|| {
        if let Some(bridge) = BRIDGE.get() {
            bridge.stop();
        }
    });
}

#[derive(Clone)]
enum Slot {
    Window(WindowHandle<EditorComponent>),
    Embedded {
        window: AnyWindowHandle,
        component: Entity<EditorComponent>,
    },
}
impl Slot {
    fn window_id(&self) -> gpui::WindowId {
        match self {
            Slot::Window(window) => window.window_id(),
            Slot::Embedded { window, .. } => window.window_id(),
        }
    }
}

/// Finds the native window of a generic view by its ID and generation.
pub type ResolveWindow = fn(u64, u32, &App) -> Option<AnyWindowHandle>;

#[derive(Default)]
struct Registry {
    slots: BTreeMap<u64, Slot>,
    next_id: u64,
    resolve: Option<ResolveWindow>,
}
impl gpui::Global for Registry {}

fn validate_open(value: &Open) -> Result<(), Error> {
    if value.text.len() > MAX_TEXT || value.title.len() > 1024 {
        return Err(error(
            ErrorCode::ResourceExhausted,
            "editor text or title too large",
        ));
    }
    if !value.width.is_finite()
        || !value.height.is_finite()
        || !(100.0..=16384.0).contains(&value.width)
        || !(100.0..=16384.0).contains(&value.height)
    {
        return Err(error(
            ErrorCode::InvalidArgument,
            "window dimensions must be finite and between 100 and 16384",
        ));
    }
    Ok(())
}

fn execute(
    request: Request,
    bridge: &Arc<Bridge>,
    cx: &mut App,
) -> Result<(u64, response::Result), Error> {
    use request::Command::*;
    let id = request.editor_id;
    let command = request
        .command
        .ok_or_else(|| error(ErrorCode::InvalidArgument, "missing command"))?;
    let global = matches!(command, Open(_) | OpenEmbedded(_) | Hello(_) | Quit(_));
    if global != (id == 0) {
        return Err(error(
            ErrorCode::InvalidArgument,
            "invalid editor ID for command",
        ));
    }
    match command {
        Hello(value) => {
            if value.major != 1 {
                return Err(error(
                    ErrorCode::UnsupportedVersion,
                    "unsupported editor protocol version",
                ));
            }
            Ok((
                0,
                response::Result::Capabilities(Capabilities {
                    major: 1,
                    max_text_bytes: MAX_TEXT as u32,
                    max_editors: MAX_EDITORS as u32,
                }),
            ))
        }
        Quit(_) => {
            cx.quit();
            Ok((0, response::Result::Ack(Ack {})))
        }
        Open(value) => {
            validate_open(&value)?;
            if cx.global::<Registry>().slots.len() >= MAX_EDITORS {
                return Err(error(ErrorCode::ResourceExhausted, "too many editors"));
            }
            let id = cx.update_global::<Registry, _>(|registry, _| {
                registry.next_id += 1;
                registry.next_id
            });
            let bounds = Bounds::centered(None, size(px(value.width), px(value.height)), cx);
            let events = bridge.clone();
            let window = cx
                .open_window(
                    WindowOptions {
                        window_bounds: Some(WindowBounds::Windowed(bounds)),
                        ..Default::default()
                    },
                    move |window, cx| {
                        window.set_window_title(&value.title);
                        let component = cx.new(|cx| {
                            EditorComponent::new(
                                EditorOptions {
                                    text: value.text,
                                    read_only: value.read_only,
                                    show_gutter: value.show_gutter,
                                    soft_wrap: value.soft_wrap,
                                    width_table: native_width_table(value.width_table),
                                },
                                window,
                                cx,
                            )
                        });
                        cx.subscribe(&component, move |_, change: &Change, cx| {
                            let kind = match bounded_state(id, change.0.clone()) {
                                Ok(state) => event::Kind::Changed(state),
                                Err(error) => event::Kind::Error(error),
                            };
                            events.event(id, kind, cx);
                        })
                        .detach();
                        component.update(cx, |component, cx| component.focus(window, cx));
                        component
                    },
                )
                .map_err(|e| error(ErrorCode::Internal, e.to_string()))?;
            cx.update_global::<Registry, _>(|registry, _| {
                registry.slots.insert(id, Slot::Window(window));
            });
            cx.activate(true);
            let value = window
                .update(cx, |component, _, cx| component.state(cx))
                .map_err(|e| error(ErrorCode::Internal, e.to_string()))?;
            Ok((id, response::Result::State(bounded_state(id, value)?)))
        }
        OpenEmbedded(value) => {
            if value.text.len() > MAX_TEXT {
                return Err(error(ErrorCode::ResourceExhausted, "editor text too large"));
            }
            if cx.global::<Registry>().slots.len() >= MAX_EDITORS {
                return Err(error(ErrorCode::ResourceExhausted, "too many editors"));
            }
            let resolve = cx.global::<Registry>().resolve.ok_or_else(|| {
                error(
                    ErrorCode::InvalidArgument,
                    "this host cannot embed editors in generic views",
                )
            })?;
            let window = resolve(value.view_id, value.view_generation, cx)
                .ok_or_else(|| error(ErrorCode::NotFound, "view is closed or unknown"))?;
            let id = cx.update_global::<Registry, _>(|registry, _| {
                registry.next_id += 1;
                registry.next_id
            });
            let events = bridge.clone();
            let component = window
                .update(cx, |_, window, cx| {
                    let component = cx.new(|cx| {
                        EditorComponent::new(
                            EditorOptions {
                                text: value.text,
                                read_only: value.read_only,
                                show_gutter: value.show_gutter,
                                soft_wrap: value.soft_wrap,
                                width_table: native_width_table(value.width_table),
                            },
                            window,
                            cx,
                        )
                    });
                    cx.subscribe(&component, move |_, change: &Change, cx| {
                        let kind = match bounded_state(id, change.0.clone()) {
                            Ok(state) => event::Kind::Changed(state),
                            Err(error) => event::Kind::Error(error),
                        };
                        events.event(id, kind, cx);
                    })
                    .detach();
                    component
                })
                .map_err(|_| error(ErrorCode::NotFound, "view is closed or unknown"))?;
            cx.update_global::<Registry, _>(|registry, _| {
                registry.slots.insert(
                    id,
                    Slot::Embedded {
                        window,
                        component: component.clone(),
                    },
                );
            });
            let value = component.update(cx, |component, cx| component.state(cx));
            Ok((id, response::Result::State(bounded_state(id, value)?)))
        }
        command => {
            let slot = cx
                .global::<Registry>()
                .slots
                .get(&id)
                .cloned()
                .ok_or_else(|| error(ErrorCode::NotFound, "editor handle is closed or unknown"))?;
            let closed = || error(ErrorCode::NotFound, "editor window is closed");
            let value = match slot {
                Slot::Window(window) => window
                    .update(cx, |component, window, cx| {
                        apply(id, command, component, window, cx)
                    })
                    .map_err(|_| closed())??,
                Slot::Embedded { window, component } if matches!(command, Close(_)) => {
                    cx.update_global::<Registry, _>(|registry, _| {
                        registry.slots.remove(&id);
                    });
                    drop(component);
                    let _ = window.update(cx, |_, window, _| window.refresh());
                    bridge.event(id, event::Kind::Closed(Ack {}), cx);
                    response::Result::Ack(Ack {})
                }
                Slot::Embedded { window, component } => window
                    .update(cx, |_, window, cx| {
                        component.update(cx, |component, cx| {
                            apply(id, command, component, window, cx)
                        })
                    })
                    .map_err(|_| closed())??,
            };
            Ok((id, value))
        }
    }
}

fn apply(
    id: u64,
    command: request::Command,
    component: &mut EditorComponent,
    window: &mut Window,
    cx: &mut Context<EditorComponent>,
) -> Result<response::Result, Error> {
    use request::Command::*;
    let value = match command {
        Close(_) => {
            window.remove_window();
            return Ok(response::Result::Ack(Ack {}));
        }
        Focus(_) => {
            component.focus(window, cx);
            return Ok(response::Result::Ack(Ack {}));
        }
        GetState(_) => component.state(cx),
        SetText(value) => {
            if value.text.len() > MAX_TEXT {
                return Err(error(
                    ErrorCode::ResourceExhausted,
                    "text exceeds 1 MiB bridge limit",
                ));
            }
            component.set_text(value.text, window, cx)
        }
        InsertText(value) => {
            if component
                .state(cx)
                .text
                .len()
                .saturating_add(value.text.len())
                > MAX_TEXT
            {
                return Err(error(
                    ErrorCode::ResourceExhausted,
                    "text exceeds 1 MiB bridge limit",
                ));
            }
            component
                .insert_text(&value.text, window, cx)
                .map_err(|e| error(ErrorCode::ReadOnly, e.to_string()))?
        }
        Undo(_) => component
            .undo(window, cx)
            .map_err(|e| error(ErrorCode::ReadOnly, e.to_string()))?,
        Redo(_) => component
            .redo(window, cx)
            .map_err(|e| error(ErrorCode::ReadOnly, e.to_string()))?,
        SetReadOnly(value) => component.set_read_only(value.value, cx),
        SetSelection(value) => {
            let value = value.selection.unwrap_or_default();
            component
                .set_selection(
                    NativeSelection {
                        start: native_position(value.start),
                        end: native_position(value.end),
                    },
                    window,
                    cx,
                )
                .map_err(|e| error(ErrorCode::InvalidArgument, e.to_string()))?
        }
        _ => {
            return Err(error(
                ErrorCode::InvalidArgument,
                "invalid component command",
            ));
        }
    };
    Ok(response::Result::State(bounded_state(id, value)?))
}

fn initialize(cx: &mut App) {
    release_channel::init(
        release_channel::AppVersion::load(env!("CARGO_PKG_VERSION"), None, None),
        cx,
    );
    settings::init(cx);
    cx.update_global::<settings::SettingsStore, _>(|store, cx| {
        store.set_user_settings(r#"{"buffer_font_family":"Menlo","ui_font_family":"Helvetica","session":{"restore_unsaved_buffers":false},"git":{"inline_blame":{"enabled":false}},"minimap":{"show":"never"}}"#, cx).expect("install local editor host settings");
    });
    theme_settings::init(theme::LoadThemes::JustBase, cx);
    gpui_go_zed_editor::init(cx);
    gpui_go_zed_editor::bind_default_keys(cx);
    cx.set_quit_mode(gpui::QuitMode::Explicit);
    cx.set_global(Registry::default());
}

/// Starts the editor bridge inside an application that the generic backend owns.
pub fn attach(cx: &mut App, resolve: ResolveWindow) {
    let Some(bridge) = BRIDGE.get().cloned() else {
        return;
    };
    initialize(cx);
    cx.update_global::<Registry, _>(|registry, _| registry.resolve = Some(resolve));
    start_bridge(bridge, cx);
}

/// The view and owning window of an embedded editor.
pub fn embedded_view(id: u64, cx: &App) -> Option<(AnyView, AnyWindowHandle)> {
    match cx.global::<Registry>().slots.get(&id)? {
        Slot::Embedded { window, component } => Some((component.clone().into(), *window)),
        Slot::Window(_) => None,
    }
}

fn start_bridge(bridge: Arc<Bridge>, cx: &mut App) {
    let closed = bridge.clone();
    cx.on_window_closed(move |cx, window_id| {
        let ids: Vec<u64> = cx
            .global::<Registry>()
            .slots
            .iter()
            .filter_map(|(id, slot)| (slot.window_id() == window_id).then_some(*id))
            .collect();
        for id in ids {
            cx.update_global::<Registry, _>(|registry, _| {
                registry.slots.remove(&id);
            });
            closed.event(id, event::Kind::Closed(Ack {}), cx);
        }
    })
    .detach();
    cx.spawn(async move |cx| {
        while let Ok(command) = bridge.receiver.recv().await {
            if command.reply.is_closed() {
                continue;
            }
            let id = command.request.editor_id;
            let request_id = command.request.request_id;
            let result = cx.update(|cx| execute(command.request, &bridge, cx));
            let response = match result {
                Ok((editor_id, result)) => Response {
                    request_id,
                    editor_id,
                    result: Some(result),
                },
                Err(error) => failed(request_id, id, error),
            };
            let _ = command.reply.send(response);
        }
        cx.update(|cx| cx.quit());
    })
    .detach();
}

#[unsafe(no_mangle)]
pub extern "C" fn gpui_zed_editor_run() -> i32 {
    catch_unwind(AssertUnwindSafe(|| {
        #[cfg(target_os = "macos")]
        {
            unsafe extern "C" {
                fn pthread_main_np() -> i32;
            }
            if unsafe { pthread_main_np() } == 0 {
                return -3;
            }
        }
        let Some(bridge) = BRIDGE.get().cloned() else {
            return -1;
        };
        let startup_failed = Arc::new(AtomicBool::new(false));
        let flag = startup_failed.clone();
        gpui_platform::application().run(move |cx| {
            let startup = catch_unwind(AssertUnwindSafe(|| {
                initialize(cx);
                start_bridge(bridge, cx);
            }));
            if startup.is_err() {
                flag.store(true, Ordering::Relaxed);
                gpui_zed_editor_stop();
                cx.quit();
            }
        });
        gpui_zed_editor_stop();
        if startup_failed.load(Ordering::Relaxed) {
            -2
        } else {
            0
        }
    }))
    .unwrap_or_else(|_| {
        gpui_zed_editor_stop();
        -2
    })
}
