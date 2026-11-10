// Package tw compiles Tailwind CSS v4 style utility classes, such as
// "flex items-center gap-2 rounded-md bg-blue-500 hover:bg-blue-600", into
// scene styles. It reproduces Tailwind's default theme scales and naming for
// the properties the GPUI style protocol carries; it is not a CSS engine.
// Classes that would need a property the protocol lacks fail to compile
// rather than being ignored.
package tw

import (
	"maps"
	"math"

	"github.com/moriyoshi/gpui-go/scene"
)

// RootFontSize is the rem size, in logical pixels, that GPUI uses by default.
// Lengths stay in rems where the protocol accepts them; font sizes, border
// widths, corner radii and shadows are pixel values and are converted with it.
const RootFontSize = 16

// Theme holds the scales that utilities resolve names against, mirroring
// Tailwind's theme variables. Keys are the class suffixes: Colors["red-500"]
// serves bg-red-500, FontSizes["sm"] serves text-sm. The empty key is the bare
// utility, as Radii[""] for rounded and Shadows[""] for shadow. New copies the
// theme, so later changes to these maps do not affect a compiler.
type Theme struct {
	// Spacing is one step of the numeric scale, in rems: p-4 is 4 * Spacing.
	Spacing float32
	// Colors serve bg-*, text-* and border-*.
	Colors map[string]*scene.Color
	// FontSizes are rems, for text-*.
	FontSizes map[string]float32
	// FontWeights are in [100, 900], for font-*.
	FontWeights map[string]uint32
	// FontFamilies are single family names, for font-*. The default theme has
	// none, because Tailwind's stacks name fonts GPUI cannot select by name.
	FontFamilies map[string]string
	// Radii are rems, for rounded-*.
	Radii map[string]float32
	// Containers are rems, for w-*, min-w-* and max-w-*.
	Containers map[string]float32
	// Shadows serve shadow-*; offsets, blur and spread are pixels.
	Shadows map[string][]*scene.Shadow
	// Breakpoints are rems, for variants such as md: and max-md:. Media
	// queries compare the window's width, with 16px rems.
	Breakpoints map[string]float32
}

// Default returns a copy of Tailwind CSS v4's default theme, generated from
// the pinned tailwindcss package, for the supported scales. Modify the copy
// and pass it to New to customize it.
func Default() Theme {
	colors := make(map[string]*scene.Color, len(defaultOKLCH)+len(defaultRGB))
	for name, lch := range defaultOKLCH {
		colors[name] = OKLCH(lch[0]/100, lch[1], lch[2])
	}
	for name, rgb := range defaultRGB {
		colors[name] = scene.RGB(rgb)
	}
	shadows := make(map[string][]*scene.Shadow, len(defaultShadows))
	for name, layers := range defaultShadows {
		for _, layer := range layers {
			shadows[name] = append(shadows[name], &scene.Shadow{
				Offset: &scene.Point{X: layer[0], Y: layer[1]}, Blur: layer[2], Spread: layer[3],
				Color: &scene.Color{Alpha: layer[4]},
			})
		}
	}
	return Theme{
		Spacing:      defaultSpacing,
		Colors:       colors,
		FontSizes:    maps.Clone(defaultFontSizes),
		FontWeights:  maps.Clone(defaultFontWeights),
		FontFamilies: map[string]string{},
		Radii:        maps.Clone(defaultRadii),
		Containers:   maps.Clone(defaultContainers),
		Shadows:      shadows,
		Breakpoints:  maps.Clone(defaultBreakpoints),
	}
}

// OKLCH converts an OKLCH color, with lightness in [0, 1] and hue in degrees,
// to opaque sRGB. Colors outside the sRGB gamut are clipped per channel.
func OKLCH(lightness, chroma, hue float64) *scene.Color {
	a := chroma * math.Cos(hue*math.Pi/180)
	b := chroma * math.Sin(hue*math.Pi/180)
	l := math.Pow(lightness+0.3963377774*a+0.2158037573*b, 3)
	m := math.Pow(lightness-0.1055613458*a-0.0638541728*b, 3)
	s := math.Pow(lightness-0.0894841775*a-1.2914855480*b, 3)
	encode := func(linear float64) float32 {
		var value float64
		if linear <= 0.0031308 {
			value = 12.92 * linear
		} else {
			value = 1.055*math.Pow(linear, 1/2.4) - 0.055
		}
		return float32(min(max(value, 0), 1))
	}
	return &scene.Color{
		Red:   encode(4.0767416621*l - 3.3077115913*m + 0.2309699292*s),
		Green: encode(-1.2684380046*l + 2.6097574011*m - 0.3413193965*s),
		Blue:  encode(-0.0041960863*l - 0.7034186147*m + 1.7076147010*s),
		Alpha: 1,
	}
}
