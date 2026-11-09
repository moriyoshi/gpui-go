package client

import (
	"context"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

func TestPublicEventAliasesInteroperateWithWireCallbacks(t *testing.T) {
	c, err := NewClient(context.Background(), testTransport(t, func(*pb.Request) (*pb.Response, error) { return capabilities(), nil }))
	if err != nil {
		t.Fatal(err)
	}
	state := scene.InputState{}
	var oldHandler func(context.Context, *pb.Event) error = func(_ context.Context, event *Event) error {
		state.Accept(event.GetTextChanged())
		return nil
	}
	id := c.OnTextChange(oldHandler)
	data, err := proto.Marshal(&Event{
		HandlerId: id,
		Kind:      &EventTextChanged{TextChanged: &TextChanged{Value: "e\u0301日本語🙂", EditRevision: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(pb.Event)
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatal(err)
	}
	if err := c.Dispatch(context.Background(), decoded); err != nil {
		t.Fatal(err)
	}
	if state.Value != "e\u0301日本語🙂" || state.EditRevision != 1 {
		t.Fatalf("public/wire event alias lost input acknowledgement: %+v", state)
	}
}
