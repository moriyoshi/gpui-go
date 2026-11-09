package editor_test

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/moriyoshi/gpui-go/editor"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/editor/v1"
	"google.golang.org/protobuf/proto"
)

type transport struct {
	command func(*pb.Request) *pb.Response
	event   func(context.Context) (*pb.Event, error)
}

func (m transport) Call(ctx context.Context, op editor.Operation, data []byte) ([]byte, error) {
	if op == editor.OperationEvents {
		event, err := m.event(ctx)
		if err != nil {
			return nil, err
		}
		return proto.Marshal(event)
	}
	request := new(pb.Request)
	if err := proto.Unmarshal(data, request); err != nil {
		return nil, err
	}
	var response *pb.Response
	if request.GetHello() != nil {
		response = &pb.Response{Result: &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, MaxTextBytes: editor.MaxTextBytes, MaxEditors: 64}}}
	} else {
		response = m.command(request)
	}
	response.RequestId = request.RequestId
	return proto.Marshal(response)
}
func newClient(t *testing.T, m transport) *editor.Client {
	t.Helper()
	c, err := editor.NewClient(context.Background(), m)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func opened(id uint64) *pb.Response {
	return &pb.Response{EditorId: id, Result: &pb.Response_State{State: &pb.State{EditorId: id}}}
}
func TestOpenDefaultsAndIndependentHandles(t *testing.T) {
	var id uint64
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		options := request.GetOpen()
		if options == nil || options.Width != 1000 || options.Height != 700 || !options.ShowGutter || !options.SoftWrap || options.Title != "Editor" {
			t.Fatalf("unexpected defaults: %v", request)
		}
		if request.EditorId != 0 {
			t.Fatal("open must not target an editor")
		}
		id++
		return opened(id)
	}})
	first, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID() == second.ID() || first.ID() == 0 {
		t.Fatal("expected distinct nonzero handles")
	}
}
func TestInvalidInputDoesNotReachTransport(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, transport{command: func(*pb.Request) *pb.Response { calls.Add(1); return opened(1) }})
	for _, options := range []editor.Options{
		{Text: strings.Repeat("x", editor.MaxTextBytes+1)}, {Text: string([]byte{0xff})},
		{Title: strings.Repeat("x", 1025)}, {Width: float32(math.NaN())}, {Height: float32(math.Inf(1))}, {Width: -1},
	} {
		if _, err := c.Open(context.Background(), options); err == nil {
			t.Fatalf("accepted invalid options: width=%v height=%v", options.Width, options.Height)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Open(ctx, editor.Options{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("invalid requests reached transport")
	}
}
func TestResponseCannotCrossEditorHandles(t *testing.T) {
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetOpen() != nil {
			return opened(10)
		}
		if request.EditorId != 10 {
			t.Fatalf("command targeted %d", request.EditorId)
		}
		return opened(11)
	}})
	e, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.State(context.Background()); err == nil {
		t.Fatal("accepted another editor's state")
	}
}
func TestTypedErrorAndCloseAcknowledgement(t *testing.T) {
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetOpen() != nil {
			return opened(4)
		}
		response := &pb.Response{EditorId: request.EditorId}
		if request.GetClose() != nil {
			response.Result = &pb.Response_Ack{Ack: &pb.Ack{}}
		} else {
			response.Result = &pb.Response_Error{Error: &pb.Error{Code: pb.ErrorCode_ERROR_CODE_READ_ONLY, Message: "locked"}}
		}
		return response
	}})
	e, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.InsertText(context.Background(), "日本語")
	var protocolErr *editor.ProtocolError
	if !errors.Is(err, editor.ErrReadOnly) || !errors.As(err, &protocolErr) || protocolErr.Code != editor.ReadOnly {
		t.Fatalf("typed error: %v", err)
	}
	if err := e.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestEventsValidateSequenceAndStateHandle(t *testing.T) {
	events := []*pb.Event{
		{Sequence: 2, EditorId: 3, Kind: &pb.Event_Changed{Changed: &pb.State{EditorId: 3, Revision: 7, Text: "日本語"}}},
		{Sequence: 2, EditorId: 3, Kind: &pb.Event_Closed{Closed: &pb.Ack{}}},
		{Sequence: 3, EditorId: 3, Kind: &pb.Event_Changed{Changed: &pb.State{EditorId: 9}}},
		{Sequence: 4, EditorId: 3, Kind: &pb.Event_Closed{Closed: &pb.Ack{}}},
	}
	c := newClient(t, transport{event: func(context.Context) (*pb.Event, error) { result := events[0]; events = events[1:]; return result, nil }})
	event, err := c.NextEvent(context.Background())
	if err != nil || event.State.Text != "日本語" || event.State.Revision != 7 {
		t.Fatalf("event: %v %v", event, err)
	}
	if _, err := c.NextEvent(context.Background()); err == nil {
		t.Fatal("accepted repeated sequence")
	}
	if _, err := c.NextEvent(context.Background()); err == nil {
		t.Fatal("accepted mismatched event state")
	}
	event, err = c.NextEvent(context.Background())
	if err != nil || event.Kind != editor.EditorClosed {
		t.Fatalf("close event: %v %v", event, err)
	}
}
func TestConcurrentEventConsumerRejectedAndCancellationReleasesIt(t *testing.T) {
	entered := make(chan struct{})
	c := newClient(t, transport{event: func(ctx context.Context) (*pb.Event, error) { close(entered); <-ctx.Done(); return nil, ctx.Err() }})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.NextEvent(ctx); done <- err }()
	<-entered
	if _, err := c.NextEvent(context.Background()); !errors.Is(err, editor.ErrConcurrentEvents) {
		t.Fatalf("second consumer: %v", err)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled consumer: %v", err)
	}
}

type transportFunc func(context.Context, editor.Operation, []byte) ([]byte, error)

func (f transportFunc) Call(ctx context.Context, op editor.Operation, bytes []byte) ([]byte, error) {
	return f(ctx, op, bytes)
}
func TestNegotiationAndCorrelationValidation(t *testing.T) {
	for _, scenario := range []string{"wrong request ID", "wrong version", "missing capabilities"} {
		t.Run(scenario, func(t *testing.T) {
			_, err := editor.NewClient(context.Background(), transportFunc(func(_ context.Context, _ editor.Operation, bytes []byte) ([]byte, error) {
				request := new(pb.Request)
				if err := proto.Unmarshal(bytes, request); err != nil {
					return nil, err
				}
				response := &pb.Response{RequestId: request.RequestId, Result: &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, MaxTextBytes: editor.MaxTextBytes, MaxEditors: 64}}}
				switch scenario {
				case "wrong request ID":
					response.RequestId++
				case "wrong version":
					response.GetCapabilities().Major = 2
				case "missing capabilities":
					response.Result = &pb.Response_Ack{Ack: &pb.Ack{}}
				}
				return proto.Marshal(response)
			}))
			if err == nil {
				t.Fatal("accepted invalid negotiation")
			}
		})
	}
}

func TestSetSelectionSendsPositionsAndReturnsSelection(t *testing.T) {
	var sent *pb.SetSelection
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetOpen() != nil {
			return opened(1)
		}
		sent = request.GetSetSelection()
		if sent == nil || request.EditorId != 1 {
			t.Fatalf("unexpected request: %v", request)
		}
		return &pb.Response{EditorId: 1, Result: &pb.Response_State{State: &pb.State{EditorId: 1, Revision: 7, Selection: &pb.Selection{
			Start: &pb.Position{Line: 2, Column: 3}, End: &pb.Position{Line: 4, Column: 5},
		}}}}
	}})
	field, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	start, end := editor.Position{Line: 2, Column: 3}, editor.Position{Line: 4, Column: 5}
	state, err := field.SetSelection(context.Background(), start, end)
	if err != nil {
		t.Fatal(err)
	}
	if got := sent.GetSelection(); got.GetStart().GetLine() != 2 || got.GetStart().GetColumn() != 3 || got.GetEnd().GetLine() != 4 || got.GetEnd().GetColumn() != 5 {
		t.Fatalf("sent %v", got)
	}
	if state.Selection != (editor.Selection{Start: start, End: end}) || state.Revision != 7 {
		t.Fatalf("state %+v", state)
	}
}

func TestReversedSelectionDoesNotReachTransport(t *testing.T) {
	var calls atomic.Int32
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetSetSelection() != nil {
			calls.Add(1)
		}
		return opened(1)
	}})
	field, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, positions := range [][2]editor.Position{
		{{Line: 1}, {Line: 0, Column: 9}},
		{{Line: 1, Column: 2}, {Line: 1, Column: 1}},
	} {
		_, err := field.SetSelection(context.Background(), positions[0], positions[1])
		var protocol *editor.ProtocolError
		if !errors.As(err, &protocol) || protocol.Code != editor.InvalidArgument {
			t.Fatalf("reversed selection: %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("reversed selection reached transport")
	}
}

func TestSetSelectionSendsClampAndLeavesOrderToNative(t *testing.T) {
	var sent *pb.SetSelection
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetOpen() != nil {
			return opened(1)
		}
		sent = request.GetSetSelection()
		return &pb.Response{RequestId: request.RequestId, EditorId: 1, Result: &pb.Response_State{State: &pb.State{EditorId: 1}}}
	}})
	field, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Inside one wide character, these two can land in either order.
	start := editor.Position{Line: 1, Column: 3, Clamp: editor.ClampBefore}
	end := editor.Position{Line: 1, Column: 2, Clamp: editor.ClampAfter}
	if _, err := field.SetSelection(context.Background(), start, end); err != nil {
		t.Fatal(err)
	}
	if sent.GetSelection().GetStart().GetClamp() != pb.Clamp_CLAMP_BEFORE || sent.GetSelection().GetEnd().GetClamp() != pb.Clamp_CLAMP_AFTER {
		t.Fatalf("clamp not sent: %v", sent)
	}
}

func TestSetSelectionSendsCharacterAndLeavesOrderToNative(t *testing.T) {
	var sent *pb.SetSelection
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		if request.GetOpen() != nil {
			return opened(1)
		}
		sent = request.GetSetSelection()
		return &pb.Response{RequestId: request.RequestId, EditorId: 1, Result: &pb.Response_State{State: &pb.State{EditorId: 1}}}
	}})
	field, err := c.Open(context.Background(), editor.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// Two wide characters end at column 4, so this is in order natively.
	start := editor.Position{Line: 1, Column: 3}
	end := editor.Position{Line: 1, Character: 2}
	if _, err := field.SetSelection(context.Background(), start, end); err != nil {
		t.Fatal(err)
	}
	if sent.GetSelection().GetEnd().GetCharacter() != 2 {
		t.Fatalf("character not sent: %v", sent)
	}
}

func TestOpenSendsWidthTable(t *testing.T) {
	var sent *pb.Open
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		sent = request.GetOpen()
		return opened(1)
	}})
	if _, err := c.Open(context.Background(), editor.Options{WidthTable: editor.WidthWideSymbols}); err != nil {
		t.Fatal(err)
	}
	if sent.GetWidthTable() != pb.WidthTable_WIDTH_TABLE_WIDE_SYMBOLS {
		t.Fatalf("width table not sent: %v", sent)
	}
}

type embedTarget struct{}

func (embedTarget) EmbedTarget() (uint64, uint32) { return 7, 3 }

func TestOpenEmbeddedTargetsTheView(t *testing.T) {
	var sent *pb.OpenEmbedded
	c := newClient(t, transport{command: func(request *pb.Request) *pb.Response {
		sent = request.GetOpenEmbedded()
		if sent == nil || request.EditorId != 0 {
			t.Fatalf("unexpected request: %v", request)
		}
		return opened(9)
	}})
	field, err := c.OpenEmbedded(context.Background(), embedTarget{}, editor.EmbedOptions{Text: "a", ReadOnly: true, HideGutter: true, WidthTable: editor.WidthWideAmbiguous})
	if err != nil {
		t.Fatal(err)
	}
	if sent.GetWidthTable() != pb.WidthTable_WIDTH_TABLE_WIDE_AMBIGUOUS {
		t.Fatalf("width table not sent: %v", sent)
	}
	if field.ID() != 9 || sent.ViewId != 7 || sent.ViewGeneration != 3 || sent.Text != "a" || !sent.ReadOnly || sent.ShowGutter || !sent.SoftWrap {
		t.Fatalf("id %d, sent %v", field.ID(), sent)
	}
	if _, err := c.OpenEmbedded(context.Background(), embedTarget{}, editor.EmbedOptions{Text: string([]byte{0xff})}); err == nil {
		t.Fatal("accepted invalid text")
	}
}
