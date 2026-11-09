---
outline: [2, 3]
---

# API reference

See [Set up your application](./setup) for how these fit together. Import paths:

| Package | Contains | Build requirement |
| --- | --- | --- |
| `github.com/moriyoshi/gpui-go/uitest` | Pages, locators, assertions, scenario and worker running | None |
| `github.com/moriyoshi/gpui-go/uitest/gpui`, conventionally `gpuitest` | `Application`, `Factory`, `NativeTarget` | `NativeTarget` needs the `gpui_e2e` tag |
| `github.com/moriyoshi/gpui-go/uitest/starlark` | [Starlark scenarios](./starlark) | None |

Use a page and its locators only on the goroutine running the scenario.

## Page

```text
func OpenPage(ctx context.Context, target Target, fixture string) (*Page, error)
func NewPage(target Target, app string) *Page
```

`OpenPage` asks the target to open an application. `fixture` is passed to your `Factory`, so one test program can open different screens.

| Method | Description |
| --- | --- |
| `GetByRole(role, options...)` | Locates by role, optionally narrowed by accessible name |
| `GetByText(text, options...)` | Locates text elements |
| `GetByTestID(name)`, `Locator(name)` | Locates by a name from `Application.Element`. The two are the same |
| `Keyboard()` | Returns the [keyboard](#keyboard) |
| `State(ctx)` | Returns `Application.State` after events settle |
| `Update(ctx, operation, value)` | Calls `Application.Update`, then waits for events to settle |
| `SetDefaultTimeout(d)` | Sets the action and assertion timeout. The default is five seconds, and it must be positive |
| `ID()` | Returns the target's application token |
| `Close(ctx)` | Closes the application |

## Finding elements

`RoleOptions` has `Name string` and `Exact bool`. `TextOptions` has `Exact bool`.

Matching is case-sensitive and collapses runs of whitespace. Without `Exact`, a substring matches.

| Role | Matches |
| --- | --- |
| `button`, `checkbox`, `link`, `slider`, `dialog`, `listitem` | Elements given that role with `Accessible` |
| `textbox` | Text inputs, or elements given `RoleTextInput` |
| `text` | Text elements, or elements given `RoleText` |
| `img` | Images, or elements given `RoleImage` |
| `list` | Uniform lists, or elements given `RoleList` |
| `generic` | Other containers, or elements given `RoleGeneric` |

Any other role name is an error.

An element's name is its accessibility label. Text elements and unlabelled buttons, checkboxes and links fall back to their text. Other elements have an empty name without a label.

`GetByText` matches text elements only, so a container holding the text does not also match. It searches rows that a uniform list has not rendered, and it never matches the value of a text input.

## Locator

A locator is a query, not a captured element. It runs again for every operation, so it always sees the current view.

| Selection | Description |
| --- | --- |
| `Nth(index)` | Selects a match by zero-based index. `-1` selects the last, and other negative values match nothing |
| `First()`, `Last()` | The same as `Nth(0)` and `Nth(-1)` |
| `WithTimeout(d)` | Overrides the timeout for this locator |

### Actions

Each action waits for exactly one match with rendered, nonzero bounds. A row that a uniform list has not rendered is scrolled into view first. Several matches fail immediately instead of waiting.

| Action | Requires enabled | Description |
| --- | --- | --- |
| `Click(ctx)` | Yes | Moves, presses and releases at the center of the visible bounds |
| `Hover(ctx)` | No | Moves the pointer over the element |
| `Focus(ctx)` | Yes | Focuses the element from Go, without a click |
| `Fill(ctx, value)` | Yes | Focuses a text input, selects everything and inserts `value`. Fails on other elements |
| `Press(ctx, key)` | Yes | Focuses the element, then presses `key` |
| `Check(ctx)`, `Uncheck(ctx)` | Yes | Clicks only if the state differs, then asserts it. Needs `Checked` accessibility metadata |
| `SetChecked(ctx, want)` | Yes | `Check` or `Uncheck`, from a bool |

Only text inputs can be disabled. Every other element counts as enabled.

### Reading state

| Method | Waits for | Returns |
| --- | --- | --- |
| `Count(ctx)` | Nothing | The current number of matches |
| `InputValue(ctx)` | One match | The text input's value |
| `Inspect(ctx)` | One match | A map of native state, described below |

| Key | Type | Description |
| --- | --- | --- |
| `revision` | `uint64` | Revision of the current snapshot |
| `element_id` | `uint64` | The element's ID |
| `focused_element_id` | `uint64` | The focused element in the window, or 0 |
| `rendered` | `bool` | Has nonzero bounds in the last frame |
| `enabled` | `bool` | False for a disabled text input |
| `role` | `string` | Protobuf role name |
| `name` | `string` | Accessible name |
| `text` | `string` | The element's text, including descendants and excluding input values |
| `bounds` | `map` | `x`, `y`, `width` and `height`, clipped to what is visible. Absent until laid out |
| `checked` | `bool` | Present with `Checked` metadata |

Text inputs add these keys:

| Key | Type | Description |
| --- | --- | --- |
| `value` | `string` | Current native value |
| `selection` | `[]any` | Start and end, in UTF-16 code units |
| `marked` | `[]any` or nil | Composition range, in UTF-16 code units |
| `text_bounds` | `map` | The laid-out text's rectangle |
| `epoch`, `edit_revision` | `uint64` | The field's [synchronization counters](/reference/text-input#inputstate) |

## Assertions

```text
func Expect(locator *Locator) *Assertions
```

Each assertion retries until it passes or the timeout expires.

| Assertion | Passes when |
| --- | --- |
| `ToHaveText(ctx, want)` | The element's text, including descendants, equals `want` after collapsing whitespace |
| `ToHaveValue(ctx, want)` | A text input's value is exactly `want` |
| `ToHaveCount(ctx, want)` | The locator has `want` matches |
| `ToBeVisible(ctx)` | One match has rendered bounds |
| `ToBeHidden(ctx)` | No match, or one without rendered bounds |
| `ToBeEnabled(ctx)`, `ToBeDisabled(ctx)` | Enabled state matches. `Enabled(ctx, want)` takes a bool |
| `ToBeFocused(ctx)` | The element holds focus |
| `ToBeChecked(ctx)`, `NotToBeChecked(ctx)` | Checked state matches. `Checked(ctx, want)` takes a bool |

`ToHaveValue` and the checked assertions fail at once on an element of the wrong kind. Visibility means nonzero bounds only. Opacity, and being covered by another element, are not considered.

## Keyboard

```text
func (k *Keyboard) Press(ctx context.Context, key string) error
func (k *Keyboard) InsertText(ctx context.Context, text string) error
```

`Press` takes up to 64 space-separated keystrokes and sends key-down and key-up for each, to whatever has focus. Each stroke can use GPUI names, such as `cmd-k cmd-s`, or Playwright names:

| Playwright name | GPUI name |
| --- | --- |
| `ControlOrMeta` | `cmd` on macOS, `ctrl` elsewhere |
| `Control`, `Meta`, `Shift`, `Alt` | `ctrl`, `cmd`, `shift`, `alt` |
| `ArrowLeft`, `ArrowRight`, `ArrowUp`, `ArrowDown` | `left`, `right`, `up`, `down` |
| `Enter`, `Backspace`, `Delete`, `Home`, `End`, `Tab`, `Escape`, `Space` | The lowercase name |

Join modifiers with `+`, as in `Shift+ArrowRight`.

`InsertText` delivers its whole text as one input to the focused field, so combining sequences stay intact. It fails if no text input has focus.

## Lower-level input

These functions take a `Target` and an application token, which is `Page.ID()` for a page. Elements are names from `Application.Element`.

| Function | Description |
| --- | --- |
| `Click(ctx, target, app, element)` | Clicks the center of the element's visible bounds |
| `Pointer(ctx, target, app, element, pointer)` | Sends one `*pb.AutomationPointer` phase, optionally at window-relative coordinates |
| `Key(ctx, target, app, keystrokes)` | Sends GPUI-format keystrokes with no name translation |
| `TypeText(ctx, target, app, text)` | The same as `Keyboard.InsertText` |
| `Compose(ctx, target, app, element, text, commit)` | Simulates native composition on the focused field. With `commit` false, the text stays marked |

`pb` is `github.com/moriyoshi/gpui-go/gen/gpui/v1`. `Compose` calls the same callbacks a text service would, but no real IME is involved.

## Targets

```text
type Target interface {
	Open(context.Context, string) (string, error)
	Close(context.Context, string) error
	Input(context.Context, string, string, *pb.AutomationRequest) error
	Inspect(context.Context, string, string) (map[string]any, error)
	State(context.Context, string) (map[string]any, error)
	Update(context.Context, string, string, string) error
	Clipboard(context.Context, *string) (string, error)
	Cleanup(context.Context) error
}
```

`gpuitest.NewNativeTarget(client, factory)` returns the real implementation, which also provides the optional `QueryTarget`, `FocusTarget` and `RevealTarget` extensions. A target without `QueryTarget` supports only name-based locators. It must not be used from more than one goroutine.

`Clipboard` returns the current clipboard text, and first replaces it when given a non-nil string. `Cleanup` restores the text that was there before the first call.

### Application

```text
type Application interface {
	Window() *client.Window
	Element(string) (uint64, bool)
	State() map[string]any
	Update(context.Context, string, string) error
	Close(context.Context) error
}

type Factory func(context.Context, *client.Client, string) (Application, error)
```

`Element("root")` must return a stable, nonzero ID.

## Running scenarios

```text
func RunScenario(ctx context.Context, target Target, run func(context.Context) error) error
func RunSuite(ctx context.Context, paths []string, timeout time.Duration, out io.Writer, run func(context.Context, string) error) error
```

`RunScenario` runs the callback and always calls `target.Cleanup`, with an independent deadline of `uitest.CleanupTimeout`, five seconds. A canceled context fails the scenario even if the callback returned nil.

`RunSuite` calls `run` once per entry in `paths`, each with its own `timeout`. It prints `PASS` or `FAIL` lines to `out`, continues past failures and returns all of them. The entries are just names, so they need not be files. Use `RunScenario` inside the callback:

```go
scenarios := map[string]func(context.Context) error{
	"counter": func(ctx context.Context) error { return checkCounter(ctx, target) },
}
err := uitest.RunSuite(ctx, []string{"counter"}, 30*time.Second, os.Stdout, func(ctx context.Context, name string) error {
	return uitest.RunScenario(ctx, target, scenarios[name])
})
```

## Worker processes

```text
func RunWorker(ctx context.Context, options WorkerOptions) (string, error)
func WriteWorkerReport(path string, scenarioErr error) error
func ReadWorkerReport(path string) error
```

`RunWorker` starts the worker, enforces its deadline, and returns the report path even on failure.

| `WorkerOptions` field | Description |
| --- | --- |
| `Executable` | Program to run. Required |
| `Args` | Receives the absolute report path and returns the arguments. Required |
| `Timeout` | Deadline for the whole worker. Required, and must be positive |
| `ArtifactsDir` | Where each run's `e2e-run-*/result.json` is kept. Defaults to the system temporary directory |
| `Stdout`, `Stderr` | Output destinations |
| `Env` | Worker environment. Nil inherits the parent's |

With `RunSuite`, set `Timeout` to cover every scenario and its cleanup. The worker calls `WriteWorkerReport` once, after cleanup and before native shutdown.

`ReadWorkerReport` returns an error for a missing, malformed or incomplete report, and returns the recorded failure if there was one.
