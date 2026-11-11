# Architecture

## Scope and subsystem map

GPUI-go exposes GPUI to Go through protobuf and Sable. It provides a generic snapshot-based UI API and a separate opt-in Zed editor component. The user-facing commands and APIs are documented in [README](README.md); the Rust component has an [embedding guide](rust/zed-editor/README.md).

| Layer | Source | Responsibility |
| --- | --- | --- |
| Compatibility facade | Root `client.go`, `element.go`, `input.go`, `features.go`, `run.go` | Aliases and forwarding APIs for existing root imports; implementation packages never import this facade |
| Snapshot construction | `scene/` | Independently owned protobuf builders, styles, interaction metadata and input synchronization |
| Generic client | `client/` | Transport-independent sessions, windows/revisions, events, resources, models and commands |
| Standalone host | `host/` | Main-thread startup, Sable transport, desktop/headless loop and shutdown |
| Reactive graph | `reactive/` | Goroutine-local dependency tracking, signals/memos/effects, batching and owner lifetimes |
| Utility styles | `tw/` | Tailwind-style utility lists compiled to `scene.Style`, a hover refinement and conditional refinements for breakpoint and `dark:` variants; default theme tables generated from the pinned tailwindcss package |
| Persistent UI graph | `ui/`, `ui/gox/` | Stable nodes, property bindings, keyed/conditional scopes, controlled inputs, settled snapshot publication; typed GoX custom printer |
| Generic wire contract | `proto/gpui/v1/`, `gen/gpui/v1/` | Windows, elements, styles, commands, events, capabilities |
| Core | `rust/core/` | Protocol validation, session/resource/model ownership, view/model revisions, text/UTF-16 helpers |
| Runtime | `rust/runtime/` | Sable handlers, foreground mailbox, event acknowledgement, headless execution |
| GPUI adapter | `rust/native/` | Native windows, rendering, persistent input/focus/list/model entities; resource decoding, canvas, interaction, menus/dialogs |
| Go editor library | `editor/` | Transport-independent client, typed errors, editor handles, lifecycle, standalone Run adapter |
| Editor wire contract | `proto/gpui/editor/v1/`, `gen/gpui/editor/v1/` | Negotiation, instance commands, state/change/close/error events |
| Rust editor component | `rust/zed-editor/` | Embeddable `EditorComponent`, native editing operations, focus, typed change events |
| Editor host | `rust/zed-editor/host/` | Sable/protobuf adapter, ID registry, standalone windows and app lifecycle |
| Combined editor archive | `rust/zed-editor/embed/` | Opt-in GPL staticlib linking the generic backend and editor host, so one `host.Run` serves both protocols |
| Shared UI testing | `uitest/` | Public target contracts, Page/Locator/expect, scenario lifecycle, suite deadlines and worker reports; no client, host or interpreter dependency |
| GPUI test driver | `uitest/gpui/` | Application/factory contracts, native driving and event barriers over the client |
| Starlark adapter | `uitest/starlark/`, `cmd/gpui-e2e/` | DSL evaluation/builtins/value conversion, source diagnostics and CLI wiring consuming `uitest`; `e2e/` forwards its old public API |
| Applications | `examples/counter/`, `examples/counter-gox/`, `examples/showcase/`, `examples/features/`, `examples/reactive/`, `examples/reactiveapp/`, `examples/e2eapp/`, `examples/uitest/`, `examples/editor/` | Application state, presentation, smoke scenarios, E2E fixtures and a Go-only shared-package consumer |

The distribution service in `dist/metadata-proxy/` is a Cloudflare Worker for Go module discovery, release metadata read from each GitHub Release's own `native-release-<goos>-<goarch>.json` asset, and ZIP redirects, and it serves the user documentation built from `docs/` with VitePress from the same `gpui-go.dev` root. It is independent of application runtime startup. Its module list comes from `dist/native-targets.json`, and nothing per release is checked in. Publishing a native Release deploys the Worker through `.github/workflows/deploy-metadata-proxy.yml` with the `NATIVE_VERSION` variable set to Go's `@latest` among published releases; that version alone is listed, and other published versions resolve on request.

`.github/workflows/native-release.yml` runs on manual dispatch and builds, per target in `dist/native-targets.json`, a consolidated static library, Bao `.syso` objects and bindings translated from it, and a Go module ZIP of that output, then checksums everything together and creates a draft Release targeting the dispatched commit, without a tag. The Go module and native releases share version tags: `make release` commits the `go.mod` change that requires the draft's native modules and publishes the draft on that commit, which creates the tag, so tag `vX` requires native `vX`. `check-release-tag.yml` flags a pushed tag that does not. It defaults to Bao v0.0.3's exact commit and uses its Go 1.26.10 verified-linker profile, separately from the ordinary repository Go toolchain pin. Independent signing, completion of transitive notices and final publication remain separate steps, and publication triggers the Worker deployment; the workflow does not select a production trust authority or enable the GPL editor backend.

`internal/nativeshim` is the single Go boundary to that native library, and `host` calls nothing else. With the `gpui` tag it links a local Rust archive through cgo and Sable. `gpui_prebuilt`, which implies `gpui`, instead forwards calls through a generated function table, plus a Go implementation of Sable's completion loop, under `CGO_ENABLED=0`. The shim imports no native module. Each generated `prebuilt/<goos>/<goarch>` package imports one published module, `gpui-go.dev/__binary/<goos>/<goarch>`, and fills the table from `init`; `prebuilt` imports all six. `host` imports `prebuilt` in a file constrained to `gpui && gpui_prebuilt`, so the tags alone select the module, and the root `go.mod` requires all six at the released version. `go mod tidy` in a consumer therefore downloads all six; an ordinary build uses one. The table and packages are generated from the binding manifest and target table. The GPL editor backend keeps its own cgo boundary.

The core, runtime, and generic native crates have independent manifests and lockfiles. The editor component and its host are members of the editor Cargo workspace and share its lockfile. There is no repository-root Cargo workspace.

Go dependencies flow from `scene` to generated protobuf, `client` to `scene`, `host` to `client`/Sable, `tw` to `scene`, and `ui` to `reactive`/`scene`/`client`/`tw`. `ui/gox` adds the GoX runtime. Testing adapters depend on `uitest`; the core does not import its adapters. The root facade uses aliases rather than introducing a second client or snapshot identity. `architecture_test.go` inspects transitive imports with native/automation tags selected so defaults cannot hide native startup coupling.

Public application values are aliases at their owning layer: `scene` exposes styles/geometry/drawing/interaction/accessibility, while `client` exposes events/service values. Concise typed enum constants preserve the generated values and methods. `scene.Ptr` sets optional scalar presence. Aliases introduce no new representation or conversion, so generated callers, serializers and GoX typed attributes interoperate unchanged. UI signatures use this public vocabulary; raw RPC/automation envelopes remain in the generated wire package. The generic examples use no generated-type imports, guarded by `examples_api_test.go`.

## Runtime and ownership

Go goroutines send protobuf requests through Sable. Rust handlers admit requests to a bounded foreground mailbox; GPUI executes native mutations on its main thread and resolves the awaiting Go call. Rendering and native input do not synchronously wait for Go. The long-lived platform loop crosses cgo through an ordinary C call, not Sable's short assembly crossings.

In the generic API, Go owns application state and constructs complete view snapshots. Stable element IDs preserve interaction state across renders. The native adapter owns platform entities and draws the newest accepted snapshot. Click propagation and hover behavior execute synchronously in Rust; callback IDs cross the wire instead of Go closures or pointers.

The optional model API stores opaque application bytes in session-owned native `Entity<ModelState>` instances. Revision-checked updates notify GPUI and enqueue observer events after successful mutation. Go observers choose which snapshots to rebuild; models do not carry Go pointers or implement remote Rust closures. `rust/core/src/features.rs` owns model/resource validation and command routing. Native `resources.rs`, `drawing.rs`, `interaction.rs`, and `platform.rs` adapt validated data to GPUI.

`host.Run` (also exposed as `gpui.Run`) and `editor.Run` each own the process-global Sable runtime. They cannot run together in one process. To combine them, link `rust/zed-editor/embed` with the `gpui_zed_editor_embed` tag: `host.Run` owns the runtime and `editor.Attach` connects to it. Call the selected entry point once from `main`. Native host package initialization locks Go's startup thread; importing `client`, `scene` or `ui` does not. Locking an arbitrary later goroutine does not meet AppKit's process-main-thread requirement. The application callback must return when its context is canceled.

Shutdown closes admission, releases pending calls/event pulls, waits for application work, and shuts Sable down only after active transport calls finish. Cancellation racing a committed command does not roll that command back; clients must not blindly retry mutations.

The pinned macOS GPUI platform implements native quit through `NSApplication.terminate`, which exits the process before `gpui_go_run_native` returns to Go. Native window smoke must record progress before requesting quit; exit status alone does not establish post-loop Go cleanup. The headless loop returns and permits explicit Sable/TLS teardown checks. A maintained Bao host integration must account for this native termination boundary.

## Invariants

### Generic UI contract

* A negotiated session owns its window/view handles. Handles carry generations; stale or foreign-session handles are rejected.
* Element IDs are nonzero and unique within a view. Invalid replacements are rejected before installation. View revisions increase; successful installation does not mean a frame was presented.
* The generic protocol limits messages to 4 MiB, trees to depth 32, and snapshots to 10,000 elements. Capability advertisement must agree with validation and rendering; defining a protobuf variant does not implement it.
* Protocol 1.4 adds `conditional-styles`, `window-size` and `window-color-scheme`. `Element.conditional_styles` refinements are evaluated natively at render time against the window's viewport width and color scheme, after `style` and `hover_style` respectively; Go never re-renders for a resize. An older backend would drop the field as unknown, so the Go client refuses such snapshots unless the capability is advertised. Size and color scheme are separate state: `WindowOpened` carries both, and `WindowResized` and `ColorSchemeChanged` are separate events with separate Go callbacks and `ui` signals. The native view reports each from `render` (refreshing on color scheme changes, which GPUI reports as appearance changes and does not re-render for), the session emits an event only for a changed value, and the event queue replaces an undelivered event of the same kind for the same window, so a live resize cannot overflow it. Delivered but unacknowledged events are never rewritten. Backends without real windows report the requested size with the light scheme and apply resize commands at once.
* Generic event batches are retained until a subsequent pull acknowledges their sequence. One consumer dispatches callbacks sequentially. Removing a handler makes queued events for that handler harmless.
* Single-line fields are persistent native entities. `InputState.Reset` advances a Go-owned epoch; `InputState.Accept` acknowledges native edit revisions. Stale snapshots must not overwrite newer typing, selection, or IME composition.
* Resource handles remain session-owned. Decode before installation; reject release while an installed image node references the handle. Source bytes are bounded to 2 MiB per upload, 32 MiB per session and 128 handles; the native cache separately bounds decoded assets to 64 MiB. Installed fonts cannot be unloaded and remain charged until app exit.
* Models begin at revision 1, reject stale compare-and-swap updates, and notify registered handler IDs after backend success. Bound model values to 64 KiB, live models to 1024, and observers to 128 per model. Explicitly close Go subscriptions; native release invalidates the model handle.
* Keyboard bindings use GPUI's keymap, with private view/element context scopes and user context predicates. Replace registrations when the binding set changes; ordinary view updates must preserve pending chords. Raw key/pointer handlers and propagation run in Rust. Logical menu actions resolve against the currently installed node/binding; revision-stamped pointer events from old snapshots are discarded.
* Uniform lists retain native scroll handles and render visible fixed-height rows. All row snapshots still cross the wire and count toward snapshot limits. Drawing uses bounded declarative commands, with no synchronous Go paint callback.
* Native menus target logical actions; OS path dialogs complete asynchronously through the acknowledged event stream. Limit pending dialogs to eight and menu size/depth before installation. Clipboard/URL/window services remain native-only capabilities; headless execution does not simulate OS effects.

### E2E instrumentation

The test protocol in `proto/gpui/v1/automation.proto` has its own version 1. Requests still use the generic protobuf/Sable boundary, session validation and generational view handles. The native `automation` feature advertises `test-automation-v1`; backends built without it reject requests. Published prebuilt modules enable it. It records element bounds on each render, and requests can only come from Go code built with `gpui_e2e`. Go's `client.Client.Automate` exists only under `gpui_e2e` and is also exposed through the root client alias.

The same opt-in build advertises `test-locators-v1` for live role/name/text/test-ID queries and virtual-row reveal. Shared `uitest.Page`/`Locator` objects re-resolve on each operation, reject ambiguous actions, wait for readiness and expose polling assertions. Snapshot metadata determines selector matches; this is distinct from querying an OS accessibility tree. Readiness currently checks rendered geometry and enabled input state, with final hit testing in GPUI. Command errors are surfaced without retrying a mutation that may already have committed.

Inspection aggregates descendant text for concrete containers, excluding input values. Text selectors match text leaves, while button/checkbox/link names can derive from descendant text when no explicit label exists. This supports container text assertions without making every ancestor a text-selector match.

Instrumented rendering records clipped bounds without changing element IDs or native entity ownership. The target dispatches GPUI pointer/key events, uses the focused platform input handler for text, and explicitly simulates composition through `EntityInputHandler`. It reads native selection/marked ranges and actual text-layout bounds. Dispatch may trigger drawing, so it must never retain a mutable root-view borrow across input.

An inspect barrier observes the published event watermark after earlier native messages. The public `uitest/gpui.NativeTarget` drains that watermark and callbacks' resulting commits until another barrier sees no undispatched events. Scenarios use this synchronization rather than fixed sleeps. Factories adapt example application types to `uitest/gpui.Application`; example presentation has no harness import. `uitest` also owns input helpers, cleanup, suite deadlines and process/report supervision without importing Starlark. The `uitest/starlark` adapter supplies interpreter evaluation, builtins, value conversion and source diagnostics. The CLI and Go-only example consume the same core.

The parent CLI starts one native worker for the suite. Each file has a deadline, an interpreter step budget and independent cleanup. The worker atomically writes its completion report before native shutdown. The parent requires both process success and a successful complete report, because AppKit termination may exit zero without returning through Go. Missing reports and partial runs fail. Native E2E remains a distinct layer from OS input-method, screen-reader and pixel tests.

### Reactive node graph

`reactive` owns a synchronous graph with per-graph dependency tracking. A graph runs on one application goroutine. Signal reads register current computations; reevaluation replaces subscriptions, and memo equality suppresses unchanged downstream effects. Batches drain effects in creation order and update memo dependencies before consumers. Failed computations suppress settled sinks; cycles and runaway effects are bounded errors. Scopes dispose nested computations and cleanup callbacks.

`ui.Document` owns persistent nodes and one active snapshot sink. Property effects change cached protobuf elements, transparent ranges flatten conditional/keyed children, and stable row keys retain IDs. Removed nodes dispose owned branches, handlers and bindings. Inputs adapt native epoch/edit-revision acknowledgements to string signals; native entities retain selection/composition. `Mount.Close` explicitly closes the native window and disposes document ownership. Graph disposal alone is not window closure.

`ui/gox` consumes GoX v0.3.3 typed jobs once to construct that graph. It copies pooled job data before release, preserves explicit text/style/state accessors, and rejects unsupported native tags/attributes or raw HTML jobs. Template presentation stays in examples. There is no Go callback during GPUI painting and no reactive graph crossing the FFI: publication still uses the existing whole-snapshot protobuf contract and limits. This layer adds no dependency on the GPL editor.

`examples/reactiveapp/view.gox` is the source for the example's main/button/row/details components. Generated `view.x.go` is checked in; Go code owns signals and conditional/keyed scope factories. Template generation uses a checksum-pinned standalone GoX v0.3.3 release plus `gofmt`; the compiler and its Rust formatter are tooling dependencies rather than application imports. CI checks source formatting and regenerates in scratch space for byte comparison. The root template's build counter runs inside its generated component so the native scenario observes actual template execution.

`examples/counter-gox/app` is the smaller counter rewrite, with a GoX view and one reactive signal. Its executable lives at `examples/counter-gox`; the CLI adapts the application to its native test driver. Both examples own `generate.go` directives invoking the pinned tool and `gofmt`. Make calls `go generate`, and the shared checker executes copied directives in isolated packages to compare output. Presentation remains in examples, and normal builds use checked-in Go without compiler dependencies.

### Zed editor component

* `EditorComponent` is an `rlib` view with no application loop, window creation, transport, sample content, or application chrome. Its entity can be composed inside another Rust GPUI view.
* Each local buffer owns native text, selection, focus, and undo history. Selection is reported in snapshots and set explicitly by buffer line and visual column, converted in `rust/zed-editor/src/column.rs` to match Zed's layout, with a per-position clamp for columns inside a wide character or tab; moving it does not advance the revision or emit a change. Initial text starts at revision zero with no undo entry. Programmatic edits are separate undo transactions; changed text/read-only state advances that instance's revision.
* Insert/undo/redo respect read-only state. `SetText` is an authoritative replacement permitted in read-only mode.
* The host assigns nonzero editor IDs monotonically and never recycles them within the process. Closing one editor invalidates its handle without quitting the host or affecting other buffers. The host can reopen an editor after its last window closes.
* The standalone editor host bounds text/snapshots to 1 MiB, open editors to 64, commands to 64, and events to 256. The Rust component itself has no bridge size limit. Oversized native input produces an error event; a smaller authoritative replacement can recover the buffer. Event overflow stops the host.
* Editor events have one consumer and no acknowledgement/replay guarantee. Do not apply the generic protocol's replay guarantee to the editor protocol.
* `editor.Run` displays one component per window. An editor placed in a generic snapshot is created in a window, persists across snapshots and while out of the tree, belongs to that window only, and appears at most once per view. Syntax grammars, language servers, and project/file services are not configured.

## Dependency and licensing boundary

The generic core/runtime/native crates use the Apache GPUI backend. The editor component and host are explicitly GPL-3.0-or-later and stay opt-in. Do not introduce a dependency from the generic backend to the editor workspace merely to share a helper. `rust/native/src/embed.rs` is a generic extension point that knows nothing about editors; the dependency runs from the GPL `embed` crate to the backend, and the `embedded-views` capability is advertised only when a provider is installed.

GPUI and Zed application crates use the same pinned revision. Go and Rust use the same pinned Sable revision. Cargo does not inherit an external dependency's root patch table: Rust consumers outside the editor workspace need the documented async-process, async-task, and tree-sitter-language overrides. The editor also pins upstream-compatible asset/Mermaid/websocket versions.

The editor host embeds Zed settings assets in development builds using `util/debug-embed`; otherwise Zed tries to find its own checkout at runtime. GPUI runtime shader compilation avoids requiring the optional Xcode Metal compiler. Native and headless Makefile targets force Go relinking because the Go cache does not track Rust archive contents.

## Verification boundary

Default Go tests need neither native archives nor a display. Core/runtime Rust tests exercise protocol and lifecycle behavior; tagged Go integration tests exercise Sable; native smoke programs exercise actual GPUI/editor commands. Feature smoke also round-trips and restores the real clipboard. Native unit tests use GPUI's test platform for key/focus/action/input dispatch and dialog completion. These layers do not establish actual OS IME, screen-reader, drag/drop gesture or pixel-rendering behavior. See the [testing guide](.agents/docs/TESTING.md) for exact commands and prerequisites.

