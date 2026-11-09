# GoX native printer

`github.com/moriyoshi/gpui-go/ui/gox` adapts [doors-dev/gox](https://github.com/doors-dev/gox) v0.3.3 to the [persistent GPUI graph](../README.md). GoX supplies typed template jobs and a custom-printer interface. Signals, dependency tracking, ownership, keyed reconciliation and native synchronization live in our `reactive` and `ui` packages.

Vim users can install the repository's [GoX syntax plugin](../../misc/editors/vim/README.md) for filetype detection and highlighting.

Write components in `.gox` source:

```gox
package example

import (
	"fmt"
    "github.com/doors-dev/gox"
	"github.com/moriyoshi/gpui-go/reactive"
    nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

elem Counter(count *reactive.Signal[int]) {
    <span testid="counter">
        ~(nativegox.Text(func() string { return fmt.Sprintf("Count: %d", count.Get()) }))
    </span>
}
```

Build the generated component through the native printer:

```go
root, err := nativegox.Build(ctx, document, document.Scope(), Counter(count))
```

`Build` executes a `gox.Elem` once, copies values out of pooled jobs, and releases each standard job once. Signal changes update retained bindings without rerunning the component. Use `nativegox.Text(accessor)` for dynamic text; passing the result of `signal.Get()` formats a static value. A `*ui.Node` can be embedded as a child, including `Show` and `For` ranges. Failed builds dispose their partial ownership.

Supported tags are `div`, `span`, `text`, `button` and void `input`; all are native GPUI elements. `span` and `text` are containers for their child text nodes. GoX synthetic containers become transparent fragments. Text stays literal native text, with no HTML escaping/parsing step.

| Attribute | Accepted value |
| --- | --- |
| `testid` | Nonempty string registered with the document |
| `label` | Accessibility label string |
| `role` | `scene.AccessibleRole`, such as `scene.RoleButton` |
| `style` | `*scene.Style` or `func() *scene.Style` |
| `onClick` | `ui.Handler` or `func(context.Context, *client.Event) error` |
| `checked` | `bool` or `func() bool` accessibility state |
| `value` | `*reactive.Signal[string]`, required for `input` |
| `placeholder` | Static input string |
| `disabled` | Input `bool` or `func() bool` |
| `masked` | Input `bool` or `func() bool`; draws bullets for passwords |

Buttons receive focus and button semantics; inputs use the existing persistent native input implementation. These names describe typed jobs, not HTML attributes or browser behavior. Unknown tags/attributes, raw HTML/byte/templ jobs and unwrapped function children return errors. Fragments do not accept element attributes.

The [example template](../../examples/reactiveapp/view.gox) defines the main view, reusable button, keyed row and details branch. [Application Go code](../../examples/reactiveapp/app.go) owns signals and creates `ui.For`/`Show` scopes whose factories build the row/branch components. GoX's own template conditionals/loops run during construction; persistent reactive control flow uses the scoped UI primitives.

Use `make fmt-reactive` to format the source and regenerate it, `make generate-reactive` to regenerate after an edit, and `make check-reactive-generated` to verify formatting and compare isolated generation with the checked-in Go. The pinned tool installer downloads a checksum-verified v0.3.3 release to ignored `bin/gox`, preserves its license, and caches the archive under `.cache/gox/`. Linux and macOS amd64/arm64 tooling assets are pinned. The first tool installation requires network access; later generation can use the cache. `go generate ./examples/reactiveapp` also works after `make gox-tools`.

Edit `.gox`; do not hand-edit `.x.go`. The generation pipeline applies `gofmt` to the compiler output. Checked-in generated Go keeps ordinary builds independent of the compiler/formatter, and the application dependency graph still contains only the GoX runtime. The runtime and generator are MIT-licensed and pinned to v0.3.3 (`508309b766d1cb47171b247a98b8db44e81c9146`).

The smaller [counter example](../../examples/counter-gox/README.md) uses the same pipeline. `generate.go` files are the generation entry points: Make targets call `go generate`, and isolated checks execute copied directives. Use `make generate-gox` / `make check-gox-generated` for both examples.
