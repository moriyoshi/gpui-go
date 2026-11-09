# Reusable Zed editor

The Rust editor crate builds an embeddable rlib with Render, Focusable, and typed Change events. It owns no application loop, transport, windows, or sample UI. Its host workspace member supplies the standalone adapter and registry. The public Go API is in editor/; application content and smoke live in examples/editor/.

Initial local text begins at revision zero without an undo entry. Programmatic edits finalize transaction boundaries. Insert/undo/redo respect read-only state; authoritative replacement is permitted. Closing one host window invalidates its handle without quitting or changing another editor. IDs are never recycled; reopening after the last close works.

The component has no bridge size limit. The host limits text/snapshots to 1 MiB, editors/commands to 64 each, and events to 256. Oversized native buffers produce an error event; a smaller SetText can recover. Event overflow stops the host.

This is an opt-in GPL path, separate from the Apache backend. External Cargo consumers must carry the documented upstream compatibility patches in their workspace root. The host enables util/debug-embed so settings load outside a Zed checkout.

Sources: [component](../../../rust/zed-editor/src/lib.rs), [host](../../../rust/zed-editor/host/src/lib.rs), [embedding guide](../../../rust/zed-editor/README.md), [Go API](../../../editor/editor.go), [smoke](../../../examples/editor/smoke.go).
