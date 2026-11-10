package ui_test

import (
	"strings"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/tw"
	"github.com/moriyoshi/gpui-go/ui"
	"google.golang.org/protobuf/proto"
)

func TestClassesAreRefinedByExplicitStyles(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	node := d.Div(s).TWClass("p-4 bg-blue-500 rounded-md shadow-sm hover:bg-blue-600").
		Style(&scene.Style{Padding: &scene.Edges{Top: scene.Pixels(1)}, Shadows: []*scene.Shadow{{Color: &scene.Color{Alpha: 1}, Offset: &scene.Point{}}}}).
		Hover(&scene.Style{Opacity: proto.Float32(0.5)})
	require(t, d.Err())
	snapshot, err := d.Snapshot(node)
	require(t, err)
	element := snapshot.Proto()
	style := element.Style
	if style.Padding.Top.GetPixels() != 1 || style.Padding.Left.GetRems() != 1 {
		t.Errorf("padding = %v; the explicit top should refine the class padding", style.Padding)
	}
	if style.Background == nil || style.GetCornerRadius() != 6 {
		t.Errorf("class fields were lost: %v", style)
	}
	if len(style.Shadows) != 1 || style.Shadows[0].Color.Alpha != 1 {
		t.Errorf("explicit shadows should replace the class shadows: %v", style.Shadows)
	}
	want := tw.MustCompile("hover:bg-blue-600").Hover()
	want.Opacity = proto.Float32(0.5)
	if !proto.Equal(element.HoverStyle, want) {
		t.Errorf("hover = %v, want %v", element.HoverStyle, want)
	}
	// Removing the explicit style leaves the classes alone.
	node.Style(nil).Hover(nil)
	snapshot, err = d.Snapshot(node)
	require(t, err)
	if !proto.Equal(snapshot.Proto().Style, tw.MustCompile("p-4 bg-blue-500 rounded-md shadow-sm").Style()) {
		t.Errorf("style without explicit refinement = %v", snapshot.Proto().Style)
	}
}

func TestBindTWClassFollowsSignalsAndReportsBadClasses(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	active := reactive.NewSignal(s, false)
	recompiled := 0
	node := d.Div(s).BindTWClass(func() string {
		recompiled++
		if active.Get() {
			return "p-2 bg-green-500"
		}
		return "p-2 bg-gray-200"
	})
	var frames []*pb.Element
	_, err := d.Attach(node, func(snapshot *scene.Node) error { frames = append(frames, snapshot.Proto()); return nil })
	require(t, err)
	require(t, active.Set(true))
	if len(frames) != 2 || proto.Equal(frames[0].Style, frames[1].Style) {
		t.Fatalf("class change produced %d frames", len(frames))
	}
	if !proto.Equal(frames[1].Style, tw.MustCompile("p-2 bg-green-500").Style()) {
		t.Errorf("bound style = %v", frames[1].Style)
	}

	broken := reactive.NewSignal(s, "p-2")
	d.Div(s).BindTWClass(broken.Get)
	if err := broken.Set("p-2 focus:p-4"); err == nil || !strings.Contains(err.Error(), "unsupported variant") {
		t.Errorf("bad bound class = %v", err)
	}
	if recompiled != 2 {
		t.Errorf("accessor ran %d times", recompiled)
	}
}

func TestClassConstructionErrorsAndDocumentThemes(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	d.Div(d.Scope()).TWClass("bg-brand")
	if err := d.Err(); err == nil || !strings.Contains(err.Error(), "bg-brand") {
		t.Errorf("unknown class error = %v", err)
	}

	theme := tw.Default()
	theme.Colors["brand"] = scene.RGB(0x112233)
	compiler, err := tw.New(theme)
	require(t, err)
	themed := ui.New(g.Root(), nil)
	themed.UseTheme(compiler)
	node := themed.Div(themed.Scope()).TWClass("bg-brand")
	require(t, themed.Err())
	snapshot, err := themed.Snapshot(node)
	require(t, err)
	if !proto.Equal(snapshot.Proto().Style.Background, scene.RGB(0x112233)) {
		t.Errorf("themed background = %v", snapshot.Proto().Style.Background)
	}
}

func TestExplicitStylesAlsoWinOverConditionalUtilities(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	node := d.Div(d.Scope()).TWClass("p-2 md:p-4 md:bg-red-500 dark:hover:bg-slate-700 dark:opacity-50").
		Style(&scene.Style{Padding: &scene.Edges{Top: scene.Pixels(1)}, Background: scene.RGB(0xffffff)}).
		Hover(&scene.Style{Background: scene.RGB(0x000000)})
	require(t, d.Err())
	snapshot, err := d.Snapshot(node)
	require(t, err)
	conditions := snapshot.Proto().ConditionalStyles
	// md: keeps its other padding sides; its background and dark:'s hover
	// background yield to the explicit styles, leaving only dark:opacity-50.
	if len(conditions) != 2 {
		t.Fatalf("conditions = %v", conditions)
	}
	md := conditions[0]
	if md.GetMinWidth() != 768 || md.Style.Padding.Top != nil || md.Style.Padding.Left.GetRems() != 1 || md.Style.Background != nil {
		t.Errorf("md: = %v", md)
	}
	dark := conditions[1]
	if dark.ColorScheme != scene.ColorSchemeDark || dark.HoverStyle != nil || dark.Style.GetOpacity() != 0.5 {
		t.Errorf("dark: = %v", dark)
	}
	// Without explicit styles the utilities are untouched.
	node.Style(nil).Hover(nil)
	snapshot, err = d.Snapshot(node)
	require(t, err)
	if got := snapshot.Proto().ConditionalStyles; len(got) != 2 || got[0].Style.Background == nil || got[1].HoverStyle == nil {
		t.Errorf("restored conditions = %v", got)
	}
}
