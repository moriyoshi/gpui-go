# Utility styles

The `tw` package compiles Tailwind CSS v4 style utility lists into [styles](/reference/style), with Tailwind's names, scales and default theme. Use it in place of hand-built `scene.Style` values:

```go
button := document.Div(owner, document.Text(owner, "Save")).
	TWClass("px-4 py-2 rounded-md bg-blue-500 text-white font-medium hover:bg-blue-600")
```

In a [GoX template](./gox), use the `twClass` attribute:

```text
<button twClass="px-4 py-2 rounded-md bg-blue-500 text-white hover:bg-blue-600" onClick=(save)>Save</button>
```

`hover:` utilities become a hover refinement that GPUI applies by itself, without a Go event.

## Changing utilities with signals

`BindTWClass` recompiles when a signal it reads changes. In GoX, pass a `func() string`:

```go
row := document.Div(owner, label).BindTWClass(func() string {
	if selected.Get() {
		return "p-2 rounded bg-blue-100"
	}
	return "p-2 rounded"
})
```

## Responsive and dark styles

Breakpoint and `dark:` variants work as in Tailwind. GPUI evaluates them on every frame against the window, so they follow a live resize and a system color scheme change without a round trip through Go:

```go
panel := document.Div(owner, content).
	TWClass("p-2 md:p-6 bg-white dark:bg-slate-900 text-slate-900 dark:text-white dark:hover:bg-slate-800")
```

| Variant | Applies while |
| --- | --- |
| `sm:`, `md:`, `lg:`, `xl:`, `2xl:` | The window is at least 640, 768, 1024, 1280 or 1536 pixels wide |
| `max-sm:`, `max-md:`, ... | The window is narrower than that breakpoint |
| `min-[600px]:`, `max-[50rem]:` | The window is at least, or narrower than, the given width |
| `dark:` | The window uses the dark color scheme |

Variants stack, as in `md:max-xl:dark:hover:bg-slate-700`. As in Tailwind, `max-*` refinements come first, then wider breakpoints override narrower ones, and `dark:` overrides both. Widths are the window's drawable width, the same viewport a CSS media query uses.

To change structure rather than style, read [`document.WindowSize()` or `document.ColorScheme()`](/reference/reactive-nodes#window-size-and-color-scheme) in a binding. Both need a backend with the window state and conditional style capabilities; the published v0.0.1 prebuilt modules have neither, and `OpenWindow` then returns an error rather than ignoring the variants.

## Utilities and inline styles

`Style` and the GoX `style` attribute refine the utilities field by field, as an inline style refines classes. `Hover` refines the `hover:` utilities in the same way. An explicit padding top changes only the top padding that `p-4` set. Explicit styles also win over breakpoint and `dark:` utilities for the fields they set, as an inline style wins over a media query.

## Errors

Unknown utilities, and utilities that need a property GPUI's style protocol does not have, are errors rather than being ignored. `TWClass` records them in `document.Err()`, which prevents mounting. `BindTWClass` fails the computation. Each error names the utility and why it failed:

```text
tw: class "focus:ring-2": unsupported variant "focus"; the variants are hover:, dark:, theme breakpoints such as md: and max-md:, and min-[...]: and max-[...]:
tw: class "mx-auto": auto margins are not supported by the GPUI bridge; use justify-* or items-* on the parent
```

The main gaps are variants other than `hover:`, `dark:` and breakpoints (no `focus:`, `active:` or `group-*`), display values other than flex, grid and hidden, per-side borders and per-corner radii, separate `gap-x`/`gap-y`, line height, letter spacing, z-index, transforms and transitions. See the [package guide](https://github.com/moriyoshi/gpui-go/blob/main/tw/README.md) for the full list of supported utilities.

## Themes

Customize a copy of the default theme, then give the compiler to the document before building nodes:

```go
theme := tw.Default()
theme.Colors["brand"] = tw.OKLCH(0.62, 0.19, 260)
theme.FontFamilies["display"] = "Inter"
compiler, err := tw.New(theme)
if err != nil {
	return err
}
document.UseTheme(compiler)
```

The default theme has no font families, because Tailwind's font stacks name fonts that GPUI cannot select by name. Add your own, or use `font-[Family Name]`.

Outside the reactive API, `tw.MustCompile(list).Apply(node)` sets a `scene` node's style and hover refinement.
