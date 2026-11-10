package ui_test

import (
	"fmt"
	"strings"
	"testing"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
)

func rect(width float32) *scene.DrawCommand {
	return &scene.DrawCommand{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{
		Bounds: &scene.Bounds{Origin: &scene.Point{}, Size: &scene.Size{Width: width, Height: 10}},
		Fill:   scene.RGB(0x336699),
	}}}
}

func TestBindCanvasRepaintsOnlyWhenCommandsChange(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	width := reactive.NewSignal(s, float32(10))
	label := reactive.NewSignal(s, "a")
	commands := []*scene.DrawCommand{rect(5)}
	canvas := d.BindCanvas(s, func() []*scene.DrawCommand { return []*scene.DrawCommand{rect(width.Get())} })
	root := d.Div(s, d.Canvas(s, commands...), canvas, d.BindText(s, label.Get))
	// The static canvas holds a copy.
	commands[0].GetRect().Fill = nil
	var frames []*pb.Element
	_, err := d.Attach(root, func(node *scene.Node) error { frames = append(frames, node.Proto()); return nil })
	require(t, err)
	require(t, width.Set(20))
	require(t, label.Set("b"))
	if len(frames) != 3 {
		t.Fatalf("got %d frames", len(frames))
	}
	children := frames[1].GetContainer().Children
	if children[0].GetDrawing().Commands[0].GetRect().Fill == nil {
		t.Error("mutating the caller's commands changed the static canvas")
	}
	if got := children[1].GetDrawing().Commands[0].GetRect().Bounds.Size.Width; got != 20 {
		t.Errorf("bound canvas width = %v", got)
	}
	if canvas.SetChildren(d.Text(s, "x")) == nil {
		t.Error("a canvas accepted children")
	}
}

func TestUniformListRowsComeFromKeyedRanges(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	items := reactive.NewSignal(s, []string{"a", "b", "c"})
	header := d.Text(s, "header")
	rows := ui.For(d, s, items.Get, func(item string) string { return item },
		func(scope *reactive.Scope, item *reactive.Signal[string], _ *reactive.Signal[int]) *ui.Node {
			return d.BindText(scope, item.Get).TWClass("px-2")
		})
	list := d.UniformList(s, 24, header, rows).TWClass("h-64")
	require(t, d.Err())
	var frames []*pb.Element
	_, err := d.Attach(list, func(node *scene.Node) error { frames = append(frames, node.Proto()); return nil })
	require(t, err)
	require(t, items.Set([]string{"c", "a"}))
	rowsOf := func(element *pb.Element) []string {
		var texts []string
		for _, item := range element.GetUniformList().Items {
			texts = append(texts, fmt.Sprintf("%d:%s", item.Id, item.GetText().Content))
		}
		return texts
	}
	first, second := rowsOf(frames[0]), rowsOf(frames[1])
	if len(first) != 4 || len(second) != 3 || frames[1].GetUniformList().RowHeight != 24 {
		t.Fatalf("rows %v then %v", first, second)
	}
	// Reordering keeps each row's element ID.
	if first[3] != second[1] || first[1] != second[2] || second[0] != first[0] {
		t.Errorf("rows %v then %v", first, second)
	}
	if frames[1].Style.Height.GetRems() != 16 {
		t.Errorf("list style = %v", frames[1].Style)
	}
}

func TestMediaNodesRejectInvalidConstruction(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	d.UniformList(d.Scope(), 0)
	d.Image(d.Scope(), nil)
	if err := d.Err(); err == nil || !strings.Contains(err.Error(), "row height") || !strings.Contains(err.Error(), "resource") {
		t.Errorf("construction errors = %v", err)
	}

	bound := ui.New(g.Root(), nil)
	missing := reactive.NewSignal(bound.Scope(), false)
	bound.BindImage(bound.Scope(), func() *clientlib.Resource {
		missing.Get()
		return nil
	})
	if err := bound.Err(); err == nil || !strings.Contains(err.Error(), "resource") {
		t.Errorf("nil bound image = %v", err)
	}
}
