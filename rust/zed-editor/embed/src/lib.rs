//! Links the generic GPUI backend with the Zed editor host in one process, so
//! that an editor can appear inside a generic view. GPL-3.0-or-later, and opt-in:
//! the generic backend does not depend on this crate.
use gpui::{AnyView, AnyWindowHandle, App};
use gpui_go_native::embed::{self, EmbedProvider};
use std::rc::Rc;

struct Editors;
impl EmbedProvider for Editors {
    fn view(&self, instance: u64, cx: &App) -> Option<(AnyView, AnyWindowHandle)> {
        gpui_go_zed_editor_host::embedded_view(instance, cx)
    }
}

fn setup(cx: &mut App) {
    gpui_go_zed_editor_host::attach(cx, embed::window_for);
    embed::install(Rc::new(Editors), cx);
}

/// Registers the editor protocol and the application setup. Call this before
/// the generic backend starts.
#[unsafe(no_mangle)]
pub extern "C" fn gpui_zed_embed_register() -> i32 {
    if gpui_go_zed_editor_host::gpui_zed_editor_init() != 0 || !embed::set_setup(setup) {
        return -1;
    }
    0
}
