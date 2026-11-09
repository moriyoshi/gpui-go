# Starlark adapter

`github.com/moriyoshi/gpui-go/uitest/starlark` provides `Harness`, `New` and `Check`. It owns interpreter setup, builtins, value conversion and source diagnostics, while the shared [uitest](../README.md) package owns driving semantics, cleanup, deadlines and worker reports.

```go
import starlarktest "github.com/moriyoshi/gpui-go/uitest/starlark"

err := starlarktest.New(target, output).RunFile(ctx, path)
```

The old `github.com/moriyoshi/gpui-go/e2e` package forwards the same public API and is deprecated in favor of this explicit adapter path. Maintained scripts and failure fixtures remain under `e2e/scenarios/` and `e2e/testdata/`; see the [scenario guide](../../e2e/README.md). The CLI remains `cmd/gpui-e2e` with the existing Makefile commands.

`Check` resolves scripts without native startup. A Go-only consumer imports `uitest` and the GPUI adapter, with no dependency on this package or the Starlark interpreter.
