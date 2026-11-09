# Protocol client

`github.com/moriyoshi/gpui-go/client` owns the transport-independent Go protocol API: session negotiation, request correlation, windows, snapshot revisions, events, resources, observable models and platform commands. Importing it does not initialize Sable or lock an OS thread.

Use `NewClient(ctx, transport)` when supplying a `Transport`. Standalone applications use [host.Run](../host/README.md), which creates the client. Construct views with [scene](../scene/README.md), or use [ui](../ui/README.md) for persistent reactive components.

Use public aliases for callback and service values: `Event`, `TextChanged`, `ModelState`, `Menu`, `MenuItem`, `PathPrompt`, `SavePathPrompt`, `PathResult`, `Capabilities` and their related enums. Constants include `ResourceImage`, `ResourceSVG`, `ResourceFont`, `MouseLeft`, `WindowActivate` and `ErrorStaleHandle`. Event oneof cases use names such as `EventClick` and `EventTextChanged`. See [types.go](types.go) for the full set. These are aliases of the wire types, preserving methods and identity with generated callers.

```go
handler := app.OnTextChange(func(ctx context.Context, event *client.Event) error {
    if value.Accept(event.GetTextChanged()) {
        return window.Set(ctx, render())
    }
    return nil
})
```

Handles and geometry in client signatures use `scene.Handle`, `scene.FocusTarget` and `scene.Size`. Generated RPC envelopes and the low-level tagged automation protocol remain available for transport/test adapters; normal application code does not need to import those bindings.

Client commands support concurrent callers. Event pulls have one ordered consumer; dispatch callbacks sequentially on the application state goroutine. `Window.Set` installs a whole snapshot, and consumes a revision even when cancellation races commitment. Successful installation does not prove presentation. Mutations are not automatically retried.

Resource handles and model handles belong to the negotiated session. Release resources only after installed references are removed; close observer subscriptions explicitly. Generic events retain their existing acknowledgement/replay contract. The root `gpui.Client`, `Window`, `Resource`, `Model`, errors and operation types are aliases of these canonical types.

The `Automate` method remains available only with the `gpui_e2e` build tag, and the backend must separately advertise automation capabilities. [uitest/gpui](../uitest/gpui/README.md) adapts that method to the shared test harness. See the [main guide](../README.md) for the complete feature APIs and limits.
