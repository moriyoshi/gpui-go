# Tailwind-style utilities

`github.com/moriyoshi/gpui-go/tw` compiles Tailwind CSS v4 style utility lists into `scene.Style` values. Names, scales and the default theme follow Tailwind v4.3.5, for the properties the GPUI style protocol carries. It is not a CSS engine: there is no cascade, no selectors and no stylesheet. Anything the protocol cannot express is a compile error rather than being silently dropped.

```go
card := tw.MustCompile("flex flex-col gap-2 p-4 rounded-lg bg-white shadow-md hover:shadow-lg")
node := card.Apply(scene.Div(1, scene.Text(2, "Hello")))
```

`Compile` returns an error that joins one `*tw.Error` for each bad utility in the list. `MustCompile` panics, for lists fixed at build time. `Classes.Style` returns the style, `Classes.Hover` returns the refinement GPUI applies while the pointer is over the element, or nil when the list has no `hover:` utilities, and `Classes.Conditions` returns the [conditional refinements](../docs/reference/window-state.md#conditional-styles) for breakpoint and `dark:` utilities.

## Variants

| Variant | Applies while |
| --- | --- |
| `hover:` | The pointer is over the element |
| `sm:` ... `2xl:` (theme breakpoints) | The window is at least that wide: 40, 48, 64, 80 and 96 rem at 16px |
| `max-sm:` ... `max-2xl:` | The window is narrower than that breakpoint |
| `min-[600px]:`, `max-[50rem]:` | The window is at least, or narrower than, the given px or rem width |
| `dark:` | The window uses the dark color scheme |

Variants stack in any order, and stacked widths intersect (`md:max-xl:`). Each distinct combination becomes one conditional refinement, at most 32 per list. They are ordered as Tailwind orders its variants: `max-*` refinements first, larger maximums before smaller, then minimums from narrowest to widest, then `dark:`. Where two match, the later one wins. GPUI evaluates them on every frame against the window's viewport and color scheme, with no Go round trip; they need a backend with the `conditional-styles` capability.

With the [persistent node graph](../ui/README.md), use `Node.TWClass` or `Node.BindTWClass`. With [GoX](../ui/gox/README.md), use the `twClass` attribute. In both, an explicit `style` refines the compiled utilities field by field, as an inline style refines utility classes.

```go
button := document.Div(owner, label).TWClass("px-4 py-2 rounded-md bg-blue-500 text-white hover:bg-blue-600")
row := document.Div(owner).BindTWClass(func() string {
    if selected.Get() {
        return "p-2 bg-blue-100"
    }
    return "p-2"
})
```

## Supported utilities

Numbers on the spacing scale are multiples of a quarter step, as `p-4` (1rem) or `gap-1.5`. `px` is one pixel. Square brackets take an arbitrary value, with `_` for a space: `w-[13px]`, `p-[1.5rem]`, `bg-[#1e90ff]`, `opacity-[0.35]`.

| Area | Utilities |
| --- | --- |
| Padding, margin | `p`, `px`, `py`, `pt`, `pr`, `pb`, `pl`, `ps`, `pe` and the same with `m`; negative margins such as `-mt-2` |
| Gap | `gap-*` (one gap for both axes) |
| Sizing | `w`, `h`, `size`, `min-w`, `min-h`, `max-w`, `max-h` with the spacing scale, fractions (`w-1/2`), `full`, `auto`; `w`, `min-w` and `max-w` also take container sizes (`max-w-md`) |
| Position | `relative`, `absolute`, `inset`, `inset-x`, `inset-y`, `top`, `right`, `bottom`, `left`, `start`, `end`, with negatives |
| Layout | `flex`, `grid`, `hidden`, `flex-row`, `flex-col`, `flex-wrap`, `flex-nowrap`, `flex-<n>`, `flex-auto`, `flex-initial`, `flex-none`, `grow`, `shrink`, `items-{start,center,end,stretch}`, `justify-{start,center,end,between}`, `grid-cols-<n>` |
| Color | `bg-*`, `text-*`, `border-*` with theme colors, `transparent`, `[#hex]`, and an opacity modifier (`bg-blue-500/50`, `/[0.25]`) |
| Text | `text-{xs,...,9xl}`, `text-[18px]`, `font-{thin,...,black}`, `font-[550]`, `font-[Family Name]`, `italic`, `not-italic`, `text-{left,center,right,start,end}` |
| Borders, radius | `border`, `border-<n>`, `border-[2px]`, `rounded`, `rounded-{xs,...,4xl}`, `rounded-none`, `rounded-full`, `rounded-[10px]` |
| Effects | `shadow`, `shadow-{2xs,...,2xl}`, `shadow-none`, `opacity-<percent>` |
| Interaction | `overflow-{visible,hidden,clip,scroll,auto}` and the `-x`/`-y` forms, `cursor-{pointer,default,auto}` |

As in Tailwind, axis and side utilities refine their shorthand whatever the order (`px-2 p-4` keeps the horizontal padding at `0.5rem`), and `w-*` and `h-*` refine `size-*`. Otherwise a later utility overrides an earlier one for the same property.

Lengths stay in rems where the protocol carries lengths. Font sizes, border widths, corner radii and shadows are pixel numbers, converted at `RootFontSize` (16), GPUI's default rem size. Colors are Tailwind's OKLCH palette converted to sRGB, clipped where a color lies outside sRGB, as `green-500` does.

## Not supported

Variants other than those above are errors: `focus:`, `active:`, `disabled:`, `group-*`, `peer-*`, container queries such as `@md:`, `print:` and `motion-*:`. The protocol has no focus or press state for a style to follow. So are display values other than flex, grid and hidden, `static`/`fixed`/`sticky`, `auto` margins, separate `gap-x`/`gap-y`, per-side borders and per-corner radii, line height, letter spacing, z-index, transforms, transitions, rings, outlines, shadow colors and arbitrary shadows, the `!` important modifier and CSS variable shorthands such as `bg-(--brand)`. Adding any of them needs a protocol field and Rust support first.

The default theme defines no font families: Tailwind's stacks name fonts that GPUI cannot select by name. Add families to the theme, or use `font-[Family Name]`.

## Themes

`tw.Default()` returns a copy of the default theme. Change it and pass it to `tw.New`, which validates and copies it:

```go
theme := tw.Default()
theme.Colors["brand"] = tw.OKLCH(0.62, 0.19, 260)
theme.FontFamilies["display"] = "Inter"
theme.Spacing = 0.2 // rems per step
compiler, err := tw.New(theme)
if err != nil { return err }
document.UseTheme(compiler) // before building nodes
```

A compiler is safe for concurrent use and caches compiled lists.

## Regenerating the default theme

`theme_gen.go` and `LICENSE-tailwindcss` are generated from the `theme.css` of the pinned `tailwindcss` npm package by [gen.go](gen.go). The generator downloads the tarball from the npm registry into `.cache/tailwindcss/`, checks its SHA-256, and fails on any value it does not recognize. To upgrade, update `version` and `digest` in `gen.go`, run `make generate-tw`, and review the diff. `TestGeneratedThemeIsCurrent` compares the checked-in files with a fresh generation when the tarball is cached, and skips otherwise. Nothing from the package is linked except the generated tables.
