# Standalone host

`github.com/moriyoshi/gpui-go/host` owns native startup, the Sable transport, platform loop and coordinated shutdown. Import [client](../client/README.md) and [scene](../scene/README.md) directly for applications with their own transport/host.

```go
err := host.Run(context.Background(), func(ctx context.Context, app *client.Client) error {
    window, err := app.OpenWindow(ctx, client.WindowOptions{
        Title: "Hello", Width: 560, Height: 420,
    }, scene.Div(1, scene.Text(2, "Hello, GPUI!")))
    if err != nil { return err }
    _ = window
    return app.DispatchEvents(ctx)
})
```

Call `Run` from `main`, once per process. Native builds lock the startup thread during package initialization because AppKit requires the process main thread. The application callback runs on a goroutine and must honor cancellation. Shutdown releases pending Rust requests and waits for active transport calls before freeing Sable.

Existing build commands and tags are unchanged: `gpui,sable_extern_lib,sable_portable` selects the native backend; `gpui_headless` additionally selects the headless protocol backend. Builds without `gpui` return `ErrNativeUnavailable`. Use the [Makefile](../Makefile), which supplies the archive/framework flags and forces relinking after archive changes.

All native calls go through `internal/nativeshim`. Adding `gpui_prebuilt`, with `CGO_ENABLED=0`, replaces the cgo link with a published per-platform module, so a consumer needs neither Rust nor a C toolchain. The program registers that module by importing `github.com/moriyoshi/gpui-go/prebuilt/<goos>/<goarch>`; without it, `Run` returns an error naming the package. See the [publication guide](../dist/README.md#consume-through-the-shim).

The root `gpui.Run` forwards to this package and shares the same error identity and single-use runtime guard. The separately opted-in `editor.Run` remains its own Sable owner; the two hosts cannot run together in one process. Moving startup does not enable generic embedding of the GPL editor.
