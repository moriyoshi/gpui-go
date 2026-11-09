package client

import (
	"context"
	"errors"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

type transportFunc func(context.Context, Operation, []byte) ([]byte, error)

func (f transportFunc) Call(ctx context.Context, op Operation, data []byte) ([]byte, error) {
	return f(ctx, op, data)
}

func testTransport(t *testing.T, handler func(*pb.Request) (*pb.Response, error)) Transport {
	t.Helper()
	return transportFunc(func(ctx context.Context, _ Operation, data []byte) ([]byte, error) {
		request := new(pb.Request)
		if err := proto.Unmarshal(data, request); err != nil {
			t.Fatal(err)
		}
		response, err := handler(request)
		if err != nil {
			return nil, err
		}
		if response.RequestId == 0 {
			response.RequestId = request.RequestId
		}
		return proto.Marshal(response)
	})
}

func capabilities() *pb.Response {
	return &pb.Response{Body: &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, SessionId: 42, MaxMessageBytes: MaxMessageBytes, MaxTreeDepth: 32, MaxElements: 10000}}}
}

func TestCorrelationAndProtocolErrors(t *testing.T) {
	_, err := NewClient(context.Background(), testTransport(t, func(*pb.Request) (*pb.Response, error) {
		response := capabilities()
		response.RequestId = 999
		return response, nil
	}))
	if err == nil {
		t.Fatal("accepted mismatched correlation ID")
	}
	_, err = NewClient(context.Background(), testTransport(t, func(*pb.Request) (*pb.Response, error) {
		return &pb.Response{Body: &pb.Response_Error{Error: &pb.ProtocolError{Code: pb.ErrorCode_ERROR_CODE_UNSUPPORTED, Message: "version"}}}, nil
	}))
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != pb.ErrorCode_ERROR_CODE_UNSUPPORTED {
		t.Fatalf("got %v", err)
	}
}

func TestCanceledCommitDoesNotReuseItsRevision(t *testing.T) {
	var revisions []uint64
	transport := testTransport(t, func(request *pb.Request) (*pb.Response, error) {
		if request.GetHello() != nil {
			return capabilities(), nil
		}
		if request.GetOpenWindow() != nil {
			return &pb.Response{Body: &pb.Response_WindowOpened{WindowOpened: &pb.WindowOpened{
				Window: &pb.Handle{Id: 1, Generation: 1}, View: &pb.Handle{Id: 2, Generation: 1}, Revision: 1,
			}}}, nil
		}
		commit := request.GetCommitView()
		revisions = append(revisions, commit.Snapshot.Revision)
		if len(revisions) == 1 {
			return nil, context.Canceled
		}
		return &pb.Response{Body: &pb.Response_ViewCommitted{ViewCommitted: &pb.ViewCommitted{View: commit.View, Revision: commit.Snapshot.Revision}}}, nil
	})
	client, err := NewClient(context.Background(), transport)
	if err != nil {
		t.Fatal(err)
	}
	window, err := client.OpenWindow(context.Background(), WindowOptions{Width: 100, Height: 100}, scene.Text(1, "before"))
	if err != nil {
		t.Fatal(err)
	}
	if err := window.Set(context.Background(), scene.Text(1, "racing cancellation")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := window.Set(context.Background(), scene.Text(1, "after")); err != nil {
		t.Fatal(err)
	}
	if len(revisions) != 2 || revisions[0] != 2 || revisions[1] != 3 {
		t.Fatalf("reused revision: %v", revisions)
	}
}

func TestInvalidEventBatchDoesNotAdvanceAcknowledgement(t *testing.T) {
	pulls := 0
	client, err := NewClient(context.Background(), testTransport(t, func(request *pb.Request) (*pb.Response, error) {
		if request.GetHello() != nil {
			return capabilities(), nil
		}
		if request.GetNextEvents().AfterSequence != 0 {
			t.Fatal("acknowledged invalid batch")
		}
		pulls++
		events := []*pb.Event{{Sequence: 1, Kind: &pb.Event_Click{Click: &pb.Click{}}}}
		if pulls == 1 {
			events = append(events, &pb.Event{Sequence: 1, Kind: &pb.Event_Click{Click: &pb.Click{}}})
		}
		return &pb.Response{Body: &pb.Response_Events{Events: &pb.EventBatch{Events: events}}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.NextEvents(context.Background()); err == nil {
		t.Fatal("accepted repeated event sequence")
	}
	if _, err := client.NextEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDispatchAllowsHandlerRemovalAndPreservesCancellation(t *testing.T) {
	client, err := NewClient(context.Background(), testTransport(t, func(*pb.Request) (*pb.Response, error) { return capabilities(), nil }))
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	var handler uint64
	handler = client.OnClick(func(context.Context, *pb.Event) error { calls++; client.RemoveHandler(handler); return nil })
	event := &pb.Event{HandlerId: handler, Kind: &pb.Event_Click{Click: &pb.Click{}}}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Dispatch(canceled, event); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("canceled dispatch invoked callback")
	}
	if err := client.Dispatch(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if err := client.Dispatch(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("removed handler invoked %d times", calls)
	}
	if err := client.Dispatch(context.Background(), nil); err == nil {
		t.Fatal("accepted nil event")
	}
}
