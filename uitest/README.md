# Shared UI test package

Import the public package from the existing module:

```go
import "github.com/moriyoshi/gpui-go/uitest"
```

`uitest` provides application driving, native inspection, scenario cleanup, suite deadlines and reliable native worker reports. Its package dependency graph contains no protocol client, native host or Starlark interpreter. The [GPUI adapter](gpui/README.md) owns concrete native driving, and the [Starlark adapter](starlark/README.md) owns scripting. The [Starlark harness](../e2e/README.md) is one consumer; [examples/uitest](../examples/uitest/main.go) is a Go-only native consumer of the same public API.

## Pages, locators and expectations

The shared core provides a Playwright-style API directly to Go consumers:

```go
page, err := uitest.OpenPage(ctx, target, "workbench")
if err != nil { return err }
name := page.GetByRole("textbox", uitest.RoleOptions{Name: "Name", Exact: true})
if err := name.Fill(ctx, "Hello 日本語"); err != nil { return err }
if err := uitest.Expect(name).ToHaveValue(ctx, "Hello 日本語"); err != nil { return err }
if err := page.Keyboard().Press(ctx, "ControlOrMeta+A"); err != nil { return err }
```

`Page.GetByTestID` and `Page.Locator` use application selector names; `GetByRole` uses bridge roles/names and `GetByText` uses native snapshot text. Text/name matching is case-sensitive and normalizes whitespace; `Exact` controls equality versus substring matching. Input values use `ToHaveValue`, not text queries. Locators resolve again before each operation and require one match for actions. Multiple matches fail immediately with a strict-mode error; use `Nth`, `First` or `Last` to select deliberately.

`ToHaveText` reads a container's concatenated descendant text, excluding input values. `GetByText` matches text leaves; generic ancestors do not become additional text matches. Unlabelled button, checkbox and link names derive from their descendant text.

Actions include `Click`, `Fill`, `Press`, `Focus`, `Hover`, `Check` and `Uncheck`. Checkbox operations are idempotent. Readiness waits for attachment, rendered geometry and enabled state where applicable. Virtual-list rows are revealed before actions. Assertions include text/value/count, visible/hidden, enabled/disabled, focused and checked state; they retry reads until the expected state or deadline. Default timeout is five seconds, configurable with `Page.SetDefaultTimeout` or `Locator.WithTimeout`. Timeout errors include the locator and last observation. A dispatched mutation is never retried after a transport/application error because commitment may already have happened.

`QueryTarget`, `FocusTarget` and `RevealTarget` are optional target extensions consumed by locators. Named selectors can also use a basic target's inspection. The native implementation requires `test-locators-v1` in addition to `test-automation-v1`. Keyboard presses accept GPUI names and common Playwright names such as `ArrowLeft`, `Shift+ArrowRight` and `ControlOrMeta+A`.

The API follows [Playwright locators](https://playwright.dev/python/docs/locators) and [auto-waiting](https://playwright.dev/python/docs/actionability) patterns for native GPUI. It does not implement browser/CSS selectors or the DOM. Roles/names come from bridge snapshots, not an OS accessibility-tree inspection. Readiness currently checks geometry and native input disabled state; animation stability and pointer obstruction are not prechecked. GPUI still performs actual hit testing, and scenarios should assert the resulting state. Installed OS IME, screen-reader and pixel behavior remain separate verification layers.

## Go scenarios

Functions accept the `Target` interface, so scenarios can be ordinary Go callbacks or be called by another test framework:

```go
func check(ctx context.Context, target uitest.Target) error {
    return uitest.RunScenario(ctx, target, func(ctx context.Context) error {
        app, err := target.Open(ctx, "workbench")
        if err != nil { return err }
        if err := uitest.Click(ctx, target, app, "name"); err != nil { return err }
        if err := uitest.TypeText(ctx, target, app, "Hello 日本語"); err != nil { return err }
        native, err := target.Inspect(ctx, app, "name")
        if err != nil { return err }
        if native["value"] != "Hello 日本語" {
            return fmt.Errorf("native value: %v", native["value"])
        }
        return nil
    })
}
```

`RunScenario` guarantees cleanup using an independent five-second deadline and preserves both scenario and cleanup errors. It treats cancellation as failure even if a callback returns nil. `RunSuite` runs named scenarios with per-file deadlines, continues after a failure, prints PASS/FAIL, and returns all failures. The callback can use Go assertions or evaluate any chosen language.

| API | Responsibility |
| --- | --- |
| `Target` | Open/close, raw protobuf input, native/Go state inspection, fixture updates, clipboard and cleanup |
| `gpuitest.Application`, `gpuitest.Factory` | Application-specific window, stable selectors, state, updates and resource ownership |
| `Click`, `Pointer`, `Key`, `TypeText`, `Compose` | Reusable native input operations without scripting values |
| `Page`, `Locator`, `Expect`, `Keyboard` | Live selectors, strict actions, readiness and retrying assertions shared by Go and Starlark |
| `RunScenario`, `RunSuite` | Cleanup, deadlines, continuation and aggregate errors |
| `WorkerOptions`, `RunWorker` | Start a worker, enforce its deadline and require process plus report success |
| `WriteWorkerReport`, `ReadWorkerReport` | Atomic version-1 completion/error reporting and validation |
| `gpuitest.NativeTarget`, `gpuitest.NewNativeTarget` | Native GPUI/Sable implementation, available with `gpui_e2e` |

## Native driving

Native consumers require the Go `gpui_e2e,gpui,sable_extern_lib,sable_portable` tags and a native archive built with the Rust `automation` feature. The backend must advertise `test-automation-v1`; ordinary backends reject automation. Use the existing Makefile targets:

```sh
make uitest          # build/run the Go-only consumer
make e2e             # build/run the Starlark consumer
make e2e-check       # resolve Starlark files without native linkage
```

Call `host.Run` once from the worker's `main`. Inside its application callback, import `uitest/gpui` as `gpuitest` and construct `gpuitest.NewNativeTarget(client, factory)` and drive it from that one state-owning goroutine. Factories must provide a primary window and a stable `"root"` selector. Input uses GPUI hit testing, keymaps and the focused platform input handler. Inspection returns native bounds, focus, text, and UTF-16 selection/mark ranges. The target drains published event watermarks and resulting Go commits before assertions.

`RunWorker` is independent of CLI flags and interpreters. Supply the executable, an argument builder receiving the absolute report path, a positive timeout and optional output/environment settings. The worker writes `WriteWorkerReport(reportPath, scenarioErr)` after scenario cleanup and before native shutdown. The parent validates both exit status and the report. This avoids AppKit exit-zero behavior hiding failed or incomplete suites. Reports remain in the configured artifacts directory (the system temporary directory by default); repository consumers explicitly use `.agents-workspace/tmp/`.

Default package tests need no GUI, native archive or interpreter. They exercise the public API, independent cleanup, missed deadlines and actual subprocess success/failure/missing-report/crash/timeout cases. Native consumer runs currently have macOS/arm64 evidence. Synthetic GPUI inputs do not establish installed OS IME, screen-reader or pixel behavior. Clipboard cleanup restores previous text, not rich clipboard formats.
