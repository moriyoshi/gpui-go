# Getting started

## Run the examples

Install Go 1.26.5, Rust 1.99.1 and `protoc` 35.1. On macOS you also need Apple's command line tools. The first build downloads dependencies and compiles GPUI, so it takes a while.

```sh
git clone https://github.com/moriyoshi/gpui-go
cd gpui-go
make run
```

That builds the Rust library and opens a counter. Other examples:

| Command | What it shows |
| --- | --- |
| `make showcase` | A component gallery and workspace dashboard with shared state and a background task |
| `make text-inputs` | The gallery's editable fields, for trying Unicode, selection and clipboard shortcuts |
| `make features` | Assets, canvas drawing, virtual lists, shortcuts, models and drag-and-drop |
| `make reactive` | Signals, GoX templates, conditional branches and keyed inputs |
| `make counter-gox` | The counter rewritten as a GoX template |

## Your first window

Call `host.Run` once from `main`. It takes over the process main thread for the platform event loop and runs your callback on a goroutine. Returning from the callback quits the application, and the callback must return when its context is canceled.

```go
package main

import (
	"context"
	"log"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
)

func main() {
	err := host.Run(context.Background(), func(ctx context.Context, app *client.Client) error {
		_, err := app.OpenWindow(ctx, client.WindowOptions{
			Title: "Hello from Go", Width: 560, Height: 420,
		}, scene.Div(1, scene.Text(2, "Hello, GPUI!")))
		if err != nil {
			return err
		}
		return app.DispatchEvents(ctx)
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

## Build it

Until [prebuilt modules](./prebuilt-modules) are published, your program links a Rust archive built from a checkout. From the checkout, build the archive once:

```sh
cargo build --locked --release --manifest-path rust/native/Cargo.toml
```

In your own module, point the dependency at the checkout, then build with the native tags:

```sh
go mod edit -require=github.com/moriyoshi/gpui-go@v0.0.0 \
            -replace=github.com/moriyoshi/gpui-go=/path/to/gpui-go
go mod tidy
CGO_LDFLAGS="-L/path/to/gpui-go/target/release -lgpui_go_native" \
  go build -tags gpui,sable_extern_lib,sable_portable -o hello .
```

Without the `gpui` tag the program still compiles, but `host.Run` returns `ErrNativeUnavailable`. After rebuilding the Rust archive, add `-a` to force a relink, because Go's build cache does not track it.
