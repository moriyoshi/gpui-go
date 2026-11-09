//! Extension point for native views owned by an optional host crate.
//!
//! This crate knows nothing about what is embedded. A host links this crate as
//! a library, registers a setup function, and provides views by instance ID.
use gpui::{AnyView, AnyWindowHandle, App, BorrowAppContext, Global};
use gpui_go::gpui_go_core::{features::children, pb};
use std::{
    collections::{HashMap, HashSet},
    rc::Rc,
    sync::OnceLock,
};

pub trait EmbedProvider: 'static {
    /// The view for `instance` and the window that owns it, if it is live.
    fn view(&self, instance: u64, cx: &App) -> Option<(AnyView, AnyWindowHandle)>;
}

struct Registry {
    provider: Rc<dyn EmbedProvider>,
    windows: HashMap<u64, (u32, AnyWindowHandle)>,
}
impl Global for Registry {}

static SETUP: OnceLock<fn(&mut App)> = OnceLock::new();

/// Registers code to run once inside the application, before its key bindings
/// are captured. Returns false if one is already registered.
pub fn set_setup(setup: fn(&mut App)) -> bool {
    SETUP.set(setup).is_ok()
}

/// Enables embedded views. Call this from the registered setup function.
pub fn install(provider: Rc<dyn EmbedProvider>, cx: &mut App) {
    cx.set_global(Registry {
        provider,
        windows: HashMap::new(),
    });
}

/// The native window of a generic view handle, for creating embedded views in it.
pub fn window_for(view: u64, generation: u32, cx: &App) -> Option<AnyWindowHandle> {
    let (stored, window) = cx.try_global::<Registry>()?.windows.get(&view)?;
    (*stored == generation).then_some(*window)
}

pub(crate) fn run_setup(cx: &mut App) {
    if let Some(setup) = SETUP.get() {
        setup(cx);
    }
}

pub(crate) fn supported(cx: &App) -> bool {
    cx.has_global::<Registry>()
}

pub(crate) fn bind_window(view: &pb::Handle, window: AnyWindowHandle, cx: &mut App) {
    if supported(cx) {
        let (id, generation) = (view.id, view.generation);
        cx.update_global::<Registry, _>(|registry, _| {
            registry.windows.insert(id, (generation, window));
        });
    }
}

pub(crate) fn forget_window(view: u64, cx: &mut App) {
    if supported(cx) {
        cx.update_global::<Registry, _>(|registry, _| {
            registry.windows.remove(&view);
        });
    }
}

pub(crate) fn view(instance: u64, cx: &App) -> Option<AnyView> {
    let registry = cx.try_global::<Registry>()?;
    registry.provider.view(instance, cx).map(|(view, _)| view)
}

/// Checks every embedded element before a snapshot is installed. `window` is
/// None for a window's first view, which cannot have an instance in it yet.
pub(crate) fn validate(
    root: &pb::Element,
    window: Option<AnyWindowHandle>,
    cx: &App,
) -> Result<(), String> {
    fn walk(
        element: &pb::Element,
        window: Option<AnyWindowHandle>,
        seen: &mut HashSet<u64>,
        cx: &App,
    ) -> Result<(), String> {
        if let Some(pb::element::Kind::Embedded(embedded)) = &element.kind {
            let window = window.ok_or("a window's first view cannot embed an instance")?;
            let registry = cx
                .try_global::<Registry>()
                .ok_or("no embedded view provider")?;
            let (_, owner) = registry
                .provider
                .view(embedded.instance, cx)
                .ok_or("unknown embedded instance")?;
            if owner != window {
                return Err("embedded instance belongs to another window".into());
            }
            seen.insert(embedded.instance);
        }
        for child in children(element) {
            walk(child, window, seen, cx)?;
        }
        Ok(())
    }
    walk(root, window, &mut HashSet::new(), cx)
}
