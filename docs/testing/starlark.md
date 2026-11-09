# Starlark scenarios

Scenarios can also be Starlark files using the same API with `snake_case` names, so they need no recompiling:

```python
page = open_page()
name = page.get_by_role("textbox", name = "Name", exact = True)
name.fill("Hello 日本語")
expect(name).to_have_value("Hello 日本語")
page.close()
```

The repository's own suite is written this way. To run it from a checkout:

```sh
make e2e-check   # parse and resolve the scenarios, no display needed
make e2e         # build and run the native suite
make uitest      # run the Go-only example
./bin/gpui-e2e --timeout 45s e2e/scenarios/text-input.star
```

`bin/gpui-e2e` only knows the repository's fixtures. To script your own application, call [`uitest/starlark`](#reference) from a [worker](./setup). The [scenario guide](https://github.com/moriyoshi/gpui-go/blob/main/e2e/README.md) describes the maintained scenarios and the lower-level builtins.

## Reference

```text
func New(target uitest.Target, out io.Writer) *Harness
func (h *Harness) RunFile(ctx context.Context, path string) error
func Check(path string) error
```

`Check` parses a script and resolves its names without starting GPUI. Scripts can use top-level `for`, `if` and reassignment. They have no file, shell or module-loading access.

Scripts start with `open_page(fixture = "workbench")`. Names follow the Go API in `snake_case`, with these differences:

| Go | Starlark |
| --- | --- |
| `page.GetByRole("textbox", uitest.RoleOptions{Name: "Name", Exact: true})` | `page.get_by_role("textbox", name = "Name", exact = True)` |
| `page.SetDefaultTimeout(2 * time.Second)` | `page.set_default_timeout(2000)` |
| `locator.First()`, `locator.Last()` | `locator.first`, `locator.last`, without parentheses |
| `locator.WithTimeout(time.Second).Click(ctx)` | `locator.click(timeout_ms = 1000)` |
| `page.Keyboard().InsertText(ctx, text)` | `page.keyboard.insert_text(text)` |

Assertions also accept `timeout_ms`. The script helpers `assert_eq`, `assert_true`, `assert_contains` and `log` are available too.
