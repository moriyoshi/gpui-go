# Snapshot construction

`github.com/moriyoshi/gpui-go/scene` builds the low-level protobuf snapshots accepted by [client](../client/README.md). It has no client, native startup or reactive graph dependency.

Use `Div`, `Text`, `TextInput`, `Image`, `Canvas` and `UniformList`, with node methods for styling, interaction, actions and accessibility. `Pixels`, `Percent`, `Rems`, `Auto`, `RGB` and `Padding` construct protobuf style values. Builders are mutable and belong to their application's state goroutine; constructors, setters and `Proto` copy supplied data. Choose stable nonzero element IDs within a window.

Author views using public values from this package:

```go
scene.Div(1, scene.Text(2, "Hello")).Style(&scene.Style{
    FlexDirection: scene.FlexColumn.Enum(),
    AlignItems:    scene.AlignCenter.Enum(),
    FontSize:      scene.Ptr[float32](15),
    Background:    scene.RGB(0x0f172a),
}).Accessible("Greeting", &scene.Accessibility{Role: scene.RoleText})
```

[Public type aliases](types.go) cover geometry/length/color, styles, interaction/accessibility, actions, drawing and snapshot values. Concise enum families are `Flex*`, `Align*`, `Justify*`, `Overflow*`, `Cursor*`, `Position*`, `Display*`, `TextAlign*`, `Role*` and `Path*`. Aliases preserve the protobuf type identity, accessors, enum `.Enum()` methods and serialization; existing generated-type callers remain compatible without conversion.

Use `Ptr(value)` for optional scalar fields. Explicit zero or false values stay present; nil means absent. For numeric fields, specify the type, such as `Ptr[float32](0)` or `Ptr[uint32](600)`. Drawing oneof cases are `DrawCommandRect` and `DrawCommandPath`. Callback/event and client-service values live in [client](../client/README.md), including `client.Event`. Ordinary view authoring needs no generated-binding or protobuf-helper imports; raw wire envelopes remain available in `gen/gpui/v1` for transport work.

`Image(id, handle)` takes an opaque protobuf resource handle. Upload and release the resource through the client, and pass `resource.Handle()` when constructing the snapshot. Image nodes copy that handle; snapshots do not own resource release.

`InputState.Accept` acknowledges native edit revisions. `Reset` advances the Go-owned epoch for an authoritative replacement. These operations preserve the existing native input ownership contract; see [text input](../README.md#testing-text-input).

[ui](../ui/README.md) adds persistent reactive nodes above this snapshot API. The root `gpui` package retains aliases and forwarding builders for existing applications; its `Image` wrapper continues to accept `*gpui.Resource`.
