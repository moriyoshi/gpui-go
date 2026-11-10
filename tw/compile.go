package tw

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

// Error reports one class that could not be compiled.
type Error struct {
	Class  string
	Reason string
}

func (e *Error) Error() string { return fmt.Sprintf("tw: class %q: %s", e.Class, e.Reason) }

// Classes is a compiled class list: a style and the partial refinement GPUI
// applies while the element is hovered. Both accessors return copies.
type Classes struct {
	base, hover *scene.Style
	conditions  []*scene.ConditionalStyle
}

func (c *Classes) Style() *scene.Style { return proto.Clone(c.base).(*scene.Style) }

// Hover returns nil when the list has no hover: classes.
func (c *Classes) Hover() *scene.Style {
	if c.hover == nil {
		return nil
	}
	return proto.Clone(c.hover).(*scene.Style)
}

// Conditions returns the refinements for breakpoint and dark: utilities, in
// the order GPUI applies them, or nil when the list has none.
func (c *Classes) Conditions() []*scene.ConditionalStyle {
	var conditions []*scene.ConditionalStyle
	for _, condition := range c.conditions {
		conditions = append(conditions, proto.Clone(condition).(*scene.ConditionalStyle))
	}
	return conditions
}

// Apply sets a snapshot builder's style, hover refinement and conditional
// refinements.
func (c *Classes) Apply(node *scene.Node) *scene.Node {
	return node.Style(c.base).Hover(c.hover).When(c.conditions...)
}

// Compiler compiles class lists against one theme. It is safe for concurrent
// use and caches compiled lists.
type Compiler struct {
	theme Theme
	mu    sync.Mutex
	cache map[string]*Classes
}

const cacheLimit = 1024

// New copies the theme into a compiler, rejecting values the GPUI style
// protocol cannot carry.
func New(theme Theme) (*Compiler, error) {
	if !(theme.Spacing > 0) || theme.Spacing > 1e4 {
		return nil, errors.New("tw: theme spacing must be a positive number of rems")
	}
	for name, color := range theme.Colors {
		if color == nil || !unit(color.Red) || !unit(color.Green) || !unit(color.Blue) || !unit(color.Alpha) {
			return nil, fmt.Errorf("tw: theme color %q must have channels in [0, 1]", name)
		}
	}
	for name, weight := range theme.FontWeights {
		if weight < 100 || weight > 900 {
			return nil, fmt.Errorf("tw: theme font weight %q must be in [100, 900]", name)
		}
	}
	for name, family := range theme.FontFamilies {
		if family == "" || len(family) > 256 {
			return nil, fmt.Errorf("tw: theme font family %q must be a nonempty name", name)
		}
	}
	for label, scale := range map[string]map[string]float32{"font size": theme.FontSizes, "radius": theme.Radii, "container": theme.Containers, "breakpoint": theme.Breakpoints} {
		for name, value := range scale {
			if !(value >= 0) || value > 1e4 {
				return nil, fmt.Errorf("tw: theme %s %q must be a nonnegative number of rems", label, name)
			}
		}
	}
	shadows := make(map[string][]*scene.Shadow, len(theme.Shadows))
	for name, layers := range theme.Shadows {
		if len(layers) == 0 || len(layers) > 8 {
			return nil, fmt.Errorf("tw: theme shadow %q needs one to eight layers", name)
		}
		for _, layer := range layers {
			if layer == nil || layer.Color == nil || layer.Offset == nil || !(layer.Blur >= 0) {
				return nil, fmt.Errorf("tw: theme shadow %q has an invalid layer", name)
			}
			shadows[name] = append(shadows[name], proto.Clone(layer).(*scene.Shadow))
		}
	}
	colors := make(map[string]*scene.Color, len(theme.Colors))
	for name, color := range theme.Colors {
		colors[name] = proto.Clone(color).(*scene.Color)
	}
	theme.Colors, theme.Shadows = colors, shadows
	theme.FontSizes, theme.FontWeights = maps.Clone(theme.FontSizes), maps.Clone(theme.FontWeights)
	theme.FontFamilies, theme.Radii, theme.Containers = maps.Clone(theme.FontFamilies), maps.Clone(theme.Radii), maps.Clone(theme.Containers)
	theme.Breakpoints = maps.Clone(theme.Breakpoints)
	return &Compiler{theme: theme, cache: map[string]*Classes{}}, nil
}

var defaultCompiler = sync.OnceValue(func() *Compiler {
	compiler, err := New(Default())
	if err != nil {
		panic(err)
	}
	return compiler
})

// Compile compiles a class list against the default theme.
func Compile(classes string) (*Classes, error) { return defaultCompiler().Compile(classes) }

// MustCompile is Compile for class lists known at build time; it panics on error.
func MustCompile(classes string) *Classes {
	compiled, err := Compile(classes)
	if err != nil {
		panic(err)
	}
	return compiled
}

// An op sets some style fields. Ops apply by rank, then in class order, so
// that like Tailwind, px-2 refines p-4 whichever comes first.
type op struct {
	rank  int
	apply func(*scene.Style)
}

// Compile compiles a whitespace-separated class list. Later classes override
// earlier ones for the same property. hover: is the only variant; other
// variants, unknown classes and classes the protocol cannot express are
// errors, joined so every bad class is reported.
func (c *Compiler) Compile(classes string) (*Classes, error) {
	c.mu.Lock()
	cached := c.cache[classes]
	c.mu.Unlock()
	if cached != nil {
		return cached, nil
	}
	type group struct{ base, hover []op }
	groups := map[condition]*group{}
	var errs []error
	for _, class := range strings.Fields(classes) {
		when, isHover, o, err := c.class(class)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		g := groups[when]
		if g == nil {
			g = &group{}
			groups[when] = g
		}
		if isHover {
			g.hover = append(g.hover, o)
		} else {
			g.base = append(g.base, o)
		}
	}
	if len(groups) > maxConditions+1 {
		errs = append(errs, fmt.Errorf("tw: more than %d distinct breakpoint and dark: combinations", maxConditions))
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}
	compiled := &Classes{base: &scene.Style{}}
	if g := groups[condition{}]; g != nil {
		compiled.base = build(g.base)
		if len(g.hover) != 0 {
			compiled.hover = build(g.hover)
		}
	}
	whens := slices.SortedFunc(maps.Keys(groups), compareConditions)
	for _, when := range whens {
		if when == (condition{}) {
			continue
		}
		g := groups[when]
		conditional := when.proto()
		if len(g.base) != 0 {
			conditional.Style = build(g.base)
		}
		if len(g.hover) != 0 {
			conditional.HoverStyle = build(g.hover)
		}
		compiled.conditions = append(compiled.conditions, conditional)
	}
	c.mu.Lock()
	if len(c.cache) >= cacheLimit {
		clear(c.cache)
	}
	c.cache[classes] = compiled
	c.mu.Unlock()
	return compiled, nil
}

func build(ops []op) *scene.Style {
	slices.SortStableFunc(ops, func(a, b op) int { return a.rank - b.rank })
	style := &scene.Style{}
	for _, o := range ops {
		o.apply(style)
	}
	return style
}

// splitVariants splits on colons outside brackets and parentheses.
func splitVariants(class string) []string {
	var parts []string
	depth, start := 0, 0
	for i, r := range class {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ':':
			if depth == 0 {
				parts = append(parts, class[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, class[start:])
}

func (c *Compiler) class(class string) (when condition, hover bool, o op, err error) {
	fail := func(format string, args ...any) (condition, bool, op, error) {
		return condition{}, false, op{}, &Error{Class: class, Reason: fmt.Sprintf(format, args...)}
	}
	parts := splitVariants(class)
	utility := parts[len(parts)-1]
	seen := map[string]bool{}
	for _, variant := range parts[:len(parts)-1] {
		if seen[variant] {
			return fail("repeated variant %q", variant)
		}
		seen[variant] = true
		if variant == "hover" {
			hover = true
			continue
		}
		if reason := c.variant(variant, &when); reason != "" {
			return fail("%s", reason)
		}
	}
	if when.hasMin && when.hasMax && when.min >= when.max {
		return fail("the width range never matches")
	}
	if strings.HasPrefix(utility, "!") || strings.HasSuffix(utility, "!") {
		return fail("the important modifier has no meaning without a cascade")
	}
	negative := strings.HasPrefix(utility, "-")
	if negative {
		utility = utility[1:]
	}
	if apply, ok := statics[utility]; ok && !negative {
		return when, hover, op{apply: apply}, nil
	}
	if reason := unsupported(utility); reason != "" {
		return fail("%s", reason)
	}
	// The longest root wins, as px-4 over p; values never split at a dash
	// inside an arbitrary value.
	end := strings.IndexAny(utility, "[(")
	if end < 0 {
		end = len(utility)
	}
	for i := end; i >= 0; i-- {
		if i != len(utility) && utility[i] != '-' {
			continue
		}
		root := utility[:i]
		value := ""
		if i < len(utility) {
			value = utility[i+1:]
		}
		handler, ok := functional[root]
		if !ok {
			continue
		}
		if negative && !handler.negative {
			return fail("%s utilities cannot be negative", root)
		}
		apply, rank, reason := handler.parse(c, root, value, negative)
		if reason != "" {
			return fail("%s", reason)
		}
		return when, hover, op{rank: rank, apply: apply}, nil
	}
	return fail("unknown utility")
}

// maxConditions is the bridge's limit on conditional refinements per element.
const maxConditions = 32

// A condition is the window state a variant stack selects. Widths are logical
// pixels, minimum inclusive and maximum exclusive, as CSS width >= and <.
type condition struct {
	min, max       float32
	hasMin, hasMax bool
	dark           bool
}

func (w condition) proto() *scene.ConditionalStyle {
	conditional := &scene.ConditionalStyle{}
	if w.hasMin {
		conditional.MinWidth = proto.Float32(w.min)
	}
	if w.hasMax {
		conditional.MaxWidth = proto.Float32(w.max)
	}
	if w.dark {
		conditional.ColorScheme = scene.ColorSchemeDark
	}
	return conditional
}

// compareConditions orders refinements as Tailwind orders its variants:
// max-* (largest first), then minimums (smallest first), then dark:, so a
// later refinement wins where both apply.
func compareConditions(a, b condition) int {
	key := func(w condition) []float32 {
		dark, low, high := float32(0), float32(-1), float32(math.MaxFloat32)
		if w.dark {
			dark = 1
		}
		if w.hasMin {
			low = w.min
		}
		if w.hasMax {
			high = w.max
		}
		return []float32{dark, low, -high}
	}
	return slices.Compare(key(a), key(b))
}

// variant narrows a condition by one variant: dark, a theme breakpoint such
// as md, max-md, or min-[600px]/max-[40rem].
func (c *Compiler) variant(variant string, when *condition) string {
	if variant == "dark" {
		when.dark = true
		return ""
	}
	bound, maximum := variant, false
	if rest, ok := strings.CutPrefix(variant, "max-"); ok {
		bound, maximum = rest, true
	} else if rest, ok := strings.CutPrefix(variant, "min-"); ok {
		bound = rest
	}
	var width float32
	if rems, ok := c.theme.Breakpoints[bound]; ok {
		width = rems * RootFontSize
	} else if inner, ok := arbitrary(bound); ok {
		var reason string
		if width, reason = pixelLength(inner); reason != "" {
			return "breakpoint: " + reason
		}
	} else {
		return fmt.Sprintf("unsupported variant %q; the variants are hover:, dark:, theme breakpoints such as md: and max-md:, and min-[...]: and max-[...]:", variant)
	}
	// Stacked bounds intersect, as nested media queries do.
	if maximum {
		if !when.hasMax || width < when.max {
			when.max, when.hasMax = width, true
		}
	} else if !when.hasMin || width > when.min {
		when.min, when.hasMin = width, true
	}
	return ""
}

func unsupported(utility string) string {
	switch {
	case slices.Contains([]string{"block", "inline", "inline-block", "inline-flex", "inline-grid", "contents", "table"}, utility):
		return "only flex, grid and hidden display values are supported"
	case slices.Contains([]string{"static", "fixed", "sticky"}, utility):
		return "only relative and absolute positions are supported"
	case strings.HasPrefix(utility, "gap-x-") || strings.HasPrefix(utility, "gap-y-"):
		return "the style protocol has one gap for both axes; use gap-*"
	case slices.ContainsFunc([]string{"border-x", "border-y", "border-t", "border-r", "border-b", "border-l", "border-s", "border-e"}, func(root string) bool {
		return utility == root || strings.HasPrefix(utility, root+"-")
	}):
		return "the style protocol has one border width and color for all sides"
	case slices.ContainsFunc([]string{"rounded-t", "rounded-r", "rounded-b", "rounded-l", "rounded-s", "rounded-e", "rounded-tl", "rounded-tr", "rounded-br", "rounded-bl", "rounded-ss", "rounded-se", "rounded-es", "rounded-ee"}, func(root string) bool {
		return utility == root || strings.HasPrefix(utility, root+"-")
	}):
		return "the style protocol has one corner radius for all corners"
	case slices.ContainsFunc([]string{"leading-", "tracking-", "z-", "line-clamp-", "underline", "line-through", "uppercase", "lowercase", "truncate", "whitespace-", "transition", "duration-", "animate-", "translate-", "rotate-", "scale-", "blur", "ring", "outline", "aspect-", "order-", "basis-", "col-", "row-", "self-", "content-", "place-", "flex-row-reverse", "flex-col-reverse", "items-baseline", "justify-around", "justify-evenly"}, func(prefix string) bool {
		return strings.HasPrefix(utility, prefix)
	}):
		return "not expressible in the GPUI style protocol"
	}
	return ""
}

var statics = map[string]func(*scene.Style){
	"flex":            func(s *scene.Style) { s.Display = scene.Ptr(scene.DisplayFlex) },
	"grid":            func(s *scene.Style) { s.Display = scene.Ptr(scene.DisplayGrid) },
	"hidden":          func(s *scene.Style) { s.Display = scene.Ptr(scene.DisplayNone) },
	"flex-row":        func(s *scene.Style) { s.FlexDirection = scene.Ptr(scene.FlexRow) },
	"flex-col":        func(s *scene.Style) { s.FlexDirection = scene.Ptr(scene.FlexColumn) },
	"flex-wrap":       func(s *scene.Style) { s.FlexWrap = proto.Bool(true) },
	"flex-nowrap":     func(s *scene.Style) { s.FlexWrap = proto.Bool(false) },
	"flex-auto":       func(s *scene.Style) { s.FlexGrow, s.FlexShrink = proto.Float32(1), proto.Float32(1) },
	"flex-initial":    func(s *scene.Style) { s.FlexGrow, s.FlexShrink = proto.Float32(0), proto.Float32(1) },
	"flex-none":       func(s *scene.Style) { s.FlexGrow, s.FlexShrink = proto.Float32(0), proto.Float32(0) },
	"items-start":     func(s *scene.Style) { s.AlignItems = scene.Ptr(scene.AlignStart) },
	"items-center":    func(s *scene.Style) { s.AlignItems = scene.Ptr(scene.AlignCenter) },
	"items-end":       func(s *scene.Style) { s.AlignItems = scene.Ptr(scene.AlignEnd) },
	"items-stretch":   func(s *scene.Style) { s.AlignItems = scene.Ptr(scene.AlignStretch) },
	"justify-start":   func(s *scene.Style) { s.JustifyContent = scene.Ptr(scene.JustifyStart) },
	"justify-center":  func(s *scene.Style) { s.JustifyContent = scene.Ptr(scene.JustifyCenter) },
	"justify-end":     func(s *scene.Style) { s.JustifyContent = scene.Ptr(scene.JustifyEnd) },
	"justify-between": func(s *scene.Style) { s.JustifyContent = scene.Ptr(scene.JustifySpaceBetween) },
	"text-left":       func(s *scene.Style) { s.TextAlign = scene.Ptr(scene.TextAlignLeft) },
	"text-start":      func(s *scene.Style) { s.TextAlign = scene.Ptr(scene.TextAlignLeft) },
	"text-center":     func(s *scene.Style) { s.TextAlign = scene.Ptr(scene.TextAlignCenter) },
	"text-right":      func(s *scene.Style) { s.TextAlign = scene.Ptr(scene.TextAlignRight) },
	"text-end":        func(s *scene.Style) { s.TextAlign = scene.Ptr(scene.TextAlignRight) },
	"italic":          func(s *scene.Style) { s.Italic = proto.Bool(true) },
	"not-italic":      func(s *scene.Style) { s.Italic = proto.Bool(false) },
	"relative":        func(s *scene.Style) { s.Position = scene.Ptr(scene.PositionRelative) },
	"absolute":        func(s *scene.Style) { s.Position = scene.Ptr(scene.PositionAbsolute) },
	"cursor-pointer":  func(s *scene.Style) { s.Cursor = scene.Ptr(scene.CursorPointer) },
	"cursor-default":  func(s *scene.Style) { s.Cursor = scene.Ptr(scene.CursorArrow) },
	"cursor-auto":     func(s *scene.Style) { s.Cursor = scene.Ptr(scene.CursorArrow) },
}

func init() {
	// overflow-auto scrolls; GPUI shows no scrollbar either way. clip has no
	// separate protocol value from hidden.
	for name, value := range map[string]scene.Overflow{
		"visible": scene.OverflowVisible, "hidden": scene.OverflowHidden, "clip": scene.OverflowHidden,
		"scroll": scene.OverflowScroll, "auto": scene.OverflowScroll,
	} {
		statics["overflow-"+name] = func(s *scene.Style) { s.OverflowX, s.OverflowY = scene.Ptr(value), scene.Ptr(value) }
		statics["overflow-x-"+name] = func(s *scene.Style) { s.OverflowX = scene.Ptr(value) }
		statics["overflow-y-"+name] = func(s *scene.Style) { s.OverflowY = scene.Ptr(value) }
	}
}

type handler struct {
	negative bool
	parse    func(c *Compiler, root, value string, negative bool) (apply func(*scene.Style), rank int, reason string)
}

// Edges ranks: shorthand, then axis, then side.
var edgeSides = map[string]struct {
	rank                     int
	top, right, bottom, left bool
}{
	"": {0, true, true, true, true}, "x": {1, false, true, false, true}, "y": {1, true, false, true, false},
	"t": {2, true, false, false, false}, "r": {2, false, true, false, false}, "b": {2, false, false, true, false},
	"l": {2, false, false, false, true}, "s": {2, false, false, false, true}, "e": {2, false, true, false, false},
}

func setEdges(edges **scene.Edges, sides string, length *scene.Length) {
	side := edgeSides[sides]
	if *edges == nil {
		*edges = &scene.Edges{}
	}
	e := *edges
	set := func(on bool, field **scene.Length) {
		if on {
			*field = proto.Clone(length).(*scene.Length)
		}
	}
	set(side.top, &e.Top)
	set(side.right, &e.Right)
	set(side.bottom, &e.Bottom)
	set(side.left, &e.Left)
}

var functional = map[string]handler{}

func init() {
	for sides := range edgeSides {
		functional["p"+sides] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
			length, reason := c.spacing(value)
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) { setEdges(&s.Padding, sides, length) }, edgeSides[sides].rank, ""
		}}
		functional["m"+sides] = handler{negative: true, parse: func(c *Compiler, _, value string, negative bool) (func(*scene.Style), int, string) {
			if value == "auto" {
				return nil, 0, "auto margins are not supported by the GPUI bridge; use justify-* or items-* on the parent"
			}
			length, reason := c.spacing(value)
			if reason == "" && negative {
				length, reason = negate(length)
			}
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) { setEdges(&s.Margin, sides, length) }, edgeSides[sides].rank, ""
		}}
	}
	insets := map[string]string{"inset": "", "inset-x": "x", "inset-y": "y", "top": "t", "right": "r", "bottom": "b", "left": "l", "start": "s", "end": "e"}
	for root, sides := range insets {
		functional[root] = handler{negative: true, parse: func(c *Compiler, _, value string, negative bool) (func(*scene.Style), int, string) {
			length, reason := c.size(value, false)
			if reason == "" && negative {
				length, reason = negate(length)
			}
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) { setEdges(&s.Inset, sides, length) }, edgeSides[sides].rank, ""
		}}
	}
	sizes := map[string]struct {
		rank       int
		containers bool
		fields     func(*scene.Style) []**scene.Length
	}{
		"size":  {0, false, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.Width, &s.Height} }},
		"w":     {1, true, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.Width} }},
		"h":     {1, false, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.Height} }},
		"min-w": {1, true, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.MinWidth} }},
		"min-h": {1, false, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.MinHeight} }},
		"max-w": {1, true, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.MaxWidth} }},
		"max-h": {1, false, func(s *scene.Style) []**scene.Length { return []**scene.Length{&s.MaxHeight} }},
	}
	for root, size := range sizes {
		functional[root] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
			length, reason := c.size(value, size.containers)
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) {
				for _, field := range size.fields(s) {
					*field = proto.Clone(length).(*scene.Length)
				}
			}, size.rank, ""
		}}
	}
	functional["gap"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		length, reason := c.spacing(value)
		if reason == "" && length.GetPercent() != 0 {
			reason = "gap requires pixels or rems"
		}
		if reason != "" {
			return nil, 0, reason
		}
		return func(s *scene.Style) { s.Gap = length }, 0, ""
	}}
	functional["bg"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		color, reason := c.color(value)
		if reason != "" {
			return nil, 0, reason
		}
		return func(s *scene.Style) { s.Background = color }, 0, ""
	}}
	functional["text"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		if rems, ok := c.theme.FontSizes[value]; ok {
			return func(s *scene.Style) { s.FontSize = proto.Float32(rems * RootFontSize) }, 0, ""
		}
		if inner, ok := arbitrary(value); ok && !strings.HasPrefix(inner, "#") {
			pixels, reason := pixelLength(inner)
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) { s.FontSize = proto.Float32(pixels) }, 0, ""
		}
		color, reason := c.color(value)
		if reason != "" {
			return nil, 0, "not a theme font size or color"
		}
		return func(s *scene.Style) { s.TextColor = color }, 0, ""
	}}
	functional["font"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		if weight, ok := c.theme.FontWeights[value]; ok {
			return func(s *scene.Style) { s.FontWeight = proto.Uint32(weight) }, 0, ""
		}
		if family, ok := c.theme.FontFamilies[value]; ok {
			return func(s *scene.Style) { s.FontFamily = proto.String(family) }, 0, ""
		}
		inner, ok := arbitrary(value)
		if !ok {
			return nil, 0, "not a theme font weight or family"
		}
		if weight, err := strconv.ParseUint(inner, 10, 32); err == nil {
			if weight < 100 || weight > 900 {
				return nil, 0, "font weights must be in [100, 900]"
			}
			return func(s *scene.Style) { s.FontWeight = proto.Uint32(uint32(weight)) }, 0, ""
		}
		if len(inner) > 256 {
			return nil, 0, "font family name is too long"
		}
		return func(s *scene.Style) { s.FontFamily = proto.String(inner) }, 0, ""
	}}
	functional["border"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		if value == "" {
			return func(s *scene.Style) { s.BorderWidth = proto.Float32(1) }, 0, ""
		}
		if width, err := strconv.ParseUint(value, 10, 16); err == nil {
			return func(s *scene.Style) { s.BorderWidth = proto.Float32(float32(width)) }, 0, ""
		}
		if inner, ok := arbitrary(value); ok && !strings.HasPrefix(inner, "#") {
			pixels, reason := pixelLength(inner)
			if reason != "" {
				return nil, 0, reason
			}
			return func(s *scene.Style) { s.BorderWidth = proto.Float32(pixels) }, 0, ""
		}
		color, reason := c.color(value)
		if reason != "" {
			return nil, 0, "not a border width or theme color"
		}
		return func(s *scene.Style) { s.BorderColor = color }, 0, ""
	}}
	functional["rounded"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		var pixels float32
		switch rems, ok := c.theme.Radii[value]; {
		case ok:
			pixels = rems * RootFontSize
		case value == "none":
		case value == "full":
			// Tailwind uses an infinite radius; GPUI clamps radii to the box.
			pixels = 9999
		default:
			inner, ok := arbitrary(value)
			if !ok {
				return nil, 0, "not a theme radius"
			}
			var reason string
			if pixels, reason = pixelLength(inner); reason != "" {
				return nil, 0, reason
			}
		}
		return func(s *scene.Style) { s.CornerRadius = proto.Float32(pixels) }, 0, ""
	}}
	functional["shadow"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		var layers []*scene.Shadow
		if value == "none" {
			// An empty list would leave a hover refinement's shadow unchanged.
			layers = []*scene.Shadow{{Color: &scene.Color{}, Offset: &scene.Point{}}}
		} else if theme, ok := c.theme.Shadows[value]; ok {
			layers = theme
		} else {
			return nil, 0, "not a theme shadow; shadow colors and arbitrary shadows are unsupported"
		}
		return func(s *scene.Style) {
			s.Shadows = nil
			for _, layer := range layers {
				s.Shadows = append(s.Shadows, proto.Clone(layer).(*scene.Shadow))
			}
		}, 0, ""
	}}
	functional["opacity"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		opacity, reason := fraction(value)
		if reason != "" {
			return nil, 0, reason
		}
		return func(s *scene.Style) { s.Opacity = proto.Float32(opacity) }, 0, ""
	}}
	for root, field := range map[string]func(*scene.Style) **float32{
		"grow":   func(s *scene.Style) **float32 { return &s.FlexGrow },
		"shrink": func(s *scene.Style) **float32 { return &s.FlexShrink },
	} {
		functional[root] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
			factor := float32(1)
			if value != "" {
				var reason string
				if factor, reason = number(value); reason != "" {
					return nil, 0, reason
				}
			}
			return func(s *scene.Style) { *field(s) = proto.Float32(factor) }, 1, ""
		}}
	}
	functional["flex"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		grow, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return nil, 0, "flex-* takes a whole number, auto, initial or none"
		}
		return func(s *scene.Style) { s.FlexGrow, s.FlexShrink = proto.Float32(float32(grow)), proto.Float32(1) }, 0, ""
	}}
	functional["grid-cols"] = handler{parse: func(c *Compiler, _, value string, _ bool) (func(*scene.Style), int, string) {
		if inner, ok := arbitrary(value); ok {
			value = inner
		}
		columns, err := strconv.ParseUint(value, 10, 32)
		if err != nil || columns == 0 || columns > 1024 {
			return nil, 0, "grid-cols-* takes a column count in [1, 1024]"
		}
		return func(s *scene.Style) { s.GridColumns = proto.Uint32(uint32(columns)) }, 0, ""
	}}
}

// arbitrary unwraps [value], where underscores stand for spaces and \_ for
// an underscore.
func arbitrary(value string) (string, bool) {
	inner, ok := strings.CutPrefix(value, "[")
	if !ok {
		return "", false
	}
	inner, ok = strings.CutSuffix(inner, "]")
	if !ok || inner == "" {
		return "", false
	}
	inner = strings.ReplaceAll(inner, `\_`, "\x00")
	inner = strings.ReplaceAll(inner, "_", " ")
	return strings.ReplaceAll(inner, "\x00", "_"), true
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func unit(value float32) bool   { return value >= 0 && value <= 1 }

// length parses the arbitrary lengths the protocol carries: 0, px, rem and %.
func length(text string) (*scene.Length, string) {
	parse := func(digits string) (float32, bool) {
		value, err := strconv.ParseFloat(digits, 32)
		return float32(value), err == nil && finite(value) && math.Abs(value) <= 1e6
	}
	if text == "0" {
		return scene.Pixels(0), ""
	}
	for _, unit := range []struct {
		suffix string
		make   func(float32) *scene.Length
	}{{"px", scene.Pixels}, {"rem", scene.Rems}, {"%", scene.Percent}} {
		if digits, ok := strings.CutSuffix(text, unit.suffix); ok {
			if value, ok := parse(digits); ok {
				if unit.suffix == "%" && (value < 0 || value > 100) {
					return nil, "percentages must be in [0, 100]"
				}
				return unit.make(value), ""
			}
		}
	}
	return nil, fmt.Sprintf("%q is not a px, rem or %% length", text)
}

// pixelLength is length for fields that are pixel numbers.
func pixelLength(text string) (float32, string) {
	value, reason := length(text)
	switch {
	case reason != "":
		return 0, reason
	case value.GetPercent() != 0 || strings.HasSuffix(text, "%"):
		return 0, "this property takes px or rem, not a percentage"
	case value.GetRems() != 0:
		return check(value.GetRems() * RootFontSize)
	}
	return check(value.GetPixels())
}

func check(pixels float32) (float32, string) {
	if pixels < 0 || pixels > 1e6 {
		return 0, "this property cannot be negative"
	}
	return pixels, ""
}

func number(value string) (float32, string) {
	if inner, ok := arbitrary(value); ok {
		value = inner
	}
	factor, err := strconv.ParseFloat(value, 32)
	if err != nil || !finite(factor) || factor < 0 || factor > 1e6 {
		return 0, "expected a nonnegative number"
	}
	return float32(factor), ""
}

// fraction parses an opacity: a whole percentage, or an arbitrary number in
// [0, 1] or percentage.
func fraction(value string) (float32, string) {
	if inner, ok := arbitrary(value); ok {
		if digits, ok := strings.CutSuffix(inner, "%"); ok {
			inner = digits
		} else if f, err := strconv.ParseFloat(inner, 32); err == nil && f >= 0 && f <= 1 {
			return float32(f), ""
		} else {
			return 0, "expected a number in [0, 1] or a percentage"
		}
		value = inner
	}
	percent, err := strconv.ParseFloat(value, 32)
	if err != nil || !(percent >= 0 && percent <= 100) {
		return 0, "expected a percentage in [0, 100]"
	}
	return float32(percent / 100), ""
}

// spacing resolves px, a multiple of a quarter step, a bracketed length or 0.
func (c *Compiler) spacing(value string) (*scene.Length, string) {
	if value == "px" {
		return scene.Pixels(1), ""
	}
	if inner, ok := arbitrary(value); ok {
		return length(inner)
	}
	steps, err := strconv.ParseFloat(value, 64)
	if err != nil || !finite(steps) || steps < 0 || math.Mod(steps*4, 1) != 0 || steps > 1e4 {
		return nil, fmt.Sprintf("%q is not a spacing value", value)
	}
	if steps == 0 {
		return scene.Pixels(0), ""
	}
	return scene.Rems(float32(steps) * c.theme.Spacing), ""
}

// size extends spacing with full, auto, fractions and, for widths, containers.
func (c *Compiler) size(value string, containers bool) (*scene.Length, string) {
	switch value {
	case "full":
		return scene.Percent(100), ""
	case "auto":
		return scene.Auto(), ""
	case "screen", "svw", "lvw", "dvw", "svh", "lvh", "dvh", "min", "max", "fit", "prose", "none":
		return nil, fmt.Sprintf("%q sizes are not supported", value)
	}
	if containers {
		if rems, ok := c.theme.Containers[value]; ok {
			return scene.Rems(rems), ""
		}
	}
	if numerator, denominator, ok := strings.Cut(value, "/"); ok {
		n, errN := strconv.ParseUint(numerator, 10, 16)
		d, errD := strconv.ParseUint(denominator, 10, 16)
		if errN != nil || errD != nil || d == 0 || n > d {
			return nil, fmt.Sprintf("%q is not a fraction in [0, 1]", value)
		}
		return scene.Percent(float32(n) * 100 / float32(d)), ""
	}
	return c.spacing(value)
}

func negate(value *scene.Length) (*scene.Length, string) {
	switch v := value.GetValue().(type) {
	case *pb.Length_Pixels:
		return scene.Pixels(-v.Pixels), ""
	case *pb.Length_Rems:
		return scene.Rems(-v.Rems), ""
	}
	return nil, "only px and rem lengths can be negative"
}

// color resolves transparent, a theme color or a bracketed hex color, with an
// optional /<percent> or /[<number>] opacity modifier.
func (c *Compiler) color(value string) (*scene.Color, string) {
	name, modifier := value, ""
	depth := 0
	for i, r := range value {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		case '/':
			if depth == 0 {
				name, modifier = value[:i], value[i+1:]
			}
		}
	}
	var color *scene.Color
	if name == "transparent" {
		color = &scene.Color{}
	} else if theme, ok := c.theme.Colors[name]; ok {
		color = proto.Clone(theme).(*scene.Color)
	} else if inner, ok := arbitrary(name); ok {
		var reason string
		if color, reason = hexColor(inner); reason != "" {
			return nil, reason
		}
	} else {
		return nil, fmt.Sprintf("%q is not a theme color", name)
	}
	if modifier != "" {
		alpha, reason := fraction(modifier)
		if reason != "" {
			return nil, "opacity modifier: " + reason
		}
		color.Alpha *= alpha
	}
	return color, ""
}

// hexColor parses #rgb, #rgba, #rrggbb and #rrggbbaa.
func hexColor(text string) (*scene.Color, string) {
	digits, ok := strings.CutPrefix(text, "#")
	if ok && (len(digits) == 3 || len(digits) == 4) {
		var expanded strings.Builder
		for _, r := range digits {
			expanded.WriteString(strings.Repeat(string(r), 2))
		}
		digits = expanded.String()
	}
	if !ok || (len(digits) != 6 && len(digits) != 8) {
		return nil, fmt.Sprintf("%q is not a hex color", text)
	}
	if len(digits) == 6 {
		digits += "ff"
	}
	value, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return nil, fmt.Sprintf("%q is not a hex color", text)
	}
	channel := func(shift uint) float32 { return float32(value>>shift&255) / 255 }
	return &scene.Color{Red: channel(24), Green: channel(16), Blue: channel(8), Alpha: channel(0)}, ""
}
