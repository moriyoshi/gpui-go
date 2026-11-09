package ui_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	"google.golang.org/protobuf/proto"
)

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func TestSelectiveBindingsAndOneSnapshotPerBatch(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	a := reactive.NewSignal(s, 0)
	b := reactive.NewSignal(s, "A")
	ar, br := 0, 0
	left := d.BindText(s, func() string { ar++; return fmt.Sprint(a.Get()) })
	right := d.BindText(s, func() string { br++; return b.Get() })
	root := d.Div(s, left, right)
	var frames []*pb.Element
	_, err := d.Attach(root, func(node *scene.Node) error { frames = append(frames, node.Proto()); return nil })
	require(t, err)
	require(t, g.Batch(func() error { require(t, a.Set(1)); require(t, a.Set(2)); return b.Set("B") }))
	if len(frames) != 2 || ar != 2 || br != 2 {
		t.Fatalf("bindings/snapshot counts: %d %d %d", len(frames), ar, br)
	}
	if frames[1].GetContainer().Children[0].Id != frames[0].GetContainer().Children[0].Id {
		t.Fatal("binding replaced the stable node")
	}
	require(t, b.Set("C"))
	if ar != 2 || br != 3 {
		t.Fatal("unrelated binding reran")
	}
}
func TestKeyedRowsPreserveIDsAndDisposeRemovedBindings(t *testing.T) {
	type item struct {
		ID    int
		Title string
	}
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	items := reactive.NewSignal(s, []item{{1, "A"}, {2, "B"}})
	built := 0
	rows := ui.For(d, s, items.Get, func(i item) int { return i.ID }, func(owner *reactive.Scope, value *reactive.Signal[item], _ *reactive.Signal[int]) *ui.Node {
		built++
		key := value.Peek().ID
		return d.BindText(owner, func() string { return value.Get().Title }).Name(fmt.Sprint(key))
	})
	root := d.Div(s, rows)
	frames := 0
	_, err := d.Attach(root, func(*scene.Node) error { frames++; return nil })
	require(t, err)
	one, _ := d.Element("1")
	two, _ := d.Element("2")
	require(t, items.Set([]item{{2, "B2"}, {1, "A"}}))
	snapshot, err := d.Snapshot(root)
	require(t, err)
	children := snapshot.Proto().GetContainer().Children
	if built != 2 || children[0].Id != two || children[1].Id != one || children[0].GetText().Content != "B2" {
		t.Fatalf("keyed identity lost: built=%d %v", built, children)
	}
	before := g.Stats().Computations
	require(t, items.Set([]item{{2, "B2"}}))
	if _, ok := d.Element("1"); ok || g.Stats().Computations >= before {
		t.Fatal("removed row retained alias or computations")
	}
	priorFrames := frames
	err = items.Set([]item{{2, "bad"}, {2, "duplicate"}})
	if err == nil || frames != priorFrames {
		t.Fatal("duplicate keys partially published")
	}
	require(t, items.Set([]item{{2, "Recovered"}}))
	if frames != priorFrames+1 {
		t.Fatal("valid list did not recover")
	}
}
func TestConditionalScopesStopHiddenBindings(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := d.Scope()
	show := reactive.NewSignal(s, true)
	value := reactive.NewSignal(s, 0)
	runs := 0
	branch := d.Show(s, show.Get, func(owner *reactive.Scope) *ui.Node {
		return d.BindText(owner, func() string { runs++; return fmt.Sprint(value.Get()) }).Name("details")
	}, nil)
	root := d.Div(s, branch)
	_, err := d.Attach(root, func(*scene.Node) error { return nil })
	require(t, err)
	first, _ := d.Element("details")
	require(t, show.Set(false))
	require(t, value.Set(1))
	if runs != 1 {
		t.Fatal("hidden branch kept observing")
	}
	require(t, show.Set(true))
	second, _ := d.Element("details")
	if first == second || runs != 2 {
		t.Fatal("conditional branch was not recreated correctly")
	}
}

type transport func(context.Context, clientlib.Operation, []byte) ([]byte, error)

func (f transport) Call(ctx context.Context, op clientlib.Operation, data []byte) ([]byte, error) {
	return f(ctx, op, data)
}
func client(t *testing.T) *clientlib.Client {
	t.Helper()
	c, err := clientlib.NewClient(context.Background(), transport(func(_ context.Context, _ clientlib.Operation, data []byte) ([]byte, error) {
		request := &pb.Request{}
		if err := proto.Unmarshal(data, request); err != nil {
			return nil, err
		}
		return proto.Marshal(&pb.Response{RequestId: request.RequestId, Body: &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, SessionId: 9, MaxMessageBytes: clientlib.MaxMessageBytes, MaxTreeDepth: 32, MaxElements: 10000}}})
	}))
	require(t, err)
	return c
}
func TestControlledInputAcknowledgementAndEpochReplacement(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	c := client(t)
	d := ui.New(g.Root(), c)
	s := d.Scope()
	value := reactive.NewSignal(s, "")
	field := d.Input(s, value, ui.InputOptions{})
	root := d.Div(s, field)
	var latest *pb.Element
	_, err := d.Attach(root, func(node *scene.Node) error { latest = node.Proto(); return nil })
	require(t, err)
	props := func() *pb.TextInput { return latest.GetContainer().Children[0].GetTextInput() }
	handler := props().OnChange.HandlerId
	dispatch := func(epoch, revision uint64, text string) {
		require(t, c.Dispatch(context.Background(), &pb.Event{HandlerId: handler, Kind: &pb.Event_TextChanged{TextChanged: &pb.TextChanged{Value: text, Epoch: epoch, EditRevision: revision}}}))
	}
	dispatch(0, 1, "e\u0301日本語🙂")
	if value.Get() != "e\u0301日本語🙂" || props().Epoch != 0 || props().EditRevision != 1 {
		t.Fatalf("native edit was reset: %v", props())
	}
	require(t, value.Set("Reset"))
	if props().Epoch != 1 || props().EditRevision != 0 {
		t.Fatalf("authoritative replacement did not advance epoch: %v", props())
	}
	dispatch(0, 99, "obsolete")
	if value.Get() != "Reset" {
		t.Fatal("obsolete native edit overwrote replacement")
	}
	field.Dispose()
	dispatch(1, 1, "removed")
	if value.Get() != "Reset" {
		t.Fatal("removed field retained handler")
	}
}
func TestInputMaskedFollowsItsAccessorWithoutChangingTheValue(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), client(t))
	s := d.Scope()
	value := reactive.NewSignal(s, "hunter2")
	hidden := reactive.NewSignal(s, true)
	root := d.Div(s, d.Input(s, value, ui.InputOptions{Masked: hidden.Get}))
	var latest *pb.Element
	frames := 0
	_, err := d.Attach(root, func(node *scene.Node) error { latest = node.Proto(); frames++; return nil })
	require(t, err)
	props := func() *pb.TextInput { return latest.GetContainer().Children[0].GetTextInput() }
	if !props().Masked || props().Value != "hunter2" {
		t.Fatalf("initial mask: %v", props())
	}
	before := props().Epoch
	require(t, hidden.Set(false))
	if props().Masked || props().Value != "hunter2" || props().Epoch != before || frames != 2 {
		t.Fatalf("toggling the mask disturbed the input: %v frames=%d", props(), frames)
	}
}
func TestSinkFailureIsNotAutomaticallyRetried(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := reactive.NewSignal(d.Scope(), 0)
	root := d.BindText(d.Scope(), func() string { return fmt.Sprint(s.Get()) })
	calls := 0
	failure := errors.New("reply lost after possible commit")
	_, err := d.Attach(root, func(*scene.Node) error {
		calls++
		if calls > 1 {
			return failure
		}
		return nil
	})
	require(t, err)
	if err := s.Set(1); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if err := s.Set(2); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("uncertain snapshot was retried: %d", calls)
	}
}

func TestDocumentRejectsCompetingSinksAndCanReattachAfterStopping(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	s := reactive.NewSignal(d.Scope(), "A")
	root := d.BindText(d.Scope(), s.Get)
	first, second := 0, 0
	stop, err := d.Attach(root, func(*scene.Node) error { first++; return nil })
	require(t, err)
	if _, err := d.Attach(root, func(*scene.Node) error { second++; return nil }); err == nil || second != 0 {
		t.Fatal("competing sink could consume another sink's updates")
	}
	stop()
	_, err = d.Attach(root, func(*scene.Node) error { second++; return nil })
	require(t, err)
	stop() // Stopping an old registration must not detach its replacement.
	require(t, s.Set("B"))
	if first != 1 || second != 2 {
		t.Fatalf("reattached sink lost updates: %d %d", first, second)
	}
}

func TestDocumentDisposesNodesWithExternalOwners(t *testing.T) {
	g := reactive.NewGraph()
	defer g.Dispose()
	d := ui.New(g.Root(), nil)
	value := reactive.NewSignal(g.Root(), "A")
	runs := 0
	d.BindText(g.Root(), func() string { runs++; return value.Get() }).Name("outside")
	d.Dispose()
	require(t, value.Set("B"))
	if runs != 1 || g.Stats().Computations != 0 {
		t.Fatalf("document left externally owned bindings alive: %d %+v", runs, g.Stats())
	}
	if _, ok := d.Element("outside"); ok {
		t.Fatal("document left disposed names alive")
	}
}
