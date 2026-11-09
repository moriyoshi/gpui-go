package client

import (
	"context"
	"errors"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

func TestNewBuildersCopyMetadataAndDrawing(t *testing.T) {
	draw := &pb.DrawCommand{Kind: &pb.DrawCommand_Path{Path: &pb.DrawPath{Steps: []*pb.PathStep{{Verb: pb.PathVerb_PATH_VERB_MOVE, Point: &pb.Point{X: 3}}}}}}
	canvas := scene.Canvas(1, draw)
	draw.GetPath().Steps[0].Point.X = 99
	if canvas.Proto().GetDrawing().Commands[0].GetPath().Steps[0].Point.X != 3 {
		t.Fatal("drawing aliases caller data")
	}
	interaction := &pb.Interaction{Focusable: true, KeyDown: 42}
	accessibility := &pb.Accessibility{Role: pb.AccessibleRole_ACCESSIBLE_ROLE_BUTTON}
	canvas.Interact(interaction).Accessible("paint", accessibility)
	interaction.KeyDown = 11
	accessibility.Role = pb.AccessibleRole_ACCESSIBLE_ROLE_LINK
	if canvas.Proto().Interaction.KeyDown != 42 || canvas.Proto().Accessibility.Role != pb.AccessibleRole_ACCESSIBLE_ROLE_BUTTON {
		t.Fatal("metadata aliases caller data")
	}
	action := &pb.Action{Name: "save", Payload: []byte{3}}
	node := scene.Text(2, "Save").Interact(&pb.Interaction{MouseDown: 21}).BindKeyWithContext("cmd-k cmd-s", "Editor && !ReadOnly", action, 22)
	action.Payload[0] = 9
	value := node.Proto()
	if !value.Interaction.Focusable || value.Interaction.MouseDown != 21 || value.KeyBindings[0].Action.Payload[0] != 3 || value.KeyBindings[0].Context != "Editor && !ReadOnly" {
		t.Fatal("shortcut lost metadata or retained caller data")
	}
}
func TestInteractionAndModelEventsAreDispatched(t *testing.T) {
	c := &Client{handlers: make(map[uint64]func(context.Context, *pb.Event) error)}
	var count int
	id := c.OnEvent(func(context.Context, *pb.Event) error { count++; return nil })
	events := []*pb.Event{{HandlerId: id, Kind: &pb.Event_KeyDown{KeyDown: &pb.KeyEvent{Key: "x"}}}, {HandlerId: id, Kind: &pb.Event_Action{Action: &pb.Action{Name: "go"}}}, {HandlerId: id, Kind: &pb.Event_ModelChanged{ModelChanged: &pb.ModelState{Revision: 2}}}}
	for _, e := range events {
		if err := c.Dispatch(context.Background(), e); err != nil {
			t.Fatal(err)
		}
	}
	if count != 3 {
		t.Fatalf("dispatched %d of 3 interactions", count)
	}
	c.RemoveHandler(id)
	if err := c.Dispatch(context.Background(), events[0]); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatal("removed handler called")
	}
}
func TestResourceCommandsAndModelObserverOwnership(t *testing.T) {
	ctx := context.Background()
	var observer uint64
	var delivered string
	client, err := NewClient(ctx, testTransport(t, func(request *pb.Request) (*pb.Response, error) {
		if request.GetHello() != nil {
			return capabilities(), nil
		}
		command := request.GetCommand()
		if command == nil {
			t.Fatal("missing command")
		}
		result := &pb.CommandResult{}
		switch kind := command.Kind.(type) {
		case *pb.Command_CreateModel:
			result.Model = &pb.ModelState{Entity: &pb.Handle{Id: 11, Generation: 1}, Revision: 1, Value: kind.CreateModel.Value}
		case *pb.Command_ObserveModel:
			observer = kind.ObserveModel.HandlerId
		case *pb.Command_UnobserveModel:
			if kind.UnobserveModel.HandlerId != observer {
				t.Fatal("wrong observer removed")
			}
		case *pb.Command_UpdateModel:
			return nil, &ProtocolError{Code: pb.ErrorCode_ERROR_CODE_STALE_REVISION, Message: "conflict"}
		case *pb.Command_UploadResource:
			if string(kind.UploadResource.Data) != "asset" {
				t.Fatal("bad asset payload")
			}
			result.Resource = &pb.Handle{Id: 22, Generation: 1}
		case *pb.Command_ReleaseResource:
			if kind.ReleaseResource.Resource.Id != 22 {
				t.Fatal("wrong resource")
			}
		default:
			t.Fatalf("unexpected command: %T", kind)
		}
		return &pb.Response{Body: &pb.Response_Result{Result: result}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	resource, err := client.UploadResource(ctx, pb.ResourceKind_RESOURCE_KIND_IMAGE, "image/png", []byte("asset"))
	if err != nil {
		t.Fatal(err)
	}
	handle := resource.Handle()
	handle.Id = 999
	if resource.Handle().Id != 22 {
		t.Fatal("resource handle aliases caller")
	}
	if err := resource.Release(ctx); err != nil {
		t.Fatal(err)
	}
	model, err := client.CreateModel(ctx, []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	subscription, err := model.Observe(ctx, func(_ context.Context, state *pb.ModelState) error { delivered = string(state.Value); return nil })
	if err != nil {
		t.Fatal(err)
	}
	event := &pb.Event{HandlerId: observer, Kind: &pb.Event_ModelChanged{ModelChanged: &pb.ModelState{Entity: model.Handle(), Revision: 2, Value: []byte("two")}}}
	if err := client.Dispatch(ctx, event); err != nil {
		t.Fatal(err)
	}
	if delivered != "two" {
		t.Fatal("model observer did not receive state")
	}
	_, err = model.Update(ctx, 1, []byte("stale"))
	var conflict *ProtocolError
	if !errors.As(err, &conflict) || conflict.Code != pb.ErrorCode_ERROR_CODE_STALE_REVISION {
		t.Fatalf("conflict: %v", err)
	}
	if err := subscription.Close(ctx); err != nil {
		t.Fatal(err)
	}
	delivered = ""
	if err := client.Dispatch(ctx, event); err != nil {
		t.Fatal(err)
	}
	if delivered != "" {
		t.Fatal("closed observer invoked")
	}
}
func TestPathPromptCallbackIsOneShotAndCanBeDetached(t *testing.T) {
	ctx := context.Background()
	var id uint64
	c, err := NewClient(ctx, testTransport(t, func(r *pb.Request) (*pb.Response, error) {
		if r.GetHello() != nil {
			return capabilities(), nil
		}
		id = r.GetCommand().GetPromptPaths().HandlerId
		return &pb.Response{Body: &pb.Response_Result{Result: &pb.CommandResult{OperationId: id}}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	_, err = c.PromptPaths(ctx, &pb.PathPrompt{Files: true}, func(_ context.Context, result *pb.PathResult) error {
		count++
		if !result.Cancelled {
			t.Fatal("cancellation lost")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	event := &pb.Event{HandlerId: id, Kind: &pb.Event_PathResult{PathResult: &pb.PathResult{OperationId: id, Cancelled: true}}}
	for range 2 {
		if err := c.Dispatch(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	if count != 1 {
		t.Fatal("prompt callback invoked more than once")
	}
	detach, err := c.PromptPaths(ctx, &pb.PathPrompt{Files: true}, func(context.Context, *pb.PathResult) error { t.Fatal("detached callback invoked"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	detach()
	event.HandlerId = id
	event.GetPathResult().OperationId = id
	if err := c.Dispatch(ctx, event); err != nil {
		t.Fatal(err)
	}
}
func TestUniformListCopiesRowSnapshots(t *testing.T) {
	row := scene.Text(2, "before")
	list := scene.UniformList(1, 20, row)
	row.Style(&pb.Style{Opacity: proto.Float32(0.3)})
	if list.Proto().GetUniformList().Items[0].Style != nil {
		t.Fatal("list retained mutable row")
	}
}
