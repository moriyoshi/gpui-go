package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/moriyoshi/gpui-go/client"
	countergox "github.com/moriyoshi/gpui-go/examples/counter-gox/app"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
)

const (
	indigo = 0x4f46e5
	teal   = 0x0d9488
	amber  = 0xb45309
	rose   = 0xbe123c
	panel  = 0x27272a
	muted  = 0xa1a1aa
)

type resources struct{ raster, vector *client.Resource }

func uploadResources(ctx context.Context, app *client.Client) (*resources, error) {
	picture := image.NewRGBA(image.Rect(0, 0, 240, 160))
	for y := range 160 {
		for x := range 240 {
			wave := uint8(127 + 127*math.Sin(float64(x)/18)*math.Cos(float64(y)/14))
			picture.Set(x, y, color.RGBA{R: uint8(x), G: wave, B: uint8(255 - y), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, picture); err != nil {
		return nil, err
	}
	raster, err := app.UploadResource(ctx, client.ResourceImage, "image/png", encoded.Bytes())
	if err != nil {
		return nil, err
	}
	vector, err := app.UploadResource(ctx, client.ResourceSVG, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="80" height="80"><path d="M14 42l18 18 34-40" stroke="#000" stroke-width="10" fill="none"/></svg>`))
	if err != nil {
		return nil, err
	}
	return &resources{raster: raster, vector: vector}, nil
}

// builder allocates IDs and keeps the samples visually consistent.
type builder struct {
	next   uint64
	images *resources
}

func (b *builder) id() uint64 { b.next++; return b.next }

type tweak func(*scene.Style)

func (b *builder) styled(node *scene.Node, base *scene.Style, tweaks []tweak) *scene.Node {
	for _, apply := range tweaks {
		apply(base)
	}
	return node.Style(base)
}

func (b *builder) page(children ...*scene.Node) *scene.Node {
	return scene.Div(b.id(), children...).Style(&scene.Style{
		Width: scene.Percent(100), Height: scene.Percent(100), Padding: scene.Padding(20), Gap: scene.Pixels(14),
		FlexDirection: scene.FlexColumn.Enum(), Background: scene.RGB(0x18181b), TextColor: scene.RGB(0xf4f4f5),
		FontSize: scene.Ptr[float32](15),
	})
}

func (b *builder) box(text string, background uint32, tweaks ...tweak) *scene.Node {
	return b.styled(scene.Div(b.id(), scene.Text(b.id(), text)), &scene.Style{
		Padding: scene.Padding(10), Background: scene.RGB(background), CornerRadius: scene.Ptr[float32](6),
	}, tweaks)
}

func (b *builder) caption(text string) *scene.Node {
	return scene.Text(b.id(), text).Style(&scene.Style{TextColor: scene.RGB(muted), FontSize: scene.Ptr[float32](13)})
}

func (b *builder) row(children ...*scene.Node) *scene.Node {
	return scene.Div(b.id(), children...).Style(&scene.Style{
		FlexDirection: scene.FlexRow.Enum(), Gap: scene.Pixels(14), FlexWrap: scene.Ptr(true),
	})
}

// demo labels one variation, drawn inside a fixed-size panel.
func (b *builder) demo(label string, width, height float32, tweaks []tweak, children ...*scene.Node) *scene.Node {
	inner := b.styled(scene.Div(b.id(), children...), &scene.Style{
		Width: scene.Pixels(width), Height: scene.Pixels(height), Background: scene.RGB(panel),
		CornerRadius: scene.Ptr[float32](6), Padding: scene.Padding(8), Gap: scene.Pixels(6),
	}, tweaks)
	return scene.Div(b.id(), b.caption(label), inner).Style(&scene.Style{
		FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(6),
	})
}

func (s *session) open(ctx context.Context, width, height float32, root *scene.Node) (*client.Window, error) {
	return s.app.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: width, Height: height}, root)
}

// still is a shot of one snapshot, with an optional step before capture.
func still(name string, width, height float32, view func(*builder) *scene.Node, before ...func(context.Context, *client.Window) error) shot {
	return shot{name, func(ctx context.Context, s *session) error {
		b := &builder{images: s.images}
		window, err := s.open(ctx, width, height, view(b))
		if err != nil {
			return err
		}
		defer window.Close(context.WithoutCancel(ctx))
		for _, step := range before {
			if err := step(ctx, window); err != nil {
				return err
			}
		}
		return s.capture(ctx, name)
	}}
}

func rowFlex(extra ...tweak) []tweak {
	return append([]tweak{func(style *scene.Style) { style.FlexDirection = scene.FlexRow.Enum() }}, extra...)
}

func columnFlex(extra ...tweak) []tweak {
	return append([]tweak{func(style *scene.Style) { style.FlexDirection = scene.FlexColumn.Enum() }}, extra...)
}

func allShots() []shot {
	shots := []shot{
		still("overview", 620, 310, overview),
		still("intro-counter", 400, 190, introCounter),
		still("div-card", 340, 130, divCard),
		still("div-layouts", 640, 190, divLayouts),
		still("div-button", 300, 110, divButton),
		still("text-title", 340, 90, func(b *builder) *scene.Node {
			return b.page(scene.Text(b.id(), "Welcome back").Style(&scene.Style{
				FontSize: scene.Ptr[float32](24), FontWeight: scene.Ptr[uint32](600), TextColor: scene.RGB(0xf4f4f5),
			}))
		}),
		still("text-styles", 460, 260, textStyles),
		still("text-mixed", 460, 90, textMixed),
		still("input-sample", 400, 110, inputSample),
		still("input-states", 400, 260, inputStates),
		still("input-masked", 400, 260, inputMasked, func(ctx context.Context, window *client.Window) error {
			return window.Focus(ctx, 1000)
		}),
		still("input-unicode", 400, 110, inputUnicode, func(ctx context.Context, window *client.Window) error {
			return window.Focus(ctx, 1000)
		}),
		still("image-kinds", 520, 230, imageKinds),
		still("image-boxes", 560, 210, imageBoxes),
		still("canvas-sample", 220, 170, func(b *builder) *scene.Node { return b.page(canvasSample(b)) }),
		still("canvas-rects", 560, 170, canvasRects),
		still("canvas-paths", 620, 190, canvasPaths),
		still("list-top", 340, 240, listView),
		still("list-scrolled", 340, 240, listView, func(ctx context.Context, window *client.Window) error {
			return window.ScrollTo(ctx, 900, 999)
		}),
		still("node-button-states", 420, 120, buttonStates),
		still("node-drag", 460, 130, dragAndDrop),
		still("node-roles", 460, 130, roles),
		still("style-size", 620, 180, styleSize),
		still("style-spacing", 620, 180, styleSpacing),
		still("style-direction", 620, 160, styleDirection),
		still("style-align", 680, 180, styleAlign),
		still("style-justify", 680, 130, styleJustify),
		still("style-wrap", 620, 170, styleWrap),
		still("style-grow", 620, 130, styleGrow),
		still("style-grid", 620, 170, styleGrid),
		still("style-position", 620, 190, stylePosition),
		still("style-overflow", 620, 240, styleOverflow),
		still("style-borders", 680, 150, styleBorders),
		still("style-shadows", 640, 150, styleShadows),
		still("style-opacity", 640, 130, styleOpacity),
		still("style-text", 620, 230, styleText),
		{name: "events-log", run: eventsLog},
		{name: "reactive-text-input", run: reactiveInput},
		{name: "reactive-show", run: reactiveShow},
		{name: "reactive-for", run: reactiveFor},
		{name: "gox-counter", run: goxCounter},
	}
	return append(append(shots, goxShots()...), reactiveGuideShots()...)
}

// introCounter uses the element IDs drawn in the introduction's diagram.
func introCounter(*builder) *scene.Node {
	return scene.Div(1,
		scene.Text(2, "Count: 1"),
		scene.Div(3, scene.Text(4, "Increment")).Style(&scene.Style{
			Padding: scene.Padding(12), Background: scene.RGB(indigo), CornerRadius: scene.Ptr[float32](8),
		}),
	).Style(&scene.Style{
		Width: scene.Percent(100), Height: scene.Percent(100), Padding: scene.Padding(32), Gap: scene.Pixels(20),
		FlexDirection: scene.FlexColumn.Enum(), Background: scene.RGB(0x18181b), TextColor: scene.RGB(0xf4f4f5),
		FontSize: scene.Ptr[float32](20),
	})
}

func overview(b *builder) *scene.Node {
	name := scene.InputState{Value: "Ada Lovelace"}
	rows := make([]*scene.Node, 0, 20)
	for i := range 20 {
		rows = append(rows, scene.Text(b.id(), fmt.Sprintf("Row %d", i)))
	}
	return b.page(
		b.row(
			b.demo("Container", 170, 90, columnFlex(), b.box("Child A", indigo, compact), b.box("Child B", teal, compact)),
			b.demo("Text", 170, 90, nil, scene.Text(b.id(), "Hello, GPUI!").Style(&scene.Style{FontSize: scene.Ptr[float32](20)})),
			b.demo("Text input", 170, 90, nil, scene.TextInput(b.id(), name, scene.TextInputOptions{})),
		),
		b.row(
			b.demo("Image", 170, 90, nil, scene.Image(b.id(), b.images.raster.Handle()).Style(&scene.Style{Width: scene.Pixels(112), Height: scene.Pixels(74)})),
			b.demo("Canvas", 170, 90, nil, scene.Canvas(b.id(), rect(4, 4, 60, 40, teal), stroke(76, 60, 150, 8)).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100)})),
			b.demo("Uniform list", 170, 90, nil, scene.UniformList(b.id(), 24, rows...).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100)})),
		),
	)
}

func divCard(b *builder) *scene.Node {
	return b.page(scene.Div(b.id(),
		scene.Text(b.id(), "Build finished"),
		scene.Text(b.id(), "All 12 steps passed"),
	).Style(&scene.Style{
		FlexDirection: scene.FlexColumn.Enum(),
		Gap:           scene.Pixels(8),
		Padding:       scene.Padding(16),
		Background:    scene.RGB(0x1e293b),
		CornerRadius:  scene.Ptr[float32](8),
	}))
}

func divLayouts(b *builder) *scene.Node {
	three := func() []*scene.Node {
		return []*scene.Node{b.box("A", indigo), b.box("B", teal), b.box("C", amber)}
	}
	return b.page(b.row(
		b.demo("FlexRow", 180, 120, rowFlex(), three()...),
		b.demo("FlexColumn", 180, 120, columnFlex(func(style *scene.Style) { style.Gap = scene.Pixels(2); style.Padding = scene.Padding(4) }),
			b.box("A", indigo, compact), b.box("B", teal, compact), b.box("C", amber, compact)),
		b.demo("DisplayGrid, 2 columns", 180, 120, []tweak{func(style *scene.Style) {
			style.Display = scene.DisplayGrid.Enum()
			style.GridColumns = scene.Ptr[uint32](2)
		}}, b.box("A", indigo), b.box("B", teal), b.box("C", amber)),
	))
}

func compact(style *scene.Style) { style.Padding = scene.Padding(5) }

func saveButton(b *builder, background uint32) *scene.Node {
	return scene.Div(b.id(), scene.Text(b.id(), "Save")).Style(&scene.Style{
		Padding:      scene.Padding(10),
		Background:   scene.RGB(background),
		CornerRadius: scene.Ptr[float32](6),
		Cursor:       scene.CursorPointer.Enum(),
	})
}

func divButton(b *builder) *scene.Node {
	return b.page(b.row(saveButton(b, indigo)))
}

func buttonStates(b *builder) *scene.Node {
	return b.page(b.row(
		b.demo("Base style", 170, 50, rowFlex(), saveButton(b, indigo)),
		b.demo("Hover style, applied directly", 170, 50, rowFlex(), saveButton(b, 0x6366f1)),
	))
}

func textStyles(b *builder) *scene.Node {
	line := func(text string, style *scene.Style) *scene.Node { return scene.Text(b.id(), text).Style(style) }
	return b.page(
		line("FontSize 24", &scene.Style{FontSize: scene.Ptr[float32](24)}),
		line("FontWeight 700", &scene.Style{FontWeight: scene.Ptr[uint32](700)}),
		line("FontWeight 300", &scene.Style{FontWeight: scene.Ptr[uint32](300)}),
		line("Italic", &scene.Style{Italic: scene.Ptr(true)}),
		line("TextColor", &scene.Style{TextColor: scene.RGB(0x34d399)}),
		line("FontFamily Menlo", &scene.Style{FontFamily: scene.Ptr("Menlo")}),
	)
}

func textMixed(b *builder) *scene.Node {
	return b.page(b.row(
		scene.Text(b.id(), "Plain"),
		scene.Text(b.id(), "bold").Style(&scene.Style{FontWeight: scene.Ptr[uint32](700)}),
		scene.Text(b.id(), "colored").Style(&scene.Style{TextColor: scene.RGB(0xfbbf24)}),
		scene.Text(b.id(), "italic").Style(&scene.Style{Italic: scene.Ptr(true)}),
	))
}

func fieldStyle() *scene.Style {
	return &scene.Style{
		Width:        scene.Pixels(320),
		Padding:      scene.Padding(8),
		BorderWidth:  scene.Ptr[float32](1),
		BorderColor:  scene.RGB(0x475569),
		CornerRadius: scene.Ptr[float32](6),
	}
}

func inputSample(b *builder) *scene.Node {
	return b.page(scene.TextInput(b.id(), scene.InputState{Value: "Ada Lovelace"}, scene.TextInputOptions{Placeholder: "Your name"}).Style(fieldStyle()))
}

func inputStates(b *builder) *scene.Node {
	field := func(label, value string, disabled bool) *scene.Node {
		input := scene.TextInput(b.id(), scene.InputState{Value: value}, scene.TextInputOptions{Placeholder: "Your name", Disabled: disabled}).Style(fieldStyle())
		return scene.Div(b.id(), b.caption(label), input).Style(&scene.Style{FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(6)})
	}
	return b.page(field("Empty, showing Placeholder", "", false), field("With a value", "Ada Lovelace", false), field("Disabled", "Read only", true))
}

func inputMasked(b *builder) *scene.Node {
	field := func(id uint64, label, value string) *scene.Node {
		input := scene.TextInput(id, scene.InputState{Value: value}, scene.TextInputOptions{Placeholder: "Password", Masked: true}).Style(fieldStyle())
		return scene.Div(b.id(), b.caption(label), input).Style(&scene.Style{FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(6)})
	}
	return b.page(field(1000, "Focused, with a value", "correct horse"), field(b.id(), "Mixed scripts, one bullet per character", "こんにちは 🦀 café"), field(b.id(), "Empty, showing Placeholder", ""))
}

func inputUnicode(b *builder) *scene.Node {
	return b.page(scene.TextInput(1000, scene.InputState{Value: "こんにちは 🦀 café"}, scene.TextInputOptions{}).Style(fieldStyle()))
}

func imageKinds(b *builder) *scene.Node {
	box := &scene.Style{Width: scene.Pixels(200), Height: scene.Pixels(134)}
	return b.page(b.row(
		b.demo("ResourceImage (PNG)", 216, 150, nil, scene.Image(b.id(), b.images.raster.Handle()).Style(box)),
		b.demo("ResourceSVG, tinted with TextColor", 216, 150, nil, scene.Image(b.id(), b.images.vector.Handle()).Style(&scene.Style{
			Width: scene.Pixels(134), Height: scene.Pixels(134), TextColor: scene.RGB(0x2dd4bf),
		})),
	))
}

func imageBoxes(b *builder) *scene.Node {
	sized := func(width, height float32) *scene.Node {
		return scene.Image(b.id(), b.images.raster.Handle()).Style(&scene.Style{Width: scene.Pixels(width), Height: scene.Pixels(height)})
	}
	return b.page(b.row(
		b.demo("Matching proportions", 160, 130, nil, sized(144, 96)),
		b.demo("Narrow box", 160, 130, nil, sized(70, 114)),
		b.demo("Short box", 160, 130, nil, sized(144, 40)),
	))
}

func rect(x, y, width, height float32, fill uint32) *scene.DrawCommand {
	return &scene.DrawCommand{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{
		Bounds: &scene.Bounds{Origin: &scene.Point{X: x, Y: y}, Size: &scene.Size{Width: width, Height: height}},
		Fill:   scene.RGB(fill),
	}}}
}

func stroke(x0, y0, x1, y1 float32) *scene.DrawCommand {
	return path(0x34d399, 3,
		&scene.PathStep{Verb: scene.PathMove, Point: &scene.Point{X: x0, Y: y0}},
		&scene.PathStep{Verb: scene.PathLine, Point: &scene.Point{X: x1, Y: y1}},
	)
}

func path(paint uint32, width float32, steps ...*scene.PathStep) *scene.DrawCommand {
	return &scene.DrawCommand{Kind: &scene.DrawCommandPath{Path: &scene.DrawPath{Color: scene.RGB(paint), StrokeWidth: width, Steps: steps}}}
}

func canvasSample(b *builder) *scene.Node {
	return scene.Canvas(b.id(),
		&scene.DrawCommand{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{
			Bounds: &scene.Bounds{
				Origin: &scene.Point{X: 10, Y: 10},
				Size:   &scene.Size{Width: 130, Height: 70},
			},
			Fill:   scene.RGB(0x2563eb),
			Radius: 8,
		}}},
		&scene.DrawCommand{Kind: &scene.DrawCommandPath{Path: &scene.DrawPath{
			Color:       scene.RGB(0x34d399),
			StrokeWidth: 4,
			Steps: []*scene.PathStep{
				{Verb: scene.PathMove, Point: &scene.Point{X: 10, Y: 100}},
				{Verb: scene.PathLine, Point: &scene.Point{X: 140, Y: 100}},
			},
		}}},
	).Style(&scene.Style{
		Width:  scene.Pixels(160),
		Height: scene.Pixels(120),
	})
}

func canvasBox(b *builder, commands ...*scene.DrawCommand) *scene.Node {
	return scene.Canvas(b.id(), commands...).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100)})
}

func canvasRects(b *builder) *scene.Node {
	demo := func(label string, item *scene.DrawRect) *scene.Node {
		item.Bounds = &scene.Bounds{Origin: &scene.Point{X: 8, Y: 8}, Size: &scene.Size{Width: 104, Height: 62}}
		return b.demo(label, 150, 100, nil, canvasBox(b, &scene.DrawCommand{Kind: &scene.DrawCommandRect{Rect: item}}))
	}
	return b.page(b.row(
		demo("Fill", &scene.DrawRect{Fill: scene.RGB(0x2563eb)}),
		demo("Border and BorderWidth", &scene.DrawRect{Border: scene.RGB(0xfbbf24), BorderWidth: 3}),
		demo("Radius", &scene.DrawRect{Fill: scene.RGB(teal), Radius: 18}),
	))
}

func canvasPaths(b *builder) *scene.Node {
	point := func(x, y float32) *scene.Point { return &scene.Point{X: x, Y: y} }
	start := &scene.PathStep{Verb: scene.PathMove, Point: point(10, 90)}
	return b.page(b.row(
		b.demo("PathLine", 130, 120, nil, canvasBox(b, path(0x34d399, 3, start, &scene.PathStep{Verb: scene.PathLine, Point: point(100, 15)}))),
		b.demo("PathQuadratic", 130, 120, nil, canvasBox(b, path(0x34d399, 3, start, &scene.PathStep{Verb: scene.PathQuadratic, Point: point(105, 90), Control1: point(55, -20)}))),
		b.demo("PathCubic", 130, 120, nil, canvasBox(b, path(0x34d399, 3, start, &scene.PathStep{Verb: scene.PathCubic, Point: point(105, 15), Control1: point(40, -40), Control2: point(75, 150)}))),
		b.demo("PathClose, filled", 130, 120, nil, canvasBox(b, path(0xf472b6, 0, start,
			&scene.PathStep{Verb: scene.PathLine, Point: point(100, 90)},
			&scene.PathStep{Verb: scene.PathLine, Point: point(55, 12)},
			&scene.PathStep{Verb: scene.PathClose}))),
	))
}

func listView(b *builder) *scene.Node {
	rows := make([]*scene.Node, 0, 1000)
	for i := range 1000 {
		rows = append(rows, scene.Text(uint64(2000+i), fmt.Sprintf("Row %d", i)))
	}
	return b.page(scene.UniformList(900, 28, rows...).Style(&scene.Style{
		Width: scene.Percent(100), Height: scene.Pixels(190), Background: scene.RGB(panel),
		Padding: scene.Padding(8), CornerRadius: scene.Ptr[float32](6),
	}))
}

func dragAndDrop(b *builder) *scene.Node {
	return b.page(b.row(
		b.demo("Drag source", 190, 60, nil, b.box("Drag me", indigo)),
		b.demo("Drop target", 190, 60, nil, b.box("Drop here", 0x3f3f46)),
	))
}

func roles(b *builder) *scene.Node {
	check := func(label string, checked bool) *scene.Node {
		mark := "[ ] "
		if checked {
			mark = "[x] "
		}
		return scene.Div(b.id(), scene.Text(b.id(), mark+label)).Focusable(0).Accessible(label, &scene.Accessibility{Role: scene.RoleCheckbox, Checked: scene.Ptr(checked)})
	}
	return b.page(b.caption("Composed checkboxes using RoleCheckbox"), check("Notifications", true), check("Weekly digest", false))
}

func styleSize(b *builder) *scene.Node {
	return b.page(b.row(
		b.demo("Width 120", 170, 100, columnFlex(), b.box("Pixels(120)", indigo, func(style *scene.Style) { style.Width = scene.Pixels(120) })),
		b.demo("Width 100%", 170, 100, columnFlex(), b.box("Percent(100)", teal, func(style *scene.Style) { style.Width = scene.Percent(100) })),
		b.demo("MaxWidth, MinHeight", 170, 100, columnFlex(), b.box("Capped", amber, func(style *scene.Style) {
			style.MaxWidth = scene.Pixels(90)
			style.MinHeight = scene.Pixels(70)
		})),
	))
}

func styleSpacing(b *builder) *scene.Node {
	return b.page(b.row(
		b.demo("Padding 24", 170, 100, nil, b.box("Text", indigo, func(style *scene.Style) { style.Padding = scene.Padding(24) })),
		b.demo("Margin, top and left", 170, 100, nil, b.box("Text", teal, func(style *scene.Style) {
			style.Margin = &scene.Edges{Top: scene.Pixels(20), Left: scene.Pixels(30)}
		})),
		b.demo("Gap 24", 170, 100, rowFlex(func(style *scene.Style) { style.Gap = scene.Pixels(24) }), b.box("A", amber), b.box("B", amber)),
	))
}

func styleDirection(b *builder) *scene.Node {
	return b.page(b.row(
		b.demo("FlexRow", 270, 90, rowFlex(), b.box("A", indigo), b.box("B", teal), b.box("C", amber)),
		b.demo("FlexColumn", 270, 90, columnFlex(func(style *scene.Style) { style.Gap = scene.Pixels(2); style.Padding = scene.Padding(3) }),
			b.box("A", indigo, tiny), b.box("B", teal, tiny), b.box("C", amber, tiny)),
	))
}

func tiny(style *scene.Style) { style.Padding = scene.Padding(3) }

func alignDemo(b *builder, label string, value scene.Alignment) *scene.Node {
	return b.demo(label, 145, 110, rowFlex(func(style *scene.Style) { style.AlignItems = value.Enum() }),
		b.box("A", indigo), b.box("B", teal, func(style *scene.Style) { style.Padding = scene.Padding(20) }), b.box("C", amber))
}

func styleAlign(b *builder) *scene.Node {
	return b.page(b.row(
		alignDemo(b, "AlignStart", scene.AlignStart), alignDemo(b, "AlignCenter", scene.AlignCenter),
		alignDemo(b, "AlignEnd", scene.AlignEnd), alignDemo(b, "AlignStretch", scene.AlignStretch),
	))
}

func justifyDemo(b *builder, label string, value scene.Justification) *scene.Node {
	return b.demo(label, 145, 60, rowFlex(func(style *scene.Style) { style.JustifyContent = value.Enum() }),
		b.box("A", indigo, compact), b.box("B", teal, compact), b.box("C", amber, compact))
}

func styleJustify(b *builder) *scene.Node {
	return b.page(b.row(
		justifyDemo(b, "JustifyStart", scene.JustifyStart), justifyDemo(b, "JustifyCenter", scene.JustifyCenter),
		justifyDemo(b, "JustifyEnd", scene.JustifyEnd), justifyDemo(b, "JustifySpaceBetween", scene.JustifySpaceBetween),
	))
}

func styleWrap(b *builder) *scene.Node {
	items := func() []*scene.Node {
		nodes := make([]*scene.Node, 0, 7)
		for i := range 7 {
			nodes = append(nodes, b.box(fmt.Sprintf("Item %d", i+1), indigo, compact))
		}
		return nodes
	}
	return b.page(b.row(
		b.demo("FlexWrap false", 270, 100, rowFlex(func(style *scene.Style) { style.OverflowX = scene.OverflowHidden.Enum() }), items()...),
		b.demo("FlexWrap true", 270, 100, rowFlex(func(style *scene.Style) { style.FlexWrap = scene.Ptr(true) }), items()...),
	))
}

func styleGrow(b *builder) *scene.Node {
	grow := func(value float32) tweak {
		return func(style *scene.Style) { style.FlexGrow = scene.Ptr(value) }
	}
	return b.page(b.demo("FlexGrow 0, 1 and 3", 570, 50, rowFlex(), b.box("0", indigo, compact), b.box("1", teal, compact, grow(1)), b.box("3", amber, compact, grow(3))))
}

func styleGrid(b *builder) *scene.Node {
	cells := func(count int) []*scene.Node {
		nodes := make([]*scene.Node, 0, count)
		for i := range count {
			nodes = append(nodes, b.box(fmt.Sprint(i+1), indigo, compact))
		}
		return nodes
	}
	grid := func(columns uint32) []tweak {
		return []tweak{func(style *scene.Style) {
			style.Display = scene.DisplayGrid.Enum()
			style.GridColumns = scene.Ptr(columns)
		}}
	}
	return b.page(b.row(
		b.demo("GridColumns 2", 270, 100, grid(2), cells(4)...),
		b.demo("GridColumns 4", 270, 100, grid(4), cells(8)...),
	))
}

func stylePosition(b *builder) *scene.Node {
	badge := b.box("Absolute", rose, compact, func(style *scene.Style) {
		style.Position = scene.PositionAbsolute.Enum()
		style.Inset = &scene.Edges{Top: scene.Pixels(10), Right: scene.Pixels(10)}
	})
	return b.page(b.row(
		b.demo("Default flow", 270, 120, nil, b.box("Content", indigo)),
		b.demo("PositionAbsolute, Inset top and right", 270, 120, []tweak{func(style *scene.Style) { style.Position = scene.PositionRelative.Enum() }}, b.box("Content", indigo), badge),
	))
}

func styleOverflow(b *builder) *scene.Node {
	tall := func() *scene.Node {
		return b.box("Taller than its parent", indigo, func(style *scene.Style) { style.Height = scene.Pixels(160) })
	}
	mode := func(value scene.Overflow) []tweak {
		return []tweak{func(style *scene.Style) { style.OverflowY = value.Enum() }}
	}
	return b.page(b.row(
		b.demo("OverflowVisible", 180, 60, mode(scene.OverflowVisible), tall()),
		b.demo("OverflowHidden", 180, 60, mode(scene.OverflowHidden), tall()),
		b.demo("OverflowScroll", 180, 60, mode(scene.OverflowScroll), tall()),
	))
}

func styleBorders(b *builder) *scene.Node {
	tile := func(label string, tweaks ...tweak) *scene.Node {
		return b.box(label, panel, append([]tweak{func(style *scene.Style) {
			style.Width = scene.Pixels(140)
			style.Height = scene.Pixels(90)
			style.CornerRadius = nil
		}}, tweaks...)...)
	}
	return b.page(b.row(
		tile("Background", func(style *scene.Style) { style.Background = scene.RGB(indigo) }),
		tile("BorderWidth 3", func(style *scene.Style) {
			style.BorderWidth = scene.Ptr[float32](3)
			style.BorderColor = scene.RGB(0xfbbf24)
		}),
		tile("CornerRadius 20", func(style *scene.Style) {
			style.CornerRadius = scene.Ptr[float32](20)
			style.Background = scene.RGB(teal)
		}),
		tile("All three", func(style *scene.Style) {
			style.BorderWidth = scene.Ptr[float32](2)
			style.BorderColor = scene.RGB(0xf4f4f5)
			style.CornerRadius = scene.Ptr[float32](12)
			style.Background = scene.RGB(rose)
		}),
	))
}

func styleShadows(b *builder) *scene.Node {
	tile := func(label string, shadows ...*scene.Shadow) *scene.Node {
		return b.box(label, 0x3f3f46, func(style *scene.Style) {
			style.Width = scene.Pixels(170)
			style.Height = scene.Pixels(80)
			style.Shadows = shadows
		})
	}
	black := &scene.Color{Alpha: 0.7}
	return b.page(b.row(
		tile("Blur 12", &scene.Shadow{Color: black, Offset: &scene.Point{Y: 4}, Blur: 12}),
		tile("Offset 10, 10", &scene.Shadow{Color: black, Offset: &scene.Point{X: 10, Y: 10}}),
		tile("Colored, Spread 4", &scene.Shadow{Color: scene.RGB(indigo), Offset: &scene.Point{}, Blur: 8, Spread: 4}),
	))
}

func styleOpacity(b *builder) *scene.Node {
	tile := func(value float32) *scene.Node {
		return b.box(fmt.Sprintf("Opacity %.2g", value), indigo, func(style *scene.Style) {
			style.Width = scene.Pixels(130)
			style.Opacity = scene.Ptr(value)
		})
	}
	return b.page(b.row(tile(1), tile(0.75), tile(0.5), tile(0.25)))
}

func styleText(b *builder) *scene.Node {
	aligned := func(label string, value scene.TextAlignment) *scene.Node {
		return scene.Div(b.id(), scene.Text(b.id(), label)).Style(&scene.Style{
			Width: scene.Pixels(180), TextAlign: value.Enum(), Background: scene.RGB(panel), Padding: scene.Padding(8),
		})
	}
	inherited := scene.Div(b.id(), scene.Text(b.id(), "Set on the parent"), scene.Text(b.id(), "Inherited by each child")).Style(&scene.Style{
		FlexDirection: scene.FlexColumn.Enum(), TextColor: scene.RGB(0x34d399), FontSize: scene.Ptr[float32](20), FontWeight: scene.Ptr[uint32](600),
	})
	return b.page(
		b.row(aligned("TextAlignLeft", scene.TextAlignLeft), aligned("TextAlignCenter", scene.TextAlignCenter), aligned("TextAlignRight", scene.TextAlignRight)),
		b.caption("Text properties inherit from an ancestor"),
		inherited,
	)
}

func eventsLog(ctx context.Context, s *session) error {
	b := &builder{images: s.images}
	var window *client.Window
	var lines []string
	handler := uint64(0)
	const target = 5000
	render := func() *scene.Node {
		b.next = 0
		rows := []*scene.Node{b.caption("Events received by one OnEvent handler")}
		for _, line := range lines {
			rows = append(rows, scene.Text(b.id(), line))
		}
		return scene.Div(target, b.page(rows...)).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100)}).
			OnAction("save", handler).OnAction("open", handler)
	}
	handler = s.app.OnEvent(func(ctx context.Context, event *client.Event) error {
		lines = append(lines, fmt.Sprintf("element %d, action %s, payload %q", event.ElementId, event.GetAction().Name, event.GetAction().Payload))
		return window.Set(ctx, render())
	})
	defer s.app.RemoveHandler(handler)
	var err error
	if window, err = s.open(ctx, 460, 170, render()); err != nil {
		return err
	}
	defer window.Close(context.WithoutCancel(ctx))
	for _, call := range [][2]string{{"save", "draft"}, {"open", "notes.txt"}, {"save", "final"}} {
		if err := window.DispatchAction(ctx, target, call[0], []byte(call[1])); err != nil {
			return err
		}
	}
	for len(lines) < 3 {
		events, err := s.app.NextEvents(ctx)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := s.app.Dispatch(ctx, event); err != nil {
				return err
			}
		}
	}
	return s.capture(ctx, "events-log")
}

// reactivePage shares document setup across the ui package samples.
func reactivePage(s *session) (*reactive.Graph, *ui.Document, *reactive.Scope) {
	graph := reactive.NewGraph()
	document := ui.New(graph.Root(), s.app)
	return graph, document, document.Scope()
}

func pageStyle() *scene.Style {
	return &scene.Style{
		Width: scene.Percent(100), Height: scene.Percent(100), Padding: scene.Padding(20), Gap: scene.Pixels(12),
		FlexDirection: scene.FlexColumn.Enum(), Background: scene.RGB(0x18181b), TextColor: scene.RGB(0xf4f4f5),
		FontSize: scene.Ptr[float32](15),
	}
}

func reactiveInput(ctx context.Context, s *session) error {
	graph, document, owner := reactivePage(s)
	defer graph.Dispose()
	name := reactive.NewSignal(owner, "Ada")
	preview := document.BindText(owner, func() string { return "Hello, " + name.Get() + "!" })
	field := document.Input(owner, name, ui.InputOptions{Placeholder: "Your name"}).Style(fieldStyle())
	mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: 400, Height: 130}, document.Div(owner, field, preview).Style(pageStyle()))
	if err != nil {
		return err
	}
	defer mount.Close(context.WithoutCancel(ctx))
	if err := name.Set("Ada Lovelace"); err != nil {
		return err
	}
	return s.capture(ctx, "reactive-text-input")
}

func reactiveShow(ctx context.Context, s *session) error {
	for _, state := range []struct {
		name string
		open bool
	}{{"reactive-show-closed", false}, {"reactive-show-open", true}} {
		if err := func() error {
			graph, document, owner := reactivePage(s)
			defer graph.Dispose()
			open := reactive.NewSignal(owner, state.open)
			details := document.Show(owner, open.Get,
				func(scope *reactive.Scope) *ui.Node { return document.Text(scope, "Details are visible") },
				func(scope *reactive.Scope) *ui.Node { return document.Text(scope, "Details are hidden") },
			)
			label := document.BindText(owner, func() string { return fmt.Sprintf("open = %t", open.Get()) })
			mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: 300, Height: 100}, document.Div(owner, label, details).Style(pageStyle()))
			if err != nil {
				return err
			}
			defer mount.Close(context.WithoutCancel(ctx))
			return s.capture(ctx, state.name)
		}(); err != nil {
			return err
		}
	}
	return nil
}

type task struct {
	ID    int
	Title string
}

func reactiveFor(ctx context.Context, s *session) error {
	graph, document, owner := reactivePage(s)
	defer graph.Dispose()
	tasks := reactive.NewSignal(owner, []task{{1, "Write docs"}, {2, "Take screenshots"}, {3, "Deploy"}})
	rows := ui.For(document, owner, tasks.Get,
		func(item task) int { return item.ID },
		func(scope *reactive.Scope, item *reactive.Signal[task], index *reactive.Signal[int]) *ui.Node {
			return document.BindText(scope, func() string {
				return fmt.Sprintf("%d. %s (key %d)", index.Get()+1, item.Get().Title, item.Get().ID)
			})
		},
	)
	mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: 300, Height: 130}, document.Div(owner, rows).Style(pageStyle()))
	if err != nil {
		return err
	}
	defer mount.Close(context.WithoutCancel(ctx))
	if err := s.capture(ctx, "reactive-for"); err != nil {
		return err
	}
	if err := tasks.Set([]task{{3, "Deploy"}, {1, "Write docs"}, {2, "Take screenshots"}}); err != nil {
		return err
	}
	// Reordering reuses the same window, so it must be captured again.
	id, err := s.onlyWindow(ctx)
	if err != nil {
		return err
	}
	delete(s.seen, id)
	return s.capture(ctx, "reactive-for-reordered")
}

func goxCounter(ctx context.Context, s *session) error {
	app, err := countergox.Open(ctx, s.app, "docshots")
	if err != nil {
		return err
	}
	defer app.Close(context.WithoutCancel(ctx))
	for range 3 {
		if err := app.Increment(); err != nil {
			return err
		}
	}
	return s.capture(ctx, "gox-counter")
}
