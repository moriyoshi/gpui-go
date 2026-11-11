# Window size and color scheme

A window reports its drawable size and its color scheme, light or dark, to Go. The two are separate: each has its own value and its own change callback. Styles can also depend on them without involving Go at all: [conditional styles](#conditional-styles) are evaluated natively on every frame, so they follow a live resize smoothly.

## Size

```text
func (w *Window) Size() (size client.WindowSize, ok bool)
func (w *Window) OnResized(handler func(context.Context, client.WindowSize) error) (remove func())
```

`client.WindowSize` has `Width` and `Height`: the drawable area in logical pixels. The platform can round it, so a 600 by 300 window may report 601 by 301. `Window.Resize` asks for a new size; a native window reports the size it actually gets, and the headless backend applies the requested size at once.

A live resize can change the size on every frame. Resizes the application has not yet received are replaced by newer ones, so an `OnResized` callback sees the latest size rather than every step.

## Color scheme

```text
func (w *Window) ColorScheme() (scheme scene.ColorScheme, ok bool)
func (w *Window) OnColorSchemeChanged(handler func(context.Context, scene.ColorScheme) error) (remove func())
```

The scheme is `scene.ColorSchemeLight` or `scene.ColorSchemeDark`, the setting CSS calls `prefers-color-scheme`. It follows the system setting, and the platform's vibrant and high-contrast variants report as their base scheme.

```go
window.OnColorSchemeChanged(func(ctx context.Context, scheme scene.ColorScheme) error {
	return applyTheme(scheme == scene.ColorSchemeDark)
})
```

## Delivery and capabilities

`Size` and `ColorScheme` return the values reported when the window opened, updated as the event loop receives changes. Callbacks run on the goroutine that calls `DispatchEvents` or `Dispatch`, after the value is updated, and only for an actual change. Call the returned function to remove a callback. Neither kind of change reaches `OnEvent` handlers.

`ok` is false on a backend without the `window-size` or `window-color-scheme` capability, such as the v0.0.1 prebuilt modules.

In a [`ui` application](./reactive-nodes#window-size-and-color-scheme), read `document.WindowSize()` or `document.ColorScheme()` inside a binding instead.

## Conditional styles

```text
func (n *Node) When(conditions ...*scene.ConditionalStyle) *Node
```

Each `scene.ConditionalStyle` refines the element while its window matches every condition it sets:

| Field | Type | Description |
| --- | --- | --- |
| `MinWidth` | `*float32` | Matches when the viewport is at least this wide, in logical pixels |
| `MaxWidth` | `*float32` | Matches when the viewport is narrower than this. Must be above `MinWidth` when both are set |
| `ColorScheme` | `scene.ColorScheme` | `scene.ColorSchemeDark` or `scene.ColorSchemeLight`; unspecified matches either |
| `Style` | `*scene.Style` | Applied after the element's style |
| `HoverStyle` | `*scene.Style` | Applied after the element's hover style, while hovered |

At least one condition and one style are required, and an element can have up to 32. Matching refinements apply in order, so a later one wins where both set a field. Widths compare like CSS media queries: `MinWidth` is inclusive and `MaxWidth` exclusive.

```go
card := scene.Div(1, title, body).
	Style(&scene.Style{Padding: scene.Padding(8)}).
	When(
		&scene.ConditionalStyle{MinWidth: scene.Ptr[float32](768), Style: &scene.Style{Padding: scene.Padding(24)}},
		&scene.ConditionalStyle{ColorScheme: scene.ColorSchemeDark, Style: &scene.Style{Background: scene.RGB(0x0f172b)}},
	)
```

[Utility styles](/guide/utility-styles#responsive-and-dark-styles) produce these from `md:` and `dark:` utilities.

A backend without the `conditional-styles` capability would ignore the refinements, so `OpenWindow` and `Window.Set` return an `ERROR_CODE_UNSUPPORTED` error before sending such a snapshot.
