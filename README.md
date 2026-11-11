# gpui-go

A Go wrapper for [GPUI](https://gpui.rs), using [Sable](https://github.com/moriyoshi/sable) to await Rust operations from goroutines. A versioned protobuf contract describes windows, view snapshots, layout, styles, references, commands, and events.

The bridge exposes serializable GPUI primitives through a reusable Go API. Headless integration exercises the actual Sable transport; native builds and desktop smoke tests are verified on macOS/arm64. [Coverage](#coverage) distinguishes implemented features from remaining limits. User documentation is at [gpui-go.dev](https://gpui-go.dev), with its VitePress source in [docs/](docs/index.md).

## License

Unless otherwise noted, gpui-go is licensed under the [Apache License, Version 2.0](LICENSE). This includes the Go packages, protobuf schemas, examples, and generic Rust core/runtime/native backend.

The optional Rust Zed editor component and host under `rust/zed-editor/` are licensed under [GPL-3.0-or-later](rust/zed-editor/LICENSE-GPL). Applications distributed with that editor linked in must comply with the GPL's requirements for the combined work, including corresponding source. The Go `editor` client and editor protobuf schemas remain Apache-2.0; their license does not remove the obligations of linking the GPL backend.

Third-party code and assets retain their respective licenses. Preserve applicable license texts and attribution notices, including the [project notice](NOTICE) and [native input attribution](rust/native/NOTICE).

## Packages and migration

The module separates snapshot construction, protocol clients and native startup:

| Package | Responsibility |
| --- | --- |
| [scene](scene/README.md) | Snapshot builders, style helpers and input epoch/revision state |
| [client](client/README.md) | Transport-independent sessions, windows, events, resources, models and commands |
| [host](host/README.md) | Standalone Run, Sable lifecycle, process main thread and desktop/headless loop |
| [reactive](reactive/README.md) | Signals, memos, effects, batching and owner scopes |
| [ui](ui/README.md), [ui/gox](ui/gox/README.md) | Persistent reactive nodes and the typed GoX construction adapter |
| [tw](tw/README.md) | Tailwind CSS v4 style utility lists compiled to native styles |
| [uitest](uitest/README.md) | Shared Page/Locator/expect, target contracts, scenario lifecycle and worker reports |
| [uitest/gpui](uitest/gpui/README.md) | Concrete GPUI driver and application factory contracts |
| [uitest/starlark](uitest/starlark/README.md) | Starlark evaluation, builtins and source diagnostics |
| [editor](editor/README.md) | Separate opt-in Zed editor API and standalone host |

Existing root `gpui` types are aliases of `client` and `scene` types, and its builders/Run forward to those packages. Existing root imports remain source-compatible. New libraries should import `client`/`scene` directly so native build tags do not pull in standalone startup. The low-level `scene.Image` takes `resource.Handle()`; root `gpui.Image` retains its resource argument.

Root compatibility aliases share the canonical client and snapshot-builder identities; reflection reports their defining packages. The refactor does not add duplicate client or window state behind the facade.

Application value types have public aliases too: use `scene.Style`, `scene.Accessibility`, `scene.DrawCommand`, `client.Event`, `client.ModelState` and client menu/dialog types. Concise enums such as `scene.FlexColumn`, `scene.RoleButton` and `client.ResourceImage` replace generated spellings. `scene.Ptr[float32](15)` sets an optional scalar. These aliases retain the underlying protobuf identity and methods; the generated schema remains available for raw transport work. Generic examples, including the GoX template, need no generated-binding imports.

Move `uitest.Application`, `Factory`, `NativeTarget` and `NewNativeTarget` imports to `uitest/gpui`, conventionally aliased `gpuitest`. Locator, scenario and worker APIs stay in `uitest`. Move Starlark imports from `e2e` to `uitest/starlark`; the old `e2e` path remains a deprecated forwarding API. Scenario files stay under `e2e/`. Example application constructors return their own `*App`; the runner adapts them to `gpuitest.Factory`.

These are Go package changes within one module. Protobuf schemas, generated paths, native feature tags, Rust crate boundaries and the explicit GPL editor boundary are unchanged.

## Run

Install Go 1.26.10, Rust 1.99.1, and `protoc` 35.1. macOS native builds also need Apple's command line tools. Dependencies are fetched by Go and Cargo; a network-enabled first build is required.

The native crate enables GPUI's runtime shader compilation by default, so the optional Xcode Metal compiler is not required. Metal shaders compile through the GPU driver when the application opens its first window. Building the native crate with Cargo's `--no-default-features` uses precompiled shaders instead and requires the Metal toolchain.

```sh
make run                 # build the combined Rust library and Go counter
make smoke               # open, install revision 2, observe its event, quit
make headless-smoke      # same protocol flow without a window server
make showcase           # interactive component gallery / workspace dashboard
make text-inputs        # open directly on the editable Inputs page
make showcase-smoke     # exercise gallery controls and a background build
make showcase-headless-smoke # exercise the same actions without a window server
make features           # assets, canvas, virtual list, shortcuts, models, drag/drop
make features-smoke     # native resources, models, clipboard and window commands
make reactive           # signals, GoX templates, conditional branches, keyed inputs
make counter-gox        # the counter rewritten in GoX with public scene types
make counter-gox-smoke  # generated counter view update and event round trip
```

Call `host.Run` (or the compatible `gpui.Run`) from `main`, once per process. Native builds lock Go's startup thread for the platform loop. The application callback runs on a goroutine, and must return when its context is canceled. Returning from it quits the application.

```go
err := host.Run(context.Background(), func(ctx context.Context, app *client.Client) error {
    window, err := app.OpenWindow(ctx, client.WindowOptions{
        Title: "Hello from Go", Width: 560, Height: 420,
    }, scene.Div(1, scene.Text(2, "Hello, GPUI!")))
    if err != nil {
        return err
    }
    _ = window
    return app.DispatchEvents(ctx)
})
```

See [the counter](examples/counter/main.go) for click handlers, styles, and view updates. Its `-smoke` option exercises installation and events; it does not simulate a native mouse click.

## Reactive components and GoX

The public [reactive](reactive/README.md) and [ui](ui/README.md) packages provide a SolidJS-like signal and node graph over the existing client. Components build once; signal reads track dependencies, memos cache derived values, and effects update only dependent properties. Batches publish one settled snapshot. Owner scopes clean up effects, handlers and conditional branches; keyed rows preserve element IDs and native input state when reordered.

[ui/gox](ui/gox/README.md) is a native custom printer for [GoX](https://github.com/doors-dev/gox), pinned to v0.3.3. It consumes typed template jobs, retains explicit accessor bindings, and embeds `ui.Show`/`ui.For` ranges. GoX supplies the template runtime; the reactive engine lives in Go packages that can also be used directly. No template compiler or browser is needed to run the example.

Run `make reactive` to edit a name, batch counter updates, toggle a memo-backed branch, and add/remove/reorder editable rows. Presentation and fixture operations live in [examples/reactiveapp](examples/reactiveapp/app.go); [examples/reactive](examples/reactive/main.go) is the interactive entry point. [The E2E scenario](e2e/scenarios/reactive.star) checks selective calculations, cleanup and native input identity, focus and selection across reordering.

The example's markup is authored in [view.gox](examples/reactiveapp/view.gox), with checked-in generated `view.x.go`. Run `make fmt-reactive` to format and regenerate, or `make generate-reactive` after editing; `make check-reactive-generated` verifies the result without Git. These targets install a checksum-pinned GoX v0.3.3 tool under `bin/` and cache it locally. Normal builds consume the generated Go without the compiler. See the [example guide](examples/reactive/README.md).

[examples/counter-gox](examples/counter-gox/README.md) is a smaller rewrite of the original counter: one count signal and a GoX view with the same layout and Increment button. Run `make counter-gox`. Both examples generate through their `generate.go` directives: install with `make gox-tools`, then run `go generate ./examples/reactiveapp ./examples/counter-gox/app`, or use `make generate-gox`. `make check-gox-generated` verifies both outputs by executing those directives in isolated scratch packages.

[Vim syntax highlighting](misc/editors/vim/README.md) provides `.gox` filetype detection and highlights markup, template control flow and embedded Go expressions. Install it as a Vim runtime package; `make test-vim` checks the highlighting without a display or native build.

The graph is synchronous and belongs to one application goroutine. Its bindings are fine-grained in Go; the protobuf boundary still carries complete snapshots with the existing limits. This initial API supports native containers, text, buttons and single-line inputs; it does not implement the browser DOM or all SolidJS facilities. See the package guides for ownership, errors and supported typed attributes.

## Component showcase

[examples/showcase](examples/showcase) is a larger application with four pages and shared state:

- **Overview:** metric cards, a build table with segmented filters, progress, and Start/Pause/Reset buttons. Progress comes from a real background Go task.
- **Controls:** checkboxes, a notification switch, accent radio choices, density segments, a region dropdown, a bounded worker-count stepper, and primary/secondary/disabled buttons.
- **Inputs:** editable name/email fields, live Unicode value previews, a search field that filters the build table, sample/reset buttons, and a disabled field.
- **Activity:** a scrollable history of setting changes and build events.

The header switches between light and dark themes. Saving preferences opens a dismissible toast; resetting the workspace opens a confirmation dialog with a clickable backdrop. Preferences live in memory for the session. Stable element IDs keep GPUI interaction state attached to the same controls while Go replaces snapshots.

Buttons and selection controls are composed in Go from GPUI layout, text, hover, and click primitives. The main buttons, navigation, checkboxes, and segments expose focus and accessibility semantics; focused click targets support GPUI keyboard activation. Text fields use a persistent Rust editor implementing GPUI's native `EntityInputHandler` interface. Fields support mouse focus/drag selection, Tab/Shift+Tab, arrow and Shift+arrow selection, Home/End, Backspace/Delete, Cmd/Ctrl+A/C/X/V, Unicode grapheme movement, and IME composition. Hover feedback executes in Rust without a Go callback. Full screen-reader and OS IME validation remain open.

`make showcase-smoke` invokes the same registered Go handlers as UI clicks, checks each state transition and protobuf view revision, then waits for the background build to reach 100%. This is programmatic control testing, not native mouse automation. `make showcase-headless-smoke` runs the same sequence through Sable without a display.

The example's event loop combines `NextEvents` + `Client.Dispatch` with a background-result channel. All model changes and view commits run on that one goroutine. Worker results carry a generation so pausing, resetting, or restarting a job cannot apply stale progress to a newer job. See [main.go](examples/showcase/main.go), [model.go](examples/showcase/model.go), and [ui.go](examples/showcase/ui.go) for the event loop, state transitions, and composed controls.

## Testing text input

Run `make text-inputs` or `./bin/showcase -page Inputs`. Click a field and try Japanese text, emoji, combining characters, selection, and clipboard shortcuts. The Go value preview updates as you type, including composition events. The search field filters project/branch names live. **Load Unicode sample** and **Clear fields** exercise explicit Go-side replacement without recreating the editor. Fields are single-line, normalize pasted line breaks to spaces, and accept up to 64 KiB of UTF-8. Undo/redo and multiline editing are not implemented yet.

Use `InputState` when mirroring an editable value in Go:

```go
name := gpui.InputState{Value: "Ada Lovelace"}
handler := app.OnTextChange(func(ctx context.Context, event *client.Event) error {
    if name.Accept(event.GetTextChanged()) {
        return window.Set(ctx, render())
    }
    return nil
})
// Inside render():
field := gpui.TextInput(10, name, gpui.TextInputOptions{
    Placeholder: "Your name", OnChange: handler,
})
```

`InputState.Accept` acknowledges a native edit revision. `InputState.Reset(value)` increments a Go-owned epoch for an authoritative replacement. Rust preserves native focus, selection, composition, and newer edits when an older snapshot comes back from Go; queued events from before a reset are ignored. The native editor adapts GPUI's Apache-licensed input example; attribution is in [rust/native/NOTICE](rust/native/NOTICE).

## Starlark E2E scenarios

[The E2E harness](e2e/README.md) runs `.star` files through a native Go/Sable/GPUI application. Eleven scenarios cover real GPUI hit testing, model updates, shortcuts/chords, Unicode editing/selection, simulated composition, disabled fields, clipboard shortcuts, virtual-list scrolling, independent windows, reactive keyed components, the GoX counter and a Playwright-style Page/Locator/expect API. Its shared core is the public [github.com/moriyoshi/gpui-go/uitest](uitest/README.md) package, which has no Starlark dependency and supports the same locator/action/assertion API in Go.

```sh
make e2e-check           # parse and resolve; no display or native archive
make e2e                 # build and run the native suite
make uitest              # build/run the Go-only shared-package consumer
./bin/gpui-e2e --timeout 45s e2e/scenarios/text-input.star
```

Automation requires both the Rust `automation` feature and Go `gpui_e2e` tag; ordinary local builds reject it. Published prebuilt modules include the feature, so `-tags gpui_prebuilt,gpui_e2e` is enough. The harness drains event barriers, cleans up each file and requires an atomic completion report from its native worker before reporting success. Failures include Starlark source backtraces and exit nonzero. Reports are retained under `.agents-workspace/tmp/`.

Inputs are synthesized inside GPUI in a native graphical process. Composition calls the native text-service interface; it does not drive an installed OS IME. Screen-reader output and pixels are not asserted. The clipboard scenario changes the actual clipboard and restores its previous text, without preserving rich clipboard formats. See the [harness guide](e2e/README.md) for selecting scenarios and adding application fixtures.

## Reusable Zed editor

[editor](editor/README.md) is a public Go package for persistent Zed editor instances. [rust/zed-editor](rust/zed-editor) is an embeddable Rust component library (`rlib`); [its host crate](rust/zed-editor/host) provides the optional standalone GPUI/Sable runtime. **These native editor components link GPL-3.0-or-later Zed code.** The ordinary counter, showcase, and default tests do not link that backend. See [the license](rust/zed-editor/COPYING).

The component owns text, selection, and undo history. It renders only an editor, exposes native focus and typed change events, and installs no window, toolbar, sample text, transport, or application loop. A Rust application can compose `Entity<EditorComponent>` into any GPUI layout; see [the embedding guide](rust/zed-editor/README.md). Host settings/theme setup and optional editing key bindings are separate from component construction.

The Go API exposes `Client.Open`, `Client.NextEvent`, and `Client.Quit`; each `Editor` has `State`, `SetText`, `InsertText`, `SetSelection`, `Undo`, `Redo`, `SetReadOnly`, `Focus`, and `Close`. It supports concurrent commands, typed protocol errors, and one ordered event consumer. Each instance has a nonzero ID that is never reused during the host process. Closing an editor invalidates its handle without quitting the host. The supplied Go host presents one component per native window; composition inside the generic `gpui.Div` snapshot API is not implemented.

```go
import "github.com/moriyoshi/gpui-go/editor"

err := editor.Run(context.Background(), func(ctx context.Context, app *editor.Client) error {
    field, err := app.Open(ctx, editor.Options{
        Title: "Document", Text: "Hello, 日本語 🦀", Width: 900, Height: 600,
    })
    if err != nil { return err }
    for {
        event, err := app.NextEvent(ctx)
        if err != nil { return err }
        if event.Kind == editor.EditorClosed && event.EditorID == field.ID() {
            return nil
        }
    }
})
```

`editor.Run` owns the process-global Sable runtime and runs once from `main`; it cannot run alongside another owner such as `gpui.Run`. Its callback must honor cancellation, and returning from it quits the host. `editor.NewClient` accepts an application-supplied `Transport` for custom hosts implementing [the editor protocol](proto/gpui/editor/v1/editor.proto). The Go client, protocol, and transport tests build without native tags; `Run` reports `ErrNativeUnavailable` in that configuration.

[examples/editor](examples/editor) is a separate application consuming this public API. It opens two independently editable windows, logs changes with editor IDs, and exits when both close. All sample text and smoke assertions live in the example.

```sh
make editor-lib          # build the reusable native host archive
make editor              # build/run examples/editor
make editor-smoke        # exercise native instances and their lifecycle
# make zed-editor and make zed-editor-smoke remain aliases
```

The opt-in Go tags are `gpui_zed_editor,sable_extern_lib,sable_portable`; link `target/zed-editor/debug/libgpui_go_zed_editor_host.a` with the macOS frameworks declared in `editor/run_native.go`. The Makefile handles these flags and forces relinking after Rust archive changes. The first editor build is large because Zed's editor depends on project/workspace crates. It uses a separate development directory, `target/zed-editor`, with debug information disabled and settings assets embedded. The library and host share a Cargo lockfile and the pinned Zed revision used by the main backend.

The protocol has version negotiation, configurable initial text/window size/gutter/soft wrap, per-editor state/change/close/error events, and explicit lifecycle commands. `SetText` is an authoritative replacement permitted in read-only mode; native edits and insert/undo/redo commands honor read-only mode. The standalone bridge limits command text and snapshots to 1 MiB, windows to 64, its foreground mailbox to 64 commands, and events to 256. These are host limits, not Rust component limits. Native input can exceed the snapshot limit: Go receives a `ResourceExhausted` event, and smaller authoritative replacements recover the editor. Event overflow stops the host. There is no event acknowledgement/replay or automatic retry; cancellation can race commitment or event receipt.

The native smoke passed on macOS/arm64 for independent buffers and initial undo histories, multiline Unicode text, change events, insertion, undo/redo, read-only rejection and authoritative replacement, focus, close events, stale handles, and reopening after the last window closes. It does not synthesize typing or IME input. This remains plain-text editing: syntax grammars, LSP, file/project services, accessibility validation, and Zed's full default keymap are not configured.

## Protocol and ownership

[proto/gpui/v1](proto/gpui/v1) is the public contract. Generated Go bindings are checked in under `gen/gpui/v1`; Rust bindings are generated by `prost-build` when building the core crate. The wire format uses protobuf messages directly, without gRPC.

| Layer | Responsibility |
| --- | --- |
| Go application | Business state, handlers, goroutines, building immutable snapshots |
| Go client | Correlation IDs, session negotiation, revisions, event cursors |
| Sable | Asynchronous request/response delivery, parking goroutines, cancellation |
| Rust runtime | Bounded foreground mailbox and acknowledged event queue |
| Rust native adapter | Main-thread GPUI entities, layout, painting, click listeners |

An open window owns one root view. Closing it invalidates both handles. Handles include a generation and belong to a negotiated application session. Elements have stable, nonzero IDs unique within each view. Go callbacks are represented by monotonically allocated handler IDs, never Go pointers.

`Window.Set` replaces a complete snapshot and implicitly notifies GPUI. Styles omitted from the next snapshot return to defaults. Successful responses acknowledge installation, not presentation. Revisions must increase; the Go wrapper consumes a revision even if a call is canceled, because the update may already have committed. Rust validates a replacement before changing the current view.

GPUI renders the most recent snapshot entirely in Rust; it never synchronously waits for Go during a frame. A Go event consumer invokes handlers sequentially. Application state should stay on that goroutine or use application-owned synchronization. Builders are mutable; `Proto`, `Div`, and `Style` copy their input data.

Click propagation policy is declared in the snapshot and runs synchronously in Rust. Clicks captured from an obsolete view revision are discarded. Removing a Go handler makes already queued events for it harmless; IDs are never reused.

Event batches are retained until a later `NextEvents` call acknowledges their last received sequence. Canceling a pending pull can be retried without losing events. There is one event consumer per client. Acknowledgement means receipt, not successful execution of every callback; applications needing processing guarantees should manage that at the application level.

Commands and event pulls use separate Sable operation IDs. Foreground admission is bounded at 256 messages; unacknowledged events at 1024. Overflow is an explicit protocol error, not silent input loss. The maximum protobuf message is 4 MiB, with 10,000 elements and a maximum tree depth of 32. Cancellation before foreground execution skips a queued operation; cancellation racing commitment is best-effort and never rolls back a committed change. Requests are not automatically retried or deduplicated.

The portable Sable backend is the initial default (`sable_portable`, Rust `default-features = false`). It avoids private Go runtime ABIs. GPUI retains its platform event loop and executors: there is no claim of one poller or one executor for the whole GUI process.

## Core features

[examples/features](examples/features/main.go) combines the APIs below in one application. Use `./bin/features -font /path/to/font.ttf` to install an application font and select its family through `Style.FontFamily`. Font files are supplied by the application.

| Area | Go API and behavior |
| --- | --- |
| Resources | `UploadResource`, `Image`, `Resource.Release`: PNG/JPEG/GIF/WebP raster images, SVG icons, and fonts; snapshots reference opaque handles |
| Drawing | `Canvas`: local-coordinate rectangles and filled/stroked paths, including quadratic/cubic curves |
| Lists | `UniformList`, `Window.ScrollTo`: persistent native scroll state, layout/painting of visible rows |
| Keyboard and actions | `Focusable`, `BindKey`, `BindKeyWithContext`, `OnAction`, `Window.DispatchAction`; GPUI key contexts, predicates and multi-stroke chords |
| Input events | `Interact` and `OnEvent`: key up/down, mouse up/down/move, scrolling, focus changes, external file drops, typed in-process drag/drop |
| Accessibility | `Accessible`: roles, labels, description, selection, expanded/checked state and values; focus and text-input value actions |
| Style and animation | Flex/grid/hidden display, wrapping, max sizes, opacity, font families, alignment, italic text, shadows; `Animate` for native opacity animation |
| Models | `CreateModel`, `Read`, revision-checked `Update`, `Observe`, `Release`: Rust-owned GPUI entities containing opaque application bytes |
| Platform | Window title/size/activation/minimize/zoom/fullscreen; clipboard text; HTTP/HTTPS/mailto URLs; native menus and asynchronous open/save path dialogs |

Bind native events to handler IDs registered with `OnEvent`. Configure the synchronous propagation policy in the snapshot; handlers run later through `Dispatch` or `DispatchEvents`. `BindKeyWithContext` predicates are evaluated along the native focus path against `Interaction.KeyContext`. Shortcut registrations are replaced when bindings change, and ordinary snapshot updates preserve pending chords.

Models begin at revision 1. `Update(ctx, expectedRevision, bytes)` rejects stale revisions without mutation. Values can contain an application-defined protobuf; the bridge does not interpret them. `Observe` reports successful updates, without an initial replay. Close subscriptions explicitly. A model change does not rebuild a view automatically: the Go callback decides whether to call `Window.Set`.

`SetMenus` replaces the application menu bar. Items target a stable view/element and a bound logical action; removed targets are ignored on selection. `PromptPaths` and `PromptSavePath` acknowledge immediately, then deliver a one-shot callback through normal event dispatch. Their returned detach function removes the callback; it does not dismiss an OS dialog. These APIs return paths without reading or writing files.

Resources must be released after all referencing views are closed or replaced. Each upload is limited to 2 MiB; a session holds at most 128 resources and 32 MiB of source bytes. Native raster decoding is bounded to 4096 by 4096 pixels and the native asset cache to 64 MiB. Animated raster formats currently use their first frame. Font faces stay installed until application exit and continue to count toward the native budget after logical release. Models are limited to 1024 per session, 64 KiB per value, and 128 observers per model. Lists still transmit all row snapshots within the usual element/message limits; virtualization covers native layout and painting, not paging.

## Coverage

Protocol minor 4 retains the window/snapshot/event operations and `Command` for resources, models, actions, focus, scrolling and platform services, and adds natively evaluated conditional styles and window size and color scheme reporting. Capabilities explicitly advertise `resources`, `drawing`, `uniform-list`, `interaction`, `actions`, `focus`, `accessibility`, `extended-style`, `animation`, `models`, `conditional-styles`, `window-size`, and `window-color-scheme`. The native backend additionally advertises `platform-services`, `menus`, and `path-prompts`; headless execution rejects OS services. Event batches are bounded by both count and encoded size.

The supported element set is containers, text, persistent single-line text inputs, uploaded images/SVG, canvases, and uniform-height virtual lists. This covers the bridge's core serializable primitives. It does not expose every Rust GPUI method: arbitrary Rust contexts/closures/`Render` implementations, custom GPU shaders, variable-height or paged lists, styled text runs, and snapshot patches remain outside the current contract. Animation currently covers opacity. Generic inputs still lack undo/redo and multiline editing. The separate GPL editor remains opt-in and is not embedded in generic snapshots.

Native tests exercise GPUI focus/key/action dispatch, chords, pointer hitboxes, Unicode input handling and asynchronous picker completion through GPUI's test platform. Desktop smoke exercises resource decoding, clipboard round trips, model observation, menus, window commands and scrolling. Actual OS IME composition, drag/drop gestures, screen-reader behavior, pixel rendering, and Linux/Windows native runtime validation remain unverified.

## Development

The [native module metadata proxy](dist/metadata-proxy/README.md) is a Cloudflare Worker for serving Go module metadata and redirecting module ZIP downloads to GitHub Releases. It also serves the site built from [docs/](docs/index.md) on the same domain. It reads each version's metadata from the release itself, so publishing a release is the only step needed to advertise it.

The [native publication guide](dist/README.md) describes the workflow, run by manual dispatch, that builds per-platform static libraries, Bao `.syso` packages and Go module ZIPs, with checksums and a draft Release. Publishing the release with `make release` is an explicit later step: it signs a tag on a commit requiring the new modules, and it deploys the Worker. With `-tags gpui_prebuilt` and `CGO_ENABLED=0`, a program uses the published module for its target; see [Prebuilt modules](docs/guide/prebuilt-modules.md).

Coding-agent guidance starts in [AGENTS.md](AGENTS.md), with the subsystem map in [ARCHITECTURE.md](ARCHITECTURE.md). The [agent harness guide](.agents/docs/HARNESS.md) records the adopted Cornus/yesnodb conventions and maintenance skills. Run `make harness-check` after editing that harness.

```sh
make generate            # pinned protobuf and GoX generators; Rust at build
make check-reactive-generated # source formatting and isolated GoX regeneration
make test                # Go race tests and display-independent Rust tests
make test-runtime        # Rust mailbox, cancellation, queue, and shutdown tests
make test-native         # GPUI test-platform key/input/action/dialog dispatch
make integration         # Go → Sable → Rust → Go, including feature lifecycles
make e2e-check           # Starlark syntax/name resolution without native linkage
make e2e                 # opt-in native input scenarios and completion reports
make docs-screenshots   # macOS only: recapture the documentation images from native windows
make fmt
```

The default `go test ./...` does not link a native library or need a display. `gpui.Run` reports `ErrNativeUnavailable` in that build. A native build uses `gpui,sable_extern_lib,sable_portable`; headless builds additionally use `gpui_headless`. The Makefile supplies the combined Rust archive through `CGO_LDFLAGS`.

Sable is pinned to `0c6fe56eb09937dee354532d14c265a99534e8a2` on both sides. GPUI and `gpui_platform` are pinned together to Zed `e231594b6b8818ea8d668ee2eb73e1fb09300654`. Cargo lockfiles are included for the core, runtime, native crates, and editor workspace; the Makefile uses `--locked` for their builds and tests.

Shutdown closes the foreground mailbox, completes accepted requests, wakes event consumers, joins the application callback, then stops and frees Sable after active transport calls finish. `Run` owns the process-global Sable runtime; embedding into another owner of Sable's global runtime is not currently supported.
