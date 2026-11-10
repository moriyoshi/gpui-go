# Persistent GPUI node graph

`github.com/moriyoshi/gpui-go/ui` binds the [reactive graph](../reactive/README.md) to the existing GPUI client. Components construct nodes once. Reactive bindings change their properties, and a document publishes one complete protobuf snapshot after a settled batch when the snapshot differs.

Inside a `host.Run` application callback, with `app *client.Client`:

```go
graph := reactive.NewGraph()
defer graph.Dispose()
document := ui.New(graph.Root(), app)
owner := document.Scope()
count := reactive.NewSignal(owner, 0)
label := document.BindText(owner, func() string {
    return fmt.Sprintf("Count: %d", count.Get())
}).Name("count")
button := document.Div(owner, document.Text(owner, "Increment"))
button.Accessible("Increment", &scene.Accessibility{
    Role: scene.RoleButton,
}).Focusable(0).OnClick(func(ctx context.Context, event *client.Event) error {
    return count.Update(func(value int) int { return value + 1 })
})
root := document.Div(owner, label, button)
mount, err := document.OpenWindow(ctx, client.WindowOptions{
    Title: "Reactive counter", Width: 500, Height: 300,
}, root)
if err != nil { return err }
defer mount.Close(context.Background())
return app.DispatchEvents(ctx)
```

`Div`, `Text`, `BindText`, `Input`, `Image`/`BindImage`, `Canvas`/`BindCanvas` and `UniformList` create nodes. A uniform list takes children like a `Div`, and each serialized child, including the rows of a `For` or `Show` range, becomes one virtualized row. `Fragment` is a transparent range without an additional native layout container. `SetChildren` validates document ownership, cycles, duplicate children and existing parents before modifying a node; omitted children are disposed. Node IDs remain stable across property updates. `Name` registers an application selector retrievable through `Document.Element`, including for `uitest/gpui.Application`.

`Style`/`BindStyle`, `Hover`, `TWClass`/`BindTWClass`, `Accessible`/`BindAccessibility`, `Focusable` and `OnClick` configure concrete elements. `TWClass` takes a Tailwind-style utility list compiled by [tw](../tw/README.md) with the theme set by `Document.UseTheme`; `Style` and `Hover` refine the compiled style and hover refinement field by field, and also win over breakpoint and `dark:` utilities. `Document.WindowSize` and `Document.ColorScheme` return the mounted window's size and color scheme; each is tracked like a separate signal read, so a binding follows only the one it reads. Fragment nodes only manage children. Use `scene.Style`, `scene.Accessibility`, concise scene enum constants and `client.Event` callbacks. These public aliases retain wire identity and require no generated-binding imports. Handlers execute later in Go event dispatch and are automatically batched; no Go closure is called during Rust rendering. Node disposal removes owned computations, descendants, names and registered handlers.

After mounting, group imperative node operations (such as `SetChildren`, `Style` or `Dispose`) in `graph.Batch`, or call `graph.Flush` afterward. These methods mark the document dirty; reactive signal propagation and event handlers already provide a flush boundary. Owners must belong to the document's graph. Use `Document.Scope` for normal construction; disposing the document also releases its nodes created with other owners in that graph.

## Inputs and structural control

`Document.Input(owner, stringSignal, InputOptions)` binds a persistent native single-line input to a string signal. Native edits acknowledge their edit revision without resetting selection or composition. Authoritative signal replacements advance the input epoch. `InputOptions.Disabled` can read reactive state. This uses the existing generic input behavior and limits; it does not introduce multiline editing or the GPL editor backend.

`Document.Show(owner, condition, yes, no)` owns a transparent range. The selected factory runs when the condition changes; the old branch's scope, nodes and handlers are disposed. Factory reads are untracked. Put reactive calculations inside bindings or memos owned by the supplied branch scope.

`ui.For(document, owner, items, key, render)` retains rows by comparable application key. The renderer receives a row scope, an item signal and an index signal. Read them inside bindings to observe immutable item replacement and reordering. Row factories run once for each inserted key. Reordering preserves native entities; deletion disposes the row's ownership. Duplicate keys fail before reconciliation publishes any change. Keys must be stable for an item's lifetime.

## Mounting and limits

`Snapshot` exports a cloned root through `scene.Node`; `Attach` supplies a custom snapshot sink for tests or another host. A sink must not mutate the document or graph. Each document supports one active sink. Its returned stop function is idempotent and permits reattachment. `OpenWindow` owns one native window; close its `Mount` explicitly before shutting down the client. Disposing a graph/document unsubscribes computations but does not itself close native windows.

Construction errors from fluent methods are available through `Document.Err` and prevent mounting. Computation failures propagate through the graph and suppress publication until recovery. A sink error remains visible and stops automatic publication to that sink: cancellation can race a committed native update, so blind mutation retries are unsafe.

Fine-grained updates currently apply to Go calculations and cached properties. The wire still sends complete snapshots, with the existing 4 MiB message, depth-32 and 10,000-element limits. Stable IDs let Rust preserve input/focus entities. This is a native node graph, without browser layout or DOM/CSS compatibility. The low-level GPUI snapshot API remains available for primitives outside this initial node API.

Run `make reactive` for the [example](../examples/reactiveapp/app.go), which combines batched counters, a memo-backed conditional branch and keyed editable rows. [The GoX adapter](gox/README.md) builds this graph from typed template jobs. [The native scenario](../e2e/scenarios/reactive.star) checks dependency isolation, branch disposal and retained input identity/focus/selection after reordering.
