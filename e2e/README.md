# Starlark E2E harness

The harness executes `.star` scenarios against a real Go application, Sable transport and native GPUI renderer. It consumes the public [uitest package](../uitest/README.md) for input, application driving, event barriers, lifecycle cleanup, suite deadlines and worker reports. Starlark evaluation, builtins, source diagnostics and value conversion live in [uitest/starlark](../uitest/starlark/README.md); the old `e2e` Go package forwards its public API. [uitest/gpui](../uitest/gpui/README.md) owns native driving and `Application`/`Factory`. Fixtures and presentation live in [examples/e2eapp](../examples/e2eapp/) and [examples/reactiveapp](../examples/reactiveapp/); runner factories adapt their application types without a harness dependency in those applications.

## Commands

Run from the repository root with the normal pinned toolchains and native graphical prerequisites:

```sh
make e2e-check       # parse AND resolve all scenarios; no native library/display
make build-e2e      # opt-in instrumented native archive and bin/gpui-e2e
make e2e            # build and run all passing scenarios
make e2e-no-display # Linux without a display; all but clipboard, which that platform lacks
./bin/gpui-e2e --timeout 45s e2e/scenarios/text-input.star
make e2e E2E_SCENARIOS='e2e/scenarios/controls.star e2e/scenarios/composition.star'
make test-e2e-native # GPUI unit tests with the instrumentation compiled
make uitest         # Go-only consumer of the same shared package
```

The Rust `automation` feature and Go `gpui_e2e` tag are independent opt-ins. Locally built native archives do not advertise `test-automation-v1` and reject automation requests unless built through `e2e-lib`. Published prebuilt modules enable the feature, so they only need the Go tag. `build-e2e` preserves its archive as `target/release/libgpui_go_e2e.a` and forces a fresh Go link. Ordinary native targets rebuild without instrumentation. `--check` also works with `go run ./cmd/gpui-e2e`, without native tags.

The parent starts one native worker for the suite. Each file gets a deadline (30 seconds by default), an interpreter step budget and cleanup with an independent five-second deadline. Windows, handlers, models and clipboard text are cleaned up on success, assertion failure or cancellation. Shared `uitest` code supervises the worker and validates its atomic version-1 completion report before reporting success. This matters on macOS because AppKit may terminate the native process with status zero without returning from GPUI's application loop. Missing reports, malformed reports, assertions and crashes fail the suite. Reports remain under `.agents-workspace/tmp/e2e-run-*/result.json`.

## Writing scenarios

Use the shared Page/Locator API for new scenarios:

```python
page = open_page()
name = page.get_by_role("textbox", name = "Name", exact = True)
name.fill("Hello 日本語")
expect(name).to_have_value("Hello 日本語")
expect(name).to_be_focused()
page.get_by_role("button", name = "Increment native model").click()
expect(page.get_by_test_id("count")).to_have_text("Count: 1")
page.close()
```

`page.locator(name)` and `get_by_test_id(name)` use fixture selector names, while `get_by_role` and `get_by_text` query current native snapshot metadata. Locators are live and strict: an ambiguous action fails; `.nth(index)`, `.first` and `.last` select explicitly. Actions include `.click()`, `.fill(value)`, `.press(key)`, `.focus()`, `.hover()`, `.check()` and `.uncheck()`. Getters include `.count()`, `.input_value()` and `.inspect()`. `page.keyboard.press` accepts GPUI or common Playwright key names; `insert_text` uses the focused native input handler.

`expect(locator)` supports `to_have_text`, `to_have_value`, `to_have_count`, `to_be_visible`, `to_be_hidden`, `to_be_enabled`, `to_be_disabled`, `to_be_focused`, `to_be_checked` and `not_to_be_checked`. Actions and expectations accept `timeout_ms`; `page.set_default_timeout(ms)` changes the five-second default. The shared Go core owns readiness checks, virtual-row reveal, assertion polling and timeout diagnostics. The Starlark adapter only unpacks arguments and returns values. See [shared API semantics and limits](../uitest/README.md#pages-locators-and-expectations).

`page.state()` and `page.update(operation, value="")` expose application fixture state/updates. The earlier free-function API below remains available for low-level input and composition scenarios.

`open_page("reactive")` opens the [reactive component example](../examples/reactiveapp/app.go). Its selectors include `counter`, `name`, `greeting`, `details`, and keyed `item-1`, `item-2`, etc. The fixture exposes counter/batch/toggle/reverse/add/remove operations and binding execution counts. `to_have_text` includes descendant text on containers; text queries match leaves, and input values remain separate.

Top-level `for`/`if` and reassignment are supported. Check and execution share their language options and builtin registry. Scripts are trusted repository test code; arbitrary shell/file APIs and Starlark module loading are not provided.

```python
app = open_app("workbench")
click(app, "name")
type_text(app, "Hello 日本語")
assert_eq(state(app)["name"], "Hello 日本語")
key(app, "home shift-end")
assert_eq(inspect(app, "name")["selection"], [0, 9])
close_app(app)
```

| Builtin | Behavior |
| --- | --- |
| `open_app(fixture="workbench")` | Open an independent application fixture and return its token |
| `close_app(app)` | Release that application's window, handlers, model and subscription |
| `click(app, element, x=None, y=None)` | Dispatch native pointer move/down/up; omitted coordinates use the clipped rendered center |
| `mouse_down`, `mouse_up`, `mouse_move` | Dispatch individual pointer phases; `pressed=True` marks a drag move; explicit coordinates are window-relative |
| `key(app, keystrokes)` | Dispatch space-separated native key strokes with key-up; chords use the real keymap |
| `type_text(app, text)` | Insert a text payload through the currently focused platform input handler |
| `compose(app, element, text, commit=False)` | Simulate native text-service composition/commit callbacks on the focused field |
| `inspect(app, element="root")` | Read native revision, focus, clipped bounds, rendered status, text, and input state |
| `state(app)` | Read the Go application's state after event dispatch settles |
| `update(app, operation, value="")` | Invoke an explicit fixture operation, such as authoritative replacement or an unrelated snapshot commit |
| `clipboard(text=None)` | Read/write real clipboard text, reserving its previous text for cleanup |
| `assert_eq`, `assert_true`, `assert_contains`, `log` | Assertions with source backtraces and progress output |

`inspect` reports UTF-16 selection and marked ranges; `marked` is `None` after commit. Input `text_bounds` is the actual text-layout rectangle, useful for mouse selection without assuming container padding. `rendered=False` identifies an offscreen virtual row; clicking a target without rendered bounds is an error. `state` is a Go-side observation; assert native input state or rendered text as well when a scenario needs to prove that Go and native entities agree.

The native target uses inspect barriers and published event watermarks to drain callbacks and their resulting view commits. It does not use arbitrary sleeps to guess when updates are visible. A maximum of 64 barriers catches applications that never settle.

The workbench exposes `root`, `increment`, `count`, `name`, `mirror`, `reset`, `disabled`, `other`, `list`, `selected` and `row-0` through `row-999`. Its updates are `replace` (new input value), `rerender`, `disable` (`"false"` re-enables), and `scroll_last`. Other applications can implement their own factory and selectors without adding presentation or application state to the harness.

## Scenarios and evidence limits

| Scenario | Observes |
| --- | --- |
| [controls](scenarios/controls.star) | Native clicks/misses, keyboard activation, model observation, shortcut chords surviving commits |
| [locators](scenarios/locators.star) | Shared Page/Locator/expect API, role/text/test-ID queries, fill/press, checkbox idempotence, current snapshots and automatic virtual-row reveal |
| [reactive](scenarios/reactive.star) | One template build, selective/batched bindings, memo equality, branch disposal, keyed input identity/focus/selection through reordering |
| [GoX counter](scenarios/counter-gox.star) | Generated counter view, native click/Enter activation, rendered/Go counts and stable count-node identity |
| [text input](scenarios/text-input.star) | Decomposed accents/emoji/Japanese, grapheme deletion, UTF-16 selection, Go epochs, mixed-control Tab navigation |
| [composition](scenarios/composition.star) | Mark replacement, commit, Go echoes and authoritative resets through native composition callbacks |
| [disabled input](scenarios/disabled-input.star) | Disabled edits/click focus, skipped Tab stops and re-enabling |
| [mouse selection](scenarios/mouse-selection.star) | Pointer drag selection through Unicode and selection deletion |
| [clipboard](scenarios/clipboard.star) | Copy/cut/paste shortcuts, clipboard text and single-line normalization |
| [virtual list](scenarios/virtual-list.star) | Actual last-row prepaint after scrolling, native row click and retained scroll state |
| [window isolation](scenarios/window-isolation.star) | Independent inputs/models, closing one window, and reopening after the last close |

The clipboard scenario changes the actual pasteboard and restores its previous text. It does not preserve rich clipboard formats. Use an explicit scenario list excluding `clipboard.star` if those formats must be retained.

These are synthetic GPUI inputs in a native graphical process. They exercise actual renderer/input/Go transport behavior, but do not inject OS events, drive an installed OS IME, validate screen-reader output or compare pixels. Composition is explicitly simulated. Native validation currently targets macOS/arm64; Linux/Windows runtime results remain unverified. The separate GPL Zed editor host retains its own smoke tests and is not linked by this runner.

The deliberately failing [assertion control](testdata/assertion-fails.star) is excluded from the passing suite. Verify failure reporting, cleanup and process status with:

```sh
./bin/gpui-e2e e2e/testdata/assertion-fails.star e2e/scenarios/controls.star
# Expected: assertion traceback, controls PASS, final exit status 1.
```

The excluded [strictness control](testdata/strictness-fails.star) similarly rejects an ambiguous textbox fill before mutating a field. Follow it with `e2e/scenarios/locators.star` to verify cleanup and continuation; the final status must be 1.

## Cornus provenance

The structure follows [Cornus's Starlark harness](https://github.com/moriyoshi/cornus/tree/6997fb55d23f209b5afec6afc74b1f3b14ebe7f4/pkg/e2e): scenario files, Go builtins, a replaceable target, matching check/run language options, per-file cleanup and explicit assertions. The inspected local Cornus revision was `6997fb55d23f209b5afec6afc74b1f3b14ebe7f4`. This implementation is specific to GPUI and does not import Cornus or its infrastructure dependencies. Starlark is pinned independently in `go.mod`.
