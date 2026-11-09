# Testing guide

The Makefile is the command source of truth; toolchains/generators are pinned. First dependency fetch needs network access. Existing caches can be used with `CARGO_HOME="$PWD/.cache/cargo"`, `CARGO_TARGET_DIR="$PWD/target"`, `GOCACHE="$PWD/.cache/go-build"`, and, after fetching, `CARGO_NET_OFFLINE=true`. Do not assume another machine has those caches.

The editor targets intentionally share `target/zed-editor` across sessions. Put logs, fixtures, and scratch scripts in `.agents-workspace/tmp/`.

## Layers

| Layer | Commands | Observes |
| --- | --- | --- |
| Harness | `make harness-check` | Required docs, local link targets, skill metadata, prose punctuation |
| Pure Go | `go test -race ./...` | Client validation/correlation, input versions, event-consumer rules, example state transitions |
| Package boundaries | `go test -run 'TestPackageArchitecture|TestCompatibility' .` | Root aliases/builders and tagged transitive imports; pure libraries/fixtures cannot pull in native startup, and Go-only consumers exclude Starlark |
| Reactive graph and nodes | `go test -race ./reactive ./ui ./ui/gox` | Dynamic dependencies, memo equality, batching, failure/cleanup, keyed reconciliation, input epochs and typed GoX printer |
| GoX example generation | `make test-gox-tools check-gox-generated` | Archive checksum/license checks, formatted templates and byte agreement with isolated execution of maintained go:generate directives |
| Core Rust | `make test-core` | Generic sessions/validation and text helpers |
| Runtime Rust | `make test-runtime` | Mailboxes, cancellation, queues, shutdown |
| Actual Sable | `make integration` | Go/Rust calls with the portable transport, without a display |
| Headless examples | `make headless-smoke`, `make showcase-headless-smoke` | Protocol and application flows |
| Native examples | `make smoke`, `make showcase-smoke` | Actual GPUI installation and registered application commands |
| Native feature example | `make features-smoke` | PNG/SVG resources, model action/observation, focus/list/window/menu commands, real clipboard round trip and restoration |
| GPUI dispatch | `make test-native` | Test-platform focus, keymaps/chords/predicates, keyboard activation, pointer hitboxes, Unicode input handler, menu actions, asynchronous picker completion/cancellation |
| Starlark resolution | `make e2e-check` | Maintained scenario syntax and builtin/name resolution without native startup |
| Native E2E | `make e2e` | Eleven Go/Sable/GPUI scenarios, including shared Page/Locator/expect, semantic queries, strictness, virtual-row reveal, persistent reactive keyed inputs and the GoX counter |
| Native E2E without a display | `make e2e-no-display` on Linux | GPUI's display-less platform: every `make e2e` scenario except clipboard, which that platform does not provide. Real layout, focus, hit testing and input dispatch; no rendered pixels |
| Go UI test consumer | `make uitest` | Same public locator/action/assertion core without Starlark; native fields, model shortcut and checkbox state. Also runs without a display on Linux |
| Component alone | `cargo check --locked --manifest-path rust/zed-editor/Cargo.toml` | Rust library independent of the host |
| Column conversion | `CARGO_TARGET_DIR="$PWD/target/zed-editor" cargo test --locked --manifest-path rust/zed-editor/Cargo.toml -p gpui-go-zed-editor --lib` | Mapping between byte columns and visual columns |
| Editor workspace | `cargo clippy --locked --manifest-path rust/zed-editor/Cargo.toml --workspace -- -D warnings` | Component and host compilation/lints |
| Embedded editor | `make editor-embed-smoke` | One process with both backends: placement in a generic view, persistence across snapshots, rejected placements, independent editors and close paths. Requires the printed pass line because AppKit can exit with status zero. No keyboard or pointer input |
| Native editor | `make editor-smoke` | Independent buffers/initial undo, Unicode/events, editing/read-only, focus/close/stale handles/reopen |

`make test` combines core Rust tests with the Go race suite. Default Go tests use Run stubs and injectable transports; they need no GUI archive or display.

The reactive example is authored in `examples/reactiveapp/view.gox`; edit that file and run `make fmt-reactive` or `make generate-reactive`. The pipeline uses pinned GoX v0.3.3 and then `gofmt`. `make gox-tools` installs a checksum-verified release in ignored `bin/gox` with its license; archive cache is under `.cache/gox/`. First installation needs network access, while normal Go/native builds consume checked-in generated Go and do not need the generator. `check-reactive-generated` works without Git and does not rewrite source/generated files. Keep template expressions explicit about accessor bindings; GoX if/for construction is separate from persistent `ui.Show`/`For` ownership.

Generation commands are declared in `generate.go`: `go generate ./examples/reactiveapp ./examples/counter-gox/app` regenerates both templates, and Make invokes those directives rather than duplicating compiler commands. `make check-gox-generated` copies source/directives into isolated packages and executes `go generate` for byte comparison. `make counter-gox-smoke` checks a programmatic update and event; `counter-gox.star` separately checks actual native click/Enter activation and stable node identity.

## Native prerequisites and evidence

Native validation currently targets macOS/arm64 with Apple's command line tools and a graphical session. Runtime shader compilation avoids the optional Xcode Metal compiler. Disabling that feature changes the prerequisites.

Denied pasteboard/window-server service access inside a sandbox can fail before application code runs. Distinguish that environment error from a protocol bug; use normal permission mechanisms for an authorized GUI run.

Run the selected native entry point once from `main` on the startup thread. `host.Run` (also exposed as `gpui.Run`) and `editor.Run` each own Sable and cannot share one process. Pure libraries import `client`/`scene` directly; importing the compatibility facade with native tags deliberately imports the standalone host.

Native and headless targets force Go relinking with `-a` because its build cache does not track external Rust archive contents. A rebuilt archive plus a cached executable can still execute old native code. Account for this in ad-hoc builds too.

Gallery smoke invokes Go handlers, not physical clicks. Editor smoke issues commands to actual Zed entities, not typing/IME events. For input changes, separately exercise the affected native interaction. Use `make text-inputs`, `make reactive` and `make editor` for manual checks; preserve meaningful Unicode fixtures.

`make features-smoke` uses the actual pasteboard and restores its prior text. Add `-font /path/to/font.ttf` to `./bin/features -smoke` to exercise native font upload. Native unit tests use GPUI's test platform, which validates dispatch without OS dialogs or a screen-reader. They do not prove OS IME composition, actual drag/drop gestures, accessibility behavior or pixel output. The native CI job runs these unit tests and builds the counter, showcase, features and reactive examples; desktop smoke still requires a graphical session.

The [Starlark E2E guide](../../e2e/README.md) describes the opt-in Rust/Go instrumentation and scenarios. `make test-e2e-native` checks compilation and GPUI unit regressions with instrumentation; `go test -race -tags gpui_e2e ./...` checks the Go test API and harness without linking a native archive. CI resolves scripts, tests both native configurations, builds the runner and, on Linux without a display, runs `make uitest` and `make e2e-no-display`. Suite execution on a real display is explicit. This is not the `gpui_headless` build, which has no GPUI and cannot run these suites.

The native E2E parent requires a complete successful worker report as well as a successful process exit. A zero AppKit worker exit alone is insufficient. The deliberately failing `e2e/testdata/assertion-fails.star` followed by `e2e/scenarios/controls.star` must produce a source traceback, a later controls PASS and final status 1. Keep this behavioral negative control separate from the passing suite. Reports live under `.agents-workspace/tmp/e2e-run-*/result.json`.

The shared [uitest package](../../uitest/README.md) owns process reports, lifecycle, suite deadlines and locator semantics. The [GPUI driver](../../uitest/gpui/README.md) owns native input/event barriers and application factories; the [Starlark adapter](../../uitest/starlark/README.md) owns interpreter wiring. Core subprocess tests verify success, failed/missing reports, nonzero exit and timeout behavior. `go list -tags gpui_e2e -deps ./uitest ./uitest/gpui ./examples/uitest` must contain neither the Starlark adapter nor interpreter. The excluded `strictness-fails.star` followed by `locators.star` verifies ambiguous actions fail and later scenarios still run. Assertions retry reads; dispatched mutations must not be retried after a possibly committed command error.

E2E keys/pointers are synthesized inside the native GPUI process. Composition invokes native text-service callbacks, not an installed OS IME. Clipboard scenarios reserve and restore previous text, without preserving rich clipboard formats. Screen-reader/pixel/physical OS gesture and editor typing coverage remain separate.

## Regression selection

* Wire validation and stale handles: Rust core/host behavior and Go interpretation tests.
* Queue overflow/cancellation/shutdown: runtime/integration checks that observe blocked callers being released.
* Input synchronization: Go epoch/revision and Rust text helpers, plus platform input checks when routing matters.
* Native undo/isolation/lifecycle: real editor smoke; a fake transport cannot establish Zed behavior.
* Resources/models/actions: core rejection and lifetime/CAS tests, actual Sable integration, and the native feature smoke for decoding/platform effects.
* A checker: run valid input and a deliberately invalid isolated fixture.

Ask how a test could pass while its claim is false. Use an isolated negative control when sensitivity needs demonstration; a compile error is not runtime evidence. Do not sabotage shared production files.
