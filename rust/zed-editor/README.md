# Reusable Zed editor component

`gpui-go-zed-editor` is a [GPL-3.0-or-later](LICENSE-GPL) Rust `rlib` wrapping Zed's `editor::Editor`. It provides `EditorComponent`, `EditorOptions`, `EditorState`, and `Change`. It owns a local buffer and persistent native editing state. There is no application loop, window creation, Sable dependency, sample content, toolbar, or status UI in this crate.

The component and its `host/` crate are covered by GPL-3.0-or-later. The full license text is provided in [COPYING](COPYING), identical to the retained [LICENSE-GPL](LICENSE-GPL). Applications distributed with these components linked in must comply with the GPL's requirements for the combined work, including corresponding source. The [Go editor client](../../editor/README.md) and protobuf schemas are covered by the project's default [Apache-2.0 license](../../LICENSE); see the [project licensing scope](../../README.md#license).

`host/` is a separate workspace member that builds `libgpui_go_zed_editor_host.a`. It supplies the foreground command mailbox, protobuf adapter, editor-ID registry, window lifecycle, and standalone application setup used by the public Go `editor` package. The Go application is in `../../examples/editor`.

## Embed in a Rust GPUI view

Use `gpui_go_zed_editor::gpui` to access the matching GPUI version. Initialize Zed's release channel, settings, and theme in your application, then call `gpui_go_zed_editor::init(cx)` once. The component does not replace application settings. The host's `initialize` function is a reference for a standalone local-buffer setup. Development hosts loading Zed settings outside its checkout need `util` with `debug-embed`, as the supplied host configures.

Create and retain the component entity in a parent view:

```rust
use gpui_go_zed_editor::{Change, EditorComponent, EditorOptions};
use gpui_go_zed_editor::gpui::{self, prelude::*};

struct DocumentView {
    editor: gpui::Entity<EditorComponent>,
    _changes: gpui::Subscription,
}

impl DocumentView {
    fn new(window: &mut gpui::Window, cx: &mut gpui::Context<Self>) -> Self {
        let editor = cx.new(|cx| EditorComponent::new(
            EditorOptions { text: "Hello, 日本語".into(), ..Default::default() },
            window,
            cx,
        ));
        let changes = cx.subscribe(&editor, |_, _, change: &Change, _| {
            // Application state, persistence, validation, or other reactions.
            println!("revision {}", change.0.revision);
        });
        Self { editor, _changes: changes }
    }
}

impl gpui::Render for DocumentView {
    fn render(&mut self, _: &mut gpui::Window, _: &mut gpui::Context<Self>) -> impl gpui::IntoElement {
        gpui::div().size_full().flex().flex_col()
            .child("Application-owned header")
            .child(gpui::div().flex_1().min_h_0().child(self.editor.clone()))
    }
}
```

The component implements `Render`, `Focusable`, and `EventEmitter<Change>`. Construction leaves focus under the host's control. Call `focus` or use its focus handle. Retain subscriptions for as long as you want notifications.

Call methods through `entity.update(cx, |component, cx| ...)`. `state` synchronizes and returns an owned text/revision/read-only snapshot. `set_text` performs an authoritative replacement, while `insert_text`, `undo`, and `redo` return `ReadOnly` when editing is disabled. Each programmatic text edit is a separate undo transaction. Initial text starts at revision zero with no undo entry. `set_selection` replaces all selections with one, scrolls it into view, works in read-only mode and returns `InvalidSelection` for a reversed range or a position outside the buffer or inside a character. `set_read_only` emits a change when the value changes. Native text changes also emit `Change`; no transport limit applies to this Rust component.

`native_editor()` exposes the underlying Zed entity for native extensions. It is an advanced escape hatch: call `state` after changes that do not emit Zed's buffer-edited event. Key bindings are application policy; optionally call `bind_default_keys(cx)` to install the supplied basic macOS editing shortcuts.

## Dependency configuration

The workspace lockfile and patches pin the Zed revision used by this repository. Cargo does not propagate a dependency's `[patch.crates-io]` table into an external consuming workspace. External Rust applications must copy the three overrides from this crate's `Cargo.toml` into their own workspace root. These are the async-process and async-task forks required by Zed and the tree-sitter-language override that unifies grammar types. Keep the exact revisions when doing so, and resolve tree-sitter-language to the patched version rather than an older registry lock entry.

The direct version constraints on asset and Mermaid dependencies match Zed's lockfile and avoid incompatible upstream patch releases. The host uses `util/debug-embed` to embed settings assets in development builds. No global Cargo configuration or shared upstream source changes are needed.

The component supports local plain-text editing. Syntax grammars, language servers, project/workspace services, file persistence, and a full Zed keymap are responsibilities of the consuming application.
