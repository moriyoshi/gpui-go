# Set up your application

## 1. Name what tests need to find

Tests find elements by role and accessible name, by text, or by a stable name you assign. Set up the elements in your application:

```go
package main

import (
	"context"
	"fmt"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
)

// App is the application under test. It does not import uitest.
type App struct {
	graph    *reactive.Graph
	document *ui.Document
	mount    *ui.Mount
	count    *reactive.Signal[int]
	name     *reactive.Signal[string]
}

func Open(ctx context.Context, c *client.Client) (*App, error) {
	graph := reactive.NewGraph()
	document := ui.New(graph.Root(), c)
	owner := document.Scope()
	a := &App{
		graph:    graph,
		document: document,
		count:    reactive.NewSignal(owner, 0),
		name:     reactive.NewSignal(owner, ""),
	}

	increment := document.Div(owner, document.Text(owner, "Increment")).
		Accessible("Increment", &scene.Accessibility{Role: scene.RoleButton}).
		OnClick(func(context.Context, *client.Event) error {
			return a.count.Update(func(v int) int { return v + 1 })
		})
	count := document.BindText(owner, func() string {
		return fmt.Sprintf("Count: %d", a.count.Get())
	}).Name("count")
	field := document.Input(owner, a.name, ui.InputOptions{Placeholder: "Your name"}).
		Accessible("Name", &scene.Accessibility{Role: scene.RoleTextInput}).
		Style(&scene.Style{Width: scene.Pixels(320), Height: scene.Pixels(40)})
	greeting := document.BindText(owner, func() string {
		return "Hello, " + a.name.Get()
	}).Name("greeting")
	root := document.Div(owner, increment, count, field, greeting).Name("root")
	if err := document.Err(); err != nil {
		graph.Dispose()
		return nil, err
	}

	mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "Sample", Width: 480, Height: 320}, root)
	if err != nil {
		graph.Dispose()
		return nil, err
	}
	a.mount = mount
	return a, nil
}
```

The name is `Name` on [`ui` nodes](/reference/reactive-nodes#node-methods) and `testid` in [GoX](/reference/gox-tags#attributes). A `scene` application has no registry, so its `Element` method maps names to the element IDs it already uses.

A placeholder is not an accessible name. Give a field a label with `Accessible`, or `GetByRole("textbox", ...)` cannot match it by name.

## 2. Adapt it to the test driver

`gpuitest.Application` is a small interface. Your application implements it without importing the harness:

```go
package main

import (
	"context"
	"fmt"

	"github.com/moriyoshi/gpui-go/client"
)

func (a *App) Window() *client.Window             { return a.mount.Window }
func (a *App) Element(name string) (uint64, bool) { return a.document.Element(name) }
func (a *App) State() map[string]any {
	return map[string]any{"count": a.count.Peek(), "name": a.name.Peek()}
}
func (a *App) Update(_ context.Context, operation, value string) error {
	switch operation {
	case "set-name":
		return a.name.Set(value)
	default:
		return fmt.Errorf("unknown update %q", operation)
	}
}
func (a *App) Close(ctx context.Context) error {
	defer a.graph.Dispose()
	return a.mount.Close(ctx)
}
```

| Method | Purpose |
| --- | --- |
| `Window` | The application's primary window |
| `Element` | Resolves a stable name to an element ID. `"root"` is required and must be the root of the view, because role and text queries search below it |
| `State` | Go-side values a test can read |
| `Update` | Named operations a test can trigger, such as replacing a value from outside the UI |
| `Close` | Releases the window, handlers and anything else the application owns |

## 3. Write the test program

```go
//go:build gpui_e2e

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/uitest"
	gpuitest "github.com/moriyoshi/gpui-go/uitest/gpui"
)

func main() {
	report := flag.String("worker-report", "", "set by the parent process")
	flag.Parse()
	if err := run(context.Background(), *report); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, report string) error {
	if report == "" {
		return supervise(ctx)
	}
	return host.Run(ctx, func(ctx context.Context, c *client.Client) error {
		target := gpuitest.NewNativeTarget(c, func(ctx context.Context, c *client.Client, _ string) (gpuitest.Application, error) {
			return Open(ctx, c)
		})
		scenario, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		err := uitest.RunScenario(scenario, target, func(ctx context.Context) error {
			return checkCounter(ctx, target)
		})
		return uitest.WriteWorkerReport(report, err)
	})
}

func supervise(ctx context.Context) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	report, err := uitest.RunWorker(ctx, uitest.WorkerOptions{
		Executable: binary,
		Args:       func(path string) []string { return []string{"--worker-report", path} },
		Timeout:    45 * time.Second,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	})
	if err != nil {
		return err
	}
	fmt.Println("passed, report:", report)
	return nil
}

func checkCounter(ctx context.Context, target uitest.Target) error {
	page, err := uitest.OpenPage(ctx, target, "main")
	if err != nil {
		return err
	}
	increment := page.GetByRole("button", uitest.RoleOptions{Name: "Increment", Exact: true})
	if err := increment.Click(ctx); err != nil {
		return err
	}
	if err := uitest.Expect(page.GetByTestID("count")).ToHaveText(ctx, "Count: 1"); err != nil {
		return err
	}
	name := page.GetByRole("textbox", uitest.RoleOptions{Name: "Name"})
	if err := name.Fill(ctx, "Ada 日本語"); err != nil {
		return err
	}
	if err := uitest.Expect(name).ToBeFocused(ctx); err != nil {
		return err
	}
	if err := uitest.Expect(page.GetByTestID("greeting")).ToHaveText(ctx, "Hello, Ada 日本語"); err != nil {
		return err
	}
	if err := name.Fill(ctx, "Grace"); err != nil {
		return err
	}
	if err := uitest.Expect(name).ToHaveValue(ctx, "Grace"); err != nil {
		return err
	}
	state, err := page.State(ctx)
	if err != nil {
		return err
	}
	if state["count"] != 1 || state["name"] != "Grace" {
		return errors.New("unexpected Go state")
	}
	return page.Close(ctx)
}
```

Three rules matter here:

- **Write the report inside the `host.Run` callback.** Code after `host.Run` may never run on macOS.
- **Do not call `DispatchEvents`.** The target dispatches events itself after each action, on the goroutine running your scenario. Your handlers run on that goroutine too, so scenario code must stay on it.
- **Use a `main` program, not `go test`.** `host.Run` has to own the process main thread and can only run once per process.

## 4. Build and run

```sh
CGO_ENABLED=0 go build -tags gpui,gpui_prebuilt,gpui_e2e -o uitest-app .
./uitest-app
```

[Prebuilt modules](/guide/prebuilt-modules) already include test automation, so `gpui_e2e` is the only tag you add. As in any prebuilt build, the program imports the package for its platform, such as `github.com/moriyoshi/gpui-go/prebuilt/linux/amd64`. On macOS and on a Linux desktop, the tests open real windows.

::: warning Not published yet
No prebuilt version exists, so this command cannot resolve today. Use [a source build](#build-the-native-library-from-source) for now.
:::

## Build the native library from source

A native archive you build yourself leaves test automation out unless you ask for it, so enable the Rust `automation` feature. From a checkout:

```sh
cargo build --locked --release --manifest-path rust/native/Cargo.toml --features automation
cp target/release/libgpui_go_native.a target/release/libgpui_go_e2e.a
```

The copy matters, because the next ordinary `cargo build` overwrites `libgpui_go_native.a` with an archive that has no automation.

Then link your test program against the copy:

```sh
CGO_LDFLAGS="-L/path/to/gpui-go/target/release -lgpui_go_e2e" \
  go build -a -mod=mod -tags gpui,gpui_e2e,sable_extern_lib,sable_portable -o uitest-app .
./uitest-app
```

The `-a` flag forces a relink, because Go's build cache does not track the Rust archive. See [Getting started](/guide/getting-started#build-it) for pointing your module at the checkout.

If the archive was built without `automation`, the first `OpenPage` fails with `native archive lacks test-automation-v1`.
