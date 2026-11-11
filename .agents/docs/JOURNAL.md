# Journal

Append findings, decisions, and verification here. [LTM](LTM/INDEX.md) holds editable topic knowledge. Ordinary work does not rewrite earlier entries.

## 2026-11-07: Agent harness adoption

Adopted the shared documentation/memory structure and maintenance workflows from pinned Cornus and yesnodb revisions. [HARNESS](HARNESS.md) records source paths and intentional adaptations. Root `./ARCHITECTURE.md` is canonical.

Seeded LTM from current code and existing docs, without importing upstream project histories or claiming a fresh native test. The repository remains a source export without .git; no Git initialization, commit, dependency update, or unrelated source cleanup is part of adoption.

Unicode input fixtures remain intact. The opt-in GPL editor component/host remain separate from the Apache generic backend. Source-specific deployment/site rules and blanket copyleft restrictions were not imported.

Validation evidence will be appended after the harness checks finish.

## 2026-11-07: Harness validation

`make harness-check` passed for 24 maintained documents and nine skills. Four stdlib fixture tests passed: a link whose target is removed is rejected, code/external links remain literal while invalid prose punctuation is detected, a skill/directory name mismatch fails, and unclosed frontmatter fails. Fixtures are isolated under `.agents-workspace/tmp/`.

All nine skills passed the skill-creator `quick_validate.py` validator. Its PyYAML dependency was installed only in an ignored scratch virtual environment, not as a project/runtime dependency. The maintained harness checker itself requires only Python's standard library.

`make -n test editor-lib build-editor harness-check` verified that existing build/test recipes still resolve alongside the added target. The protocol CI job now runs the harness check before toolchain setup. No Go/Rust application code changed, so native builds/smoke and application tests were not rerun for this docs/tooling-only adoption. Local link fragments and external URLs are outside the checker's coverage; source versions and runtime claims were reviewed against the fetched harnesses and current code/Makefile.

## 2026-11-07: GPUI bindings over protobuf and Sable

Retrospective record of the implementation work preceding harness adoption. The entries below describe that work and its recorded verification; they do not claim new test runs during this journal update.

Implemented the generic Go client, generated protobuf bindings, Rust validation core, Sable runtime, and native GPUI adapter. The wire contract describes windows, view snapshots, styles, references, commands, events, and capability negotiation without gRPC. Go owns application state; native entities and rendering remain on GPUI's foreground thread. IDs and serialized messages cross the language boundary rather than Go pointers or callbacks invoked during painting.

The client negotiates a session, validates response correlation, and manages window/view handles and increasing snapshot revisions. Rust validates replacements before installation. Generic event batches remain queued until acknowledged by a later pull; one event consumer dispatches registered Go handlers. Cancellation does not roll back a command that already committed.

Sable is pinned to `0c6fe56eb09937dee354532d14c265a99534e8a2` on both sides. GPUI/Zed crates use `e231594b6b8818ea8d668ee2eb73e1fb09300654`. Core/runtime/native are separate Cargo projects with their own lockfiles, not members of a root workspace. Native startup locks Go's original OS thread and enters the platform loop through an ordinary long-lived cgo call. Shutdown releases pending callers before freeing the Sable runtime.

Earlier validation covered the Go race suite, Rust core/runtime tests, actual Sable integration, and a native counter smoke on macOS/arm64. Counter smoke installed a newer view, received its protobuf event, and quit; it did not automate a mouse click. Current reproduction commands and evidence boundaries are in [TESTING](TESTING.md).

## 2026-11-07: Composed component showcase

Added [examples/showcase](../../examples/showcase/) to exercise a larger Go application: Overview, Controls, Inputs, and Activity pages; metric cards and build rows; filters, selection controls, dropdown/stepper, theme changes, toast/dialog presentation, and background build progress. Buttons and selection controls are composed from the generic layout/text/hover/click primitives rather than bindings to a separate widget toolkit.

Application updates are serialized through the event loop. Background results carry a generation so results from an earlier paused/reset job cannot mutate a newer job. Stable element IDs preserve native interaction state while Go replaces snapshots.

The recorded native and headless showcase smoke checks exercised registered application handlers and background completion. They establish programmatic application/protocol behavior, not physical click routing. Keyboard activation for composed controls and accessibility remain open scope, tracked in [TODO](TODO.md).

## 2026-11-07: Persistent single-line text inputs

Added the `TextInput` element, persistent Rust input entities, and the Go [InputState](../../input.go) mirror. The native implementation adapts GPUI's Apache-licensed input example; attribution remains in [rust/native/NOTICE](../../rust/native/NOTICE).

The important synchronization rule is that a Go echo must not replace newer native typing, selection, or composition. `InputState.Reset` advances a Go-owned epoch and resets the acknowledged edit revision. `Accept` rejects a different epoch or a non-increasing native edit revision. Native entities survive ordinary snapshot replacement through stable element IDs.

Fields support single-line editing with native input handling, grapheme movement, selection, clipboard shortcuts, and composition paths. Line breaks are normalized to spaces and values are limited to 64 KiB. Generic input undo/redo, multiline editing, and password masking are not implemented; the separate Zed editor does not retroactively add those capabilities to this element.

Go input-state and Rust text/UTF-16 tests cover synchronization and text boundaries. The Inputs page exposes live value previews, search filtering, Unicode replacement, clearing, and a disabled field. Automated native typing, clipboard, and IME coverage were not established by the command smoke tests. Preserve combining-character and emoji fixtures as input data rather than normalizing them during documentation cleanup.

## 2026-11-07: Driving Zed's actual editor

The user asked to drive the component behind Zed's editor pane. Built and exercised a local multiline `editor::Editor` with no project or language server, using a separate protobuf command surface over Sable. Zed's application editor crates are GPL-3.0-or-later, so the backend was made explicitly opt-in and kept separate from the Apache GPUI adapter.

Several build/runtime failures produced durable integration guidance:

* Cargo does not inherit the root patch table of an external Git dependency. Without Zed's async-process fork, compilation failed on `Child::adopt_raw_pid`. The editor workspace carries pinned async-process and async-task overrides plus the tree-sitter-language override. An older locked registry copy of tree-sitter-language also had to be resolved to the patched version to avoid distinct `LanguageFn` types.
* Newer dependency resolutions were incompatible with the pinned Zed source. The final manifest constrains `yawc` to 0.4.2, the rust-embed family to 8.11.0, and merman-core/merman-render to 0.8.0-alpha.5. Preserve these compatibility constraints rather than repairing failures with an unscoped dependency update.
* Zed's development settings loader searched for its own checkout and panicked when launched here. The standalone host enables `util/debug-embed` so settings assets are compiled in. Startup panic containment belongs inside the launch callback entered from Objective-C; an outer wrapper around the application loop did not contain the original callback unwind.
* Go reused an executable linked against an older Rust archive after Cargo rebuilt. The editor Makefile target now uses `go build -a`; a Cargo success alone is not proof that the executable contains the changed native code.
* Runtime shader compilation avoided the missing optional Metal compiler. GUI service access denied inside the sandbox was an environment prerequisite, addressed through the normal authorized native-run mechanism rather than application changes or system tool installation.

The initial editor smoke verified Unicode multiline text, change delivery to Go, insertion, native undo/redo, and read-only rejection. That prototype was subsequently replaced by the reusable component/host split described below; its former demonstration-specific toolbar and status commands are not part of the current library contract.

## 2026-11-07: Reusable editor component, host, and Go API

The user clarified that the editor crate must be a reusable library and the application must live under `examples/`. Refactored [rust/zed-editor](../../rust/zed-editor/) into an `rlib` exposing `EditorComponent`, `EditorOptions`, `EditorState`, `Change`, focus support, and editing operations. The component renders the editor without owning a window, application loop, Sable transport, sample text, or application chrome. Native applications can retain its entity and compose it inside another GPUI view.

Moved standalone settings/theme setup, protobuf/Sable handlers, window creation, and the editor-ID registry into [host](../../rust/zed-editor/host/), a separate workspace member. Initialization and optional key bindings are distinct from component construction. Initial text is placed in a local buffer before constructing the native editor, producing revision zero without an undo entry. Persistent native entities own selection, text, and undo history.

Added the public [Go editor package](../../editor/) with injectable `Transport`, negotiation/capabilities, typed protocol errors, concurrent commands, one ordered event consumer, and editor handles. Operations cover open/state, authoritative replacement, insertion, undo/redo, read-only state, focus, close, and host quit. `SetText` is intentionally allowed in read-only mode. Closing an instance invalidates its handle without affecting another buffer or quitting the host; IDs are never reused during that process.

The host limits text/snapshots to 1 MiB, open editors to 64, queued commands to 64, and events to 256. These are transport/host limits, not limits of the Rust component. Oversized native input emits a resource error, and a smaller authoritative replacement can recover. Event overflow stops the host. Editor events do not have the generic runtime's acknowledgement/replay semantics, so cancellation racing receipt can lose an event.

Added [examples/editor](../../examples/editor/) as a consumer of the public Go API. It opens two independent editor windows and logs changes by editor ID. Sample content and native smoke assertions live there, with no cgo or Sable lifecycle code in the application. Current commands are `make editor`, `make editor-lib`, and `make editor-smoke`; the former zed-editor target names remain aliases. The [embedding guide](../../rust/zed-editor/README.md) explains Rust composition and external workspace patch requirements.

The current Go host still presents one component per window. Embedding an editor inside a generic Go snapshot, syntax/LSP/file/project services, full keymap integration, accessibility validation, and Linux/Windows native verification remain outside the implemented scope. Existing backlog entries capture those limits; this refactor does not authorize their implementation.

## 2026-11-07: Reusable editor verification evidence

Recorded earlier in this session, after the component/host split:

| Check | Observed result |
| --- | --- |
| `go test -race ./...` | Passed, including the new transport-independent editor contract tests and existing package tests |
| Component-only `cargo check --offline --locked --manifest-path rust/zed-editor/Cargo.toml` | Passed without depending on the standalone host |
| Editor `cargo clippy --offline --locked --manifest-path rust/zed-editor/Cargo.toml --workspace -- -D warnings` | Passed for component and host |
| `make build-editor` with cached/offline dependencies | Built the host archive and forced a fresh Go link |
| `./bin/editor -smoke` | Passed against actual native Zed entities; final retained log reports success at 09:57:19 on 2026-11-07 |
| Rust/Go formatting and regenerated editor protobuf byte comparison | Passed |

The native smoke checked independent initial text and undo histories, Unicode replacement/change events, insertion, undo/redo, typed read-only rejection, authoritative replacement while locked, focus, close events, stale-handle rejection, continued operation of the second editor, and a new monotonic ID after closing the last window and reopening. The final smoke log and component/Clippy logs were re-read while preparing this retrospective entry. Older logs were written to system temporary storage before the harness convention existed; future scratch logs belong under `.agents-workspace/tmp/`.

Go contract tests cover default options, rejected input without transport dispatch, response/editor-ID validation, typed errors, ordered events, concurrent-consumer rejection, cancellation, and negotiation. Native smoke proves native editing commands and lifecycle; it does not synthesize mouse, typing, clipboard, or IME events.

This journal update changes documentation only. Application tests, dependency fetches, builds, and native smoke were not rerun to write these entries; historical evidence above is explicitly distinguished from validation of the documentation edit.

## 2026-11-07: Backlog reconciled with implementation history

Updated [TODO](TODO.md) against the current code, examples, Makefile, CI, and the implementation entries above. Preserved the six existing follow-up areas, added explicit completion criteria, and separated editor composition/delivery, verification/tooling, and deferred feature decisions. Added concrete host failure-path coverage, opt-in editor CI coverage, generic native archive freshness investigation, and the generic input feature-scope decision. Recorded completed implementation work separately without marking native input automation or unimplemented services as complete.

This is backlog maintenance, not authorization to implement the listed work. No Go/Rust behavior or test expectations changed, and no fresh application/native test result is claimed.

## 2026-11-07: Core GPUI coverage expansion and verification plan

The user requested implementation of the previously identified core API gaps. Work targets the generic Apache backend: resources, drawing, virtual lists, interaction/actions/focus, accessibility, styles/animation, serializable observable models, and platform/window services. This entry records the plan, not a completed gate.

| Failure class | Verification layer |
| --- | --- |
| Invalid drawing/style/interaction metadata installs partial state | Rust core validation and backend-failure atomicity tests |
| Resource references outlive release or alias stale handles | Core resource/session tests and native upload/release smoke |
| Model conflicts overwrite a newer value or observers leak | Core compare-and-swap/observer tests and Go/Sable integration |
| Native keys/clicks target the wrong node or obsolete snapshot | GPUI test-context input dispatch plus core event validation |
| Canvas/assets/list rendering compiles but fails at runtime | Native feature example and actual window smoke |
| Headless backend claims unavailable OS services | Backend capability/unsupported-command tests |
| Go builders retain caller data or discard new event kinds | Go API/dispatch tests |

Keep platform services explicit, resource/model/event queues bounded, and all drawing/input dispatch decisions on the native foreground thread. Preserve the editor GPL separation and distinguish GPUI test-context input simulation from real OS/IME automation. Final evidence and any remaining boundaries will be appended after validation.

## 2026-11-07: Core GPUI coverage expansion results

Implemented generic protocol minor 3 and reusable Go APIs for uploaded raster/SVG/font resources, declarative rectangle/path drawing, uniform-height virtual lists, native focus and interaction, logical actions and GPUI keymaps, accessibility metadata/actions, extended layout/styles, opacity animation, observable serializable models, native menus, asynchronous path prompts, clipboard/URL services and window commands. Added `examples/features/` as a consumer; application content and smoke assertions stay there. Main composed showcase controls now expose native focus and accessibility semantics.

The core owns bounded resource/model registries with generational handles. Model values remain opaque bytes with compare-and-swap revisions; successful native changes emit acknowledged observer events. Native entities own input/focus/list state; Go callbacks never run during painting. Assets decode before publication, installed image references block release, and native font residency continues until exit. Menus reference current logical actions; path prompts enqueue completion/cancellation events without blocking the foreground loop. Headless execution rejects native platform services, including window-service commands.

Keyboard bindings use GPUI's actual keymap engine with private view/element scopes, context predicates and multi-stroke chords. Rebuild registrations only when their contents change. A native regression commits an unrelated snapshot between chord strokes and observes the completed action; malformed predicates are rejected before changing the installed view. Pointer tests dispatch native GPUI input inside and outside the hitbox. Unicode input tests exercise the actual input handler, including a stable ID changing from a generic node to an input.

| Check | Observed result |
| --- | --- |
| Go build, vet, race tests | Passed for all packages; default tests remain display/native/network independent after dependency fetch |
| Rust core tests | Passed: 2 text tests, 10 session/model validation tests, 6 new feature/lifetime/CAS/error tests |
| Rust runtime tests | Passed: 6 mailbox/cancellation/event/shutdown tests |
| Actual Sable integration | Passed with forced relinking, including model observation/conflicts, resource lifetime, action delivery, focus/list commands, input synchronization and shutdown |
| GPUI native unit tests | Passed: 9 tests for key/raw-key dispatch, replacement/focus, keyboard activation, pointer hitboxes, chords/predicates, pending chords through commit, Unicode input, menu dispatch, and async picker completion/cancellation |
| Rust Clippy | Passed for core, runtime and native with `--all-targets -- -D warnings` |
| Native release builds | Counter, showcase and features built with pinned dependencies, `--locked`, default runtime shaders, and forced Go relinking |
| Native feature smoke | Passed at 11:11:54 with real PNG/SVG decoding and local Arial font upload, model action/observation, focus/list/menu/window commands, clipboard text round trip/restoration and release |
| Counter desktop smoke | Passed at 11:11:57: installed revision 2 and received its protobuf event |
| Showcase desktop/headless smoke | Both passed: 25 controls, background progress and 46 view commits |
| Formatting and generation | Go/Rust formatting passed; isolated protobuf regeneration byte-matched all checked-in Go bindings |
| Harness validation | Passed: 24 documents, 9 skills and 4 checker regressions |

Initial native test iterations exposed a mismatched test closure binding and an Enter simulation that omitted key-up; both were corrected before passing the dispatch gates. Clippy exposed new nested-condition/style issues and an existing intentionally reversed text-range fixture. Simplified the new code and expressed that fixture as explicit range data without changing its assertion. No assertions or warnings were disabled. Native linking still emits the existing harmless duplicate-library warning.

Reproduced external archive cache behavior in an isolated cgo probe under `.agents-workspace/tmp/`: archive version 1 returned 1; rebuilding only the archive to version 2 and running ordinary `go build` still returned 1; `go build -a` returned 2. Counter/showcase/features builds and headless integration/smoke targets now force linking, as the editor already did. Scratch evidence remains in the ignored workspace, with no experimental API added to the libraries. CI now runs GPUI native unit tests and builds the feature example alongside counter/showcase.

Added only the pinned, already-transitive `image` 0.25.10 as a direct native dependency (MIT OR Apache-2.0) and the same pinned GPUI as a test dependency with `test-support` (Apache-2.0). Inspected their actual package manifests. Zed/Sable revisions remain matched; no GPL editor dependency was introduced into the generic backend. This is an affected-dependency review, not a full distribution license audit.

Limits remain explicit in README and TODO: opacity-only animation; fixed-height lists with all row snapshots supplied up front; declarative canvas paths/rectangles; opaque model values; no generic styled text runs, GPU shaders, snapshot patches, generic multiline/undo/password input, or generic GPL editor embedding. Native unit tests use GPUI's test platform. The desktop smoke allows painting but asserts no pixel output, and menu installation is separate from actual OS menu selection. OS IME composition, screen-reader behavior, physical drag/drop, editor OS typing, and Linux/Windows native runtime remain unverified. No editor component/host behavior changed, and its native gate was not rerun.

The workspace is a source export without `.git`; generation was checked by isolated byte comparison, without initializing Git or claiming a Git diff. Updated README, architecture, operational docs/LTM and TODO to match the implemented surfaces and verification limits. Logs are under `.agents-workspace/tmp/`.

## 2026-11-07: Work summary and findings

At the user's request, recorded this compact handoff for the completed core feature expansion. The detailed implementation and verification entry above remains the primary evidence record.

Work completed:

* Extended the generic protobuf contract to minor 3 and regenerated Go bindings. Added reusable APIs in `features.go` and `model.go`, core validation/ownership in `rust/core/src/features.rs`, and native resource, drawing, interaction and platform adapters under `rust/native/src/`.
* Implemented images/SVG/fonts, declarative canvas drawing, uniform virtual lists, keyboard/pointer/focus events, actions and shortcuts, accessibility metadata/actions, extended styles and opacity animation, observable native models, menus, asynchronous path dialogs, clipboard/URL services and window commands.
* Added `examples/features/` with interactive controls and an explicit native smoke mode. Updated composed showcase controls, Makefile targets and native CI coverage. Preserved the separation between the generic Apache backend and opt-in GPL editor.
* Updated README, architecture, testing/quality guidance, LTM and TODO with implemented behavior, limits and remaining work. Existing journal entries were preserved.

Findings to preserve:

* Use GPUI's keymap engine for context predicates and multi-stroke chords. Replacing bindings on every snapshot clears pending chords; compare registrations and rebuild only when bindings change. Native tests now cover an unrelated commit between chord strokes.
* Persistent native entities must own focus and input state. Stable IDs also need correct handling when an element changes kind; the Unicode regression covers a generic node becoming a text input.
* Validate/decode before publishing state. Resource release must reject installed references; model updates must reject stale revisions and notify observers only after backend success. Font release removes the logical handle but cannot unload the native font face.
* An external Rust archive change does not invalidate Go's build cache. An isolated cgo probe reproduced stale code after ordinary `go build`; `-a` picked up the new archive. Generic native and headless recipes now force relinking.
* Native command smoke, GPUI test-platform input dispatch and actual OS interaction establish different behavior. Headless execution must reject unavailable platform services rather than report a successful OS effect.

Verification from the preceding implementation work: Go build/vet/race tests, 18 core Rust tests, 6 runtime tests, actual Sable integration, 9 GPUI native tests, affected Clippy/formatting checks, isolated generated-binding comparison and native example builds passed. macOS/arm64 feature smoke exercised PNG/SVG and a supplied font, model observation, focus/list/menu/window commands, clipboard text round trip/restoration and release. Counter and showcase desktop smokes and showcase headless smoke also passed.

Retained evidence is under `.agents-workspace/tmp/`, including `features-integration-final.log`, `native-dispatch-tests.log`, `features-smoke-final.log`, `features-counter-smoke-final.log`, `features-showcase-smoke.log`, `features-showcase-headless.log` and `clippy-features-final.log`. These logs were re-read for this summary; application builds/tests and native smokes were not rerun for this documentation-only request.

Remaining scope and verification gaps are tracked in [TODO](TODO.md): actual OS IME/screen-reader/drag-drop behavior, editor platform typing and generic embedding, Linux/Windows native validation, and advanced primitives beyond the current declarative drawing, opacity animation and fixed-height list contract. Pixel output and physical OS menu selection are not established by the current smoke tests. Start the interactive feature example with `make features`; use `make features-smoke` and `make test-native` for their respective verification layers.

## 2026-11-07: Starlark native E2E harness

Implemented the user's requested Cornus-style E2E harness. Inspected the clean local Cornus harness/CLI at revision `6997fb55d23f209b5afec6afc74b1f3b14ebe7f4`; GitHub page fetches were unavailable, so implementation evidence came from the local source. Adopted scenario files, Go builtins, replaceable targets, matching parse/resolve/execution settings, per-file cleanup and explicit assertions. The GPUI implementation is independent of Cornus's infrastructure dependencies. Its [guide](../../e2e/README.md) records commands, API, provenance and evidence limits.

The reusable `e2e/` package runs Starlark against a target/application interface; `examples/e2eapp/` owns workbench presentation, models, sample data, selectors and fixture updates. `cmd/gpui-e2e/` provides check mode, per-file deadlines, source backtraces and native parent/worker execution. Starlark is pinned to `v0.0.0-20260708150628-5395d018f003`, with `golang.org/x/sys` 0.42.0. Inspected Starlark's actual BSD-style license; this is an affected-dependency review, not a full distribution audit.

The independently versioned automation protocol uses the existing protobuf/Sable/session/generational-ID boundary. Rust `automation` and Go `gpui_e2e` are explicit opt-ins; normal backends reject requests and do not advertise `test-automation-v1`. Instrumented rendering captures clipped bounds while preserving IDs/entities. Pointer/key requests dispatch through GPUI; text uses its focused platform input handler; composition explicitly invokes native text-service callbacks. Inspection exposes actual native text, UTF-16 selection/mark ranges, text-layout bounds and focus. Event barriers drain published watermarks and callbacks' resulting commits without fixed sleeps. The target keeps its receipt cursor aligned with whole batches even when a callback fails.

| Failure class | Observable verification |
| --- | --- |
| Script typos or language options differ between check/run | Maintained scripts resolve; default Go harness tests cover undefined names, top-level control and reassignment |
| Assertions/timeouts leak applications or continue actions | Harness regressions verify cleanup, stop-after-failure, independent cleanup context and interpreter step budget |
| Native pointer/key routing fails while direct handler invocation succeeds | Native scenarios click hitboxes/misses, activate buttons, complete chords, edit fields and assert native plus Go state |
| Focus/selection/marked text is replaced by Go echoes | Unicode, mouse selection, composition and epoch-reset scenarios inspect persistent native state |
| Virtual scroll command acknowledges without showing the row | The list scenario requires the last row to be prepainted and then receives its native click |
| A zero native worker exit hides a failure or incomplete suite | Parent requires an atomic complete/successful report; missing/malformed/incomplete/error reports are rejected; an intentional native assertion exits 1 |
| Test commands leak into normal backends or accept stale IDs | Core version/session/view/element rejection tests, tagged Go capability/result checks and both native build configurations |

Eight maintained scenarios cover controls/model observation, Unicode text/selection and mixed-control Tab order, simulated composition, disabled input, mouse drag selection, clipboard shortcuts/normalization, virtual lists and independent windows/reopening. The failing assertion fixture lives under `e2e/testdata/` and is excluded from the passing glob.

The scenarios found and drove fixes for three existing native input defects:

* An extra ancestor mouse-focus listener overrode the focus assigned by GPUI to a descendant input. Removed the redundant listener and rely on GPUI's default focus/prevent-default behavior.
* Explicit tracked focus handles had no Tab-stop flags; Div flags alone did not configure those handles. Configure native handles, preserve disabled exclusions and install root fallback navigation for mixed composed controls.
* Disabled fields still participated in GPUI's automatic click focus. Prevent default in the disabled field's capture phase. A disabled click now preserves another field's focus and editing.

Added GPUI unit regressions for ancestor focus, mixed-control Tab/Shift+Tab navigation and disabled click focus. Initial harness dispatch retained a mutable root entity while GPUI could redraw, producing a native borrow panic; dispatch now holds cloned handles and no root lease. The mouse-selection scenario was corrected to use native text-layout bounds instead of assuming container padding; its selection assertion was retained.

The native negative control exposed a separate runner hazard: AppKit's terminate path can exit zero without returning through Go. A parent now starts one native worker, and the worker atomically writes a completion/error report before shutdown. The parent owns the exit status and treats missing reports as failures. The deliberate control observes count 1 versus expected 99 after a real native click, reports its source line, cleans up, runs the following controls scenario successfully, and exits 1. It must remain a behavioral failure check, not a compile-failure proxy.

Added `make e2e-check`, `build-e2e`, `e2e` and `test-e2e-native`, with forced Go relinking and a named E2E archive copy under the existing target directory. CI resolves scenarios, runs both native unit configurations and builds the runner; graphical scenario execution remains explicit. Updated README, architecture, operational docs/LTM and TODO. Final gate results are recorded below after the last ordinary-backend smoke checks.

## 2026-11-07: Shared UI test core and locator API

The user requested extraction of the interpreter-independent E2E core and chose a public package within the existing module. Added `github.com/moriyoshi/gpui-go/uitest` for application/target interfaces, native input and event barriers, scenario cleanup, suite deadlines, input helpers, process supervision and atomic version-1 completion reports. The Starlark adapter now consumes this API rather than owning those facilities. Fixtures depend on `uitest.Application`; the Go-only consumer under `examples/uitest/` exercises the same public package without Starlark in its dependency graph. No separate module or external publication operation was introduced.

Moved process/report tests into the shared package and added actual controlled subprocess cases for successful completion, failed reports, missing reports, nonzero process exit and timeout. Public callback scenarios preserve cancellation and cleanup errors, including a callback that incorrectly returns nil after its deadline. Raw input helpers copy caller metadata. CI builds both the Starlark runner and Go-only consumer.

The user then requested a Playwright-like API. Added interpreter-independent `Page`, live `Locator`, `Keyboard` and `Expect` types in `uitest`. Core behavior includes role/name/text/test-ID queries, strict single-element actions, deliberate ordinal selection, readiness/timeout diagnostics, virtual-row reveal, fill/press/focus/hover, idempotent checkbox operations and retrying read assertions. Common key names such as `ControlOrMeta+A` and `Shift+ArrowRight` are translated in Go. The adapter exposes these as `open_page`, page/locator methods and `expect(locator).to_*` without implementing their semantics again.

The opt-in native protocol advertises `test-locators-v1`. Queries inspect the current snapshot and preserve opaque IDs; native inspection includes enabled/checked/name/role metadata. Text queries exclude input values and normalize whitespace without Unicode normalization. Revealing a virtual row uses persistent native list handles. A dispatched mutation is never retried after a command/application error because cancellation can race commitment. Readiness checks rendered geometry and native input disabled state; animation stability and obstruction are not prechecked. This is a native bridge API, not browser/CSS compatibility or an OS accessibility-tree query.

Inspected the official Playwright locator and actionability documentation when choosing the API shape. The new `locators.star` scenario exercises semantic lookup, strict count assertions, fill/press, checkbox idempotence, current snapshots and automatic virtual-row reveal. The Go-only consumer uses the same high-level API. The excluded strictness control attempts to fill three matching textboxes and fails before dispatch; the following locator scenario passes and the parent exits 1. The earlier wrong-count assertion control also continues to exit 1 after cleanup.

Verification completed before starting the next requested engine work:

| Check | Result |
| --- | --- |
| Go build/vet and default race tests | Passed for all packages |
| `go test -race -tags gpui_e2e ./...` | Passed, including test API, shared lifecycle/worker/locator tests and adapter tests |
| Package dependency graph | `uitest` and `examples/uitest` exclude the Starlark adapter and interpreter |
| Rust core/runtime and actual Sable integration | Core validation and 6 runtime checks passed; `make integration` passed with fresh linking |
| Native GPUI tests | 12 ordinary-build tests and 13 instrumented tests passed; the added query test covers roles/names, normalized text and exclusion of input values |
| Native Clippy | Passed with instrumentation and all targets; ordinary native, core and runtime checks also passed during the harness work |
| Scenario resolution | All 9 maintained scenarios resolved without native startup |
| Native Starlark suite | All 9 scenarios passed; report under `.agents-workspace/tmp/e2e-run-729451658/result.json` |
| Go-only native consumer | Passed; report under `.agents-workspace/tmp/e2e-run-1341234301/result.json` |
| Strictness negative control | Expected status 1, source traceback for 3 matching textboxes, then locator scenario PASS |
| Ordinary feature/showcase smoke | Passed after the input fixes, including clipboard text round trip; showcase reported 25 controls and 46 commits |
| Formatting and generated bindings | Passed; isolated regeneration byte-matched checked-in bindings |
| Harness docs | Passed: 24 documents, 9 skills and 4 checker regressions |

Logs use `locator-*`, `uitest-*` and `e2e-*` under `.agents-workspace/tmp/`. One dependency-graph check initially used the unwritable default Go cache; rerunning with the documented local `GOCACHE` passed. Native link commands retain the existing duplicate-library warning. The workspace remains a source export without Git metadata. Synthetic GPUI input, simulated composition, OS input methods, screen readers and pixel verification remain explicitly distinct. No GPL editor component/host changes or native editor rerun were required.

## 2026-11-07: Reactive node graph and GoX native printer

Completed the shared harness/locator task above before starting the user's requested SolidJS-like node graph. Inspected [GoX](https://github.com/doors-dev/gox) at `508309b766d1cb47171b247a98b8db44e81c9146` (v0.3.3), including its actual cursor/printer implementation and MIT license. GoX emits typed jobs through a replaceable printer; it is not itself a signal/dependency engine. Its pooled jobs must be consumed and released once. This is a useful native construction boundary without rendering HTML or linking the template compiler/formatter. Also consulted Solid's official fine-grained reactivity and batching documentation for dependency/ownership behavior.

Added three reusable public packages:

* `reactive`: typed signals, equality-aware memos, effects, dynamic subscriptions, nested batches, untracked reads, owner/run scopes and cleanup, deterministic effect scheduling, failure recovery, cycle detection and bounded runaway errors. Each graph belongs to one goroutine and has its own tracking context. Memo dependencies update before consumers; source writes do not expose diamond glitches. Cleanup reads do not subscribe an evaluating parent.
* `ui`: persistent nodes with stable IDs, property bindings, conditional scopes, keyed rows and controlled single-line inputs. Native acknowledgements use `InputState.Accept`; authoritative replacements use `Reset`. Document disposal also releases nodes constructed with other owners in its graph. One active snapshot sink publishes after successful stabilization; errors from a possibly committed sink remain visible without automatic mutation retries.
* `ui/gox`: a GoX v0.3.3 native printer that constructs the graph once, copies typed attributes before pooled job release, preserves explicit text/style/state accessors and embeds existing node ranges. Supports native containers/text/buttons/inputs and typed protobuf styles/accessibility. Unknown tags/attributes, raw HTML/byte/templ output and unwrapped function children fail explicitly; failed builds dispose partial ownership.

Pinned the direct GoX dependency to v0.3.3; its module requires `golang.org/x/sys` 0.45.0, replacing the prior 0.42.0. Cached/offline `go mod tidy` passed. Dependency inspection confirms the printer imports only GoX's runtime and internal utilities, and the shared `uitest`/Go-only consumer still excludes both Starlark and its adapter. This was an affected-dependency review, not a full distribution license audit. No Rust dependency revisions or protocol declarations changed, and no GPL editor linkage was added.

Added the interactive `examples/reactive/` entry point and application/fixture under `examples/reactiveapp/`, with `make reactive`/`build-reactive` and native CI build coverage. The example uses ordinary GoX cursor operations, the same runtime interface produced by compiled templates; template generation is not required. It combines a counter, two-write batch, a memo-backed conditional branch, Unicode name input and keyed editable rows with add/remove/reverse controls. Presentation and fixture diagnostics remain under examples.

The tenth maintained Starlark scenario exercises this application through the existing public Page/Locator/expect API. Native inspection now aggregates descendant text for container assertions while excluding input values. Text selectors continue to match leaves; unlabelled buttons/checkboxes/links can derive names from their child text, without giving generic ancestors the entire tree's accessible name. Extended the native query regression for those distinctions.

| Failure class | Verification |
| --- | --- |
| Unrelated reads rerun bindings or stale branches remain subscribed | Go dynamic-dependency/cleanup tests and native binding execution counters |
| Two writes publish intermediate derived values or equal memos propagate | Go diamond/batch/equality tests; native batch changes the counter once while parity-dependent text does not rerun |
| Cleanup reads create accidental dependencies | Dispose a temporary child during an effect, then change the cleanup-read signal; the parent must not rerun |
| Failed or runaway calculations publish a partial frame | Settled-sink tests retain errors across explicit flushes and recover only after a relevant input change |
| Multiple sinks consume shared dirtiness or old detach functions remove replacements | Reject competing sinks before dispatch; stop/reattach tests observe all later frames |
| Document disposal leaves externally owned nodes/handlers alive | External-owner binding/name regression and node/handler cleanup tests |
| Keyed reconciliation recreates rows, leaks removed scopes or partially accepts duplicate keys | Go tests inspect child order, IDs, replacement text, computation removal and duplicate-key failure/recovery |
| A reorder test passes because nothing moved | Native scenario requires the row's rendered Y position to change, then checks the same ID, selection and focus and types into the retained input |
| Native edit acknowledgements reset fields or obsolete events overwrite resets | Go input epoch/revision tests plus native Unicode editing and retained row values after reorder/removal |
| Templates rerun, escape native text or leave bindings after failure | Actual GoX printer tests count template calls, preserve literal text and reject raw output with zero retained computations |

Final verification:

| Check | Result |
| --- | --- |
| Go build/vet/default race and `gpui_e2e` race suites | Passed for all packages, including 14 new graph/node/printer tests |
| Scenario resolution | All 10 maintained scenarios passed `make e2e-check` |
| Native E2E suite | All 10 passed; report `.agents-workspace/tmp/e2e-run-2592416625/result.json` |
| Go-only native harness consumer | Passed; report `.agents-workspace/tmp/e2e-run-1761485199/result.json` |
| Assertion and strictness negative controls | Each returned the expected status 1 with source diagnostic, cleanup and a subsequent scenario PASS |
| Native GPUI unit tests | 12 ordinary tests and 13 instrumented tests passed; final instrumented run includes aggregate-name/text regression |
| Native Clippy and formatting | Instrumented all-target Clippy with warnings denied and manifest formatting passed |
| Native builds | Freshly linked E2E/Go-only consumers and ordinary reactive example passed; existing duplicate-library link warning remains |
| Dependencies | Offline tidy and runtime/interpreter exclusion checks passed |

Evidence uses `reactive-*` logs under `.agents-workspace/tmp/`, including `reactive-go-gates.log`, `reactive-native-gates.log`, `reactive-native-gates-final.log`, `reactive-suite.log`, `reactive-go-consumer.log`, `reactive-negative-assert.log`, `reactive-negative-strict.log` and `reactive-example-build.log`. Public/package guides, canonical architecture, operational overview/testing/gates, LTM/index and TODO were updated. The final harness documentation check is recorded below after these entries are installed.

Fine-grained work currently applies to Go calculations and cached properties; the wire still sends complete protobuf snapshots with existing limits. This is a synchronous native component foundation rather than full Solid/browser compatibility: stores/proxies, async resources, transitions, hydration and DOM/CSS semantics are outside its current scope. Native evidence remains macOS/arm64 with synthetic GPUI events; installed OS IME, accessibility output and pixel correctness remain separate. The GPL editor and unchanged core/runtime transport were not rerun for this task. No commit, push or external publication was performed in this source-export workspace.

Final documentation gate passed: `make harness-check` validated 25 documents and 9 skills, and all 4 checker regressions passed. `gofmt -l` produced no output for the changed reactive/UI/example/CLI sources. Retained output: `.agents-workspace/tmp/reactive-doc-gates.log`.

## 2026-11-07: Work summary and findings for shared testing and reactive components

At the user's request, appended this handoff for the completed shared testing, locator API and reactive engine work. The preceding implementation entries retain the detailed evidence and design decisions.

Work completed:

* Extracted `github.com/moriyoshi/gpui-go/uitest` as the public Go harness core within this module. Application driving, event barriers, cleanup, deadlines and native worker reports are independent of Starlark. Both the Starlark adapter and `examples/uitest/` consume it.
* Added shared Playwright-style Page, live Locator, Keyboard and Expect APIs, including semantic queries, strict actions, readiness, virtual-row reveal and polling assertions. The Starlark layer only adapts arguments and values.
* Built `reactive`, `ui` and `ui/gox`: typed signals/memos/effects, batching and owner cleanup; persistent nodes, conditional branches, keyed rows and controlled native inputs; and a GoX v0.3.3 custom printer that builds components once.
* Added `examples/reactive/`, its application fixture and a tenth native E2E scenario. Updated public guides, architecture, Makefile/CI, operational docs, LTM and TODO. Run `make reactive` for the interactive example, `make e2e` for Starlark scenarios and `make uitest` for the Go-only harness consumer.

Findings to preserve:

* GoX provides typed construction jobs, while our packages own reactivity and native synchronization. Copy pooled job data before releasing it once; retain accessor functions explicitly rather than formatting their current values during construction. GoX's compiler/formatter is absent from the application dependency graph.
* Track actual reads on each evaluation and remove obsolete dependencies. Memo equality suppresses unchanged downstream work; update dependencies before consumers to avoid intermediate diamond results. Cleanup callbacks must run untracked, and hidden/deleted branches must dispose their computations and handlers.
* Stable keys and IDs preserve native input entities only when edit acknowledgements and authoritative resets remain distinct. The native reorder test verifies movement in rendered bounds, then retained identity, focus and selection, followed by further typing. Checking an unchanged ID alone could pass even if reordering did nothing.
* Failed/runaway computations must keep blocking publication until recovery. One document supports one active sink; competing sinks could consume shared dirtiness. Surface uncertain sink failures without automatically retrying a mutation that may already have committed.
* Live locators re-resolve after updates. Retry observations and readiness checks, while surfacing dispatched mutation errors directly. Container text assertions include descendant text; text queries match leaves and exclude input values.
* Native worker exit status alone is insufficient on AppKit. Require an atomic complete/successful report as well. Keep intentionally failing assertion and ambiguity controls outside the passing suite, and verify cleanup, continuation and final status 1.

Re-read the retained `reactive-*` evidence for this summary: all 10 native scenarios and the Go-only consumer passed; default and `gpui_e2e` Go race suites passed; 12 ordinary and 13 instrumented native unit tests passed; native Clippy/formatting and native builds passed. The assertion and strictness controls produced their expected diagnostics and subsequent scenario PASS. Dependency checks confirmed that the Go harness consumer excludes Starlark and the GoX adapter excludes compiler/formatter packages. Reports include `.agents-workspace/tmp/e2e-run-2592416625/result.json` and `.agents-workspace/tmp/e2e-run-1761485199/result.json`.

Bindings update selectively in Go, but transport still sends whole protobuf snapshots under the existing limits. The graph is synchronous and belongs to one application goroutine. Full Solid/browser facilities, snapshot patches and generic GPL editor embedding remain outside the implemented scope. Native evidence is macOS/arm64 with synthetic GPUI input; installed OS IME, screen-reader, pixel and other-platform validation remain separate. Existing [TODO](TODO.md) and package guides record those boundaries. No external release, commit or push was performed.

This request changes only the journal. Runtime tests and native applications were not rerun; their results above come from the preceding implementation's retained logs.

Documentation verification for this append: `make harness-check` passed, including 25 documents, 9 skills and all 4 checker regressions.

## 2026-11-07: Go package structure refactor

Implemented the package plan approved by the user, within the existing Go module. [README migration guidance](../../README.md#packages-and-migration) and [canonical architecture](../../ARCHITECTURE.md) describe the resulting boundaries.

* `scene` owns protobuf snapshot builders, style/interaction helpers and input epoch/revision state. `client` owns transport negotiation, sessions, windows/revisions, event dispatch, resources, models and platform commands. Split the old mixed feature implementation by responsibility.
* `host` owns standalone Run, cgo/Sable, process startup-thread locking, desktop/headless loop selection and shutdown. Native tags, C entry points, platform frameworks, runtime guard and pending-call shutdown ordering are preserved. Root `gpui` aliases canonical types and forwards constructors/Run, retaining existing root entry points without duplicate state.
* `ui` imports `client` and `scene` directly. `reactive` remains independent of GPUI, and `ui/gox` remains the typed construction adapter. Pure libraries no longer transitively import the root facade or native startup under native build tags.
* `uitest` retains target contracts, Page/Locator/expect, input helpers, scenario cleanup/deadlines and worker supervision. `uitest/gpui` owns the native driver and application/factory contracts; `uitest/starlark` owns interpreter wiring. The old `e2e` Go package is a deprecated forwarding API; scenario/testdata paths remain unchanged.
* Example constructors return their own exported `*App` values with no test-harness import. CLI and Go-only test entry points supply factories adapting those applications to `uitest/gpui.Application`. Presentation remains under examples.

Migration details matter: root types are aliases, so client/window/resource/snapshot identities interoperate across the facade and canonical imports, while reflection now reports the canonical defining package. Root `Image` retains its resource argument; `scene.Image` takes and copies an opaque handle from `resource.Handle()`. Moving resource ownership into `scene` would create a reverse dependency on the client. The explicit source import change is moving `uitest.Application`, `Factory`, `NativeTarget` and `NewNativeTarget` to `uitest/gpui`. Forwarding the driver from the shared core would create an import cycle because that adapter consumes core contracts. Shared locator role parsing stays in the core through `ParseRole`, avoiding duplicated adapter vocabularies.

Existing behavior tests moved with their owners: private client tests to `client`, snapshot ownership tests to `scene`, Sable integration to `host`, and interpreter tests to `uitest/starlark`. The moved interpreter resolution test now locates the unchanged scenario directory. The integration test uses a test-only raw request helper to prove actual Rust stale-handle rejection without exporting a new production client method.

Added facade interoperability and nil-Run regressions, plus a dependency regression checking 12 actual transitive package graphs with native/automation tags selected. The policy distinguishes reusable libraries, interpreter adapters and the executable that deliberately imports the host. It rejects reverse package dependencies, native startup/cgo in pure libraries, Starlark in the Go-only consumer and GoX compiler packages. An isolated source overlay injected a blank root import into `ui`: compilation succeeded, and the architecture test failed on the resulting root/host/Sable dependency path. The overlay did not modify production source. Evidence: `.agents-workspace/tmp/refactor-architecture-control/result.log`.

The user also asked whether GoX templates can build a reactive node graph. `ui/gox.Build` already accepts a `gox.Elem` component and constructs persistent nodes once. Explicit accessor values such as `nativegox.Text(func() string { return count.Get() })` retain reactive reads; directly evaluating a value during construction captures its current state. The example uses cursor calls, and template source generation is not integrated into this repository. This refactor preserves that interface and does not add a compiler workflow.

Verification completed:

| Check | Result |
| --- | --- |
| Go build/vet/default race and `gpui_e2e` race suites | Passed for all packages; retained original behavior tests and added boundary/facade regressions |
| Pure libraries with `CGO_ENABLED=0` | Client, scene, reactive, UI, GoX adapter, shared test core and Starlark adapter passed |
| Tagged dependency check and overlay negative control | Valid graph passed; injected native dependency produced the expected test failure |
| `make integration` | Passed with fresh linking, including all 6 runtime tests and actual Go/Sable/Rust protocol/input/model/shutdown behavior through the relocated host |
| Scenario resolution | All 10 scripts passed `make e2e-check` |
| Native Starlark suite | All 10 passed; report `.agents-workspace/tmp/e2e-run-105130728/result.json` |
| Go-only native consumer | Passed; report `.agents-workspace/tmp/e2e-run-708079365/result.json` |
| Assertion/strictness negative controls | Expected status 1, source diagnostics, cleanup and a subsequent scenario PASS in both cases |
| Native builds | Freshly linked runner, Go-only consumer, counter, showcase, features and reactive example passed |
| Ordinary desktop command smokes | Counter observed revision 2; showcase observed 25 controls and 46 commits; features exercised PNG/SVG, models/actions/observation, focus/list/menu/window commands, resource release and clipboard text restoration |
| Formatting | `gofmt -l` reported no changed Go sources |

Retained logs use `refactor-*` under `.agents-workspace/tmp/`: `refactor-go-gates.log`, `refactor-boundaries.log`, `refactor-integration.log`, `refactor-native-build.log`, `refactor-suite.log`, `refactor-go-consumer.log`, `refactor-negative-assert.log`, `refactor-negative-strict.log`, `refactor-examples-build.log` and the three ordinary smoke logs. A recursive Go package scan initially overlapped Cargo writes and reported a disappearing metadata file under `target/release/deps/`; sequencing those phases passed. One scratch smoke supervisor expected a nonexistent showcase message; its process exited zero and the actual existing success message was verified from source/logs, without changing application assertions or rerunning a completed smoke. The existing duplicate-library linker warning remains.

Updated package guides, migration instructions, architecture, overview/testing/quality guidance, LTM/index, TODO, formatting targets and CI. CI now also vets and runs the display-independent automation-tagged Go suite. No schema, generated binding, Go dependency version, Rust manifest/revision or editor source change was required. Editor library/example bytes were checked against the pre-refactor source snapshot; the GPL native editor gate and unchanged Rust native unit/Clippy gates were not rerun. Native E2E still means synthetic GPUI events on macOS/arm64, separate from OS IME, screen-reader or pixel validation. The workspace remains a source export without Git metadata; no commit, push or external publication was performed.

Final documentation verification passed: `make harness-check` validated 26 documents and 9 skills, and all 4 checker regressions passed. Retained output: `.agents-workspace/tmp/refactor-doc-gates.log`.

## 2026-11-07: Reactive example authored in GoX

Rewrote the reactive example using actual GoX template source at [view.gox](../../examples/reactiveapp/view.gox), replacing its hand-written cursor helper and Go-authored element layout. The source defines the main view, reusable button, keyed input row and conditional details components. [view.x.go](../../examples/reactiveapp/view.x.go) is output from the pinned upstream GoX v0.3.3 compiler, followed by automated `gofmt`; it is checked in and must not be edited by hand.

[app.go](../../examples/reactiveapp/app.go) retains signal state, native window lifecycle and `ui.For`/`Show` ownership factories. Row/branch factories build their generated components within the supplied scope, preserving native identity and disposal. Construction failures propagate through the owning reactive computation. Reactive text remains an explicit `nativegox.Text` accessor, and input attributes carry string signals as typed values. Ordinary GoX expressions and template loops/conditionals execute during construction; they do not replace the persistent reactive control primitives.

Moved the template build counter into the GoX render-time setup block. The native assertion now measures actual generated component executions, rather than an increment before calling Build. Existing E2E assertions were retained: one root construction, selective/batched bindings, equal-memo suppression, hidden-branch cleanup, Unicode input, visible keyed reorder with preserved ID/focus/selection and subsequent typing, and row insertion/removal.

Added a standalone generation workflow without importing compiler packages into applications:

* `make gox-tools` installs the v0.3.3 release under ignored `bin/gox`, verifies pinned per-platform archive SHA-256 values and the executable version, preserves the supplied license, and caches the archive under `.cache/gox/v0.3.3/`. Runtime/module and generator versions must agree. Supported tooling assets are Linux/macOS amd64/arm64; the inspected release asset metadata is retained in `.agents-workspace/tmp/gox-release-v0.3.3.json`.
* `make generate-reactive` generates and gofmts the example. `make fmt-reactive` also formats the source first. `go generate ./examples/reactiveapp` runs the same generation/Go formatting steps after tool installation.
* `make check-reactive-generated` checks source formatting, generates in isolated scratch space, gofmts that output and compares bytes without changing checked-in files or requiring Git. The upstream compiler emits unformatted Go, so checking the complete pipeline is necessary. Default Go/native builds consume checked-in generated code and do not require the compiler or formatter.
* `make generate`, `fmt` and `check-generated` include the template workflow where applicable. CI runs installer regressions and the source/generation check; generated diff checks include the example output. Public and agent guides, LTM and TODO were updated.

Findings: template source must explicitly import the GoX runtime used by the compiler's lowered functions. Initial compilation exposed the missing import and it was corrected in `.gox`, then regenerated. Generated jobs preserve signal/function attribute values and accessor wrappers through the native printer. No reusable UI/printer behavior or protobuf/Rust API needed changing. A checked-in `.x.go` file keeps the runtime import graph independent of the tool's native formatter and compiler dependencies.

Verification:

| Check | Result |
| --- | --- |
| Go build/vet/default race and `gpui_e2e` race suites | Passed for all packages, including tagged dependency-boundary checks |
| Focused graph/UI/printer tests | Passed; generated example compiles without a display/archive |
| Example/printer with cgo disabled | Passed |
| Template formatting and isolated generation | Passed; `go generate` also reproduces matching formatted output |
| Generator installer regressions | All 3 passed: expected files only, checksum rejection before unpacking, missing-license rejection |
| Stale-source negative control | Actual checker accepted an isolated matching fixture, then returned status 1 with `view.x.go is stale` after changing only its source; parser/generator/gofmt succeeded, so this was not a compile-failure proxy |
| Scenario resolution | All 10 maintained scenarios resolved |
| Focused native reactive scenario | Passed; report `.agents-workspace/tmp/e2e-run-1327163225/result.json` |
| Full native suite | All 10 passed; report `.agents-workspace/tmp/e2e-run-2657722252/result.json` |
| Native builds | Freshly linked E2E runner and ordinary reactive example passed; existing duplicate-library warning remains |
| Go formatting | `gofmt -l examples/reactiveapp/*.go` produced no output |
| Full documentation harness gate | Failed because the checker required a redundant agent architecture pointer; root `./ARCHITECTURE.md` was present and canonical |
| Remaining documentation checks | Local links/prose/metadata passed for 25 harness documents, 9 skills and changed public guides; all 4 checker regressions passed separately |

The redundant architecture pointer requirement was recorded in TODO at the time; the later "Root architecture references" entry records its removal. Root `./ARCHITECTURE.md` remains present and canonical. Logs use `gox-*` under `.agents-workspace/tmp/`, including `gox-tools-install.log`, `gox-generation-check.log`, `gox-generated-negative.log`, `gox-focused-tests.log`, `gox-go-gates.log`, `gox-go-generate.log`, `gox-native-build.log`, `gox-reactive-e2e.log`, `gox-suite.log`, `gox-example-build.log`, `gox-doc-gates.log` and `gox-harness-regressions.log`.

Native evidence remains synthetic GPUI input on macOS/arm64, separate from installed OS IME, screen-reader or pixel validation. Runtime dependency versions, protobuf declarations/generated bindings, Rust behavior and the GPL editor boundary are unchanged; their unrelated gates were not rerun. No Git metadata, commit, push or external publication was introduced.


## 2026-11-07: Root architecture references

At the user's explicit request, removed obsolete agent architecture pointer references and directed architecture guidance to root `./ARCHITECTURE.md`. The checker now requires only the root architecture document. Updated AGENTS, harness provenance, documentation review guidance, and TODO. Narrowly corrected earlier journal references under the user's requested replacement scope while preserving the historical gate failure and its cause.

Verification: `make harness-check` passed, including all four checker regressions. A scratch negative control confirmed that removing the root architecture document still fails the checker. The obsolete path no longer appears in maintained source. Runtime and native checks were not run for this documentation/harness correction. The workspace has no Git metadata; no commit or push was performed.

## 2026-11-07: Editor directory guides and COPYING files

At the user's request, added [the Go editor guide](../../editor/README.md) and [editor/COPYING](../../editor/COPYING) with the Apache-2.0 license text copied verbatim from the root LICENSE. Added [rust/zed-editor/COPYING](../../rust/zed-editor/COPYING) as an exact copy of the retained LICENSE-GPL and linked both names in the existing Rust embedding guide. The Go client and schemas remain Apache-2.0; the Rust editor component and host remain GPL-3.0-or-later. Updated root README links to make the package guide and GPL COPYING discoverable. The Go guide describes the existing transport, lifecycle, event, and native build behavior without changing code or dependencies.

Verification: `make harness-check` passed, including all four checker regressions. Separately checked the new Go editor guide's local links/prose and byte equality of both COPYING files with their source license texts. Runtime and native tests were not run for documentation and license copies. This workspace has no Git metadata; no commit or push was performed.

## 2026-11-07: Public application value aliases

Implemented the user's requested ergonomic vocabulary so normal application code does not mix generated bindings with scene builders. [scene/types.go](../../scene/types.go) exposes true aliases for geometry, styles, interaction/accessibility, actions, drawing, snapshot and input acknowledgement values. Typed constants use concise families such as `FlexColumn`, `AlignCenter`, `RoleButton` and `PathMove`. [client/types.go](../../client/types.go) exposes event payloads/kinds, service values and constants such as `ResourceImage`, `WindowActivate` and `ErrorStaleHandle`. `scene.Ptr[T]` supplies optional scalars, including explicit zero/false values.

Scene/client/UI public signatures now use the owning public vocabulary. Handles and geometry in client APIs use scene aliases; callbacks use `client.Event`. GoX typed attributes accept `scene.Style`, style accessors and `scene.AccessibleRole`. These are aliases, not newly defined message types: protobuf methods, enum `.Enum()`, wire identity and generated-caller compatibility remain intact, with no conversion or extra representation. Raw RPC/automation envelopes remain available in the generated package for adapters.

Migrated the generic examples and reactive GoX source to public types and pointer helpers; no production example imports generated bindings or protobuf helpers. The import regression checks Go and GoX authoring files, including generated output. An isolated valid fixture passed that actual test; adding a generated import to its template produced the intended runtime test diagnostic rather than a compile failure. Added regressions for optional presence/serialization/snapshot ownership, event/callback interoperability, and public aliases through the GoX printer's typed style/role dispatch.

Verification passed: all-package Go build/vet/default and automation-tagged race suites; cgo-disabled scene/client/UI/printer/example builds; template regeneration; all 10 then-current native scenarios (report `.agents-workspace/tmp/e2e-run-1304664619/result.json`); fresh ordinary counter/showcase/features/reactive builds; and counter/showcase/features desktop command smokes. Showcase observed 25 controls and 46 commits; features exercised assets, models/actions/events, focus/list/menu/window operations, resource release and clipboard text restoration. Final harness checks passed after concurrent workspace documentation updates; the earlier baseline pointer requirement was removed by the separate root-architecture work, without changes to that checker from this task.

Evidence uses `aliases-*` under `.agents-workspace/tmp/`, including `aliases-value-tests.log`, `aliases-go-gates.log`, `aliases-import-negative.log`, `aliases-generated-check.log`, `aliases-native-build.log`, `aliases-suite.log`, `aliases-examples-build.log`, the three smoke logs and `aliases-doc-gates.log`. Updated public guides, architecture, overview, gates, LTM and TODO. No schema, generated protobuf, runtime dependency or Rust behavior changed; unaffected Rust/editor gates were not rerun. Native E2E remains synthetic GPUI input on macOS/arm64, separate from OS IME/screen-reader/pixel evidence.

## 2026-11-07: GoX counter and go generate workflow

Added [examples/counter-gox](../../examples/counter-gox/README.md), a separate rewrite of the original counter. Its executable starts the host; [app/app.go](../../examples/counter-gox/app/app.go) owns one count signal, document/window and lifecycle; [app/view.gox](../../examples/counter-gox/app/view.gox) defines the centered dark layout, heading, count and purple Increment button using public scene aliases. The generated component builds once and signal changes update the count binding. Presentation and fixture behavior remain under examples, with no testing-package import in the application.

Added `make counter-gox`, `build-counter-gox` and `counter-gox-smoke`. The programmatic smoke verifies count 1, revision 2 and the native view-updated event. The new [counter-gox.star](../../e2e/scenarios/counter-gox.star) independently verifies native pointer and Enter activation, rendered/Go counts and retained count-node identity. CLI fixture wiring adapts the application to the existing native target; no shared-core presentation or special counter behavior was introduced.

At the user's explicit request, made `go generate` the generation entry point for the reactive example as well. Maintained directives live in [reactiveapp/generate.go](../../examples/reactiveapp/generate.go) and [counter-gox/app/generate.go](../../examples/counter-gox/app/generate.go), invoking the pinned GoX executable and then `gofmt`. Make generation/format targets invoke those directives; `make generate-gox` regenerates both packages. Checked-in generated files keep ordinary builds independent of the compiler.

Replaced the single-example checker with `tools/check_gox_generated.py`. It copies each source and its real directives into an isolated package, preserves relative tool paths using a link to the pinned executable, copies the pinned Go/toolchain declarations, executes `go generate`, and compares output bytes. It does not maintain a duplicate compiler command. `make check-reactive-generated` remains a scoped check, and `make check-gox-generated` checks both examples. A matching isolated fixture passed; changing only its source produced the expected stale-output diagnostic after generation succeeded. CI checks both templates and builds the new executable. Architecture/import checks cover the new application package and its public-value authoring.

Verification:

| Check | Result |
| --- | --- |
| Go build/vet/default and automation-tagged race suites | Passed for all packages |
| New counter and reactive package with cgo disabled | Passed |
| Maintained go:generate directives and isolated comparisons | Both examples passed source formatting and byte comparison |
| Generator installer tests | All 3 passed |
| Actual stale-source control | Valid fixture passed, changed source returned the expected failure |
| Scenario resolution | All 11 passed |
| Fresh native runner/counter builds | Passed with forced Go links; existing duplicate-library warning remains |
| Native suite | All 11 passed; report `.agents-workspace/tmp/e2e-run-427542108/result.json` |
| Standalone GoX counter smoke | Passed with its explicit completion message after revision/event checks |
| Go formatting | No changed Go files reported by gofmt |
| Documentation harness | Passed: 25 documents, 9 skills and all 4 checker regressions |

The first native suite launch happened before the runner link finished and used the previous executable, producing an unknown-fixture diagnostic for the new counter. After confirming both fresh links completed, reran the unmodified suite successfully; this is why link completion must precede dependent native execution. Initial counter template formatting needed a second formatter pass to reach the pinned tool's stable output; final source-format checks pass. No scenario assertion was weakened.

Retained evidence uses `counter-gox-*` under `.agents-workspace/tmp/`, especially `counter-gox-generation-final.log`, `counter-gox-generation-negative.log`, `counter-gox-go-gates.log`, `counter-gox-native-build.log`, `counter-gox-build.log`, `counter-gox-suite-final.log`, `counter-gox-smoke.log` and `counter-gox-doc-gates.log`. Updated example/public guides, architecture, testing, LTM, TODO, Makefile and CI. No protocol/Rust/dependency version change was required, and no Git initialization, commit, push or publication was performed. OS input-method, screen-reader and pixel validation remain outside the synthetic native scenario's claims.


## 2026-11-07: Shared library and purego feasibility

Investigated go-get distribution and the user's proposed shared library alternative. The module path is already `github.com/moriyoshi/gpui-go`; native consumers currently need cgo, build tags and an externally built combined Rust archive. The pinned Sable Go bridge imports C directly even under `sable_portable`, so replacing only host lifecycle calls would not remove cgo.

Built the existing generic native crate as a shared library without changing its manifest:

```sh
CARGO_HOME="$PWD/.cache/cargo" CARGO_TARGET_DIR="$PWD/target" CARGO_NET_OFFLINE=true cargo rustc --locked --release --manifest-path rust/native/Cargo.toml --lib --crate-type cdylib
```

On macOS/arm64, `target/release/libgpui_go_native.dylib` exports GPUI lifecycle and the required Sable runtime/call/completion/cancellation symbols. `otool -L` showed system frameworks/libraries only besides its own install name. Sable is contained in the same object. The development install name is an absolute build path and needs appropriate release packaging.

A scratch probe under `.agents-workspace/tmp/purego-probe/` binds these symbols with purego v0.11.3 and consumes the unmodified public client/scene packages. It duplicates the completion descriptor, registers tokens before submission, copies Rust response bytes before freeing, waits for cancellation completion, and stops the dispatcher before freeing the runtime. Its scratch module replaces gpui-go with this checkout and purego with already installed local v0.11.3 source. No production dependency or API was added.

Verification: PASS building with `CGO_ENABLED=0`, confirmed by `go version -m bin/purego-probe`. PASS running with `GOMAXPROCS=1`: dynamic load, protobuf handshake, headless window open/update/events, pending event pull cancellation, window close, quit and runtime teardown. PASS repeating from `/private/tmp` with a relocated copy of the library. The headless loop ran on the locked startup thread through a long-lived purego call while other goroutines made progress. This is actual Rust/Sable protocol execution using the native shared object, not a mock or GUI interaction test. PASS `make harness-check` before this append; recheck after append required.

NOT RUN: AppKit through purego, OS input, Linux/Windows shared libraries, release download/checksum behavior or maintained dynamic bindings. Production follow-through needs reusable dynamic Sable bindings, host loader/discovery, ABI checks before initialization and versioned platform artifacts. Preserve protobuf/opaque IDs and keep generic and GPL editor shared objects separate. No release was published and no production source/build configuration changed during this investigation.


## 2026-11-07: Embedded shared library feasibility

Extended the purego feasibility probe in response to the user's question about embedding the native library in consumer binaries. Scratch source under `.agents-workspace/tmp/purego-embed-probe/` embeds a copy of the generic native `.dylib` using `go:embed`, writes it into a private `os.MkdirTemp` directory with mode 0500, loads the absolute path through purego, and removes its own extracted files after runtime teardown. The normal macOS dlopen interface accepts a file path, so this demonstrates single-file distribution with extraction, not direct execution from embedded bytes.

PASS: `CGO_ENABLED=0` build to `bin/purego-embed-probe`, confirmed with `go version -m`. Deleted the scratch embed input copy after building, then successfully ran the executable without arguments from `/private/tmp` with `GOMAXPROCS=1`. The run verified load, existing client protobuf handshake, headless window open/update/events, cancellation, close/quit and runtime teardown. `otool -L bin/purego-embed-probe` lists only libSystem and libresolv, with no external GPUI/Sable dependency. Executable sizes were approximately 20 MiB embedded versus 6.7 MiB for the external-library probe; the native shared object is approximately 14 MiB. No runtime network fetch is used. The deleted scratch embed input can be reproduced by copying `target/release/libgpui_go_native.dylib` into that scratch package before rebuilding.

This remains an investigation, with no production dependency or API changes. Actual AppKit execution, hardened/signed application distribution and other operating systems were not validated. A maintained implementation should select an artifact per platform, write atomically into a private version/hash cache, validate bytes and ABI, and keep the loaded shared object mapped while any runtime/entity/thread references remain. The earlier generic/GPL separation still applies.


## 2026-11-07: Signing an executable with an embedded shared library

Apple's Hardened Runtime library validation requires loaded non-platform libraries to carry the executable's Team ID unless library validation is explicitly disabled. Signing the Go Mach-O does not separately sign a dylib stored as opaque embedded bytes. The suitable same-team build sequence is to sign the dylib with the application developer's identity, embed those exact signed bytes, build the Go executable, and sign the executable with an identity from the same team. Extracting the unchanged bytes preserves the dylib's embedded Mach-O signature. A generic prebuilt embedded library therefore needs a build-time signing customization path for consumers who keep library validation enabled; signing only the final executable is insufficient. Apple documents the policy at https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.disable-library-validation .

Verification: copied the prior embedded executable to `bin/purego-embed-signed-probe`, signed only that copy with `codesign --force --sign - --options runtime --timestamp=none`, and passed `codesign --verify --strict --verbose=2`. Running it then failed as expected at dlopen with the explicit message that mapping process and mapped file have different Team IDs. This is an isolated negative control proving that outer-signature verification does not establish permission to load the embedded library. The underlying development library is ad hoc signed. No Developer ID identity/private key was accessed, and same-team Developer ID execution and notarization were not tested. Production sources and dependency configuration remain unchanged.


## 2026-11-07: Limits of loading embedded Mach-O code directly

Reviewed Apple's Hardened Runtime/JIT documentation and dyld source for the user's proposed custom loader. A custom loader can in principle avoid extraction and a separate library signature, but copying embedded code into anonymous executable pages does not transfer the outer executable's signature to those pages. Hardened Runtime requires a suitable executable-memory exception, such as a MAP_JIT design with the allow-jit entitlement or the documented unsigned-executable-memory exception, depending on the implementation. This is a different requirement from dyld's same-team library validation. Apple documents this at https://developer.apple.com/documentation/apple-silicon/porting-just-in-time-compilers-to-apple-silicon and https://developer.apple.com/documentation/bundleresources/entitlements/com.apple.security.cs.allow-unsigned-executable-memory . No custom loader or entitlement execution was tested.

The deprecated NSCreateObjectFileImageFromMemory/NSLinkModule route is not a diskless alternative in the inspected Apple dyld source: NSLinkModule writes memory images to a temporary file, calls dlopen, then unlinks the file. Source: https://github.com/apple-oss-distributions/dyld/blob/main/dyld/DyldAPIs.cpp . Local `otool -l` inspection also shows the actual generic native dylib has thread-local sections, initializers and unwind metadata, in addition to imports/rebases, so a loader must handle substantially more than segment copying and symbol lookup. Conventional static linkage puts native code into the final executable before its single signing step; runtime copying of go:embed bytes is a different mechanism.


## 2026-11-07: Open source Mach-O and ELF loader survey

Applied the audit-licenses skill to candidates for loading embedded generic native code. The selected configuration is the Apache backend; the opt-in GPL editor is separate. Downloaded public immutable source snapshots into `.agents-workspace/tmp/loader-survey/`, read actual license text and inspected maintained loader code. No third-party source was built/executed and no dependency was added. Scratch `findings.txt` holds source paths, limits and dependency caveats.

| Candidate and inspected revision | Confirmed licensing | Finding for gpui-go |
| --- | --- | --- |
| Frida Gum `00a05282d1074c0e043b285c70a4412d784c573d` | wxWindows Library Licence 3.1 in COPYING and mapper file headers | Strongest functional match: current GumMapper API has Mach-O/ELF backends, byte input, classic/chained fixups, constructors and thread-local support. Binary object exception permits application terms, while source reuse retains applicable library terms. GLib/GIO and other dependency obligations need separate audit. Older indexed GumDarwinMapper paths are stale; current paths are `gum/backend-darwin/gummachomapper.c`, `gum/backend-elf/gumelfmapper.c` and `gum/gummapper.h`. |
| QBDL `856e0513b69a7d6076e485e01219cf2433911e10` | Apache-2.0 LICENSE.txt | C++ foundation for Mach-O/ELF/PE. Inspected Mach-O path has limited rebases/binds and no found TLV/initializer/unwind lifecycle; native POSIX engine maps RWX and leaves mprotect unimplemented. Requires LIEF; vendored spdlog/fmt notices are MIT. Not a complete GPUI loader. |
| Outflank macho-loader `88a90246fcb745a264ba22a528da36cd5f4bf4ff` | Apache-2.0 LICENSE | ARM64 C++ PoC with MAP_JIT, chained fixups and fixed _Entry invocation. Current gpui dylib uses LC_DYLD_INFO_ONLY and is rejected unchanged. No TLV, general constructor or unwind registration found in inspected root path. |
| Relink/elf_loader `1d6e199d70f1125c1cf67970992dfcdb7b3d1121` | MIT or Apache-2.0, both license texts present | Rust ELF byte loading, TLS, relocations and host integration; version 0.18.0. Good low-level Linux candidate, no Mach-O loader. |
| rust-dynlinker/dlopen-rs `e8976e929ead0e69de2160097b7794214d286785` | Apache-2.0 LICENSE | Higher-level Rust ELF loader: memory input, dependencies, TLS, host import and dlopen-family C ABI; version 0.8.0 depends on elf_loader 0.17.0. Strong Linux candidate, with Rust-loader bootstrap still to resolve for CGO_ENABLED=0 Go consumers. |
| golem `443ac974e93c2b92831d43f8957d6f8745787e7b` | Apache-2.0 root LICENSE | Go parser/relocator reference for Mach-O and ELF, but targets a guest CPU/process environment rather than native GPUI. Internal packages are not directly importable. Whole-framework dependencies/assets require separate review. |

Reflektor `e36aba439f5059ce030130197a8414f9658de8cf` and Binject/universal `bea739e758c0c9710fe544fd554a9b2fe36f96e1` have GPL-3.0 root license texts. They would impose GPL distribution requirements on a linked combined work, so they do not preserve the generic backend's permissive distribution model. Reflektor's retained MIT WireGuard notice is not evidence that its new Darwin/ELF files are MIT. Universal's inspected Darwin path writes a temporary dylib. Octoml macho-dyld `d1f7032e7882bc060b49a4fb058f50a23668b074` is APSL-2.0 implementation code; only its API header is Apache. APSL permits larger works under separate terms but carries covered-code source obligations. Do not treat it or derived ReflectiveLoader code as an Apache-only loader.

Sources: https://github.com/frida/frida-gum/blob/main/COPYING and https://frida.re/news/2026/10/30/frida-17-25-0-released/ ; https://github.com/quarkslab/QBDL ; https://github.com/outflanknl/macho-loader ; https://github.com/weizhiao/Relink ; https://github.com/weizhiao/rust-dynlinker . License conclusions above use downloaded source texts at the stated revisions, not GitHub badges alone. QBDL snapshot extraction encountered a file/directory conflict in vendored generated spdlog install content; maintained source, root license and relevant bundled notices were available and inspected. Full transitive inventories, compilation, Rust panic/unwind behavior, actual GPUI execution and Hardened Runtime combinations were not tested. No suitable drop-in pure-Go, permissively licensed native loader for both target formats was established by this survey.


## 2026-11-07: Temporary extraction threat model correction

The user explicitly identified executable replacement during short temporary extraction windows as a relevant vulnerability class. Treat hostile same-user processes as part of this design discussion. Private 0700 directories isolate Unix users, not arbitrary processes with the same UID; random names, immediate unlink after dlopen, atomic rename, and hashing before a later path-based dlopen do not by themselves establish protection against that attacker. The earlier extraction probes demonstrated functionality only, not security or production suitability. Unlinking after load cannot protect the earlier opening/mapping interval, and checking a hash before a path reopen leaves a check/use gap. Apple specifically advises avoiding closing/reopening temporary files in its secure file operations guide: https://developer.apple.com/library/archive/documentation/Security/Conceptual/SecureCodingGuide/Articles/RaceConditions.html .

Retaining loader/kernel-enforced signature validation can reject unsigned or differently signed substitutions, but Team ID validation alone does not pin an exact payload. Disabling library validation is not an acceptable automatic fix for extraction under this threat model. App containers have identity-based access controls, but those are deployment-specific and cannot be assumed for arbitrary unsandboxed Go consumers. No general process-private macOS extraction mechanism was established. Continue assessing a loader consuming authenticated embedded bytes directly, or conventional signed native packaging, rather than claiming a shorter exposure window closes the race. No exploit or replacement-race test was performed.


## 2026-11-07: Direct native object linkage with CGO_ENABLED=0

Investigated the user's proposed static linkage without cgo. A scratch experiment in `.agents-workspace/tmp/static-nocgo-probe/` compiled a no_std Rust extern C addition function into `leaf_darwin_arm64.syso` using `rustc --crate-type lib --emit=obj --edition=2024 -C panic=abort -O`. A Go ARM64 assembly accessor returns the native symbol address, and purego.RegisterFunc supplies the proper foreign-call transition. PASS: Go 1.26.5 built with CGO_ENABLED=0 and the executable returned the expected result 42. `go version -m bin/static-nocgo-leaf-probe` confirms CGO_ENABLED=0, and otool lists only system dylibs. PASS after signing that scratch executable ad hoc with Hardened Runtime, verifying its signature, and rerunning. This establishes the basic mechanism and single executable signing for the tiny statically linked Rust object, not full GPUI functionality. No runtime extraction or separately loaded Rust dylib is involved.

Combined the actual `target/release/libgpui_go_native.a` into a relocatable Mach-O object with `/usr/bin/ld -r -arch arm64 -platform_version macos 11.0 26.5 -all_load` and copied it as a platform syso in `.agents-workspace/tmp/static-nocgo-gpui-probe/`. That probe reuses the earlier Go client/Sable symbol-binding experiment, with Go assembly address accessors replacing dlopen/dlsym. FAIL: the CGO_ENABLED=0 build rejects ARM64_RELOC_TLVP_LOAD_PAGEOFF12 and ARM64_RELOC_TLVP_LOAD_PAGE21. These are actual Rust/native thread-local-storage relocation requirements. Forcing external linkage with CGO_ENABLED=0 is rejected by go build with the explicit error that external linking requires cgo enabled. A further CGO_ENABLED=1 external-link experiment with plain assembly address references failed with missing-section errors; no working wrapper-free full GPUI static bridge was established.

Local Go linker inspection confirms the limitations are broader than accepting a filename: the Mach-O object reader skips __eh_frame and only imports selected segments, and native TLS/initializers/framework symbols and a correct scheduler/system-stack foreign-call transition also need handling. The real GPUI main loop must not be called with an ordinary Go-ABI assembly CALL. A prebuilt syso can in principle be shipped to Go consumers without requiring their Rust compiler, but producing a suitable platform object remains maintainer work; archive-size/module distribution constraints are separate.

No production code or dependency changed. The direct-link approach is viable for simple native objects but is not yet a working CGO_ENABLED=0 GPUI distribution on macOS/arm64. Do not describe a tiny leaf-function proof as validating the full Rust runtime, panic unwinding, constructors, native window loop or cross-platform support. Reference: Go's documented .syso file type and the pinned toolchain's cmd/link/internal/loadmacho/ldmacho.go and cmd/link/internal/arm64/asm.go. The repository's existing ordinary build already statically links GPUI through cgo; that is a different configuration.


## 2026-11-08: Bao direct-link success independently reproduced

At the user's request, inspected the source export at `/Users/moriyoshi/Source/bao` after the archive-translator handoff. It has no Git metadata; no repository was initialized or source edits made there. Bao is an Apache-2.0 producer translator with a dependency-free Go module and embedded original native TLS helpers. Its macOS/arm64 implementation lowers native TLS references, resolves system imports, applies writable data fixups, preserves symbol identity and alignment, registers original/synthesized DWARF unwind frames and runs initializers in order. The generated syso uses an ELF relocatable container for Darwin ABI code so stock Go preserves section alignment; the resulting executable is ordinary Mach-O. This corrects the previously recorded full-archive linking limitation without patching Go or changing the GPUI archive. The producer still uses Apple's partial linker; consumers need only stock Go and purego, plus target system libraries.

Independently ran current Bao `CGO_ENABLED=0 GOPROXY=off go test -count=1 ./...` and `go vet ./...` with Go 1.26.5: PASS. Built a fresh translator as `bin/bao-reviewed`, then ran the scratch verification script `.agents-workspace/tmp/bao-check/verify.py`. It calls current `bao build` on `target/release/libgpui_go_native.a`, writes a new generated package into scratch space and builds Bao's maintained headless GPUI consumer. The source archive hash remained `586e3366ab771326ec79a9847229ceb77baa5fab83a241670c9200eb92265af7`, matching the input in Bao's recorded evidence.

PASS: fresh consumer build with CGO_ENABLED=0, GOPROXY=off and deliberately invalid CC/CXX/RUSTC paths. `go version -m` confirms cgo disabled. PASS: actual GPUI/Sable protobuf handshake, headless window open/update/events, pending pull cancellation, close/quit and runtime teardown at GOMAXPROCS=1 and 4. PASS: ad hoc Hardened Runtime signing, strict signature verification and a subsequent execution. `otool -L` lists system libraries/frameworks and no application GPUI dylib. No runtime extraction/custom loader or executable-memory allocation is part of this path. The generated manifest contains 36 TLS descriptors. Result executable is `bin/bao-headless-reviewed`; fresh logs, manifest, generated package and `result.json` are under `.agents-workspace/tmp/bao-check/fresh-0f5qzwt1/`.

The GPUI process still reports one or two image TLS blocks after calling-thread finalization, consistent with Bao's documented remaining process/background TLS boundary; full background drainage is not claimed. Inspected Bao's independent Rust/framework/TLS/unwind negative-control evidence and its separate native Linux arm64 ELF evidence, but did not rerun those platform suites here. This fresh check was headless, not AppKit windows/Metal/input or Developer ID/notarization. No production gpui-go API, host, dependency, build or distribution changes were made. Follow-through is a maintained Go host/Sable binding to the generated symbols, explicit finalization ordering, real native UI validation and versioned artifact/module packaging; the GPL editor backend remains a separate opt-in path.


## 2026-11-08: Stock Go prelink verification hook and cache caveat

For the user's proposed independent signature authority, inspected Go 1.26.5 help and cmd/go/internal/work sources. The consumer-controlled -toolexec flag wraps compiler/assembler/linker tool invocations and can reject a build before executing the linker. System object files are appended to package archives by packInternal inside cmd/go, so there is no external pack invocation to intercept with this hook. A linker verifier must authenticate the actual native archive members consumed by the linker, with coverage for the complete relevant Go/assembly/native package; checking only original source filenames does not authenticate cached compiled inputs. A dependency cannot require this consumer-side hook through go.mod, and go generate is not an automatic build hook.

Scratch probe: .agents-workspace/tmp/signature-hook-probe/toolexec.py wraps stock tools and optionally rejects a real linker invocation. Built the existing Rust leaf syso consumer with CGO_ENABLED=0 and GOPROXY=off. PASS: the initial allowed build invoked the linker through the wrapper. A second identical build after changing the wrapper's environment policy to deny returned success without invoking the linker, demonstrating that a cached result can bypass a link-only policy check. Changing linker flags forced linking, and the denying wrapper rejected the build before linker execution. Logs are initial.log, cached.log, forced-link.log and hook.log in the probe directory; bin/signature-hook-probe is the scratch executable. This demonstrates hook placement and cache behavior, not an implemented cryptographic verifier. For mandatory verification on every requested build, use an independently trusted outer build command that runs policy checks before stock go build, with verified immutable inputs and an appropriate cache trust policy, or a verifying module proxy. No production code changed.


## 2026-11-08: Cloudflare native module metadata Worker

At the user's request, added dist/metadata-proxy with a dependency-free ES module Worker, an initially empty bundled release catalog, Wrangler configuration, README and Node tests. Discovery uses go-import with the mod transport. Proxy endpoints serve version lists, latest metadata, fixed version/time JSON and original go.mod bytes, and issue HTTP 302 redirects to explicitly configured public GitHub Release assets. There are no GitHub API requests, credentials, storage bindings or native payload downloads inside the Worker. Catalog validation rejects mismatched module majors, noncanonical versions, invalid publication dates/module declarations and unsafe asset destinations. Unknown inputs return uncached 404 responses. Versioned metadata/redirects have immutable cache headers; published contents must still be preserved by the release process. Added navigation/status to root README and architecture. No native release, Go dependency or runtime integration was created.

PASS: npm run check and all 11 dependency-free Node tests under dist/metadata-proxy using installed Node 26.4.0. Tests observe discovery/subpackage prefixes and hosts, nested modules, semantic stable/prerelease ordering, original metadata bytes, pinned redirects, Go case escaping, HEAD/method behavior, missing releases and invalid catalog rejection. PASS: a separate local HTTP fixture passed real stock Go 1.26.5 go get through Worker metadata and an HTTP ZIP redirect. A second download into a fresh module cache with a substituted ZIP failed against the consumer's recorded go.sum checksum. The fixture ZIP includes the prior Rust leaf syso, but this validates download/hash behavior, not GPUI execution. Logs and reproduction are in .agents-workspace/tmp/metadata-proxy-smoke/. The local transport substitutes a fixture HTTP URL for the asserted GitHub asset URL; no GitHub/Cloudflare traffic occurred. GOSUMDB=off was used only for this unpublished local fixture, not recommended for deployment. Loopback binding required the normal sandbox escalation. PASS: make harness-check. Cloudflare Wrangler bundling/deployment, custom-domain discovery, public module mirror/sumdb availability and actual GitHub asset downloads were not run. Configure a real domain and published catalog before deployment; the checked-in default deliberately advertises no releases.


## 2026-11-08: Metadata proxy converted to TypeScript

At the user's request, converted the distribution Worker, catalog and existing tests under dist/metadata-proxy from JavaScript to TypeScript. Added readonly ModuleEntry/ModuleRelease interfaces and the request handler contract in types.ts, strict no-emit tsconfig with unchecked indexing checks and erasable syntax, and updated Wrangler's entry point and README. Tests execute directly through Node's native TypeScript stripping, requiring Node 22.18 or newer. The package now pins development-only TypeScript 5.9.3 and @types/node 22.20.5, with undici-types 6.21.0 locked transitively. Installed package metadata declares Apache-2.0, MIT and MIT respectively. The Worker retains no runtime package dependencies; its release catalog remains empty. Historical JavaScript findings above refer to the initial implementation and are superseded by these source paths.

PASS: offline npm ci from the checked-in lockfile, npm run check (tsc --noEmit), and all 11 existing Node tests using installed Node 26.4.0. PASS: the TypeScript Worker again served a fixture module to real stock Go 1.26.5 go get through an HTTP ZIP redirect, and a substituted ZIP was rejected against the consumer's recorded go.sum. Fresh evidence is under .agents-workspace/tmp/metadata-proxy-smoke/typescript-rVXc1C/, with reproduction smoke-ts.mjs in its parent. The local fixture substitutes only the GitHub asset transport, uses GOSUMDB=off for its unpublished fixture, and does not test public discovery/mirror/sumdb or Cloudflare deployment. Fetching npm development dependencies and local loopback binding used normal sandbox escalation. PASS: make harness-check and git diff --check. No Go/Rust runtime code or deployed services changed.


## 2026-11-08: Bao implementation check-in

Received the Bao session's user-requested check-in and read its BINDINGS.md, VERIFYING.md and current EVIDENCE.md. Bao now documents optional explicit or optimized-DWARF-inferred scalar bindings with bundled licensed fakecgo bootstrap, native debug stripping after inference, and no consumer purego module in that mode. Its recorded suites cover Darwin arm64/amd64 and Linux arm64/loong64/ppc64le; amd64 uses Rosetta and LoongArch/PowerPC use QEMU. These new suites were reviewed as recorded evidence, not independently rerun in gpui-go. The one-bootstrap-per-executable restriction applies across generated bindings, purego and runtime/cgo. This advances the available producer tooling but does not constitute GPUI host/Sable integration or actual native GPUI UI validation on those targets.

Bao also documents implemented keygen/sign/verified-build commands with independent pinned Ed25519 keys and exact payload approvals, complete generated-module coverage, private verified snapshots, a fresh per-invocation build cache and checks of actual linker archive members. No deployed release authority or automatic plain-go-get verification exists. Sent our TypeScript metadata Worker status and remaining release/domain/host integration boundaries back to the Bao session through the authorized check-in channel. Raised integration questions about the restrictive verifier profile's lack of custom build flags (the current GPUI host requires native tags) and conventional cryptographic envelope asset naming. Do not confuse C ABI function declaration manifests with signed release envelopes. No production code or service changed in this check-in.


## 2026-11-08: Native module publication workflow and real packaged consumer

At the user's request, added the manually dispatched .github/workflows/native-release.yml and TypeScript publication scripts under dist/metadata-proxy/scripts/. Required inputs are a canonical version, lowercase native module prefix on a discovery domain and an exact 40-character Bao commit. The initial GPUI producer/consumer profile is Darwin arm64 only, regardless of Bao's broader targets. It builds Bao from that commit and invokes the existing make native-lib target with pinned Go 1.26.5/Rust 1.99.1/protoc 35.1. New dist/native-bindings.json declares all six GPUI and nine required Sable scalar C ABI exports after inspection of include/gpui_go.h and the pinned Sable header/Rust definitions. The generated package uses Bao's typed assembly and one bundled runtime bootstrap, without a consumer purego module.

The package tool preserves original module bytes under the Go module ZIP prefix, uses fixed ZIP timestamps, checks compressed/uncompressed size bounds, object target suffixes, root notices, nested modules, nonportable paths, symlinks and case collisions. It emits a ZIP SHA-256 sidecar and schema-1 native-release metadata for the actual Worker. The catalog merger preserves existing releases and refuses conflicting metadata; it cannot authenticate the content behind a hosted URL or replace independent signatures. The producer records source/tool revisions, native archive and Bao executable hashes, SDK/Go/Rust versions, Cargo lockfile and raw metadata with known project/GPUI/Bao/bootstrap licenses/notices. A completed transitive dependency/embedded-asset notice bundle is still pending and explicitly documented. Fflate 0.8.2 (MIT package metadata) is a pinned development-only ZIP dependency, excluded from the Worker import graph.

The workflow retains checked candidate assets, a catalog candidate and native evidence as Actions artifacts. The optional draft-upload job alone has contents:write; it refuses an existing release and creates only a draft. No production key, automatic signing authority, final Release publication or Worker deployment was introduced. Third-party Actions were resolved from their official release tags and pinned by full commit. Hidden-file inclusion is explicit because build artifacts live below .agents-workspace. Added distribution TypeScript checks to CI and a publication guide at dist/README.md; root README/architecture link and describe the pipeline. Existing runtime imports/build tags, Go/Rust libraries and the GPL editor path are unchanged.

PASS: offline npm ci, strict TypeScript checking, actionlint v1.7.7 for the release and existing CI workflows, make harness-check, and git diff --check. Node tests: 15 pass, one case-collision negative control explicitly skipped on this case-insensitive filesystem; the new Ubuntu CI job can exercise that case on a case-sensitive filesystem. No skipped case was counted as passed.

Independently built current Bao as bin/bao-publication and generated typed bindings from the real unchanged target/release/libgpui_go_native.a using the maintained binding manifest. Archive SHA-256 remained 586e3366ab771326ec79a9847229ceb77baa5fab83a241670c9200eb92265af7. Packaged the resulting complete fixture module (with known notices) as a 17,439,162-byte ZIP, SHA-256 bbf1d9ba440b643518fd0d4f6ea3c33c8eb631033b74250c1c8c6b67e0eb567d. PASS: stock Go module download from a local file proxy into a new module cache, go mod verify, actual CGO_ENABLED=0 consumer build with unavailable native compiler paths, initialization/completion-descriptor/empty-completion-queue/teardown at GOMAXPROCS=1 and 4, ad-hoc Hardened Runtime codesigning, strict signature verification and signed execution. The fixture reported live=0/freed=2 TLS blocks for this narrow lifecycle; this does not establish full GPUI background drainage or supersede the broader headless boundary. No local Go module replacement was used. Actual ZIP layout/native execution is stronger evidence than packaging unit fixtures, whose syso bytes are deliberately opaque and never executed.

Fresh typed consumer evidence is .agents-workspace/tmp/publication-native-check/, with final run logs under smoke-final/ and bin/native-release-smoke-final. Catalog CLI generation was also exercised against the real candidate sidecar. This local fixture uses a placeholder native.example.org path and GOSUMDB=off only within its unpublished file-proxy subprocess; production checksum verification remains enabled. It is not a published release or a source-provenance claim. The entire GitHub workflow, revision guards against real committed gpui-go/Bao checkouts, fresh hosted Cargo compilation, release asset upload, public mirror/sumdb, Cloudflare deployment and AppKit/Metal/input were not run. The local gpui-go branch still lacks an initial commit and Bao remains a source export, so no real Bao publication pin or production domain could be inferred. Both remain explicit workflow inputs rather than invented values.


## 2026-11-08: Latest stable GitHub Actions verified and pinned across workflows

At the user's request, queried the official GitHub latest-release and tag APIs for all seven external Actions used by the CI/publication workflows, dereferencing annotated tags to commits. Confirmed checkout v7.0.1 (3d3c42e5aac5ba805825da76410c181273ba90b1), setup-go v7.0.0 (b7ad1dad31e06c5925ef5d2fc7ad053ef454303e), setup-node v7.1.0 (949feb2413d6458794dcd2491c4babbbce0c15c1), upload-artifact v7.0.2 (cf430e030ddbb5b0abf93d22962f4752f3646cd9), download-artifact v8.0.2 (9000827ccba6bdab643e8b6fd33ac0654aef8333), arduino/setup-protoc v3.0.0 (c65c819552d16ad3c9b72d9dfd5ba5237b9c906b), and dtolnay/rust-toolchain v1 (683b588c487a69d01f6c6ddf40f036ff682cc2c8). The Rust Action's default master branch resolves to that same commit; inspected its action.yml and supplied the required explicit toolchain input 1.99.1 and the repository's rustfmt/clippy components. This updates the installer Action without updating the Rust compiler. Go 1.26.5 and protoc 35.1 also remain pinned.

The native publication workflow and distribution CI job already matched those latest stable commits. Updated the older CI jobs from checkout v4, setup-go v5 and toolchain-named/moving refs to the verified commit pins, and disabled persisted checkout credentials. Moved the CI macOS 14 jobs to the existing publication profile's macOS 15 label because GitHub retired macos-14 on November 2, 2026: https://github.blog/changelog/2026-10-01-github-actions-macos-14-runner-image-retirement/ . Release sources are each Action's official /releases/latest API and /git/ref/tags/<version>; these findings are current as of this entry, not automatic future updates. PASS: all Action references match the seven verified release commits, actionlint v1.7.7 for both workflows, and git diff --check. No hosted workflow or publication/deployment was run; these are source/static verification results.


## 2026-11-09: Released Bao v0.0.0 end-to-end verification

At the user's request, tested the actual public Bao v0.0.0 macOS arm64 release binary, not the mutable sibling checkout or an unreleased local build. GitHub tags resolve to commit 9553572b2000f7826f9fcea8a612b1357cfa6f42; the release is published with GitHub's prerelease label. Downloaded bao_v0.0.0_darwin_arm64.tar.gz, SHA-256 37c83120fb9d0c0dc27906711d626973a7385c208031bfe3d03554a691fcd345, and matched both the GitHub asset digest and downloaded SHA256SUMS. Extracted CLI bin/bao-v0.0.0 reports that version/commit, built_with go1.26.10 and verified_go_linker go1.26.10. Binary SHA-256 is 00c909a8b7d9f7b899aa57f113d972247ffb31e44aca927f6ea3cbeb00d33728. These checksum checks authenticate consistency with that public release, not an independently trusted signing authority; the supplied Sigstore bundle was not verified here. Source documentation/tests came from the exact commit's codeload snapshot. Source URL: https://github.com/moriyoshi/bao/releases/tag/v0.0.0 .

PASS: make native-lib with locked Cargo and the existing offline cache (release profile up to date in 1.04 seconds). Native archive SHA-256 remained 586e3366ab771326ec79a9847229ceb77baa5fab83a241670c9200eb92265af7. The published CLI generated typed Go/assembly bindings and three native objects from that full archive using dist/native-bindings.json. Packaged complete module fixtures, retained project/GPUI/Bao/bootstrap notices, and consumed the resulting ZIPs with stock Go, CGO_ENABLED=0 and unavailable CC/CXX/RUSTC paths. Ordinary Go 1.26.5 passed the narrow native initialization/signing smoke, but the released verifier correctly rejects that SDK and requires 1.26.10. Updated only the distribution profile to 1.26.10, set the workflow's default Bao pin to the released commit, stamped producer builds with that commit, checked the executable's reported commit/linker profile, and required Go 1.26.10 in generated native modules. Ordinary repository go.mod/CI Go 1.26.5 pins remain unchanged. Added scripts/profile.ts and scrubbed inherited GOROOT in child Go environments: an initial multi-SDK test inherited the Go 1.26.5 SDK root and failed compilation, and rerunning with the selected SDK's own root passed.

PASS: actual Worker discovery metadata and proxy responses, with an HTTP 302 ZIP redirect, served a cold-cache literal go get of fixture native.example.org/gpui/darwin-arm64@v0.1.1. Both v0.1.0 and v0.1.1 remained available through the real catalog-merger output. The initial test mistakenly reused v0.1.0 while changing go.mod, and Go correctly rejected the earlier recorded go.sum hash; corrected the test by adding v0.1.1 rather than deleting checksums. The final Go 1.26.10 ZIP is 17,439,168 bytes with SHA-256 2fc7dc04202926deeab6c7181891a6b9c88e58ce669dab99b1867d364d3970ea. Go dependencies other than the unpublished fixture retained the public checksum database; GONOSUMDB excludes only native.example.org. The generated native module has no local replacement and no consumer purego dependency. The current unpublished GPUI client/scene/generated protocol source is a local regular-file module fixture, not a public Go release. The local server validates the intended GitHub asset redirect URL but substitutes a local ZIP transport because no GPUI native Release/domain was published. It does not prove deployed Cloudflare/public native sumdb availability.

PASS: the generated typed binding consumer executed the full actual headless GPUI protocol (handshake, window open/update/events, pending pull cancellation, close/quit and ordered Sable teardown) at GOMAXPROCS 1 and 4, then after ad-hoc Hardened Runtime signing and strict signature verification. The broader headless case reports one or two remaining background/process TLS blocks with two destroyed blocks; do not claim complete background drainage. The narrow initialization fixture reports zero live/two destroyed blocks. otool lists only system frameworks/libraries, with no application GPUI dylib. No runtime extraction, native compilers, purego registration or extra native shared library is used by these consumers.

PASS: actual native AppKit window handshake/open/update/quit at GOMAXPROCS 1 and 4 and after signing. Tests write explicit progress markers because an initial zero-exit run had no post-loop Go output. All four pre-quit markers are present; the native-loop-returned marker is absent. Inspection of pinned gpui_macos/src/platform.rs lines 578-594 confirms quit asynchronously calls NSApplication.terminate. Thus the GUI proof establishes native window commands and process termination, not post-loop Go/Sable cleanup. Headless explicit cleanup remains a separate passing layer. Recorded this termination boundary in architecture and the host-integration follow-up; no Rust or Go runtime implementation changed. Pixels, actual keyboard/mouse/IME and Developer ID/notarization were not tested.

PASS: released keygen/sign/verified-build with an ephemeral local test key and independently supplied test policy covering the complete v0.1.1 native module. The final signed headless build reports three authenticated native archive members, fresh private build cache and a complete executable hash; it runs the same protocol successfully. A consumer-init sentinel is absent during verification and appears only on explicit execution. After warming the ordinary shared cache, changing a real Rust error-string byte inside a structurally valid cached native ELF payload is rejected with 'signed bytes differ at native_0_darwin_arm64.syso', no output executable and no consumer initialization. Restored the fixture cache's exact original bytes/mode afterward. An initial verifier attempt rejected a harness symlink in the live local GPUI replacement; retained the verifier's strict rules and all user symlinks, then used a clean client/scene/protobuf source fixture instead. This is a supported fixture boundary, not an authentication bypass. No production keys or approval authority were introduced.

PASS: Go 1.26.10 CGO_ENABLED=0 source tests and CLI build for the exact Bao release snapshot, strict TypeScript checks, actionlint for both workflows, git diff --check, and make harness-check. Node suite: 15 pass and one case-collision check explicitly skipped on the case-insensitive filesystem. The full hosted Actions run and publication source Git guards were not exercised: gpui-go still has no HEAD, and no source revision was fabricated to bypass that guard. No GitHub assets, Cloudflare deployment, commits or pushes were performed. Existing production Go host/Sable cgo integration remains unchanged.

Machine-readable summary: .agents-workspace/tmp/bao-v0.0.0-e2e-xuigiles/result.json. Reproductions/logs include http-headless-upgrade.mjs, final-go-get.log, final-headless-{1,4,signed}.log, signature-headless-final.py, verified-headless-final.log, substituted-syso-final.log, source-tests.log, source-build.log and native-window-markers-{1,4,signed}/. Published CLI is bin/bao-v0.0.0; final generated consumers include bin/bao-v0.0.0-headless-final and bin/bao-v0.0.0-verified-headless-final. The test fixtures and local keys remain ignored scratch data.


## 2026-11-09: Per-platform native release pipeline and umbrella shim

Took over the native release work at the user's request. `.github/workflows/native-release.yml` now runs a target matrix driven by `dist/native-targets.json`: consolidated static library (`.a`, `.lib` on Windows), Bao `.syso` package translated from that library, Go module ZIP of that package, then one `SHA256SUMS` over every asset with an optional draft Release. Targets are darwin arm64/amd64, linux amd64/arm64 and windows amd64/arm64. Windows uses GNU-ABI Rust targets because Bao rejects MSVC Rust targets, so the `.lib` is a GNU-ABI COFF archive. `dist/metadata-proxy/scripts/build-release.ts` was split into build-archive, build-syso, build-module and collect-release.

Added `internal/nativeshim` as the only Go boundary to the native library. Generated `prebuilt_<goos>_<goarch>.go` files forward each function in `dist/native-bindings.json` to its per-platform module under `-tags gpui,gpui_prebuilt` and CGO_ENABLED=0, with a Go completion pump replacing Sable's cgo package. Plain `-tags gpui` keeps the existing cgo/Sable link, which local Rust development and the headless and automation archives still need. `host` no longer imports C or Sable. The GPL editor keeps its own cgo boundary. The module prefix is the placeholder `native.example.org/gpui`, because no discovery domain has been chosen; the release job refuses it for drafts.

Finding: stock Go's Mach-O binder (cmd/link/internal/ld/macho.go, 1.26.10) emits `BIND_OPCODE_SET_DYLIB_ORDINAL_IMM | uint8(d)&0xf` for ordinals 1 through 127, so ordinal 16 is written as 0 (this image). Bao's package names 15 dylibs. The minimal smoke passed, but `examples/counter` reaches package net, which adds libresolv, moved libobjc to ordinal 16 and aborted in dyld with `Symbol not found: _class_addIvar` (53 binds to this-image in dyld_info). `scripts/darwin-imports.ts` now binds AppKit/Foundation through Cocoa and CoreGraphics/CoreText/ColorSync through ApplicationServices, reducing 15 dylibs to 12, and fails the build above 14. Re-run: 0 this-image binds. The earlier dyld abort is the negative control for this fix. This edits Bao's generated output; verified-build compatibility was not tested.

PASS on darwin/arm64 with the local Bao v0.0.0 binary and Go 1.26.10: all stage scripts through `release:collect`, with `--source-sha unverified-local` because this repository still has no HEAD. PASS: `release:smoke`, meaning file-proxy ZIP download, go mod verify, CGO_ENABLED=0 build with unavailable compilers, native init at GOMAXPROCS 1 and 4, ad hoc Hardened Runtime signing, and `examples/counter -smoke` headless through the shim. PASS: native window `examples/counter -smoke` with `-tags gpui,gpui_prebuilt` built against the module directory. PASS: `make integration` (cgo through the shim), native counter smoke, `go build`, `go vet`, `go test -race ./...` (Go 1.26.5), 19 Node tests with 1 skip for case-insensitive filesystems, actionlint, make harness-check. PASS: all six target proxies compile and vet under GOOS/GOARCH with stub modules, in scratch. PASS: `sha256sum`-equivalent check of SHA256SUMS; negative controls rejected a tampered library against its sidecar and the placeholder prefix.

FAIL: darwin/amd64 builds through stage 4 but the executable aborts under Rosetta with `symbol not found in flat namespace '_section$end$__DATA$_CTR0GR_ISIZE_FN'`. Bao's boundary synthesis does not cover this x86_64 output; tracked in TODO. Not run: the hosted workflow, any Linux or Windows link/execution (only `cargo check --target x86_64-pc-windows-gnu` and stub-module proxy compiles), actual draft Release upload, Worker deployment, signing. Native window evidence is the smoke's command path, not pixels or OS input. Logs and candidate assets are under `.agents-workspace/tmp/release-pipeline/`.


## 2026-11-09: Native release pipeline work summary and additional findings

Supplements the preceding pipeline entry, which holds the main design, Go linker finding and pass/fail evidence. Work summary by area:

- Producer: `dist/native-targets.json` is the one list of targets and the module prefix. `dist/metadata-proxy/scripts/` gained `targets.ts`, `build-archive.ts`, `build-syso.ts`, `build-module.ts`, `collect-release.ts` and `darwin-imports.ts`; `build-release.ts` was removed; `release.ts` gained `writeChecksum` and `assertSourceRevision`; `smoke-release.ts` is now target-generic and also builds the host through the shim.
- Workflow: jobs are `plan`, `library`, `syso-and-module` and `release`. The `module_prefix` input was replaced by the table, and a `targets` input selects a subset. `.github/actions/llvm-mingw` installs LLVM-MinGW 20261104 using the digests recorded in Bao's `ci/windows-llvm.ps1`.
- Go: `internal/nativeshim` holds the generator, six generated proxies, the prebuilt Sable completion pump and the cgo backend. `host/run_sable.go`, `host/run_native.go` and `host/run_headless.go` call only the shim. `architecture_test.go` permits the new package for `examples/uitest`. `make generate-nativeshim` regenerates the proxies, and a default `go test` fails if they are stale.
- Documentation: `dist/README.md`, `ARCHITECTURE.md`, `README.md`, `host/README.md` and the TODO section "Native release pipeline follow-up".

Additional findings not recorded above:

- The prebuilt pump must hold Sable's result address as a uintptr. An empty result can carry a dangling sub-page sentinel, which the garbage collector rejects if it sits in a pointer-typed stack slot. Sable's `callResultBytes` documents the same hazard.
- With `CGO_ENABLED=0`, package `github.com/moriyoshi/sable` cannot be compiled because its implementation is in cgo files. That is why the shim owns `Call`, `Start` and `Shutdown` rather than leaving `host` on Sable.
- Bao requires Go's internal linker, so the prebuilt path is documented for `CGO_ENABLED=0` only. Bao's README describes a cgo-enabled internal-link profile, which was not tested here.
- The minimal `dist/native-smoke` consumer cannot detect the dylib ordinal failure: it does not reach package net, so libobjc stays at ordinal 15. Only the real host build exposed it, which is why `release:smoke` now runs both.
- In that minimal consumer libresolv was listed at position 16 but had no binds, so its ordinal truncation was harmless. A passing launch there says nothing about ordinal headroom.
- AppKit and Foundation are installed under `Versions/C`, not `Versions/A`. A rewrite keyed on `Versions/A` misses those two and yields 13 dylibs rather than 12, which still passes the guard, so the guard does not catch a wrong path. The real paths came from the generated `symbols_darwin_arm64.go`, and the map was corrected before the module was rebuilt.
- Bao reads a `.lib`-named archive: a darwin/arm64 archive copied to `x.lib` generated three syso objects. This covers the filename only, not a Windows COFF input.
- `cargo check --locked --target x86_64-pc-windows-gnu` passes for `rust/native`, including `gpui_windows`. It proves no linking, and no Windows runtime behavior.
- The Linux Docker VM here has 2 CPUs and 7.7 GiB and was running unrelated containers, so no Linux release build was attempted locally.

Tooling pitfalls hit during the run:

- Without `CARGO_TARGET_DIR`, Cargo writes to `rust/native/target`, not the repository `target/`. `build-archive.ts` now sets it to match the Makefile, and the stray directory was removed.
- zsh `NO_CLOBBER` silently kept an old log: a rerun redirected with `>` failed before starting, and the old failure was reread as a new one. Use `>|` for logs in scratch space. The same option blocked two heredoc rewrites in `host/`.
- `timeout` is not installed on this macOS host. `perl -e 'alarm shift; exec @ARGV' SECONDS CMD...` was used instead.
- Go module caches are read-only, so a scratch check directory needs `chmod -R u+w` before removal.

Precautionary workflow edits, neither exercised on a hosted runner: the duplicate-release check uses `grep` instead of `rg`, which was not confirmed to exist on hosted images; and the optional Windows arguments use a form that stays safe under `set -u` in older Bash.

Candidate evidence, built with `--source-sha unverified-local`, Rust 1.99.1, macOS SDK 26.5 and Bao commit 9553572b2000f7826f9fcea8a612b1357cfa6f42, for fixture version v0.0.1-rc.1. Sizes are bytes:

| Asset | Size | SHA-256 |
| --- | --- | --- |
| darwin-arm64.a | 73872296 | d174824046217dcab721063f071db7960a1309943e9768e45fa39bf347e5a830 |
| darwin-arm64-syso.zip | 17408015 | 51f04f14dd4f9fb11a012b5b9acd9ff2fe4dba4c6931c8d5450175d21647f79f |
| darwin-arm64.zip | 17648732 | 706443ce0ba5f47877906b74ee62a3054c0d7b9447e543c0eb236c949ef8a128 |
| darwin-amd64.a | 74086216 | 7a602d744df7fc76540620a9f98a2d7148798a7499ab0c60ca673f13cc7e6f83 |
| darwin-amd64-syso.zip | 18449683 | 5d1a56b36deafeef5c2a57828a6516bc89d19bed59072985236d0593293b51eb |
| darwin-amd64.zip | 18691332 | 6d1c1615abfd923f5f45257fcb725207a5192525a06e98ed4f9e83237c50701c |

These are local fixtures, not published artifacts. The darwin/amd64 ZIP does not launch. Scratch outputs are in `.agents-workspace/tmp/release-pipeline/`: `candidate/` for assets and `SHA256SUMS`, `darwin-arm64/check/` for smoke logs and commands, and `local.mod` for the module-directory replacement used in the native window and Rosetta builds. The native window and Rosetta executables were removed after their runs.

No commit, push, Release, Worker deployment or signing was performed in this work. Open items are in TODO.


## 2026-11-09: Hosted v0.0.0 rehearsal, Linux prelink and Worker CD

At the user's request, followed rehearsal run 42562229131 on commit 1ff1f21 and fixed what could be fixed, then added Worker deployment. Result of that run: `plan` and all six `library` jobs passed, so GPUI builds a static library for every target on hosted runners. `syso-and-module` passed on darwin/arm64 and failed elsewhere; `release` was skipped because every matrix leg must pass.

darwin/amd64 failed `release:smoke` with the dyld error already recorded. Treating an absent section as an empty range in Bao's `synthesizeBoundaries` passed Bao's tests but did not change the failure, so that edit was dropped.

Windows failed in Bao's own `go test ./...`: the `verify` package assumes POSIX permissions and executable names. The workflow now skips Bao's tests on Windows. Run by hand on macOS against the real windows/amd64 library and LLVM-MinGW 20261104, Bao then asks for DLL mappings one undefined symbol at a time. After 40 mappings it stops with `bare Windows import "SysFreeString" must be a function; use an explicit dllimport declaration for IAT references`. There is no Bao option for this, so Windows stays blocked on a Bao change.

Linux failed with `unsupported ELF program headers or extended section indices`: the archive combines to 334,676 sections on arm64, above SHN_LORESERVE, and Go's ELF loader has no extended-index support either, so skipping Bao's strip step would not help. Measured on the real linux/arm64 artifact in an Ubuntu 24.04 container with the released Bao binary: archive member selection alone leaves 334,676 sections, and GC with kept init arrays 163,230. `scripts/linux-prelink.ts` now does GC plus section merging through a linker script. Findings on the way: `.eh_frame` must be kept explicitly or GC removes it; statically linking libxcb, libfreetype and a source-built libxkbcommon reached Bao but the Go link then failed with an AArch64 LDST64 relocation error, so static bundling was abandoned; merged text needs folded aliases because LLVM's identical code folding left 362 symbols on one body and Go's findfunc table allows about 256 functions per 4 KiB; `SUBALIGN` alone did not help; and objcopy must localize and strip in two passes because it keeps globals of a relocatable object.

Bao v0.0.0 only binds a hardcoded list of glibc and C++ runtime libraries, while GPUI needs libxcb, libxkbcommon and libfreetype (107 unresolved symbols). `dist/bao-patches/gpui-native-targets.patch` extends the list in linuxelf/translate.go. The workflow applies it to the pinned commit, and its hash is recorded as `bao_patch_sha256` in `publication.json`. Patched Bao's `linuxelf` and `translate` tests pass.

PASS in the Ubuntu 24.04 arm64 container with the real hosted linux/arm64 library, the patched pinned Bao and Go 1.26.10: `prelinkLinuxArchive` (25 sections, 4,097 aliases folded), Bao translation (231 imports), CGO_ENABLED=0 build with unavailable compilers, and headless `examples/counter -smoke` through the shim at GOMAXPROCS 1 and 4. ldd shows libxcb, libxkbcommon and libxkbcommon-x11 bound dynamically. Not run: linux/amd64 past stage 1, the full workflow with these fixes (they are uncommitted), or any Linux native window.

Worker CD: `.github/workflows/deploy-metadata-proxy.yml` deploys on relevant pushes to main and takes its domain from `modulePrefix`, stopping after bundling while that is a placeholder. `.github/workflows/activate-native-release.yml` turns a published Release into a catalog pull request after verifying metadata against SHA256SUMS. `scripts/check-deployment.ts` exercises discovery, version list, go.mod bytes, ZIP redirect and the release asset. Wrangler 4.158.0 is pinned in `dist/metadata-proxy/deploy/` with its own lockfile so release runners do not install it. The release workflow's placeholder check moved into `plan`. PASS: actionlint, TypeScript, 21 Node tests with 1 skip, `wrangler deploy --dry-run` bundling. The deployment check's unit test drives the real Worker and has a missing-asset negative control. Not run: any actual deployment, activation run or Cloudflare API call. No Cloudflare account, domain or repository secret was available to this session.


## 2026-11-09: Documentation site and gpui-go.dev module root

At the user's request, added a VitePress 1.6.4 site under `docs/` with its own lockfile, deployed by the metadata Worker through a static assets binding on `gpui-go.dev`. `modulePrefix` is now the bare domain, so native modules are `gpui-go.dev/<goos>-<goarch>` and share the root namespace with the documentation. `run_worker_first` sends every request to the Worker: `createSite` gives `/proxy/`, catalog module paths and non-GET/HEAD methods to the metadata handler and everything else to the assets. Documentation pages stay under paths such as `/guide/` so a module route cannot hide them. The main module path remains `github.com/moriyoshi/gpui-go`. `release:check-deployment` now also requires an HTML home page, which keeps it meaningful while the catalog is empty, and the deploy workflow builds the site first.

Finding: `go mod tidy` in an external consumer fails because it resolves the tagged prebuilt imports, which are unpublished. The getting-started page uses `go build -mod=mod` instead.

PASS: npm run check; Node suite 24 tests, 23 pass, 1 existing case-collision skip, including root-prefix routing and a missing-home-page negative control; go build, the nativeshim staleness test and full go test; make harness-check; git diff --check; VitePress build with its dead-link check; `wrangler deploy --dry-run --domain gpui-go.dev`. PASS: `wrangler dev` served pages, clean-URL redirects, 404s and proxy routes through the real assets binding, viewed in Chrome. PASS: documented snippets compiled with and without native tags in a scratch module under `.agents-workspace/tmp/docs-snippets/`, and the first-window sample linked against `target/release/libgpui_go_native.a` using the exact documented commands. The linked sample was not launched. Not run at the time of this entry: real Cloudflare deployment (the local Wrangler login had expired), the hosted workflow, public discovery and any go get against gpui-go.dev. actionlint was not installed.

## 2026-11-09: Element reference with native screenshots

At the user's request, added `docs/reference/` with a page per element kind plus node methods, style properties, events, reactive nodes and GoX tags, written from `scene`, `ui`, `ui/gox`, the protobuf schema and the Rust validators and renderer. `examples/docshots` opens each sample as a real GPUI window and saves it with `screencapture -l`, using `tools/docs_window_ids.swift` to find its window. `make docs-screenshots` regenerates all 41 PNGs under `docs/reference/screenshots/`. macOS only.

Findings recorded in the reference: setting `FlexDirection` forces flex display even when the same style sets `Display` to grid or none, because `apply_style` writes it after `display`. Images keep their aspect ratio: a narrower box centers them and a shorter box lets them extend past the bottom, so they do not simply fill the box. Disabled text inputs have no distinct default appearance. `Node.Interact` replaces what `Focusable` and `BindKey` set earlier. A snapshot text input value containing a line break is rejected, although native typing and paste normalize them. Calling element methods on a `ui` fragment dereferences a nil element.

PASS: one complete `bin/docshots` run on macOS/arm64 produced all 41 images, which were reviewed on a contact sheet. Those windows are real rendering, but the states shown are set from Go: focus with `Window.Focus`, scrolling with `ScrollTo`, events with `DispatchAction`. No mouse or keyboard input was synthesized, and the hover image applies the hover color as a base style. PASS: all 24 Go fences in the reference compile under a scratch harness that tolerates only unused variables, `go test ./...`, Node suite 23 pass and 1 existing skip, make harness-check, git diff --check, VitePress build, `wrangler deploy --dry-run --domain gpui-go.dev`. Real Cloudflare deployment remains not run because the local Wrangler login has expired.

## 2026-11-09: Bao v0.0.1 adoption and uitest without a display

Bao v0.0.1 (commit 18c47c82d494c5a7fff7b4ecebd02a98c728d7a5) replaces v0.0.0 as the workflow default. Target options moved to `dist/bao.config.json`; `dist/bao-patches/` and the `bao_patch_sha256` provenance field are gone. Kept in gpui-go because v0.0.1 does not address them: the Linux prelink and the Darwin dylib consolidation. Bao's own tests stay skipped on Windows, because its `verify` package is unchanged and needs Bazel and POSIX permissions.

Evidence on the real archives. Hosted run 42596797159 on 5fe54e0: PASS syso, module and `release:smoke` for darwin/arm64, darwin/amd64, linux/arm64 and linux/amd64 (first full linux/amd64 path). FAIL windows/amd64 and arm64: `spawn ...\\bin\\bao ENOENT`, because `go build -o` does not add `.exe`; the workflow now adds it, not yet rerun. Local with the released binary: darwin/arm64 `release:smoke` PASS; darwin/amd64 counter under Rosetta at GOMAXPROCS 1 and 4 PASS; windows/amd64 and arm64 translated, packaged and linked, not executed. A local Ubuntu 24.04 linux/amd64 container hit a Go internal compiler error compiling `runtime` under emulation; the hosted pass shows that was the emulated environment.

CI `TestPackageArchitecture` failed on a cold module cache because `CombinedOutput` mixed `go: downloading` lines into the JSON. Reproduced with an empty GOMODCACHE; stderr is now separate; passes on cold and warm caches.

`uitest` has two different headless cases.

`gpui_headless` links `rust/runtime` only. Its `Headless` backend makes open, commit and close no-ops, performs no GPUI layout, advertises no features, returns Unsupported for automation and for platform services (clipboard), and accepts Focus and ScrollTo as no-ops. Input exists only as `gpui_go_headless_click` and `gpui_go_headless_text_change`, straight into the mailbox. `uitest/gpui.NativeTarget` needs `test-automation-v1`, and locator actions need rendered bounds, focus and key dispatch. Built with `gpui_e2e,gpui_headless`, it stops with `native archive lacks test-automation-v1`. It cannot serve uitest without becoming a GPUI backend.

GPUI's own display-less Linux platform is different. `gpui_platform::application()` starts headless when the environment names no display server. `HeadlessWindow` does real layout and text shaping and discards the scene. `automation.rs` calls `window.draw` and `window.dispatch_event` directly, so it does not need platform frame or input callbacks. Linux only: Windows refuses `open_window` while headless, and macOS `open_window` has no headless branch (not run).

Experiment in Ubuntu 24.04 linux/arm64, no DISPLAY or WAYLAND variables, cgo build of `rust/native --features automation`, one process per scenario:
- Unmodified: the window opens and scenarios run. `make uitest`'s consumer fails waiting for `Count: 1`. Of 11 scenarios 2 pass, and which ones varies between builds; the others stop at the last-window `close_app`/`close` with `application exited` or `context canceled`, and clipboard fails on content. Forcing `gpui_platform::headless()` gives the same pattern.
- Causes, neither specific to headless: GPUI's `QuitMode::Default` quits on non-macOS when the last window closes, racing the close response (`window-isolation` passes closing the first of two windows and fails only at the last); and `examples/e2eapp` binds only `cmd-j` while `ControlOrMeta` is `ctrl` on Linux.
- With only `.with_quit_mode(gpui::QuitMode::Explicit)` and an added `ctrl-j` binding, in the container copy only: the uitest consumer and 10 of 11 scenarios PASS in 3 of 3 rounds. `clipboard.star` FAILS in all 3 (`got ""`), matching `DisplayConnection`'s headless clipboard, which drops writes and reads None.

Not run: Linux with a real display, so the same two defects there are inferred from source; linux/amd64; GPUI headless on macOS or Windows. Headless windows are `is_active` false and draw nothing, so this covers layout, focus, hit testing and input dispatch, not pixels or OS input. The patches were scratch-only and no repository behavior changed for this finding. Commands and logs: `.agents-workspace/tmp/uitest-headless/`, `.agents-workspace/tmp/bao-v0.0.1/`.


## 2026-11-09: uitest and E2E without a display on Linux

Applied the fixes found in the entry above and added CI coverage.

* `rust/native/src/lib.rs` starts GPUI with `QuitMode::Explicit`. The `Message::WindowClosed` path already quits when the last view goes away, so GPUI's non-macOS last-window quit only raced the close reply.
* `examples/e2eapp/app.go` binds `ctrl-j` as well as `cmd-j`, because `ControlOrMeta` resolves to `ctrl` on Linux.
* `internal/nativeshim/cgo_native.go` gained a `#cgo linux LDFLAGS` directive beside the darwin one. No native Linux `make` target could link without it; the earlier experiment passed these libraries by hand. Builds tagged `gpui_headless` or `gpui_prebuilt` exclude that file.
* `make e2e-no-display` runs every scenario except `clipboard.star`, which GPUI's headless Linux platform cannot serve. CI job `native-linux-no-display` (ubuntu-24.04) runs `make uitest` and that target.

Verification: in a clean Ubuntu 24.04 linux/arm64 container with no display variables, Go 1.26.5 and Rust 1.99.1, `make uitest` and `make e2e-no-display` passed 3 of 3 rounds, with all 10 scenarios in one worker process. Negative controls: `clipboard.star` still fails with `got ""`, and the unmodified build from the earlier entry failed `uitest` and 9 of 11 scenarios. On macOS, `make test-native` (12 tests), `make test-e2e-native` (13 tests), clippy with `automation` and `-D warnings`, and `go test .` passed.

Not run: the new job on hosted linux/amd64, Linux with a real display, and `make e2e` or `make uitest` on macOS after the quit-mode change. Coverage is layout, focus, hit testing and synthesized input; not pixels or OS input. Logs: `.agents-workspace/tmp/uitest-headless/make-*.log`.


## 2026-11-09: End-to-end and headless testing documentation

Replaced `docs/guide/testing.md` with a setup guide for applications outside this repository, and added `docs/guide/headless-testing.md` and `docs/reference/uitest.md`. The headless page separates GPUI's display-less Linux platform, which runs `uitest`, from the `gpui_headless` protocol backend, which cannot.

The guide's three Go files were compiled and run unchanged as an external consumer (scratch: `.agents-workspace/tmp/docs-snippets/uitest-sample/`), with `-replace` and an `automation` archive.

Verification. macOS/arm64 with real windows, after the `QuitMode::Explicit` change that the previous entry had not covered: `make e2e-check`, `make uitest`, `make e2e` (11 of 11) and the sample all PASS. Ubuntu 24.04 linux/arm64 container with no display variables, on the current tree: `make uitest`, `make e2e-no-display` (10 of 10) and the sample 3 of 3 rounds PASS (logs: `.agents-workspace/tmp/uitest-sample-linux/`). A variant confirmed `Fill(ctx, "")` clears a field. Docs build, `make harness-check` passed.

Finding: `uitest/README.md` says timeout errors include the last observation. When the deadline expires during a native call, the condition error path returns without it, for example `uitest: testid("count") to have text "Count: 2": context deadline exceeded`. The site documents the actual behavior; the code is unchanged.

Not run: hosted CI job, linux/amd64, Linux with a real display (so `env -u DISPLAY -u WAYLAND_DISPLAY` on a desktop is from GPUI source only), and the guide's GitHub Actions snippet. The site is not deployed. All input was synthesized inside GPUI: no OS input, IME, screen-reader or pixel coverage.


## 2026-11-09: Prebuilt modules include automation

`release:archive` now builds `rust/native` with `--features automation`, so consumers can run `uitest` with `-tags gpui,gpui_prebuilt,gpui_e2e`. No new C symbols are involved: automation uses the existing protobuf request path. Go's `Automate` remains behind `gpui_e2e`, so requests can only come from in-process test code. The cost in every prebuilt consumer is recording element bounds on each render, plus about 6 KB of archive on darwin/arm64 relative to the earlier rc build, which came from slightly older source.

`release:smoke` links `examples/uitest` on every target and runs it on Linux, where GPUI needs no display. The run fails unless the archive advertises `test-automation-v1`, which guards against the Cargo flag being dropped. The feature flag is target-independent, so the Linux run covers the build step for the other targets.

Verification: the full archive, syso, module and smoke stages passed for darwin/arm64 locally and linux/arm64 in an Ubuntu 24.04 container with no display variables, including the prebuilt driver run on Linux. The prebuilt driver also passed 3 of 3 runs on macOS with a real window, which goes beyond what `release:smoke` does there. Negative control: `test-automation-v1` occurs once in the new darwin/arm64 archive and not at all in the earlier one. `npm run check` and `npm test` passed.

Not run: amd64 or Windows archives with the feature, and the hosted workflow. Logs: `.agents-workspace/tmp/automation-release/`.


## 2026-11-09: reactive update allocations and Flush scaling

Measured whether `sync.Pool` would help `reactive`. It did not: a prototype pooling dependency slices and maps, flush maps and scope maps halved allocations but left time within 0 to 10 percent of the original. A graph belongs to one goroutine, so the pool's synchronization has no use, and `*Scope` is handed to user code, so scopes themselves cannot be recycled. Per-computation reuse was faster and removed more allocations, so that was applied instead.

Changes: `computation.update` reuses a sort buffer and clears `deps` instead of replacing it; a `Scope` creates its maps on first use; run-limit counting is a per-computation counter stamped with a flush ID, because clearing a reused map costs time proportional to its peak size and one large flush would slow every later flush; `Flush` reads a `failing` set instead of scanning every node. A computation disposed during its own failing run stays out of that set, matching the removal from `nodes` that the scan relied on.

Median of 5 runs, Apple M1 Max, before and after: one signal to one effect 462 ns and 10 allocs to 265 ns and 2 allocs; signal to 100 memos to one effect 58.2 us and 1016 allocs to 38.3 us and 300 allocs; one update among 10,000 unrelated effects 72.0 us to 0.29 us; Go UI path with 100 changing bindings including protobuf encoding 184 us and 2261 allocs to 154 us and 1550 allocs. With one of 100 bindings changing, the UI path stayed near 125 us and 1350 allocs: the cost there is the whole-tree snapshot and protobuf work, not the graph.

Verification: `go test -race ./...`, `go vet ./reactive`, `gofmt -l reactive`. New tests cover disposal unblocking frames, disposal during a failing run and the per-flush run limit. Negative controls through `-overlay` each failed the expected tests: removing the disposed guard, removing the `failing` delete on dispose, not resetting the run counter, and not recording failures. These are Go-side microbenchmarks; no real application was profiled and the native side is excluded. Scratch: `.agents-workspace/tmp/reactive-bench/`.


## 2026-11-09: GoX guide examples and screenshots

Rewrote `docs/guide/gox.md` around compiled samples. The templates live in `examples/docshots/view.gox` (generated `view.x.go`, covered by `GOX_SOURCES`, `generate-gox` and `check-gox-generated`) and `gox_shots.go` captures them. The page's template and Go excerpts were inserted from those sources, so they match what compiled.

Finding: in a GoX `span`, literal text and each `~( )` become separate text elements, and the plain container stacks them, so `<span>Hello, ~("Ada")!</span>` renders on three lines. Seen in the native capture; documented in the guide as "One text per line". No code change.

Verification on macOS/arm64: `make check-gox-generated`, `go vet ./examples/docshots`, `go test .`, docs build and `make harness-check` pass. `bin/docshots -only gox-` captured real windows, and all 12 images on the page were viewed and load in the browser. States were set from Go signals, so the `Switch` handler was compiled but not clicked. The full 41-image `make docs-screenshots` was not rerun.


## 2026-11-09: Reactive and Zed editor guides, Text input merged

Expanded `docs/guide/reactive.md` from `examples/docshots/reactive_guide.go` and `docs/guide/editor.md` from the new `examples/editorshots` (`make docs-editor-screenshots`, a separate program because `editor.Run` cannot share a process with `host.Run`). Page excerpts were inserted from those sources. Merged `docs/guide/text-input.md` into `docs/reference/text-input.md`.

Editor windows take keyboard focus when opened, so typing elsewhere during a capture run reaches the sample buffer. One `editor-undo` capture showed stray keystrokes. `editorshots` now locks the buffer read-only while waiting and verifies its exact text before the insert and undo shots.

Verification on macOS/arm64, real windows: reactive captures show the expected run counts (A 4 vs B 1, count 4 vs parity 1, separate 3 vs batched 2) and rows keeping their build number through a reorder; the mirror sample's preview text was asserted. All images were viewed. Docs build, `make harness-check` and `go test .` pass. States were set from Go, so click handlers in these samples compiled but were not clicked, and the guide's first editor listing was not compiled on its own.

Syntax highlighting is feasible but not implemented: the pinned Zed `grammars` crate ships Go grammar and queries, `language::Buffer::set_language` exists, and the base theme has syntax colors. It would need a grammar dependency, a language registry in the GPL component, and a versioned editor protocol option.


## 2026-11-09: Editor extension instructions, highlighting validated in scratch

Added `docs/guide/editor-extensions.md`. Its syntax highlighting steps were applied end to end in a scratch copy (`.agents-workspace/tmp/editor-syntax/repo`): Rust component, `Open.language = 8`, host, Go `Options.Language`. The repository's editor still has no highlighting; only the docs and `SaveOnChange` in `examples/editorshots` changed.

PASS on macOS/arm64 against real Zed: `Language: "go"` highlights (capture `docs/reference/screenshots/editor-syntax.png`), an unknown name returns `no grammar for language "cobol"`, `buffer_font_size` in the host settings JSON takes effect, and a build without `--locked` added only `grammars` and `tree-sitter-go` to the lockfile. `make docs-editor-screenshots` cannot regenerate `editor-syntax.png` until the feature exists in the repository.

Rust excerpts for highlighting were compared with the scratch source. Not run: other languages, the `ctrl-a` binding example, `make editor-smoke` or clippy on the scratch change, and the new-command checklist, which is read from existing commands. The scratch build shared `target/zed-editor`; `make editor-lib` restored the normal archive (no `tree_sitter_go` symbols).


## 2026-11-09: Masked text input

Added `bool masked = 7` to `TextInput`, exposed as `scene.TextInputOptions.Masked`, `ui.InputOptions.Masked` (an accessor, so a show/hide toggle can change it) and the GoX `masked` attribute. GPUI has no text input element or masking API, and Zed's masking lives in the GPL `editor` crate, so this is implemented in the Apache `rust/native/src/input.rs` widget.

A masked field shapes one bullet per grapheme. Selection, caret, marked range, click and IME positions convert between real and displayed byte offsets. Copy and cut are no-ops, paste works, `text_for_range` returns `None`, and accessibility gets `Role::PasswordInput` with bullets as the value. Go and automation still see the real value. Not covered: OS secure keyboard entry, and an input method's own candidate window. An older native archive would ignore the field and show plaintext, because there is no capability check for it.

Verification on macOS: `go build ./...`, `go vet ./...`, `go test -race ./...`, core tests, `make test-native` (19), `make test-e2e-native` (20), Clippy in both configurations, `make integration`, `make uitest`, `make e2e` (11 scenarios). Eleven negative controls on a scratch copy of `input.rs` each failed the expected tests, covering shaping, copy, cut, text services, role, click, marked range, caret, selection, bounds, character index and toggling. A real window with `hunter2` and mixed-script values drew bullets with the caret after the last bullet. Physical keyboard input and a real IME were not exercised, and neither was a screen reader: the accessibility role and value are checked at the widget, not in a platform accessibility tree.

Build hazard found on this machine: `target/release/libgpui_go_native.dylib` from 2026-11-07, left by a build when the crate was not staticlib-only. `-lgpui_go_native` prefers it over the fresh `.a`, so `make build` and other `-lgpui_go_native` targets, including `docs-screenshots`, silently run old native code. `otool -L` shows it. `make uitest` and `make e2e` are unaffected because they link `libgpui_go_e2e.a`. Verification linked a copy of the `.a` from a directory with no dylib. Logs: `.agents-workspace/tmp/masked-ctl/`.


## 2026-11-09: SVG images were never painted

`gpui::svg()` paints only with a color in its own style and does not inherit one, and the native renderer set none, so every `ResourceSVG` image was invisible. The feature smoke only proved decoding. `rust/native/src/lib.rs` now threads the inherited `TextColor` through `RenderEnv` (default black, matching GPUI's text default) and sets it on the SVG. Hover text color does not reach it. SVGs are single-color masks, so the `docshots` and `features` samples now use a check stroke alone rather than a white disc with a black check.

The first capture after the fix was still blank because `docs-screenshots` linked the stale 2026-11-07 dylib recorded in the earlier build hazard note. That target now names the archive. All 41 element reference and later native captures were regenerated; only `image-kinds.png` changed byte for byte, so the stale library had not otherwise affected the images. `make test-native` passed 19 tests. No automated test observes SVG pixels; the evidence is the native capture.


## 2026-11-09: Work summary for release, uitest, reactive and masked input

This summarizes the work whose details are in the entries "Bao v0.0.1 adoption and uitest without a display", "uitest and E2E without a display on Linux", "Prebuilt modules include automation", "reactive update allocations and Flush scaling" and "Masked text input".

Outcomes:

* Release pipeline moved to Bao v0.0.1. Hosted run 42596797159 passed for darwin and Linux on both architectures. Windows jobs failed only because the Bao binary lacked `.exe`; that fix has not had a hosted run.
* Headless terms are now distinguished. `gpui_headless` has no GPUI and cannot serve `uitest`. GPUI's display-less Linux platform can, except for clipboard. An earlier suggestion that Linux needed Xvfb was wrong.
* `make uitest` and `make e2e-no-display` run without a display on Linux, after fixing the last-window quit race, the `cmd-j`-only fixture binding and the missing Linux link flags. CI job `native-linux-no-display` has only been reproduced locally on linux/arm64.
* Published native libraries include `automation`. `release:smoke` links the `uitest` driver on every target and runs it on Linux.
* `sync.Pool` was rejected for `reactive` on measurements; per-computation reuse and a `failing` set were applied instead.
* `TextInput` supports masking through protocol field 7.

State at the end of this work: the release, CI and display-less changes are in `main` at 483540f, which replaced commit 55d02c4 from `fix/linux-no-display-uitest`. The `reactive` and masked input changes are not committed. No publishing, deployment or signing was performed. Open items are in TODO.

Correction to "Masked text input": there were twelve distinct negative controls, not eleven, the twelfth being character index. Shaping, copy, cut, text services, role, click and toggling were run before the final refactor of the selection helpers and not repeated; the other five were repeated on the final code.

Process findings for future sessions:

* A wrong native binary can look like a wrong implementation. When a real-window result contradicts passing native tests, run `otool -L` on the binary before debugging the code. A concurrent build was suspected for the plaintext screenshot, but it stayed plaintext after that build ended; the stale dylib in the masked input entry was the cause.
* In this shell, `cp` prompts before overwriting, `>` refuses existing files and bare `make` can fail as an unresolved zsh function. A `cp` restore hung on its prompt and left two temporary `eprintln!` lines in `rust/native/src/input.rs` for about ten minutes while another build shared the tree. Use explicit Python writes, `>|` and `/usr/bin/make`, and do not put debug edits in a tree another build is using; a build from that window would log input values to stderr.
* Negative controls found real test gaps. Four offset mappings had no observing test at first, and a composition test used text whose real and displayed offsets did not diverge enough to fail.


## 2026-11-09: Documentation site work summary and findings

Summary of the documentation session, and the parts that the entries above do not record. All of it is uncommitted relative to `483540f`, and the site is not deployed.

Current structure of `docs/`, which supersedes the paths named in earlier entries:

* Guide: `guide/introduction`, `getting-started`, `prebuilt-modules`, `concepts` (renamed from `views-and-events`), `reactive`, `gox`, `editor`, `limits`.
* Element reference: `reference/*`. `guide/features.md` was folded into `reference/index.md` as "Features by area". `guide/text-input.md` was folded into `reference/text-input.md`.
* Testing: `testing/index`, `setup`, `writing-scenarios`, `starlark`, `headless`, `api`. These replace `guide/testing.md`, `guide/headless-testing.md` and `reference/uitest.md`.
* `guide/editor-extensions.md` no longer exists. It is the "Extending the editor" section of `guide/editor.md`.
* The top bar has only Guide and Elements. The former Reference sidebar group was dissolved: Prebuilt modules is under Start here, Zed editor under Building interfaces, Limits under Element reference.

Added visuals: three inline SVG diagrams in `guide/concepts.md` and three in the How it works section of `guide/introduction.md`, using VitePress theme variables so they follow dark mode. Their styles are shared through `.vitepress/theme/diagrams.css`, the site's first theme override. `intro-counter.png` is a native capture of the element tree drawn in the introduction. There are 75 PNG files under `reference/screenshots`.

Findings not recorded above:

* Native windows opened by the capture tools take keyboard focus (`cx.activate(true)` in the editor host). Anything typed during a capture run reaches them.
* The GoX printer gives each literal text run and each placeholder its own text element, and a `span` stacks them. This is documented behavior now, not fixed.
* `uitest` timeout errors omit the last observation when the deadline expires during a native call, contrary to `uitest/README.md`. Still unfixed.
* Three API hazards are documented rather than fixed: `FlexDirection` overrides `Display` in the same style, `Interact` replaces earlier `Focusable` and `BindKey`, and element methods on a `ui` fragment dereference a nil element.

Tooling hazards hit during this work:

* A plain `cp` onto an existing file waits for an interactive answer in this shell, which hung a background job until the user asked about it. Use `command cp -f`.
* Restoring a source file with `cp -p` keeps its old modification time, so Cargo treats it as unchanged and keeps the artifact built from the temporary edit. A debug `eprintln!` survived in `libgpui_go_native.a` this way until `touch` forced a rebuild. It is confirmed absent from the archive and from `bin/docshots`.
* A scratch build sharing `target/zed-editor` overwrites `libgpui_go_zed_editor_host.a`. `make editor-lib` restored it, confirmed by the absence of `tree_sitter_go` symbols.

A correction to the review claims above: the element reference images were reported as reviewed, but the blank SVG panel in `image-kinds.png` went unnoticed until the user pointed it out. Viewing a capture did not establish that each panel showed what its caption claimed.

Open items:

* Deployment to `gpui-go.dev` is blocked on an interactive `wrangler login`, and the hosted workflow needs the `metadata-proxy` environment secrets. Neither public module discovery nor `go get` has been exercised.
* `guide/introduction.md` still says generic text inputs have no password masking, which is stale given the masked input work.
* The Rust-side syntax highlighting recipe is validated only in scratch, and only for Go. The repository's editor has no `Language` option.
* Other `-lgpui_go_native` Makefile targets still prefer the stale 2026-11-07 dylib on this machine.
* The Models and Platform areas have only table rows in `reference/index.md`, with no reference pages.
* Other work is in progress in the same tree and was left alone, including the staged `reactive/` changes and the masked input files. The captures were built against it.


## 2026-11-10: Editor SetSelection

Added `Editor.SetSelection(ctx, start, end)` and `State.Selection` through the protocol (`SetSelection` command, `Position`, `Selection`, `State.selection = 5`), `EditorComponent::set_selection`, the host and the Go client. Positions are a zero-based line and a UTF-8 byte column, which is what Zed's `Point` uses. A position is valid when `clip_point(point, Bias::Left)` returns it unchanged, which rejects positions past a line's end, past the last line and inside a character. Nothing is clamped. Multiple cursors collapse to one, `State` reports the newest selection, and moving a selection neither advances the revision nor emits `Changed`, so a user moving the caret still produces no event.

Verification on macOS/arm64: `go test -race ./editor` covers the wire format and that a reversed range never reaches the transport. `make editor-smoke` passes against real Zed and observes where text lands: selecting `本語` at columns 3 to 9 and inserting `X` yields `日X text`. It also covers four rejected positions leaving the selection unchanged, the first event being the edit rather than the selection move, read-only use, end of buffer and a closed handle. `cargo clippy --locked --workspace -D warnings` and `cargo fmt --check` pass for the editor workspace. `editor-selection.png` shows a selection at line 120 of 200 scrolled into view.

Not covered: the Rust component's reversed-range branch, because the Go client rejects those first; multiple cursors collapsing, which needs native input to create them; and real keyboard or IME input after `SetSelection`. No negative control was run against the new checks.


## 2026-11-10: Editors inside a scene.Div

An editor can now be placed in a generic view with `scene.Embedded`. The generic protocol gained an `Embedded` element and `rust/native/src/embed.rs`, an extension point that knows nothing about editors. The new GPL `rust/zed-editor/embed` staticlib links `rust/native` (now also an `rlib`) with the editor host (likewise), so one `host.Run` serves both protocols over one Sable runtime. The Go `editor` package registers the extension from `init` under the `gpui_zed_editor_embed` tag, because registration must precede application startup. No generic crate depends on the editor, and `go list` confirms `host` does not import it.

An editor is created in an existing window, because Zed's editor needs a `Window` to construct and commands should work before it is shown. So a window's first view cannot contain one: open the window, call `OpenEmbedded`, then `Window.Set`. `embedded-views` is advertised only with a provider installed, and core rejects `Embedded` on other backends, including headless.

Finding: `make editor-embed-smoke` first exited 0 without running the checks. The callback had returned an error and AppKit terminated the process before Go could report it. The example now logs failures before returning, and the Make target requires the pass line.

Verification on macOS/arm64: `make editor-embed-smoke` passes against real Zed and the generic backend, including persistence of text and selection across snapshots, editor survival while out of the tree, four rejected placements (unknown, repeated, another window's, closed), two independent editors, and both close paths. `editor-embedded.png` shows the layout. Also passing: `make test-core` with new `tests/embedded.rs`, `make test-native` (19), `make test-e2e-native` (20), `make integration`, `make uitest`, `make editor-smoke`, `go test -race ./...`, and Clippy with warnings denied for the native crate and editor workspace.

Not run: keyboard, mouse, Tab order or IME inside an embedded editor, since nothing here synthesizes input in this archive, so focus interplay with generic controls is unverified. Linux, where `host.Run` returns and Sable shuts down with an editor event call possibly pending. `Cargo.lock` in the editor workspace gained the generic backend's dependencies, mostly Linux-only. `ui` and GoX have no embedded element yet.


## 2026-11-10: Editor positions use visual columns

`editor.Position.Column` now counts visual cells instead of UTF-8 bytes. `Line` is still a buffer line, so soft wrapping does not restart the count and a window resize cannot change a position. The conversion is in `rust/zed-editor/src/column.rs`, walks grapheme clusters and uses `unicode-width`. A position inside a tab or a wide character is `InvalidSelection`, matching the earlier rule for the inside of a character.

The module mirrors the pinned Zed layout, read from `crates/editor/src/display_map/tab_map.rs` and `invisibles.rs`: tab stops count characters rather than cells, so a wide character before a tab does not move the stop; a tab starting at or after byte 256 (`max_expansion_column`) is one cell; a lone invisible character paints a one-cell replacement. Tab size and the limit are read from the live display snapshot. A Zed update can change these rules, so recheck them when bumping the pin.

Finding: a column is not a pixel offset. A scratch capture against a digit ruler (`.agents-workspace/tmp/editor-column`) showed `z` in `"\t日\tz"` and the selection at columns 4 to 6 of `ab日本cd` on the expected characters, but CJK glyphs from the fallback font were drawn at about 1.7 cells rather than 2, so following text sits slightly left of the ruler. Zed has no cell grid to match, and a pixel-derived column would depend on font and be fractional. The guide states this.

Verification on macOS/arm64: 8 unit tests in `column.rs`; `make editor-smoke` against real Zed, including rejected positions inside tabs and an `InsertText` at column 9 of the tab line that lands before `z`, which proves the mapping reaches the right buffer byte; `make editor-embed-smoke`; `go test ./editor/ .`; Clippy for the editor workspace; docs build.

Not run: emoji and invisible characters in real Zed (unit tests only), non-default tab sizes, and real keyboard input.


## 2026-11-10: Clamping positions inside wide characters

`editor.Position` gained `Clamp` (`ClampNone`, `ClampBefore`, `ClampAfter`), a per-endpoint choice for a column inside a wide character or a tab. The zero value keeps the strict rejection. Tabs are included because they are the same case to the conversion: no character starts at that column. Past-the-end columns stay errors.

The order check moved after clamping, in `set_selection`, because clamps can reorder endpoints within one character, and `Position` no longer derives `Ord`. The Go client's reversed-range check runs only when neither end is clamped.

`Options.WidthTable` and `EmbedOptions.WidthTable` choose how East Asian Ambiguous characters count: `WidthNarrowAmbiguous` (1), `WidthWideAmbiguous` (2 for every Ambiguous character with a width, from a table generated by `tools/gen_ambiguous_widths.py` for Unicode 16) or `WidthWideSymbols` (`unicode-width`'s `width_cjk`).

Finding: `width_cjk` is not the plain table. It keeps Ambiguous letters and modifier symbols at 1 (193 characters such as `α`, `Я`, `é`) and makes 56 Neutral characters such as `⅐` 2. Compared across all code points against Python `unicodedata` 16.0.0.

Finding: Zed has no East Asian Width logic in line layout and draws by font advance. A scratch host with BIZ UDGothic as the buffer font (`.agents-workspace/tmp/editor-column`) drew `α`, `Я`, `●`, `→` and `日` two cells wide, while Menlo drew the first four at one cell. CoreText metrics put `é` at one cell in BIZ UDGothic, so even the strict table can overcount. The table therefore changes only the count, and the guide says so.

Finding: `make editor-smoke` exited 0 without its pass line when a check failed, the same AppKit termination as the embedded smoke. It now requires the pass line, and the example logs the failure first.

Verification on macOS/arm64: unit tests in `column.rs` and `ambiguous.rs`; `make editor-smoke` with clamped and rejected endpoints in real Zed, and `α→x` under all three tables with an `InsertText` before `x`; `make editor-embed-smoke`; `go test -race ./...`; Clippy for the editor workspace; docs build.


## 2026-11-10: Selecting by character count

`editor.Position.Character` is an alternative to `Column` when setting a selection. It counts Unicode code points, matching Go's `utf8.RuneCountInString` and Zed's status bar. Both nonzero is `InvalidArgument`, which keeps `Position` comparable without a pointer field. A count inside a grapheme cluster follows `Clamp`. Reported positions leave `Character` zero, so a reported position can be passed back unchanged. The Go client's reversed-range check is skipped when a position is clamped or character-counted, since only native code knows where those land.

Verification on macOS/arm64: 21 unit tests in `column.rs` and `ambiguous.rs`; `make editor-smoke`, including results for code-point counts, clamps inside a combining cluster, rejected inputs and an `InsertText` at `Character: 3` of the tab line; `make editor-embed-smoke`; `go test -race ./...`; Clippy for the editor workspace; docs build.


## 2026-11-10: Clamp names follow text order

`ClampLeft` and `ClampRight` became `ClampBefore` and `ClampAfter`, since a screen direction is wrong on right-to-left lines. Behaviour did not change: positions were already resolved in buffer order. GPUI notes incomplete UAX #9 support (`crates/gpui/src/text_system/line_layout.rs`), so screen placement of bidirectional text is Zed's and was not examined. Only a unit test on Hebrew text covers this, not real Zed.

## 2026-11-10: Windows release smoke compiler sentinels

Prepare native release run 42736637512 (v0.0.0) built every library and module, but both Windows `syso-and-module` jobs failed in `release:smoke` at the first `go build`: `go: CC environment variable is relative; must be absolute path: /gpui-release-no-cc`. A rooted path without a drive letter is not absolute for Go on Windows. `scripts/smoke-release.ts` now passes the unavailable CC, CXX and RUSTC sentinels through `path.resolve`, which adds the current drive on Windows and leaves the POSIX paths unchanged.

PASS: npm run check; Node suite 24 tests, 23 pass, 1 existing skip. Not run: the Windows smoke itself, which needs a hosted Windows runner; later Windows steps in that script have never executed.

## 2026-11-10: Windows arm64 release compile runs out of memory

Prepare native release run 42748259431 (source 0ef32f4, Bao 18c47c8) passed the Windows CC sentinel fix above, but the windows/arm64 `release:smoke` failed in the first `go build`. After about 5 minutes, the compiler failed to allocate 9.2 GB in `ssa.prove` (`poset.aliasnodes`) while compiling `gpui-go.dev/windows-arm64`. Bao 18c47c8 emitted one straight-line `init()` with a separate branch per import, section alignment check, unwind root, and on arm64 per unwind table. The `aarch64-pc-windows-gnullvm` archive has 180,745 COFF sections and 33,203 `.pdata` sections, against 9,166 and 822 for `x86_64-pc-windows-gnu`. The amd64 `init()` was already 10,361 lines and peaked at about 740 MB of compiler RSS (about 270 MB with `-d=ssa/prove/off`, measured with local go1.27.3). Bao v0.0.2 (4ca57fab0ddfd368c8ea89b83bf355682671ee27) emits table-driven initialization; the release workflow now defaults to it. Its verified linker profile is still Go 1.26.10.

Not run: the release workflow with Bao v0.0.2; the arm64 package size under v0.0.2 is unmeasured here.

## 2026-11-10: v0.0.0 catalog and prebuilt consumer scenario

Prepare native release run 42764872868 (source 035fd5f, Bao v0.0.2) passed every job, including both Windows `syso-and-module` jobs, and created the draft Release `v0.0.0`. All 18 checksummed assets downloaded from the draft match `SHA256SUMS`. `npm run release:catalog` merged the six `native-release-*.json` files into `dist/metadata-proxy/catalog.ts`, which is what `Activate native release` does after publication. The Release is still a draft, so its asset URLs return 404 to anonymous clients. Deploying this catalog before publishing would advertise versions that Go cannot download, and `release:check-deployment` would fail.

Method: `checkDeployment` ran in process against the Worker built from the new catalog, with discovery on `https://gpui-go.dev` and each GitHub asset answered from the downloaded file. A local HTTP wrapper served the same Worker as `GOPROXY`, replacing each 302 to a draft asset with the checksummed local file. The consumer was an external module outside the checkout with `GOTOOLCHAIN=go1.26.10`, `CGO_ENABLED=0`, `GONOSUMDB=gpui-go.dev` and `-mod=mod`. It required `github.com/moriyoshi/gpui-go@035fd5f`, which resolved through `proxy.golang.org` as `v0.0.0`, because the `v0.0.0` tag also names the root module. The guide's hello program and the repository's examples and `cmd/gpui-e2e` were built from that dependency.

Findings:

* `go build -mod=mod` added all six `gpui-go.dev/*` modules to the consumer's `go.mod` and downloaded all six ZIPs (about 110 MB), as the publication guide warns.
* PASS on darwin/arm64: the hello window opens. The counter, showcase, features and counter-gox `-smoke` runs pass, as do headless counter, showcase and counter-gox, `examples/uitest`, and all 11 `gpui-e2e` scenarios including clipboard. Headless `features` stops at `missing native feature platform-services`, which is expected because headless execution rejects OS services.
* PASS for darwin/amd64 under Rosetta: counter, showcase and headless counter `-smoke`, `examples/uitest`, and all 11 scenarios. The guide's earlier "aborts at launch" no longer applies.
* PASS for linux/arm64 in an `ubuntu:24.04` container with only the documented runtime packages and no `DISPLAY` or `WAYLAND_DISPLAY`: headless counter and showcase smokes, the plain native counter smoke, `examples/uitest`, and 10 scenarios. `clipboard.star` fails without a display (`got ""`), as in `make e2e-no-display`.
* linux/amd64 under Colima's qemu-user x86_64 emulation: the headless and native counter smokes pass. `showcase -smoke`, `examples/uitest` and every scenario panic with a nil dereference in `reflect.Value.Elem` under protobuf's `initOneofFieldCoders` during `proto.Merge`, on every run and also with `GODEBUG=asyncpreemptoff=1`. The release run's hosted `ubuntu-24.04` smoke ran the same `examples/uitest` driver from the same module and passed, so emulation is the likely cause, but this is not proven. A TODO tracks a real-hardware run.
* All five other targets cross-build from darwin/arm64 with `CGO_ENABLED=0`. windows/arm64 compiles in seconds with Bao v0.0.2's table-driven initialization.
* The hello window's content area is black in both the prebuilt build and a source build against the local archive, so this is not a prebuilt difference. Text visibility in the guide's sample was not investigated further.

Not run: Windows execution (no Wine here; hosted release smoke covers the headless counter only), a native X11 window on Linux, the public Go mirror and checksum database for `gpui-go.dev/*`, and Worker deployment.

## 2026-11-10: Registration packages and nested module paths

Correction to the previous entry: the plain `go build` did not download all six module ZIPs; `go get` did. Measured against the local Worker with an empty module cache each time, a `go build` with a complete `go.mod` and `go.sum` downloaded only darwin/arm64, while `go mod tidy` and `go get github.com/moriyoshi/gpui-go@v0.0.0` downloaded all six. Both load imports as if every build tag were set, and `internal/nativeshim` imported every platform module from its tagged `prebuilt_<goos>_<goarch>.go` files. That made every consumer importing `host`, including source-build users, download about 110 MB on `go mod tidy`. Putting the platforms in separate packages of one module would not help, because Go downloads whole modules.

The shim now imports no native module. `gen.go` writes `internal/nativeshim/prebuilt_backend.go`, a `Backend` table of function values with forwarders, and one package per target, `prebuilt/<goos>/<goarch>`. Each package's `register.go` is constrained to `gpui && gpui_prebuilt` and its target, imports `gpui-go.dev/__binary/<goos>/<goarch>` and calls `nativeshim.Register` from `init`; a generated `doc.go` keeps the package valid elsewhere. `prebuilt` imports all six and is used by the repository's programs through a `prebuilt.go` file tagged `gpui && gpui_prebuilt` in each of `cmd/gpui-e2e` and the counter, counter-gox, docshots, features, reactive, showcase and uitest examples. `host.Run` calls `nativeshim.Ready()` first, so a prebuilt build without a registration returns `gpui: no prebuilt native module for <target>; import _ "github.com/moriyoshi/gpui-go/prebuilt/<target>"`. The cgo shim's `Ready` returns nil. `modulePrefix` is `gpui-go.dev/__binary`, and `release:module` appends `<goos>/<goarch>`. Go rejects a leading dot in a module path element (`.binary` fails with `malformed module path ... leading dot in path element` through a proxy, though a local `replace` hides it); `__binary` passes `go get`, `go build` and `go mod tidy`. The catalog is empty again, because the `v0.0.0` assets carry the old paths.

Verification, on darwin/arm64 unless stated:

* PASS: `gofmt`; `go build ./...`, `go vet ./...` and `go test -race ./...` with Go 1.26.5 (and 1.27.3); `go vet -tags gpui ./internal/nativeshim ./host`; `npm run check`; Node suite 25 tests, 24 pass, 1 existing skip.
* Negative control: pointing `prebuilt/darwin/arm64/register.go` at the amd64 module and deleting one field failed both generator tests, which passed again after restoring the file.
* The six `v0.0.0` module ZIPs were rewrapped under the new paths with only `go.mod` and `publication.json` changed, since nothing else names the module path, and served from a `file://` proxy. External consumers with Go 1.26.10 and empty caches: importing `prebuilt/darwin/arm64`, `go mod tidy` downloaded one native ZIP and added one requirement; importing `prebuilt`, six and six; importing neither, none. The single-platform consumer opened its window; the one without an import printed the `Ready` error and exited 1; the all-platform consumer cross-built for all six targets.
* A source-build consumer now runs `go mod tidy` cleanly and builds with the cgo tags, without `-mod=mod`. The getting-started page now says so.
* With a `-modfile` requiring only the darwin/arm64 module, the repository's counter, showcase, features and counter-gox `-smoke` runs, headless counter, `examples/uitest` and all 11 `gpui-e2e` scenarios passed, and only that module's ZIP was downloaded. The counter source build with the local archive also passed its smoke.
* darwin/amd64 under Rosetta: counter and headless counter smokes and `examples/uitest` passed. linux/arm64 in `ubuntu:24.04` without a display: headless and plain counter smokes, `examples/uitest` and the 10 display-less scenarios passed.
* `release:smoke` passed for darwin/arm64 against a release directory holding the rewrapped module with matching metadata.

Not run: a hosted release workflow with the new paths, Windows execution, and linux/amd64 on real hardware.

## 2026-11-10: Native amd64 runs of the registration layout

Binaries were built on darwin/arm64 with the registration layout and the rewrapped `v0.0.0` modules (previous entry), then run on an Intel MacBookPro16,1 (i9-9880H, macOS 26.6.2) over SSH with the console session logged in.

* darwin/amd64 native: counter, showcase and features `-smoke` passed; headless counter, showcase and counter-gox passed; `examples/uitest` and all 11 `gpui-e2e` scenarios passed. A consumer without a registration import printed the `Ready` error and exited 1; the single-platform consumer kept its window open.
* linux/amd64 native under Docker Desktop (x86_64 VM, no emulation), `ubuntu:24.04`, no display, documented packages only: all six `-smoke` runs (three headless, three plain), `examples/uitest` and 10 scenarios passed; `clipboard.star` failed as it does without a display. This settles the earlier qemu-user panics as emulation artifacts.
* linux/amd64 with X11: `socat` on the Mac bridged `127.0.0.1:6000` to XQuartz's `/tmp/.X11-unix/X0` (XQuartz runs with `-nolisten tcp`), and the container used `DISPLAY=host.docker.internal:0` with a wildcard-host copy of the session cookie. With only the documented packages, the counter failed with `Failed to create surface: Failed to create surface for any enabled backend`, because GPUI's Linux renderer needs Vulkan. Adding `libvulkan1 mesa-vulkan-drivers` (lavapipe), `xwininfo` showed `"Go + GPUI + Sable"` (560x420) on XQuartz while the counter ran. Counter, showcase, features and counter-gox `-smoke`, `examples/uitest` and all 11 scenarios including clipboard passed. A plain Linux counter also passes without a display, so the smokes alone do not prove a window; the X window listing does. The bridge was stopped and the cookie copy removed afterwards.
* Intermittent darwin/amd64 exception: the Bao-linked windowed counter-gox printed its pass line, then died with SIGILL at `+[NSApplication _crashOnException:]` (resolved with `atos` against another process, since the shared cache address is the same in every process until reboot). The unified log gave `Cannot remove an observer <_NSTouchBarFinderObservation ...> for the key path "nextResponder" from <NSView ...> because it is not registered as an observer`, thrown from `-[_NSTouchBarFinderObservation invalidate]` during `NSDisplayCycleFlush` inside `-[NSApplication run]` called from `gpui_go_run_native`. It failed 5 of 5 with the registration layout and 3 of 3 with the original `v0.0.0` shim, so the layout change did not cause it. A cgo link of the release's `gpui-native-v0.0.0-darwin-amd64.a` passed 3 of 3, and Rosetta on Apple Silicon (no Touch Bar) passed 3 of 3. Under `lldb` it exited 0. With `NSZombieEnabled=YES` it passed with no zombie messages, and afterwards 3 plain runs and 10 interleaved runs of each build all passed. The early contrast was small and not interleaved, so Bao and its TLS lowering are not established as the cause. The exception points at a freed or replaced view in the responder chain observed by AppKit's Touch Bar support; the trigger probably depends on which app is frontmost. Tracked in TODO.

Not run: Windows execution, and a hosted release with the new module paths.

## 2026-11-10: Touch Bar KVO exception mechanism

The darwin/amd64 exception reproduced again while the user was active at the Intel MacBookPro16,1: Bao-linked counter-gox exited 2 on 6 of 6 runs and the cgo link of the release archive exited 0 on 6 of 6, interleaved.

* `malloc_history` under `MallocStackLogging=1`, stopped at `objc_exception_throw`, showed the reported `NSView` was allocated by `-[NSWindow initWithContentRect:...]` from GPUI's `open_window` and never freed. So this is not address reuse or a TLS destructor freeing it.
* Logging KVO `addObserver:`/`removeObserver:` calls whose observer is `_NSTouchBarFinderObservation`: the Bao build registered 19 observations, including `nextResponder` on `GPUIView`, `GPUIWindow`, `GPUIApplication` and `AccessKitSubclassOfNSView`. At removal, the AccessKit view's class had gone from `NSKVONotifying_AccessKitSubclassOfNSView` to plain `NSView`. AccessKit restores the original class on teardown, which drops KVO's subclass, so the removal throws. The cgo build logged no Touch Bar finder observations, so it never exercised this path.
* The executables differ in `LC_BUILD_VERSION`: Go's internal linker writes minos 12.0, sdk 12.0; ld64 wrote minos 26.0, sdk 26.5. AppKit gates behavior on the linked SDK, which could explain why only the Bao build engages the Touch Bar finder. Swapped variants (`vtool -set-build-version`, ad hoc re-signed) were built: the cgo binary with sdk 12.0 and the Bao binary with sdk 26.5. When run, the trigger had lapsed and all four variants passed 4 of 4, so the SDK hypothesis is untested. The `#[ctor]` initializers in `gpui_macos` that register `GPUIApplication`, `GPUIWindow`, `GPUIPanel` and `GPUIView`, which dyld runs at image load but Bao calls from Go `init`, remain a second candidate. The archive has 4 `__mod_init_func` sections, 24 `__thread_vars` sections and no `__objc_*` or `__cfstring` sections.

Result of `sdk-compare.sh 4`, run by the user while the trigger held: `counter-gox` (Bao, sdk 12.0) 2 2 2 2; `bao-sdk26` (Bao, sdk 26.5) 0 0 0 0; `counter-gox-cgo` (ld64, sdk 26.5) 0 0 0 0; `cgo-sdk12` (ld64, sdk 12.0) 2 2 2 2. The crash follows the recorded SDK version, not the linker, so Bao's TLS lowering and constructor timing are cleared. With sdk 12.0, AppKit engages the Touch Bar finder; AccessKit's `SubclassingAdapter::drop` in `accesskit_macos` 0.26.3 then calls `object_setClass(view, prev_class)` unconditionally, replacing KVO's `NSKVONotifying_AccessKitSubclassOfNSView` with `NSView`, and the finder's later `removeObserver:` throws. Go's internal linker hard-codes the build version, so a consumer's `go build` cannot raise it.

## 2026-11-10: AccessKit KVO fix backported

AccessKit had already fixed this crash: issue #823 (identical exception from `-[_NSTouchBarFinderObservation invalidate]`, reported with eframe on a MacBookPro17,1) was closed by PR #835, "fix: Fully drop macOS SubclassingAdapter if KVO is detected" (merge `451885c185`), released in `accesskit_macos` 0.27.2 on 2026-10-25. GPUI at the pinned Zed revision uses the 0.26 series, and 0.27 is semver-incompatible. `rust/vendor/accesskit_macos` is the 0.26.3 crate with `src/subclass.rs` replaced by PR #835's version: the PR's base blob `a873f7c85` equals 0.26.3's file, and the patched file hashes to the PR's result blob `f4ebc196a`. The fix clears the associated object on drop, resets the class only while AccessKit's subclass is the runtime class, makes the subclass methods fall back to the original class, reuses a subclass left behind by an earlier adapter, and panics if the view is already KVO-observed when an adapter is created. A first local variant that only skipped the restore was replaced by this backport. `rust/native/Cargo.toml` applies it through `[patch.crates-io]`; `cargo update -p accesskit_macos` also re-resolved `tempfile`'s `getrandom` edge to 0.3.4, which was restored to 0.4.3 by hand so the lock diff only drops the crate's registry source and checksum, and `cargo metadata --locked` accepts it.

Verification:

* PASS: `cargo fmt --check` on the vendored crate and `rust/native`; `cargo clippy --locked --all-targets -D warnings` on `rust/native` (the vendored crate compiles without warnings but is not a workspace member, so Clippy does not lint it); `cargo test --locked --lib` with and without `automation` (19 and 20 tests).
* `release:archive`, `release:syso` (Bao v0.0.2 built locally with the workflow's flags) and `release:module` for darwin/amd64 produced `gpui-go.dev/__binary/darwin/amd64` with the patched code (its new panic message is present in the archive and binaries, absent from the `v0.0.0` archive). counter-gox built from it through the registration layout records sdk 12.0. A cgo link of the patched archive was relabeled sdk 12.0 with `vtool`.
* On the Intel MacBookPro16,1 with `fix-compare.sh`: in the rounds where the trigger held, unpatched Bao counter-gox exited 2 three times and unpatched SDK-12 cgo once, while the patched Bao and SDK-12 cgo builds exited 0 in all 7 runs each. The trigger faded after the first round of the second batch, so the sample is small.

Not run: the editor workspace, which still resolves the registry crate; a hosted release containing the patch.

## 2026-11-10: v0.0.1 published; tag-only prebuilt selection

`v0.0.1` was built by Prepare native release run 42784160487 from `63f38f0` (the tag was moved there from `2edbf09`; the only difference is `rust/vendor/accesskit_macos/Cargo.toml.orig`) and published. All 14 jobs passed and the six modules carry `gpui-go.dev/__binary/<goos>/<goarch>` at `v0.0.1`. Deploy run 42788702061 deployed the Worker with the user's catalog commit `2ae5c43`, but its asset check ran 65 seconds before publication and failed on a 404; the live Worker serves discovery, list, `.info`, `.mod` and the ZIP redirect correctly. The release is not marked immutable. The user confirmed the build runs on native windows/amd64.

Before publication, an external consumer pointed at a local Worker built from the `v0.0.1` catalog, with gpui-go fetched directly from GitHub (`GOPRIVATE`) so nothing was recorded in the public mirror early: `go mod tidy` downloaded only `gpui-go.dev/__binary/darwin/arm64` with the per-platform import, the window opened, and the darwin/arm64 counter, counter-gox, showcase, features and headless smokes, `examples/uitest` and all 11 scenarios passed. On the Intel MacBookPro16,1 the darwin/amd64 counter, counter-gox and showcase smokes, `uitest` and all 11 scenarios passed; the release counter-gox contains the AccessKit backport and records sdk 12.0.

The user asked that programs never need a platform-dependent import. `go mod tidy` and `go get` read every build constraint as satisfied, so any import of the native modules reachable from `host` makes every consumer download all six, whichever file and tags carry it. The user chose that cost over an import. `host/prebuilt.go` (`gpui && gpui_prebuilt`) now imports `prebuilt`; the shim cannot, since the registration packages import the shim. The per-program `prebuilt.go` files are removed, and `Ready` only reports an unsupported GOOS/GOARCH. The root `go.mod` requires the six modules at `v0.0.1`, resolved through the public Go mirror and checksum database, which raised it to `go 1.26.10` and removed the `go1.26.5` toolchain line; CI's four `setup-go` pins and the install instructions moved to 1.26.10.

Verification with Go 1.26.10: `go build ./...`, `go vet ./...`, `go test -race ./...` and `go mod tidy` (no change after the requirements) pass; `go vet -tags gpui,gpui_prebuilt` with `CGO_ENABLED=0` passes for host, the shim, `prebuilt/...`, the counter and `cmd/gpui-e2e` on all six targets. An external consumer with no prebuilt import, using the public mirror and checksum database and this checkout through `replace`, downloaded six native ZIPs on `go mod tidy`, built with only the tags, opened its window, and cross-built for the other five targets. In the repository with only the tags: counter, counter-gox, showcase, features and headless counter smokes, `examples/uitest` and all 11 scenarios passed. `make harness-check` and the docs build pass. The guide and testing pages drop the import and the "not published" warnings. The version pairing between a gpui-go tag and its native modules is open in TODO.

ZIP size: `.syso` objects are about 97% of every module (darwin/arm64: 59.1 MB raw, 17.7 MB zipped). darwin objects contain 9.8 MB of `__LLVM,__bitcode` from Rust's precompiled std; linux and windows have none, and a Go executable built from the module contains no bitcode, so it costs only the download. On darwin/arm64, `lto = "fat"` with `codegen-units = 1` (set through `CARGO_PROFILE_RELEASE_*` in scratch builds under `target/size-*`) cut `__text` from 14.7 to 12.5 MB and relocations from 12.7 to 10.8 MB, but rustc then embeds bitcode in every crate: 98.4 MB, archive 144 MB, module ZIP 48.4 MB. `xcrun bitcode_strip -r` failed on the archive (`internal link edit command failed`), so bitcode dropping was requested from the Bao agent. Relocations and symbol tables (7 to 16 MB per target) and windows/arm64's per-function sections (48 MB not attributed to sections) are further candidates.

## 2026-11-10: gpui_prebuilt implies gpui

At the user's request, `gpui_prebuilt` alone now selects the prebuilt backend. Constraints that need any native backend use `gpui || gpui_prebuilt` (`host/run_sable.go`, `run_native.go`, `run_headless.go`, `integration_test.go`); the stub uses `!gpui && !gpui_prebuilt`; prebuilt-only files, including the generated table and registration packages, use `gpui_prebuilt`; the cgo files keep `gpui && !gpui_prebuilt`. `-tags gpui,gpui_prebuilt` keeps working. `release:smoke` now builds with the short form, so the hosted smoke exercises it, and the docs use `-tags gpui_prebuilt`.

Verification with Go 1.26.10: default `go build ./...`, `go vet ./...` and `go test -race ./...` pass, and an untagged counter still reports `ErrNativeUnavailable`; `go vet -tags gpui` (cgo) passes for host and the shim; with `CGO_ENABLED=0`, `go vet` passes for `gpui_prebuilt`, `gpui,gpui_prebuilt` and `gpui_prebuilt,gpui_headless` on host, the shim, `prebuilt/...`, the counter and `cmd/gpui-e2e`, and for `gpui_prebuilt,gpui_e2e` including `examples/uitest`; `go test -tags gpui_prebuilt,gpui_headless ./host` passes. Built with only `gpui_prebuilt`: counter and headless counter smokes, `examples/uitest` and all 11 scenarios passed on darwin/arm64.

## 2026-11-11: Worker reads release metadata; Bao v0.0.3

The checked-in catalog is gone (`dist/metadata-proxy/catalog.ts`, `scripts/catalog.ts` and the `Activate native release` workflow, which opened catalog pull requests). The Worker builds its module list from `modulePrefix`, the new `repository` field and `targets` in `dist/native-targets.json`. For any requested version it fetches `https://github.com/<repository>/releases/download/<version>/native-release-<goos>-<goarch>.json` and serves that version only if `validateRelease` accepts it: schema 1, the requested target, module path, repository and sole version, tag equal to the version, the expected asset name, a valid time and a `go.mod` declaring the module. The served `go.mod` and time are the release's recorded bytes, so old versions do not depend on today's template. `@v/list` and `@latest` report only the `NATIVE_VERSION` Worker variable; unlisted published versions still resolve by exact version. Malformed versions and wrong majors are 404 without a fetch, missing metadata is 404, and fetch or validation failures are uncached 502s with no redirect. Validated releases are memoized per isolate; misses and failures are retried. `release:module` and `release:collect` validate with the same function.

Deploy: `Deploy metadata proxy` now also runs on `release: published`, checks out the default branch for that event, picks the version with `release:select` (the dispatch input, or Go's `@latest` over `gh release list --exclude-drafts` tags), which checks every target's metadata and ZIP before anything deploys, and passes it as `wrangler deploy --var NATIVE_VERSION:<version>`. `release:check-deployment ORIGIN VERSION` also checks `@latest`. Nothing is committed. GitHub's `releases/latest` was not used because it returned 404: v0.0.1 is published as a prerelease, which that endpoint skips.

Verification: `npm run check`, and `npm test` on Node 26.4 and 22.18.0 (28 pass, case-collision test skipped on this filesystem). The tests include the verbatim published v0.0.1 darwin/arm64 metadata. Against live GitHub: `release:select` picked v0.0.1 from the release tags and rejected v0.0.0 (draft, metadata 404); `checkDeployment` for v0.0.1 passed for all six modules against a local Worker using the real `fetchAsset`. `wrangler deploy --dry-run --var NATIVE_VERSION:v0.0.1` bundled the JSON import. Not yet observed: a hosted release-triggered deploy and the Cloudflare runtime's redirect-following fetch.

Bao v0.0.3 (`e05f1cfb2d2054457067f3268908992ca9b1df2c`, "Drop embedded LLVM bitcode during translation") is now the release workflow default; its verified profile is still Go 1.26.10. No release has been built with it yet.

## 2026-11-11: Tailwind-style utility styles (tw)

New package `tw` compiles Tailwind CSS v4 style utility lists into `scene.Style` plus a hover refinement (the wire's existing `Element.hover_style`); no protocol or Rust change. At the user's direction there is no runtime CSS parsing and no bundled `theme.css`: `tw/gen.go` (`go run`, `make generate-tw`, part of `make generate`) downloads `tailwindcss@4.3.5` from the npm registry into `.cache/tailwindcss/`, checks SHA-256 `3e30a765...ebb8b`, reads only the supported custom properties (`--color-*`, `--spacing`, `--text-*` minus `--text-shadow-*` and line heights, `--font-weight-*`, `--radius[-*]`, `--container-*`, outer `--shadow[-*]`) and writes `tw/theme_gen.go` and `tw/LICENSE-tailwindcss`; it fails on unrecognized value forms. Colors stay OKLCH in the table and are converted to clipped sRGB in `Default()`; blue-500, red-500, green-500 (clipped), gray-500, slate-900 and neutral-500 match Tailwind's published hex values. Rem lengths stay rems; pixel-only fields convert at 16px.

Semantics: utilities are ordered by rank (shorthand, then axis, then side; `size` before `w`/`h`; `flex` before `grow`/`shrink`) and then class order, matching Tailwind's precedence for those families. `hover:` is the only variant. Anything the protocol or bridge validator cannot carry is a `*tw.Error` (joined across a list): other variants, auto margins (the bridge rejects auto edges), gap-x/y, per-side borders and corners, line height, z-index, and so on. `shadow-none` emits one transparent layer so it also overrides inside a hover refinement. In `ui`, `TWClass`/`BindTWClass` (renamed from `Class` at the user's request, so it is not mistaken for an HTML class), `Hover`, and `Document.UseTheme`; explicit `Style`/`Hover` refine the compiled styles with `proto.Merge`, and explicit shadows replace class shadows. GoX accepts a `twClass` attribute (string or `func() string`).

Verification: `go test -race ./tw ./ui/...` pass. `TestGeneratedThemeIsCurrent` runs only when the tarball is cached; a hand-edited `defaultSpacing` made it fail, and regeneration restored it. A scratch headless program (`.agents-workspace/tmp/twcheck`, `CGO_ENABLED=0 go run -tags gpui_prebuilt,gpui_headless`) opened a window whose seven class lists covered every utility family plus hover refinements; the Rust validator accepted them, and as a negative control a raw auto margin was rejected (`auto edges are not supported`). No rendered pixels or real hover were observed.

## 2026-11-11: Image, Canvas and UniformList in the ui API

`ui.Document` gains `Image`/`BindImage` (a `*client.Resource`; nil records a construction error or fails the binding), `Canvas`/`BindCanvas` (commands copied; unchanged commands publish nothing), and `UniformList(owner, rowHeight, rows...)`, whose children are managed like a `Div`'s and serialize into `UniformList.items`, so `For`/`Show` ranges supply rows. `SetChildren` now accepts uniform lists; row height outside [1, 16384] is a construction error. Embedded views, `Interact`, `BindKey` and `Animate` remain `scene`-only. No protocol or Rust change.

Verification: `go test -race ./ui` covers command copying, repaint-on-change, canvas child rejection, keyed row ID retention across a reorder inside a list, and construction errors. A scratch headless program (`.agents-workspace/tmp/uimedia`, `CGO_ENABLED=0 go run -tags gpui_prebuilt,gpui_headless`) uploaded two PNGs, switched `BindImage` between them, repainted `BindCanvas`, reordered 200 keyed rows in a `UniformList`, and scrolled it to row 150; each publication was accepted, and scrolling to row 200 was rejected (`list item out of range`) as a negative control. Pixels were not observed.

## 2026-11-11: Conditional styles and window metrics (protocol 1.4)

At the user's direction media queries are evaluated natively (option 1) and Go also gets window size, appearance and change notification. Protocol: `ConditionalStyle {optional min_width (inclusive), optional max_width (exclusive), Appearance appearance, Style style, Style hover_style}` as `Element.conditional_styles = 17`; `Appearance` and `WindowMetrics {Size viewport, Appearance}` in types.proto; `WindowOpened.metrics = 4`; `Event.window_metrics = 26`. Core: minor 4, features `conditional-styles` and `window-metrics`; validation (max 32 per element, finite widths in [0, 1e6], min < max, known appearance, at least one condition and one style, nested styles validated); `Backend::metrics` (default None) feeds `WindowOpened.metrics`, else the requested size with light appearance; `Session::window_metrics` emits only changes; a headless resize command now succeeds and applies the requested size (SetTitle stays unsupported). Runtime: `Message::WindowMetrics`; `Hub::publish` drops undelivered metrics events for the same window before the capacity check, never touching delivered-but-unacknowledged ones. Native: `render` compares `window.viewport_size()`/`window.appearance()` (vibrant variants fold to light/dark) with the last value the hub accepted and sends a message only on change; GoView subscribes to `observe_window_appearance` and refreshes, because GPUI only re-renders on bounds changes; matching conditional refinements apply after `style`, conditional hovers after `hover_style`, including in uniform-list rows. No new C export, so `dist/native-bindings.json` and the prebuilt registration table are unchanged.

Go: `client.WindowMetrics` (comparable), `Window.Metrics()` (ok false without the capability), `Window.OnMetricsChanged`; `Dispatch` routes metrics events by window and forgets windows on close; `OpenWindow`/`Set` return `ERROR_CODE_UNSUPPORTED` for snapshots with conditional styles unless `conditional-styles` is advertised (the v0.0.1 prebuilt modules would otherwise drop the field silently). `scene.Node.When`, `scene.ConditionalStyle`, `scene.Appearance*`. `tw`: breakpoint table generated from `--breakpoint-*`; variants `sm:`..`2xl:`, `max-*:`, `min-[..]:`/`max-[..]:`, `dark:`, stackable with `hover:` (stacked widths intersect; empty ranges are errors); one refinement per distinct condition, ordered max-* (descending), then minimums ascending, then dark. `ui`: conditions flow into elements, explicit `Style`/`Hover` fields are cleared from conditional refinements (Edges per side), and `Document.Metrics()` is a tracked read updated from the window.

Verification: core 26 tests (new `tests/metrics.rs`), runtime 7 (a 4096-event resize storm leaves one event per window; retried pulls see unchanged delivered events), native 21 (22 with automation): a GPUI test-platform resize reports metrics and moves a conditional width across a hit test; a negative control that disabled condition matching failed that test. Native Clippy, editor check and editor Clippy pass. Go: build, vet and race tests pass; client tests cover open/dispatch/observer/removal/close and the capability guard; `tw` and `ui` tests cover ordering, intersection and explicit-style clearing. Headless integration (`go test -race -tags gpui,sable_extern_lib,sable_portable,gpui_headless ./...` against a fresh `libgpui_go.a`) passes, including a new `exerciseWindowMetrics` (open metrics, resize event, observer, Rust rejection of an inverted range). Real macOS window (scratch `.agents-workspace/tmp/mediacheck`, `gpui_e2e` automation inspection): opened at a reported 601x301 viewport with `w-20 md:w-40` rendering 80px wide; after `Resize(900, 300)` the metrics event arrived and the box was 160px; after `Resize(700, 300)` it was 80px again. Appearance changes were not exercised natively (the system stayed light; GPUI's test platform keeps appearance simulation crate-private), so `dark:` is covered by core/native matching tests only.

Build pitfall found: `target/release/libgpui_go_native.dylib` (dated 2026-11-07, not created in this session) shadows `libgpui_go_native.a` for `-lgpui_go_native` on macOS, so Makefile builds link the stale dylib; the check above linked the archive by path. The file was left in place for the user to decide.

## 2026-11-11: Window size and color scheme are separate

Supersedes the `WindowMetrics` naming in the conditional styles entry above, at the user's request: a light/dark switch is not a metrics change, and "appearance" is the platform term (macOS `NSAppearance`, GPUI `WindowAppearance`) for a broader concept, so the protocol uses "color scheme", as CSS `prefers-color-scheme`. Protocol: `enum ColorScheme`, `ConditionalStyle.color_scheme`, `WindowOpened.viewport = 4` and `color_scheme = 5`, and separate events `WindowResized { viewport } = 26` and `ColorSchemeChanged { color_scheme } = 27`; `WindowMetrics` is gone. Capabilities `window-size` and `window-color-scheme` replace `window-metrics`. Core: `Backend::window_state` returning `WindowState { viewport, color_scheme }`, `Session::window_resized` and `Session::color_scheme_changed`, each deduplicated independently. Runtime: `Message::WindowResized` and `Message::ColorSchemeChanged`; the queue replaces an undelivered event of the same kind and window. Native: separate last-reported size and scheme. Go: `client.WindowSize`, `Window.Size`/`OnResized`, `Window.ColorScheme`/`OnColorSchemeChanged`; `ui.Document.WindowSize()` and `ColorScheme()` are separate signals; `scene.ColorScheme*`.

Verification: core 26, runtime 7 (a color scheme event never replaces a resize or the reverse), native 21/22 and Clippy, Go vet and race tests (client callbacks are independent per kind), headless integration against a fresh `libgpui_go.a` (a resize runs no color scheme callback). Real macOS window with the archive linked by path: the same 80/160/80px `md:w-40` results, and a binding reading only `Document.ColorScheme()` ran twice (construction, then the reported scheme) and never on the three resizes.

## 2026-11-11: GoX img, canvas and uniform-list tags

The user noted that the reactive Image/Canvas/UniformList nodes had no GoX tags. The printer now accepts `img` (void in GoX's tag catalog; `src` is a `*client.Resource` or accessor, required; default role image), `canvas` (a regular head written self-closing, which GoX compiles to open/submit/close with no children; `commands` is a slice or accessor, optional; children are rejected by `SetChildren`) and `uniform-list` (hyphenated names and camel-case attributes compile with the pinned gox v0.3.3; `rowHeight` as int/float32/float64 in [1, 16384], checked in the printer so `Build` returns the error; children become rows; default role list). `src`, `commands` and `rowHeight` are rejected on other tags. GoX treats an attribute set to nil as unset.

Verification: `go test -race ./ui/gox` covers static and bound img/canvas, an empty canvas, a list fed by `ui.For` with reorder, `testid` on the list and ten invalid-attribute cases. A scratch `.gox` template (`.agents-workspace/tmp/goxtags`, compiled with `bin/gox gen`) using all three tags ran headless against a fresh `libgpui_go.a`: the bridge accepted the window, an image switch, a canvas repaint and a row reorder, and `ScrollTo` on the list's `testid` succeeded.

## 2026-11-11: Native releases build on version tag pushes

The user chose shared tags with a one-release lag over a separate `native-v*` tag line. `Prepare native release` now also runs on `push: tags: ['v*']`: the version is the tag (`github.ref_name`), the source is the tagged commit, all targets build with the pinned Bao v0.0.3 (now a single default in the workflow `env`; the dispatch input is optional and empty by default), and the draft is created on the existing tag (`gh release create --verify-tag`). The plan job runs `release:check-version` (new `scripts/check-version.ts`, reusing the Worker's `isReleaseVersion`; v0/v1 only, since the module paths have no `/vN`) and, when drafting, refuses a version that already has a release, so a stray or repeated tag fails before the native builds; the final check remains for races. Dispatch keeps `version`, `targets`, `bao_revision` and `create_draft` for rehearsals and target subsets. Publishing the draft still deploys the Worker.

Pairing: the commit tagged `vX` keeps requiring native `vX-1`, because `vX` does not exist when it is tagged; requirements are bumped after publication. This closes the pairing TODO as an accepted lag; a binding-table change between releases would break prebuilt consumers of the lagging tag.

Verification: workflow YAML parses; `npm run check` and `npm test` pass (29), including a new test for accepted and rejected release versions. The tag trigger itself has not run on GitHub yet.

## 2026-11-11: Linux release link failed with "hole in findfunctab"

Prepare native release run 42926969905 (dispatch of v0.0.2 from b12df3e) failed in `syso-and-module` on linux/amd64 and linux/arm64: the release smoke's `go build` stopped with 20 `hole in findfunctab` errors. macOS and Windows passed.

Reproduction: an Ubuntu 24.04 arm64 container with Go 1.26.10, Node 22.18.0, Rust 1.99.1 and the job's Linux libraries (scratch `.agents-workspace/tmp/linux-repro`: Dockerfile, `run.sh`, `diagnose.sh`) ran release:syso/module/smoke on the run's `native-library-linux-arm64` artifact and failed the same way. Bao v0.0.2 failed identically on that library, so Bao v0.0.3 was not the cause. A Go linker patched to print `findfunctab` input showed `Textp[0]` is Bao's merged `.text` container at 0x11000 (containers are not function-table entries) and the first function symbol, a generic `apply_style` instance, at offset 0x1260; the holes were exactly the first two 4 KiB buckets. In `merged.o` a local `apply_style<Stateful<Div>>` (4704 bytes) sits at `.text` offset 0, but `objcopy --strip-unneeded` removed it: `ld -r` rewrites calls within the section as section-relative relocations, so nothing references the symbol. Stripped symbols elsewhere are harmless (the previous function's range covers them); only the section start matters, and the code reordering from this round's Rust changes put an unreferenced symbol there.

Fix: `scripts/linux-prelink.ts` names the function symbol at offset 0 of each executable section before stripping (`sectionStartFunctions`), passes it as `--keep-symbol`, and after stripping fails the prelink if a code section still starts without a function symbol (`missingSectionStarts`). The ELF reading is shared with `foldFunctionAliases`. A new fixture test covers both. With the fix, the container ran the whole job on the CI library with Bao v0.0.3 and the release smoke passed (including the Linux native runs); the unfixed prelink on the same library is the negative control. linux/amd64 was not reproduced locally; it fails the same way in CI and the fix is architecture-independent. Container pitfall: Colima's Kubernetes image garbage collection removed the scratch image between runs, and the Go module cache inside outputs is read-only, so clean it with `chmod -R u+w` first.

