# Project overview

GPUI-go wraps GPUI for Go using Sable and versioned protobuf. Read [README](../../README.md) for APIs/run commands and [architecture](../../ARCHITECTURE.md) before changing subsystem boundaries.

## Surfaces

* Root Go package: compatibility aliases/builders/Run. Canonical `scene/` owns snapshot construction and input versions, `client/` owns transport-independent sessions/windows/events/resources/models/commands, and `host/` owns native startup/Sable/shutdown. Implementation packages do not import the root facade.
* Public application values: `scene.Style`/geometry/drawing/accessibility, `client.Event`/service aliases, concise typed enums and `scene.Ptr` replace generated-type/helper imports in generic examples. Aliases preserve protobuf identity and methods; wire envelopes remain in `gen/gpui/v1` for transport adapters.
* `reactive/`, `ui/`, `ui/gox/`: synchronous owner-scoped signals/memos/effects, persistent keyed/conditional node bindings, controlled inputs and a GoX v0.3.3 native printer. See [the UI guide](../../ui/README.md). Bindings are selective in Go; wire updates remain whole snapshots.
* Reactive example authoring: [view.gox](../../examples/reactiveapp/view.gox) owns markup and generated `view.x.go` is checked in. `make fmt-reactive` formats/regenerates; `make check-reactive-generated` compares isolated generation. The checksum-pinned GoX CLI is an ignored build tool, not an application import.
* [GoX counter](../../examples/counter-gox/README.md): smaller counter rewrite, with presentation/application under `examples/counter-gox/app` and a standalone executable at `examples/counter-gox`. Both GoX examples own `generate.go`; Make and isolated checks execute `go generate`. Use `make generate-gox` / `check-gox-generated` for both.
* `rust/core/`: protocol validation, session/resource/model state, text/UTF-16 helpers.
* `rust/runtime/`: Sable handlers, mailboxes, acknowledged event queue, headless execution.
* `rust/native/`: GPUI rendering, assets/drawing, virtual lists, persistent input/focus/model entities, native keymaps, menus and asynchronous dialogs.
* `uitest/`: public shared Go testing core with Page/Locator/expect, target contracts, lifecycle/deadlines and process reports; no client, host or Starlark dependency.
* `uitest/gpui/`: native driver and application/factory contracts. `uitest/starlark/`: interpreter adapter. `e2e/` forwards the old public Go API and retains scenarios/testdata; `cmd/gpui-e2e/` composes adapters and example factories. See [migration](../../README.md#packages-and-migration) and the [harness guide](../../e2e/README.md).
* `editor/`: transport-independent Go API and optional standalone Run adapter.
* `rust/zed-editor/`: reusable GPL editor component, without application presentation or transport.
* `rust/zed-editor/host/`: separate Sable/protobuf/window adapter in the editor workspace.
* `examples/counter/`, `examples/showcase/`, `examples/features/`, `examples/reactive/`, `examples/reactiveapp/`, `examples/e2eapp/`, `examples/uitest/`, `examples/editor/`: application behavior, smoke scenarios, reactive components, E2E fixtures and a Go-only shared-core consumer.

The generic Apache backend and opt-in GPL editor backend remain separate. The editor supports independent local multiline buffers and undo/redo; it has no configured syntax/LSP/project/file services. The supplied Go editor host opens one component per window; generic Go snapshot embedding is not implemented.

Toolchains: ordinary Go builds use Go 1.26.5, with Rust 1.99.1 and protoc 35.1. Native module publication separately uses Go 1.26.10 to match Bao v0.0.0's verified-linker profile. Dependencies/generators are pinned. Generic protocol minor 3 advertises implemented feature groups; OS services are native-only. Native validation currently targets macOS/arm64. GPUI test-platform dispatch, actual desktop command/clipboard smoke, and real OS input are distinct verification layers.

Use the [quality gate](QUALITY_GATE.md), [testing guide](TESTING.md), [LTM index](LTM/INDEX.md), and [TODO](TODO.md). Verify backlog items against current code before implementing them.
