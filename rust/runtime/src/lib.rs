//! Sable transport, bounded foreground mailbox, and acknowledged event queue.
use async_channel::{Receiver, Sender};
use gpui_go_core::{Backend, Headless, MAX_MESSAGE_BYTES, Session, error, pb::*};
use prost::Message as _;
use std::{
    collections::VecDeque,
    panic::{AssertUnwindSafe, catch_unwind},
    sync::{
        Arc, Mutex, OnceLock,
        atomic::{AtomicBool, Ordering},
    },
};
use tokio::sync::{oneshot, watch};

pub const OP_REQUEST: u32 = 0x47505501;
pub const OP_EVENTS: u32 = 0x47505502;
const EVENT_CAPACITY: usize = 1024;

pub enum Message {
    Request(Box<Request>, oneshot::Sender<Response>),
    Click {
        entity: Handle,
        revision: u64,
        element: u64,
        click: Click,
    },
    TextChanged {
        entity: Handle,
        element: u64,
        change: TextChanged,
    },
    WindowClosed(Handle),
    WindowResized(Handle, Size),
    ColorSchemeChanged(Handle, ColorScheme),
    Interaction {
        entity: Handle,
        revision: u64,
        element: u64,
        kind: event::Kind,
    },
    Action {
        entity: Handle,
        element: u64,
        action: Action,
    },
    PlatformResult {
        handler_id: u64,
        result: PathResult,
    },
}

struct Events {
    queue: VecDeque<Event>,
    sequence: u64,
    acknowledged: u64,
    delivered: u64,
    closed: Option<ProtocolError>,
}

fn is_window_state(event: &Event) -> bool {
    matches!(
        event.kind,
        Some(event::Kind::WindowResized(_) | event::Kind::ColorSchemeChanged(_))
    )
}

pub struct Hub {
    pub session_id: u64,
    pub sender: Sender<Message>,
    pub receiver: Receiver<Message>,
    events: Mutex<Events>,
    changed: watch::Sender<u64>,
    pulling: AtomicBool,
    running: AtomicBool,
}

impl Hub {
    pub fn new(session_id: u64) -> Arc<Self> {
        let (sender, receiver) = async_channel::bounded(256);
        let (changed, _) = watch::channel(0);
        Arc::new(Self {
            session_id,
            sender,
            receiver,
            events: Mutex::new(Events {
                queue: VecDeque::new(),
                sequence: 0,
                acknowledged: 0,
                delivered: 0,
                closed: None,
            }),
            changed,
            pulling: AtomicBool::new(false),
            running: AtomicBool::new(false),
        })
    }

    pub fn begin(&self) -> bool {
        !self.running.swap(true, Ordering::AcqRel)
    }

    /// Published event watermark used by the opt-in native E2E barrier.
    pub fn event_sequence(&self) -> u64 {
        self.events.lock().unwrap().sequence
    }

    pub fn publish(&self, mut event: Event) -> Result<(), ProtocolError> {
        if event.encoded_len() + 72 > MAX_MESSAGE_BYTES {
            return Err(error(
                ErrorCode::InvalidArgument,
                "event exceeds message limit",
            ));
        }
        let mut events = self.events.lock().unwrap();
        if let Some(error) = &events.closed {
            return Err(error.clone());
        }
        // Window size and color scheme are state, not history: a newer report
        // replaces an undelivered one of the same kind for the same window, so
        // a live resize cannot fill the queue.
        if is_window_state(&event) {
            let delivered = events.delivered;
            let kind = event.kind.as_ref().map(std::mem::discriminant);
            events.queue.retain(|queued| {
                queued.sequence <= delivered
                    || !is_window_state(queued)
                    || queued.kind.as_ref().map(std::mem::discriminant) != kind
                    || queued.window != event.window
            });
        }
        if events.queue.len() == EVENT_CAPACITY {
            let failure = error(
                ErrorCode::Backpressure,
                "event queue overflow; application stopped",
            );
            events.closed = Some(failure.clone());
            drop(events);
            self.changed.send_modify(|n| *n = n.wrapping_add(1));
            return Err(failure);
        }
        events.sequence += 1;
        event.sequence = events.sequence;
        events.queue.push_back(event);
        drop(events);
        self.changed.send_modify(|n| *n = n.wrapping_add(1));
        Ok(())
    }

    pub fn finish(&self, failure: ProtocolError) {
        self.sender.close();
        // Complete requests already accepted by Sable before runtime teardown.
        while let Ok(message) = self.receiver.try_recv() {
            if let Message::Request(request, reply) = message {
                let _ = reply.send(failed(request.request_id, failure.clone()));
            }
        }
        self.events.lock().unwrap().closed.get_or_insert(failure);
        self.changed.send_modify(|n| *n = n.wrapping_add(1));
    }

    pub fn input(&self, message: Message) -> Result<(), ProtocolError> {
        self.sender.try_send(message).map_err(|_| {
            error(
                ErrorCode::Backpressure,
                "foreground mailbox is full or closed",
            )
        })
    }

    async fn request(&self, request: Request) -> Response {
        let request_id = request.request_id;
        if matches!(request.body, Some(request::Body::NextEvents(_))) {
            return failed(
                request_id,
                error(ErrorCode::InvalidArgument, "use OP_EVENTS for NextEvents"),
            );
        }
        let (tx, rx) = oneshot::channel();
        match self
            .sender
            .try_send(Message::Request(Box::new(request), tx))
        {
            Ok(()) => rx.await.unwrap_or_else(|_| {
                failed(
                    request_id,
                    error(ErrorCode::Closed, "foreground dispatcher stopped"),
                )
            }),
            Err(async_channel::TrySendError::Full(_)) => failed(
                request_id,
                error(ErrorCode::Backpressure, "foreground mailbox is full"),
            ),
            Err(async_channel::TrySendError::Closed(_)) => failed(
                request_id,
                error(ErrorCode::Closed, "application is closed"),
            ),
        }
    }

    async fn next_events(&self, request: Request) -> Response {
        let request_id = request.request_id;
        if request_id == 0 {
            return failed(
                request_id,
                error(ErrorCode::InvalidArgument, "request_id must be nonzero"),
            );
        }
        if request.session_id != self.session_id {
            return failed(
                request_id,
                error(ErrorCode::WrongSession, "invalid event request session"),
            );
        }
        let Some(request::Body::NextEvents(next)) = request.body else {
            return failed(
                request_id,
                error(ErrorCode::InvalidArgument, "expected NextEvents"),
            );
        };
        if next.max_events == 0 || next.max_events > 256 {
            return failed(
                request_id,
                error(ErrorCode::InvalidArgument, "max_events must be in [1, 256]"),
            );
        }
        if self.pulling.swap(true, Ordering::AcqRel) {
            return failed(
                request_id,
                error(
                    ErrorCode::Backpressure,
                    "only one event pull may be pending",
                ),
            );
        }
        struct PullGuard<'a>(&'a AtomicBool);
        impl Drop for PullGuard<'_> {
            fn drop(&mut self) {
                self.0.store(false, Ordering::Release);
            }
        }
        let _guard = PullGuard(&self.pulling);
        let mut changed = self.changed.subscribe();
        loop {
            {
                let mut events = self.events.lock().unwrap();
                if next.after_sequence < events.acknowledged
                    || next.after_sequence > events.delivered
                {
                    return failed(
                        request_id,
                        error(
                            ErrorCode::InvalidArgument,
                            "event cursor is outside delivered range",
                        ),
                    );
                }
                events.acknowledged = next.after_sequence;
                while events
                    .queue
                    .front()
                    .is_some_and(|event| event.sequence <= next.after_sequence)
                {
                    events.queue.pop_front();
                }
                if !events.queue.is_empty() {
                    // Text edits carry complete UTF-8 values. Limit bytes as
                    // well as event count so a batch stays inside the wire cap.
                    let mut bytes = 64;
                    let batch: Vec<_> = events
                        .queue
                        .iter()
                        .take(next.max_events as usize)
                        .take_while(|event| {
                            bytes += event.encoded_len() + 8;
                            bytes <= MAX_MESSAGE_BYTES
                        })
                        .cloned()
                        .collect();
                    events.delivered = events.delivered.max(batch.last().unwrap().sequence);
                    return Response {
                        request_id,
                        body: Some(response::Body::Events(EventBatch { events: batch })),
                    };
                }
                if let Some(failure) = &events.closed {
                    return failed(request_id, failure.clone());
                }
            }
            if changed.changed().await.is_err() {
                return failed(request_id, error(ErrorCode::Closed, "event queue stopped"));
            }
        }
    }

    /// Execute one mailbox entry on the UI thread. Canceled requests that have
    /// not started are skipped; cancellation racing commitment is best-effort.
    pub fn dispatch(
        &self,
        message: Message,
        session: &mut Session,
        backend: &mut impl Backend,
    ) -> bool {
        let emitted = match message {
            Message::Request(request, reply) => {
                if reply.is_closed() {
                    return true;
                }
                let (response, emitted) = session.apply(*request, backend);
                let _ = reply.send(response);
                emitted
            }
            Message::Click {
                entity,
                revision,
                element,
                click,
            } => session
                .click(&entity, revision, element, click)
                .into_iter()
                .collect(),
            Message::WindowClosed(window) => session.window_closed(&window).into_iter().collect(),
            Message::WindowResized(window, viewport) => session
                .window_resized(&window, viewport)
                .into_iter()
                .collect(),
            Message::ColorSchemeChanged(window, color_scheme) => session
                .color_scheme_changed(&window, color_scheme)
                .into_iter()
                .collect(),
            Message::Interaction {
                entity,
                revision,
                element,
                kind,
            } => session
                .interaction(&entity, revision, element, kind)
                .into_iter()
                .collect(),
            Message::Action {
                entity,
                element,
                action,
            } => session
                .native_action(&entity, element, action)
                .into_iter()
                .collect(),
            Message::PlatformResult { handler_id, result } => vec![Event {
                handler_id,
                kind: Some(event::Kind::PathResult(result)),
                ..Default::default()
            }],
            Message::TextChanged {
                entity,
                element,
                change,
            } => session
                .text_changed(&entity, element, change)
                .into_iter()
                .collect(),
        };
        for event in emitted {
            if self.publish(event).is_err() {
                backend.quit();
                return false;
            }
        }
        !session.is_closed()
    }
}

pub fn failed(request_id: u64, failure: ProtocolError) -> Response {
    Response {
        request_id,
        body: Some(response::Body::Error(failure)),
    }
}

static HUB: OnceLock<Arc<Hub>> = OnceLock::new();
pub fn hub() -> Option<Arc<Hub>> {
    HUB.get().cloned()
}

fn decode(bytes: &[u8]) -> Result<Request, Vec<u8>> {
    if bytes.len() > MAX_MESSAGE_BYTES {
        return Err(b"gpui: message size limit exceeded".to_vec());
    }
    Request::decode(bytes).map_err(|e| format!("gpui: invalid protobuf: {e}").into_bytes())
}

/// Initialize once per process, before sable.Init. No Rust panic crosses C.
#[unsafe(no_mangle)]
pub extern "C" fn gpui_go_init() -> i32 {
    catch_unwind(|| {
        if HUB.get().is_some() {
            return -1;
        }
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap();
        let id = (now.as_nanos() as u64 ^ (std::process::id() as u64).rotate_left(32)).max(1);
        let hub = Hub::new(id);
        if HUB.set(hub.clone()).is_err() {
            return -1;
        }
        let requests = hub.clone();
        sable::register(OP_REQUEST, move |bytes: Vec<u8>| {
            let hub = requests.clone();
            async move {
                let request = decode(&bytes)?;
                let response = hub.request(request).await;
                Ok(sable::Payload::Bytes(response.encode_to_vec()))
            }
        });
        sable::register(OP_EVENTS, move |bytes: Vec<u8>| {
            let hub = hub.clone();
            async move {
                let request = decode(&bytes)?;
                let response = hub.next_events(request).await;
                Ok(sable::Payload::Bytes(response.encode_to_vec()))
            }
        });
        0
    })
    .unwrap_or(-2)
}

#[unsafe(no_mangle)]
pub extern "C" fn gpui_go_run_headless() -> i32 {
    catch_unwind(AssertUnwindSafe(|| {
        let Some(hub) = hub() else {
            return -1;
        };
        if !hub.begin() {
            return -1;
        }
        let mut session = Session::new(hub.session_id);
        let mut backend = Headless;
        while let Ok(message) = hub.receiver.recv_blocking() {
            if !hub.dispatch(message, &mut session, &mut backend) {
                break;
            }
        }
        hub.finish(error(ErrorCode::Closed, "application exited"));
        0
    }))
    .unwrap_or_else(|_| {
        if let Some(hub) = hub() {
            hub.finish(error(ErrorCode::Internal, "foreground dispatcher panicked"));
        }
        -2
    })
}

/// An out-of-band stop wakes startup failures and pending event consumers.
#[unsafe(no_mangle)]
pub extern "C" fn gpui_go_stop() {
    let _ = catch_unwind(|| {
        if let Some(hub) = hub() {
            hub.finish(error(ErrorCode::Closed, "application stopped"));
        }
    });
}

/// Headless automation input. This enters the same foreground input mailbox as
/// a native click listener; it does not synthesize OS input or bypass validation.
#[unsafe(no_mangle)]
pub extern "C" fn gpui_go_headless_click(
    entity: u64,
    generation: u32,
    revision: u64,
    element: u64,
) -> i32 {
    catch_unwind(|| {
        let Some(hub) = hub() else {
            return -1;
        };
        match hub.input(Message::Click {
            entity: Handle {
                id: entity,
                generation,
            },
            revision,
            element,
            click: Click {
                button: MouseButton::Left as i32,
                click_count: 1,
                ..Default::default()
            },
        }) {
            Ok(()) => 0,
            Err(_) => -1,
        }
    })
    .unwrap_or(-2)
}

// Keep the protocol namespace available to native backends without two versions.
pub use gpui_go_core;

/// Headless text-change input enters the same foreground mailbox as the editor.
/// # Safety
/// `ptr` must reference `len` readable bytes for this call. The bytes are copied
/// synchronously; no foreign memory is retained.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn gpui_go_headless_text_change(
    entity: u64,
    generation: u32,
    element: u64,
    epoch: u64,
    edit_revision: u64,
    ptr: *const u8,
    len: usize,
) -> i32 {
    catch_unwind(|| {
        if len > gpui_go_core::text::MAX_INPUT_BYTES || (ptr.is_null() && len != 0) {
            return -1;
        }
        let bytes = if len == 0 {
            &[]
        } else {
            unsafe { std::slice::from_raw_parts(ptr, len) }
        };
        let Ok(value) = std::str::from_utf8(bytes) else {
            return -1;
        };
        let Some(hub) = hub() else {
            return -1;
        };
        match hub.input(Message::TextChanged {
            entity: Handle {
                id: entity,
                generation,
            },
            element,
            change: TextChanged {
                value: value.to_owned(),
                epoch,
                edit_revision,
                composing: false,
            },
        }) {
            Ok(()) => 0,
            Err(_) => -1,
        }
    })
    .unwrap_or(-2)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn next(after_sequence: u64) -> Request {
        Request {
            request_id: 7,
            session_id: 42,
            body: Some(request::Body::NextEvents(NextEvents {
                max_events: 128,
                after_sequence,
            })),
        }
    }

    #[tokio::test]
    async fn unacknowledged_events_are_replayed_and_acknowledged_events_are_removed() {
        let hub = Hub::new(42);
        hub.publish(Event {
            kind: Some(event::Kind::ViewUpdated(ViewUpdated::default())),
            ..Default::default()
        })
        .unwrap();
        let first = hub.next_events(next(0)).await;
        let replay = hub.next_events(next(0)).await;
        assert_eq!(first, replay);
        let response::Body::Events(batch) = first.body.unwrap() else {
            panic!()
        };
        assert_eq!(batch.events[0].sequence, 1);
        hub.finish(error(ErrorCode::Closed, "done"));
        let result = hub.next_events(next(1)).await;
        assert!(
            matches!(result.body, Some(response::Body::Error(ProtocolError { code, .. })) if code == ErrorCode::Closed as i32)
        );
        assert!(hub.events.lock().unwrap().queue.is_empty());
    }

    #[tokio::test]
    async fn canceled_event_pull_releases_consumer_and_does_not_lose_events() {
        let hub = Hub::new(42);
        let pull = {
            let hub = hub.clone();
            tokio::spawn(async move { hub.next_events(next(0)).await })
        };
        tokio::task::yield_now().await;
        assert!(hub.pulling.load(Ordering::Acquire));
        pull.abort();
        let _ = pull.await;
        assert!(!hub.pulling.load(Ordering::Acquire));
        hub.publish(Event {
            kind: Some(event::Kind::Click(Click::default())),
            ..Default::default()
        })
        .unwrap();
        assert!(matches!(
            hub.next_events(next(0)).await.body,
            Some(response::Body::Events(_))
        ));
    }

    #[tokio::test]
    async fn pending_pull_wakes_on_shutdown_and_bad_cursors_fail() {
        let hub = Hub::new(42);
        assert!(matches!(
            hub.next_events(next(1)).await.body,
            Some(response::Body::Error(_))
        ));
        let pull = {
            let hub = hub.clone();
            tokio::spawn(async move { hub.next_events(next(0)).await })
        };
        tokio::task::yield_now().await;
        hub.finish(error(ErrorCode::Closed, "done"));
        assert!(matches!(
            pull.await.unwrap().body,
            Some(response::Body::Error(_))
        ));
    }

    #[test]
    fn canceled_queued_requests_are_skipped() {
        let hub = Hub::new(42);
        let (reply, result) = oneshot::channel();
        drop(result);
        let mut session = Session::new(42);
        assert!(hub.dispatch(
            Message::Request(
                Box::new(Request {
                    request_id: 1,
                    session_id: 42,
                    body: Some(request::Body::Quit(Quit::default()))
                }),
                reply
            ),
            &mut session,
            &mut Headless
        ));
        assert!(!session.is_closed());
    }

    #[tokio::test]
    async fn text_batches_stay_below_the_message_limit_without_losing_events() {
        let hub = Hub::new(42);
        for _ in 0..100 {
            hub.publish(Event {
                kind: Some(event::Kind::TextChanged(TextChanged {
                    value: "x".repeat(gpui_go_core::text::MAX_INPUT_BYTES),
                    ..Default::default()
                })),
                ..Default::default()
            })
            .unwrap();
        }
        let first = hub.next_events(next(0)).await;
        assert!(first.encoded_len() <= MAX_MESSAGE_BYTES);
        let Some(response::Body::Events(first)) = first.body else {
            panic!()
        };
        assert!(!first.events.is_empty() && first.events.len() < 100);
        let cursor = first.events.last().unwrap().sequence;
        let second = hub.next_events(next(cursor)).await;
        assert!(second.encoded_len() <= MAX_MESSAGE_BYTES);
        let Some(response::Body::Events(second)) = second.body else {
            panic!()
        };
        assert_eq!(first.events.len() + second.events.len(), 100);
    }

    #[test]
    fn overflow_is_explicit_and_mailbox_admission_is_bounded() {
        let hub = Hub::new(42);
        for _ in 0..EVENT_CAPACITY {
            hub.publish(Event::default()).unwrap();
        }
        assert_eq!(
            hub.publish(Event::default()).unwrap_err().code,
            ErrorCode::Backpressure as i32
        );
        for _ in 0..256 {
            hub.input(Message::WindowClosed(Handle::default())).unwrap();
        }
        assert!(hub.input(Message::WindowClosed(Handle::default())).is_err());
    }

    fn on(window: u64, kind: event::Kind) -> Event {
        Event {
            window: Some(Handle {
                id: window,
                generation: 1,
            }),
            kind: Some(kind),
            ..Default::default()
        }
    }
    fn resized(window: u64, width: f32) -> Event {
        on(
            window,
            event::Kind::WindowResized(WindowResized {
                viewport: Some(Size {
                    width,
                    height: 100.,
                }),
            }),
        )
    }
    fn scheme(window: u64, color_scheme: ColorScheme) -> Event {
        on(
            window,
            event::Kind::ColorSchemeChanged(ColorSchemeChanged {
                color_scheme: color_scheme as i32,
            }),
        )
    }
    fn summary(events: &[Event]) -> Vec<String> {
        events
            .iter()
            .map(|e| {
                let window = e.window.as_ref().map_or(0, |w| w.id);
                match &e.kind {
                    Some(event::Kind::WindowResized(r)) => {
                        format!("{window}:{}", r.viewport.as_ref().unwrap().width)
                    }
                    Some(event::Kind::ColorSchemeChanged(c)) => {
                        format!("{window}:scheme{}", c.color_scheme)
                    }
                    _ => format!("{window}:other"),
                }
            })
            .collect()
    }

    #[tokio::test]
    async fn undelivered_window_state_is_replaced_per_kind_and_window() {
        let hub = Hub::new(42);
        // A resize storm far beyond the queue capacity keeps one event per window.
        for width in 0..(EVENT_CAPACITY * 4) {
            hub.publish(resized(1, width as f32)).unwrap();
        }
        hub.publish(scheme(1, ColorScheme::Dark)).unwrap();
        hub.publish(Event::default()).unwrap();
        hub.publish(resized(2, 5.)).unwrap();
        hub.publish(resized(1, 7.)).unwrap();
        hub.publish(scheme(1, ColorScheme::Light)).unwrap();
        let Some(response::Body::Events(first)) = hub.next_events(next(0)).await.body else {
            panic!()
        };
        // The latest of each kind for window 1 follows the unrelated event, and
        // a color scheme change never replaces a resize or the reverse.
        assert_eq!(
            summary(&first.events),
            ["0:other", "2:5", "1:7", "1:scheme1"]
        );
        // Delivered but unacknowledged reports are never rewritten, so a
        // retried pull sees the same batch; newer reports queue behind them.
        hub.publish(resized(1, 9.)).unwrap();
        let Some(response::Body::Events(retry)) = hub.next_events(next(0)).await.body else {
            panic!()
        };
        assert_eq!(
            summary(&retry.events),
            ["0:other", "2:5", "1:7", "1:scheme1", "1:9"]
        );
    }
}
