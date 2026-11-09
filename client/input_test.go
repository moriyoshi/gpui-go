package client

import (
	"context"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

func TestInputStateAcknowledgesEditsAndRejectsObsoleteEpochs(t *testing.T) {
	state := scene.InputState{Value: "A🙂Z"}
	if !state.Accept(&pb.TextChanged{Value: "Aé🙂Z", EditRevision: 2}) {
		t.Fatal("valid edit rejected")
	}
	if state.Accept(&pb.TextChanged{Value: "old", EditRevision: 1}) {
		t.Fatal("stale edit accepted")
	}
	state.Reset("reset")
	if state.Accept(&pb.TextChanged{Value: "late native edit", EditRevision: 100}) {
		t.Fatal("old epoch overwrote explicit reset")
	}
	if !state.Accept(&pb.TextChanged{Value: "日本語🙂", Epoch: 1, EditRevision: 1, Composing: true}) {
		t.Fatal("new epoch edit rejected")
	}
	if state.Value != "日本語🙂" || state.Epoch != 1 || state.EditRevision != 1 {
		t.Fatalf("incorrect state: %+v", state)
	}
	node := scene.TextInput(1, state, scene.TextInputOptions{Placeholder: "Type here", OnChange: 77})
	props := node.Proto().GetTextInput()
	if props.Value != state.Value || props.OnChange.HandlerId != 77 || props.Epoch != 1 || props.EditRevision != 1 {
		t.Fatalf("input snapshot lost state: %v", props)
	}
}

func TestTextChangeDispatchUsesRegisteredCallback(t *testing.T) {
	client, err := NewClient(context.Background(), testTransport(t, func(*pb.Request) (*pb.Response, error) { return capabilities(), nil }))
	if err != nil {
		t.Fatal(err)
	}
	state := scene.InputState{}
	handler := client.OnTextChange(func(_ context.Context, event *pb.Event) error { state.Accept(event.GetTextChanged()); return nil })
	if err := client.Dispatch(context.Background(), &pb.Event{HandlerId: handler, Kind: &pb.Event_TextChanged{TextChanged: &pb.TextChanged{Value: "hello 🙂", EditRevision: 1}}}); err != nil {
		t.Fatal(err)
	}
	if state.Value != "hello 🙂" {
		t.Fatal("text callback was not dispatched")
	}
}
