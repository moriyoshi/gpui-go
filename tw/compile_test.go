package tw

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

func compile(t *testing.T, classes string) *Classes {
	t.Helper()
	compiled, err := Compile(classes)
	if err != nil {
		t.Fatalf("Compile(%q): %v", classes, err)
	}
	return compiled
}

func same(t *testing.T, label string, got, want proto.Message) {
	t.Helper()
	if !proto.Equal(got, want) {
		t.Errorf("%s:\n got %v\nwant %v", label, got, want)
	}
}

func hex(c *scene.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", int(c.Red*255+0.5), int(c.Green*255+0.5), int(c.Blue*255+0.5))
}

// Tailwind v4 publishes these sRGB values for its OKLCH palette; green-500 is
// outside sRGB and checks the clipping.
func TestDefaultPaletteMatchesTailwindSRGB(t *testing.T) {
	theme := Default()
	for name, want := range map[string]string{
		"blue-500": "#2b7fff", "red-500": "#fb2c36", "green-500": "#00c950", "gray-500": "#6a7282",
		"slate-900": "#0f172b", "neutral-500": "#737373", "white": "#ffffff", "black": "#000000",
	} {
		if got := hex(theme.Colors[name]); got != want {
			t.Errorf("%s = %s, want %s", name, got, want)
		}
	}
	// 26 families of 11 shades, plus black and white.
	if len(theme.Colors) != 26*11+2 {
		t.Errorf("default theme has %d colors", len(theme.Colors))
	}
}

func TestSpacingAndSizing(t *testing.T) {
	same(t, "p-4 px-2", compile(t, "p-4 px-2").Style(), &scene.Style{Padding: &scene.Edges{
		Top: scene.Rems(1), Right: scene.Rems(0.5), Bottom: scene.Rems(1), Left: scene.Rems(0.5)}})
	// Like Tailwind, the axis utility refines the shorthand regardless of order.
	same(t, "px-2 p-4", compile(t, "px-2 p-4").Style(), compile(t, "p-4 px-2").Style())
	same(t, "pt-px pb-0.5 pl-[13px]", compile(t, "pt-px pb-0.5 pl-[13px]").Style(), &scene.Style{Padding: &scene.Edges{
		Top: scene.Pixels(1), Bottom: scene.Rems(0.125), Left: scene.Pixels(13)}})
	same(t, "-mt-2 mx-0", compile(t, "-mt-2 mx-0").Style(), &scene.Style{Margin: &scene.Edges{
		Top: scene.Rems(-0.5), Right: scene.Pixels(0), Left: scene.Pixels(0)}})
	// size-* sets two properties, so like Tailwind it sorts before w-* and h-*.
	same(t, "sizes", compile(t, "w-1/2 h-full min-w-0 max-w-md size-8 w-auto").Style(), &scene.Style{
		Width: scene.Auto(), Height: scene.Percent(100), MinWidth: scene.Pixels(0), MaxWidth: scene.Rems(28)})
	same(t, "size", compile(t, "size-8 w-1/2").Style(), &scene.Style{Width: scene.Percent(50), Height: scene.Rems(2)})
	same(t, "gap", compile(t, "gap-3").Style(), &scene.Style{Gap: scene.Rems(0.75)})
	same(t, "inset", compile(t, "absolute inset-0 -top-1 left-1/4").Style(), &scene.Style{
		Position: scene.Ptr(scene.PositionAbsolute),
		Inset:    &scene.Edges{Top: scene.Rems(-0.25), Right: scene.Pixels(0), Bottom: scene.Pixels(0), Left: scene.Percent(25)}})
}

func TestLayoutTypographyAndEffects(t *testing.T) {
	same(t, "flex", compile(t, "flex flex-col items-center justify-between flex-wrap flex-1 grow-0").Style(), &scene.Style{
		Display: scene.Ptr(scene.DisplayFlex), FlexDirection: scene.Ptr(scene.FlexColumn),
		AlignItems: scene.Ptr(scene.AlignCenter), JustifyContent: scene.Ptr(scene.JustifySpaceBetween),
		FlexWrap: proto.Bool(true), FlexGrow: proto.Float32(0), FlexShrink: proto.Float32(1)})
	same(t, "grid", compile(t, "grid grid-cols-3 hidden").Style(), &scene.Style{
		Display: scene.Ptr(scene.DisplayNone), GridColumns: proto.Uint32(3)})
	same(t, "text", compile(t, "text-sm font-semibold italic text-center text-white").Style(), &scene.Style{
		FontSize: proto.Float32(14), FontWeight: proto.Uint32(600), Italic: proto.Bool(true),
		TextAlign: scene.Ptr(scene.TextAlignCenter), TextColor: &scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}})
	same(t, "arbitrary text", compile(t, "text-[1.5rem] font-[550]").Style(), &scene.Style{
		FontSize: proto.Float32(24), FontWeight: proto.Uint32(550)})
	same(t, "border", compile(t, "border-2 border-[#ff0000] rounded-lg opacity-75").Style(), &scene.Style{
		BorderWidth: proto.Float32(2), BorderColor: &scene.Color{Red: 1, Alpha: 1},
		CornerRadius: proto.Float32(8), Opacity: proto.Float32(0.75)})
	same(t, "bare", compile(t, "border rounded").Style(), &scene.Style{BorderWidth: proto.Float32(1), CornerRadius: proto.Float32(4)})
	same(t, "overflow and cursor", compile(t, "overflow-hidden overflow-y-auto cursor-pointer").Style(), &scene.Style{
		OverflowX: scene.Ptr(scene.OverflowHidden), OverflowY: scene.Ptr(scene.OverflowScroll), Cursor: scene.Ptr(scene.CursorPointer)})
	md := compile(t, "shadow-md").Style().Shadows
	if len(md) != 2 || md[0].Offset.Y != 4 || md[0].Blur != 6 || md[0].Spread != -1 || md[0].Color.Alpha != 0.1 {
		t.Errorf("shadow-md = %v", md)
	}
	if none := compile(t, "shadow-md shadow-none").Style().Shadows; len(none) != 1 || none[0].Color.Alpha != 0 {
		t.Errorf("shadow-none = %v", none)
	}
}

func TestColorsAndOpacityModifiers(t *testing.T) {
	half := compile(t, "bg-blue-500/50").Style().Background
	if hex(half) != "#2b7fff" || half.Alpha != 0.5 {
		t.Errorf("bg-blue-500/50 = %s alpha %v", hex(half), half.Alpha)
	}
	quarter := compile(t, "bg-[#00000080]/[0.5]").Style().Background
	if math.Abs(float64(quarter.Alpha)-128.0/255/2) > 1e-6 {
		t.Errorf("bracketed alpha = %v", quarter.Alpha)
	}
	same(t, "transparent", compile(t, "bg-transparent").Style(), &scene.Style{Background: &scene.Color{}})
	same(t, "short hex", compile(t, "text-[#f80]").Style().TextColor, &scene.Color{Red: 1, Green: 0x88 / 255.0, Alpha: 1})
}

func TestHoverIsAPartialRefinement(t *testing.T) {
	compiled := compile(t, "bg-blue-500 p-2 hover:bg-blue-600 hover:shadow-none")
	if compiled.Style().Shadows != nil || compiled.Style().Padding == nil {
		t.Errorf("base = %v", compiled.Style())
	}
	hover := compiled.Hover()
	if hover.Padding != nil || hex(hover.Background) != hex(Default().Colors["blue-600"]) || len(hover.Shadows) != 1 {
		t.Errorf("hover = %v", hover)
	}
	if compile(t, "p-2").Hover() != nil {
		t.Error("a list without hover: has a hover refinement")
	}
	node := compiled.Apply(scene.Div(1)).Proto()
	same(t, "applied hover", node.HoverStyle, hover)
}

func TestRejectsWhatTheProtocolCannotExpress(t *testing.T) {
	for class, reason := range map[string]string{
		"focus:bg-red-500": "unsupported variant",
		"hover:hover:p-1":  "repeated variant",
		"print:p-4":        "unsupported variant",
		"max-nope:p-4":     "unsupported variant",
		"md:max-sm:p-1":    "never matches",
		"min-[50%]:p-1":    "breakpoint",
		"mx-auto":          "auto margins",
		"block":            "display",
		"gap-x-2":          "one gap",
		"border-t-2":       "one border",
		"rounded-tl-lg":    "one corner radius",
		"leading-tight":    "not expressible",
		"w-screen":         "not supported",
		"p-1.3":            "not a spacing value",
		"-p-2":             "cannot be negative",
		"w-3/2":            "fraction",
		"bg-nope-500":      "not a theme color",
		"bg-blue-500/150":  "opacity modifier",
		"opacity-120":      "percentage",
		"font-[1000]":      "[100, 900]",
		"text-[50%]":       "not a percentage",
		"shadow-red-500":   "not a theme shadow",
		"p-4!":             "important",
		"bogus":            "unknown utility",
		"grid-cols-0":      "column count",
	} {
		_, err := Compile(class)
		var classErr *Error
		if !errors.As(err, &classErr) || classErr.Class != class || !strings.Contains(err.Error(), reason) {
			t.Errorf("Compile(%q) = %v, want an error mentioning %q", class, err, reason)
		}
	}
	// Every bad class is reported, and good ones do not mask them.
	_, err := Compile("p-4 bogus flex mx-auto")
	if err == nil || !strings.Contains(err.Error(), `"bogus"`) || !strings.Contains(err.Error(), `"mx-auto"`) {
		t.Errorf("joined error = %v", err)
	}
}

func TestCustomTheme(t *testing.T) {
	theme := Default()
	theme.Spacing = 0.5
	theme.Colors["brand"] = OKLCH(0.6, 0.2, 300)
	theme.FontFamilies["display"] = "Inter"
	compiler, err := New(theme)
	if err != nil {
		t.Fatal(err)
	}
	// The compiler holds a copy.
	delete(theme.Colors, "brand")
	compiled, err := compiler.Compile("p-2 bg-brand font-display")
	if err != nil {
		t.Fatal(err)
	}
	style := compiled.Style()
	if style.Padding.Top.GetRems() != 1 || style.Background == nil || style.GetFontFamily() != "Inter" {
		t.Errorf("custom theme style = %v", style)
	}
	if _, err := Compile("bg-brand"); err == nil {
		t.Error("the default compiler sees a custom color")
	}
	for name, mutate := range map[string]func(*Theme){
		"spacing": func(t *Theme) { t.Spacing = 0 },
		"color":   func(t *Theme) { t.Colors["bad"] = &scene.Color{Red: 2, Alpha: 1} },
		"weight":  func(t *Theme) { t.FontWeights["heavy"] = 1000 },
		"shadow":  func(t *Theme) { t.Shadows["flat"] = nil },
	} {
		bad := Default()
		mutate(&bad)
		if _, err := New(bad); err == nil {
			t.Errorf("New accepted an invalid %s", name)
		}
	}
}

func TestCompiledListsAreIndependentCopies(t *testing.T) {
	first := compile(t, "p-4 bg-red-500")
	first.Style().Padding.Top = scene.Pixels(99)
	if compile(t, "p-4 bg-red-500").Style().Padding.Top.GetRems() != 1 {
		t.Error("mutating a returned style changed the cache")
	}
}

func TestBreakpointAndDarkVariantsBecomeOrderedConditions(t *testing.T) {
	compiled := compile(t, "p-2 lg:p-8 md:p-4 max-md:hover:bg-red-500 dark:bg-slate-900 dark:md:hover:opacity-50 min-[600px]:max-[900px]:gap-1")
	same(t, "base", compiled.Style(), &scene.Style{Padding: compile(t, "p-2").Style().Padding})
	if compiled.Hover() != nil {
		t.Errorf("base hover = %v", compiled.Hover())
	}
	conditions := compiled.Conditions()
	type summary struct {
		min, max  float32
		dark      bool
		style     bool
		hoverOnly bool
	}
	var got []summary
	for _, c := range conditions {
		got = append(got, summary{c.GetMinWidth(), c.GetMaxWidth(), c.ColorScheme == scene.ColorSchemeDark, c.Style != nil, c.Style == nil && c.HoverStyle != nil})
	}
	// Tailwind's order: max-* first, then minimums ascending, then dark:.
	want := []summary{
		{0, 768, false, false, true},   // max-md:hover:
		{600, 900, false, true, false}, // min-[600px]:max-[900px]:
		{768, 0, false, true, false},   // md:
		{1024, 0, false, true, false},  // lg:
		{0, 0, true, true, false},      // dark:
		{768, 0, true, false, true},    // dark:md:hover:
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("conditions:\n got %v\nwant %v", got, want)
	}
	if conditions[2].Style.Padding.Top.GetRems() != 1 || conditions[3].Style.Padding.Top.GetRems() != 2 {
		t.Errorf("breakpoint padding = %v, %v", conditions[2].Style.Padding, conditions[3].Style.Padding)
	}
	// Stacked minimums intersect.
	if got := compile(t, "sm:lg:p-1").Conditions()[0].GetMinWidth(); got != 1024 {
		t.Errorf("sm:lg: min = %v", got)
	}
	node := compiled.Apply(scene.Div(1)).Proto()
	if len(node.ConditionalStyles) != len(conditions) {
		t.Errorf("applied %d conditions", len(node.ConditionalStyles))
	}
	// A list without variants has no conditions.
	if compile(t, "p-2 hover:p-3").Conditions() != nil {
		t.Error("unconditional list produced conditions")
	}
}

func TestCustomBreakpoints(t *testing.T) {
	theme := Default()
	theme.Breakpoints["tablet"] = 50
	compiler, err := New(theme)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.Compile("tablet:p-1 max-tablet:p-2")
	if err != nil {
		t.Fatal(err)
	}
	if c := compiled.Conditions(); len(c) != 2 || c[0].GetMaxWidth() != 800 || c[1].GetMinWidth() != 800 {
		t.Errorf("custom breakpoint conditions = %v", c)
	}
}
