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

func TestGoXTemplateBuildsOnceAndRetainsTextBinding(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	count := reactive.NewSignal(d.Scope(), 0)
	templates := 0
	template := gox.Elem(func(cur gox.Cursor) error {
		templates++
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("testid", "root"); err != nil {
			return err
		}
		if err := cur.Set("style", func() *scene.Style {
			return &scene.Style{Opacity: scene.Ptr[float32](float32(count.Get()) / 10)}
		}); err != nil {
			return err
		}
		if err := cur.Set("role", scene.RoleButton); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Any(nativegox.Text(func() string { return fmt.Sprintf("Count < %d", count.Get()) })); err != nil {
			return err
		}
		return cur.Close()
	})
	root, err := nativegox.Build(context.Background(), d, d.Scope(), template)
	if err != nil {
		t.Fatal(err)
	}
	var latest *scene.Node
	_, err = d.Attach(root, func(node *scene.Node) error { latest = node; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := count.Set(2); err != nil {
		t.Fatal(err)
	}
	if templates != 1 || latest.Proto().GetContainer().Children[0].GetText().Content != "Count < 2" {
		t.Fatalf("template reran or text was escaped: templates=%d snapshot=%v", templates, latest.Proto())
	}
	if latest.Proto().Style.GetOpacity() != 0.2 || latest.Proto().Accessibility.GetRole() != scene.RoleButton {
		t.Fatalf("public aliases did not pass the printer's typed attribute dispatch: %v", latest.Proto())
	}
}

type stubTransport struct{}

func (stubTransport) Call(_ context.Context, _ clientlib.Operation, data []byte) ([]byte, error) {
	request := &pb.Request{}
	if err := proto.Unmarshal(data, request); err != nil {
		return nil, err
	}
	return proto.Marshal(&pb.Response{RequestId: request.RequestId, Body: &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, SessionId: 9, MaxMessageBytes: clientlib.MaxMessageBytes, MaxTreeDepth: 32, MaxElements: 10000}}})
}

func TestGoXInputMaskedAttribute(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	c, err := clientlib.NewClient(context.Background(), stubTransport{})
	if err != nil {
		t.Fatal(err)
	}
	d := ui.New(g.Root(), c)
	value := reactive.NewSignal(d.Scope(), "secret")
	hidden := reactive.NewSignal(d.Scope(), true)
	root, err := nativegox.Build(context.Background(), d, d.Scope(), gox.Elem(func(cur gox.Cursor) error {
		return errors.Join(cur.InitVoid("input"), cur.Set("value", value), cur.Set("masked", hidden.Get), cur.Submit())
	}))
	if err != nil {
		t.Fatal(err)
	}
	var latest *scene.Node
	if _, err := d.Attach(root, func(node *scene.Node) error { latest = node; return nil }); err != nil {
		t.Fatal(err)
	}
	if !latest.Proto().GetTextInput().Masked {
		t.Fatalf("masked accessor was ignored: %v", latest.Proto())
	}
	if err := hidden.Set(false); err != nil {
		t.Fatal(err)
	}
	if latest.Proto().GetTextInput().Masked {
		t.Fatalf("masked accessor did not update: %v", latest.Proto())
	}
	nonInput := gox.Elem(func(cur gox.Cursor) error {
		return errors.Join(cur.Init("div"), cur.Set("masked", true), cur.Submit(), cur.Close())
	})
	badType := gox.Elem(func(cur gox.Cursor) error {
		return errors.Join(cur.InitVoid("input"), cur.Set("value", value), cur.Set("masked", "yes"), cur.Submit())
	})
	for name, template := range map[string]gox.Elem{"non-input": nonInput, "bad type": badType} {
		if _, err := nativegox.Build(context.Background(), d, d.Scope(), template); err == nil || !strings.Contains(err.Error(), "masked") {
			t.Fatalf("%s: invalid masked attribute: %v", name, err)
		}
	}
}

func TestUnsupportedGoXOutputFailsAndDisposesPartialBindings(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	signal := reactive.NewSignal(d.Scope(), 0)
	template := gox.Elem(func(cur gox.Cursor) error {
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Any(nativegox.Text(func() string { return fmt.Sprint(signal.Get()) })); err != nil {
			return err
		}
		return cur.Raw("<script>unsupported</script>")
	})
	if _, err := nativegox.Build(context.Background(), d, d.Scope(), template); err == nil {
		t.Fatal("raw HTML was silently accepted")
	}
	if g.Stats().Computations != 0 {
		t.Fatalf("failed build leaked bindings: %+v", g.Stats())
	}
}

func TestGoXTWClassAttributeAcceptsStringsAndAccessors(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	wide := reactive.NewSignal(d.Scope(), false)
	element := func(class any) gox.Elem {
		return gox.Elem(func(cur gox.Cursor) error {
			if err := cur.Init("div"); err != nil {
				return err
			}
			if err := cur.Set("twClass", class); err != nil {
				return err
			}
			if err := cur.Set("style", &scene.Style{Opacity: scene.Ptr[float32](0.5)}); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			return cur.Close()
		})
	}
	static, err := nativegox.Build(context.Background(), d, d.Scope(), element("p-2 hover:bg-red-500"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := d.Snapshot(static)
	if err != nil {
		t.Fatal(err)
	}
	if style := snapshot.Proto().Style; style.Padding.Top.GetRems() != 0.5 || style.GetOpacity() != 0.5 || snapshot.Proto().HoverStyle.Background == nil {
		t.Errorf("class plus style = %v hover %v", style, snapshot.Proto().HoverStyle)
	}
	bound, err := nativegox.Build(context.Background(), d, d.Scope(), element(func() string {
		if wide.Get() {
			return "w-full"
		}
		return "w-1/2"
	}))
	if err != nil {
		t.Fatal(err)
	}
	if err := wide.Set(true); err != nil {
		t.Fatal(err)
	}
	snapshot, err = d.Snapshot(bound)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Proto().Style.Width.GetPercent() != 100 {
		t.Errorf("bound class width = %v", snapshot.Proto().Style.Width)
	}
	if _, err := nativegox.Build(context.Background(), d, d.Scope(), element(42)); err == nil || !strings.Contains(err.Error(), "twClass requires") {
		t.Errorf("non-string class = %v", err)
	}
}
