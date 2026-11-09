# GoX counter

This rewrites [examples/counter](../counter/main.go) using a GoX template and a reactive count signal. It keeps the centered dark layout, heading, count and purple Increment button. The button supports native clicking and keyboard activation.

```sh
make counter-gox         # build and run
make counter-gox-smoke   # signal update, revision-2 commit and event check
```

[main.go](main.go) starts the host. [app/app.go](app/app.go) owns state and the native window. [app/view.gox](app/view.gox) defines the UI using public `scene` types and explicit reactive text. The generated component is built once; clicking increments the signal, whose binding updates the count.

```sh
make gox-tools
go generate ./examples/counter-gox/app
make fmt-counter-gox
make check-gox-generated
```

The pinned GoX tool and `gofmt` run through [app/generate.go](app/generate.go). Edit `.gox`, then generate; do not edit `view.x.go` manually. Generated Go is checked in, so normal builds need no template compiler. `make generate-gox` regenerates both GoX examples through their `go:generate` directives.

The command smoke performs a programmatic signal update. [counter-gox.star](../../e2e/scenarios/counter-gox.star) separately drives native mouse and Enter activation, checks rendered/Go counts and verifies that the count node keeps its identity. Run it with `make e2e E2E_SCENARIOS=e2e/scenarios/counter-gox.star`.
