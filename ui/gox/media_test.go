package gox_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/doors-dev/gox"
	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
	"google.golang.org/protobuf/proto"
)

// uploadTransport answers Hello and hands out a fresh handle per upload.
type uploadTransport struct{ next *uint64 }

func (u uploadTransport) Call(_ context.Context, _ clientlib.Operation, data []byte) ([]byte, error) {
	request := &pb.Request{}
	if err := proto.Unmarshal(data, request); err != nil {
		return nil, err
	}
	response := &pb.Response{RequestId: request.RequestId}
	if request.GetCommand().GetUploadResource() != nil {
		*u.next++
		response.Body = &pb.Response_Result{Result: &pb.CommandResult{Resource: &pb.Handle{Id: *u.next, Generation: 1}}}
	} else {
		response.Body = &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, SessionId: 9, MaxMessageBytes: clientlib.MaxMessageBytes, MaxTreeDepth: 32, MaxElements: 10000}}
	}
	return proto.Marshal(response)
}

func mediaDocument(t *testing.T) (*ui.Document, *clientlib.Client, func()) {
	t.Helper()
	g := reactive.NewGraph()
	c, err := clientlib.NewClient(context.Background(), uploadTransport{new(uint64)})
	if err != nil {
		t.Fatal(err)
	}
	return ui.New(g.Root(), c), c, g.Dispose
}

// head builds <tag attrs...>children</tag>; void tags take no children.
func head(void bool, tag string, attrs map[string]any, children ...gox.Elem) gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		var err error
		if void {
			err = cur.InitVoid(tag)
		} else {
			err = cur.Init(tag)
		}
		for name, value := range attrs {
			err = errors.Join(err, cur.Set(name, value))
		}
		err = errors.Join(err, cur.Submit())
		if void {
			return err
		}
		for _, child := range children {
			err = errors.Join(err, cur.Any(child))
		}
		return errors.Join(err, cur.Close())
	})
}

func TestGoXImgCanvasAndUniformListTags(t *testing.T) {
	ctx := context.Background()
	d, c, dispose := mediaDocument(t)
	defer dispose()
	red, err := c.UploadResource(ctx, clientlib.ResourceImage, "image/png", []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	blue, err := c.UploadResource(ctx, clientlib.ResourceImage, "image/png", []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	s := d.Scope()
	useBlue := reactive.NewSignal(s, false)
	width := reactive.NewSignal(s, float32(10))
	items := reactive.NewSignal(s, []string{"a", "b"})
	rows := ui.For(d, s, items.Get, func(item string) string { return item },
		func(scope *reactive.Scope, item *reactive.Signal[string], _ *reactive.Signal[int]) *ui.Node {
			return d.BindText(scope, item.Get)
		})
	rect := func(w float32) []*scene.DrawCommand {
		return []*scene.DrawCommand{{Kind: &scene.DrawCommandRect{Rect: &scene.DrawRect{
			Bounds: &scene.Bounds{Origin: &scene.Point{}, Size: &scene.Size{Width: w, Height: 4}}}}}}
	}
	template := head(false, "div", nil,
		head(true, "img", map[string]any{"src": red, "label": "Logo", "twClass": "size-8"}),
		head(true, "img", map[string]any{"src": func() *clientlib.Resource {
			if useBlue.Get() {
				return blue
			}
			return red
		}}),
		head(false, "canvas", map[string]any{"commands": rect(5)}),
		head(false, "canvas", map[string]any{"commands": func() []*scene.DrawCommand { return rect(width.Get()) }}),
		head(false, "canvas", nil),
		head(false, "uniform-list", map[string]any{"rowHeight": 24, "testid": "rows", "twClass": "h-40"},
			gox.Elem(func(cur gox.Cursor) error { return cur.Any(rows) })),
	)
	root, err := nativegox.Build(ctx, d, s, template)
	if err != nil {
		t.Fatal(err)
	}
	var latest *pb.Element
	if _, err := d.Attach(root, func(node *scene.Node) error { latest = node.Proto(); return nil }); err != nil {
		t.Fatal(err)
	}
	children := latest.GetContainer().Children
	if len(children) != 6 {
		t.Fatalf("children = %v", children)
	}
	if children[0].GetImage().Resource.Id != red.Handle().Id || children[0].GetAccessibility().GetRole() != scene.RoleImage ||
		children[0].GetAccessibilityLabel() != "Logo" || children[0].Style.Width.GetRems() != 2 {
		t.Errorf("static img = %v", children[0])
	}
	if got := children[2].GetDrawing().Commands[0].GetRect().Bounds.Size.Width; got != 5 {
		t.Errorf("static canvas width = %v", got)
	}
	if len(children[4].GetDrawing().GetCommands()) != 0 {
		t.Errorf("empty canvas = %v", children[4])
	}
	list := children[5]
	if list.GetUniformList().RowHeight != 24 || len(list.GetUniformList().Items) != 2 || list.GetAccessibility().GetRole() != scene.RoleList {
		t.Errorf("uniform list = %v", list)
	}
	if id, ok := d.Element("rows"); !ok || id != list.Id {
		t.Errorf("testid = %v %v", id, ok)
	}
	require := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	require(useBlue.Set(true))
	require(width.Set(30))
	require(items.Set([]string{"b", "a", "c"}))
	children = latest.GetContainer().Children
	if children[1].GetImage().Resource.Id != blue.Handle().Id {
		t.Errorf("bound img did not switch: %v", children[1])
	}
	if got := children[3].GetDrawing().Commands[0].GetRect().Bounds.Size.Width; got != 30 {
		t.Errorf("bound canvas width = %v", got)
	}
	var texts []string
	for _, item := range children[5].GetUniformList().Items {
		texts = append(texts, item.GetText().Content)
	}
	if fmt.Sprint(texts) != "[b a c]" {
		t.Errorf("rows after reorder = %v", texts)
	}
}

func TestGoXMediaTagsRejectInvalidAttributes(t *testing.T) {
	d, _, dispose := mediaDocument(t)
	defer dispose()
	cases := map[string]struct {
		template gox.Elem
		want     string
	}{
		"img without src":       {head(true, "img", nil), "img src requires"},
		"img src type":          {head(true, "img", map[string]any{"src": "logo.png"}), "img src requires"},
		"src on div":            {head(false, "div", map[string]any{"src": "x"}), "src requires img"},
		"canvas commands type":  {head(false, "canvas", map[string]any{"commands": "line"}), "canvas commands require"},
		"commands on img":       {head(true, "img", map[string]any{"commands": "line"}), "commands requires canvas"},
		"canvas children":       {head(false, "canvas", nil, head(false, "div", nil)), "accept children"},
		"list without height":   {head(false, "uniform-list", nil), "numeric rowHeight"},
		"list height range":     {head(false, "uniform-list", map[string]any{"rowHeight": 0}), "[1, 16384]"},
		"rowHeight on div":      {head(false, "div", map[string]any{"rowHeight": 20}), "rowHeight requires uniform-list"},
		"list height as string": {head(false, "uniform-list", map[string]any{"rowHeight": "20"}), "numeric rowHeight"},
	}
	for name, c := range cases {
		if _, err := nativegox.Build(context.Background(), d, d.Scope(), c.template); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error mentioning %q", name, err, c.want)
		}
	}
	// A nil resource from an img accessor fails the binding.
	bad := head(true, "img", map[string]any{"src": func() *clientlib.Resource { return nil }})
	if _, err := nativegox.Build(context.Background(), d, d.Scope(), bad); err == nil && d.Err() == nil {
		t.Error("an img accessor returning nil was accepted")
	}
}
