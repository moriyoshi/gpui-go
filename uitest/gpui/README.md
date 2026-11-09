# GPUI test driver

Import `github.com/moriyoshi/gpui-go/uitest/gpui`, conventionally as `gpuitest`. This adapter owns `Application`, `Factory`, `NativeTarget` and `NewNativeTarget`, formerly in `uitest`. The shared [uitest](../README.md) package owns locator/action/assertion semantics, scenarios, deadlines and worker reports.

```go
factory := func(ctx context.Context, app *client.Client, name string) (gpuitest.Application, error) {
    return e2eapp.Open(ctx, app, name)
}
target := gpuitest.NewNativeTarget(app, factory)
page, err := uitest.OpenPage(ctx, target, "workbench")
```

Factories adapt an application type to the interface; applications need not import the test harness. An application exposes its primary `*client.Window`, named elements, state, fixture updates and cleanup. The `root` selector must identify a stable native element for barriers. Presentation stays under examples or in the consuming application.

`NativeTarget` requires the Go `gpui_e2e` tag and an archive built with the Rust `automation` feature. Native standalone consumers also select the existing host/Sable tags. Use `make e2e` or `make uitest`. Driving happens on the application's one state-owning goroutine, through actual native input dispatch and published event barriers. Inspection and locator queries do not inspect the OS accessibility tree.

This import migration is intentional: forwarding `uitest.NewNativeTarget` to a driver that imports `uitest` would create a Go package cycle. `uitest.Page`, `Locator`, `Expect`, target interfaces, raw helpers and worker APIs retain their existing import path. Native selection/composition/input and worker-report behavior are unchanged.
