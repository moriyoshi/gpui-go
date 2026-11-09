package gpui_test

import (
	"context"
	"errors"
	"testing"

	gpui "github.com/moriyoshi/gpui-go"
	"github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

type compatibilityTransport func(context.Context, client.Operation, []byte) ([]byte, error)

func (f compatibilityTransport) Call(ctx context.Context, op client.Operation, data []byte) ([]byte, error) {
	return f(ctx, op, data)
}

func TestCompatibilityFacadeInteroperatesWithCanonicalPackages(t *testing.T) {
	ctx := context.Background()
	var installed *pb.Element
	c, err := gpui.NewClient(ctx, compatibilityTransport(func(_ context.Context, _ client.Operation, data []byte) ([]byte, error) {
		request := new(pb.Request)
		if err := proto.Unmarshal(data, request); err != nil {
			return nil, err
		}
		response := &pb.Response{RequestId: request.RequestId}
		switch {
		case request.GetHello() != nil:
			response.Body = &pb.Response_Capabilities{Capabilities: &pb.Capabilities{Major: 1, SessionId: 9, MaxMessageBytes: gpui.MaxMessageBytes, MaxTreeDepth: 32, MaxElements: 10000}}
		case request.GetCommand().GetUploadResource() != nil:
			response.Body = &pb.Response_Result{Result: &pb.CommandResult{Resource: &pb.Handle{Id: 7, Generation: 3}}}
		case request.GetOpenWindow() != nil:
			installed = request.GetOpenWindow().InitialView.Root
			response.Body = &pb.Response_WindowOpened{WindowOpened: &pb.WindowOpened{Window: &pb.Handle{Id: 1, Generation: 1}, View: &pb.Handle{Id: 2, Generation: 1}, Revision: 1}}
		default:
			response.Body = &pb.Response_Error{Error: &pb.ProtocolError{Code: pb.ErrorCode_ERROR_CODE_UNSUPPORTED, Message: "test error"}}
		}
		return proto.Marshal(response)
	}))
	if err != nil {
		t.Fatal(err)
	}
	var canonical *client.Client = c
	resource, err := canonical.UploadResource(ctx, pb.ResourceKind_RESOURCE_KIND_IMAGE, "image/png", []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	var legacyResource *gpui.Resource = resource
	style := &pb.Style{Background: gpui.RGB(0x123456)}
	var root *scene.Node = gpui.Div(1, gpui.Image(2, legacyResource), scene.Text(3, "shared identity")).Style(style)
	style.Background.Red = 0
	window, err := canonical.OpenWindow(ctx, gpui.WindowOptions{Width: 200, Height: 100}, root)
	if err != nil {
		t.Fatal(err)
	}
	var legacyWindow *gpui.Window = window
	if installed.Style.Background.Red == 0 || installed.GetContainer().Children[0].GetImage().Resource.Id != 7 {
		t.Fatalf("compatibility builders lost snapshot ownership or resource identity: %v", installed)
	}
	err = legacyWindow.Close(ctx)
	var failure *gpui.ProtocolError
	if !errors.As(err, &failure) || failure.Code != pb.ErrorCode_ERROR_CODE_UNSUPPORTED {
		t.Fatalf("compatibility error type lost: %v", err)
	}
	if !errors.Is(gpui.ErrNativeUnavailable, host.ErrNativeUnavailable) {
		t.Fatal("compatibility host error has a different identity")
	}
}

func TestCompatibilityRunRejectsNilBeforeStartingHost(t *testing.T) {
	if err := gpui.Run(context.Background(), nil); err == nil {
		t.Fatal("nil application accepted")
	}
}
