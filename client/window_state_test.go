package client

import (
	"context"
	"errors"
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

func stateClient(t *testing.T, features []string, opened *pb.WindowOpened) (*Client, *int) {
	t.Helper()
	sent := 0
	client, err := NewClient(context.Background(), testTransport(t, func(request *pb.Request) (*pb.Response, error) {
		switch {
		case request.GetHello() != nil:
			response := capabilities()
			response.GetCapabilities().Features = features
			return response, nil
		case request.GetOpenWindow() != nil:
			sent++
			response := &pb.WindowOpened{Window: &pb.Handle{Id: 1, Generation: 1}, View: &pb.Handle{Id: 2, Generation: 1}, Revision: 1}
			if opened != nil {
				response.Viewport, response.ColorScheme = opened.Viewport, opened.ColorScheme
			}
			return &pb.Response{Body: &pb.Response_WindowOpened{WindowOpened: response}}, nil
		case request.GetCommitView() != nil:
			sent++
			commit := request.GetCommitView()
			return &pb.Response{Body: &pb.Response_ViewCommitted{ViewCommitted: &pb.ViewCommitted{View: commit.View, Revision: commit.Snapshot.Revision}}}, nil
		}
		return &pb.Response{Body: &pb.Response_Ack{Ack: &pb.Ack{}}}, nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	return client, &sent
}

func resizedEvent(window uint64, width, height float32) *Event {
	return &Event{Window: &pb.Handle{Id: window, Generation: 1}, Kind: &pb.Event_WindowResized{
		WindowResized: &pb.WindowResized{Viewport: &pb.Size{Width: width, Height: height}},
	}}
}

func schemeEvent(window uint64, scheme scene.ColorScheme) *Event {
	return &Event{Window: &pb.Handle{Id: window, Generation: 1}, Kind: &pb.Event_ColorSchemeChanged{
		ColorSchemeChanged: &pb.ColorSchemeChanged{ColorScheme: scheme},
	}}
}

func TestSizeAndColorSchemeHaveSeparateCallbacks(t *testing.T) {
	ctx := context.Background()
	client, _ := stateClient(t, []string{"window-size", "window-color-scheme"}, &pb.WindowOpened{
		Viewport: &pb.Size{Width: 800, Height: 600}, ColorScheme: scene.ColorSchemeLight,
	})
	window, err := client.OpenWindow(ctx, WindowOptions{Title: "w", Width: 800, Height: 600}, scene.Text(1, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if size, ok := window.Size(); !ok || size != (WindowSize{800, 600}) {
		t.Fatalf("opened size = %v %v", size, ok)
	}
	if scheme, ok := window.ColorScheme(); !ok || scheme != scene.ColorSchemeLight {
		t.Fatalf("opened color scheme = %v %v", scheme, ok)
	}
	var sizes []WindowSize
	var schemes []scene.ColorScheme
	removeSize := window.OnResized(func(_ context.Context, size WindowSize) error {
		sizes = append(sizes, size)
		return nil
	})
	window.OnColorSchemeChanged(func(_ context.Context, scheme scene.ColorScheme) error {
		schemes = append(schemes, scheme)
		return nil
	})
	for _, event := range []*Event{
		resizedEvent(1, 800, 600), // unchanged
		resizedEvent(1, 640, 480),
		schemeEvent(1, scene.ColorSchemeLight), // unchanged
		schemeEvent(1, scene.ColorSchemeDark),
		schemeEvent(1, scene.ColorSchemeUnspecified), // invalid
		resizedEvent(7, 1, 1),                        // another window
	} {
		if err := client.Dispatch(ctx, event); err != nil {
			t.Fatal(err)
		}
	}
	// A resize does not run color scheme callbacks, and the reverse.
	if len(sizes) != 1 || sizes[0] != (WindowSize{640, 480}) || len(schemes) != 1 || schemes[0] != scene.ColorSchemeDark {
		t.Fatalf("callbacks: sizes %v, schemes %v", sizes, schemes)
	}
	if scheme, _ := window.ColorScheme(); scheme != scene.ColorSchemeDark {
		t.Errorf("color scheme = %v", scheme)
	}
	removeSize()
	if err := client.Dispatch(ctx, resizedEvent(1, 320, 240)); err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 1 {
		t.Error("a removed callback ran")
	}
	if size, _ := window.Size(); size != (WindowSize{320, 240}) {
		t.Errorf("size after removal = %v", size)
	}
	failing := errors.New("observer failed")
	window.OnColorSchemeChanged(func(context.Context, scene.ColorScheme) error { return failing })
	if err := client.Dispatch(ctx, schemeEvent(1, scene.ColorSchemeLight)); !errors.Is(err, failing) {
		t.Errorf("observer error = %v", err)
	}
	if err := window.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err := client.Dispatch(ctx, resizedEvent(1, 50, 50)); err != nil {
		t.Fatal(err)
	}
	if size, _ := window.Size(); size.Width != 320 {
		t.Error("a closed window recorded a resize")
	}
}

func TestOlderBackendsReportNoWindowStateAndRefuseConditionalStyles(t *testing.T) {
	ctx := context.Background()
	client, sent := stateClient(t, nil, nil)
	conditional := scene.Div(1, scene.Div(2, scene.Text(3, "x")).When(&scene.ConditionalStyle{
		MinWidth: scene.Ptr[float32](768), Style: &scene.Style{Opacity: scene.Ptr[float32](0.5)},
	}))
	_, err := client.OpenWindow(ctx, WindowOptions{Title: "w", Width: 10, Height: 10}, conditional)
	var protocolErr *ProtocolError
	if !errors.As(err, &protocolErr) || protocolErr.Code != ErrorUnsupported || *sent != 0 {
		t.Fatalf("conditional styles on an older backend = %v (sent %d)", err, *sent)
	}
	window, err := client.OpenWindow(ctx, WindowOptions{Title: "w", Width: 10, Height: 10}, scene.Text(1, "x"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := window.Size(); ok {
		t.Error("an older backend reported a size")
	}
	if _, ok := window.ColorScheme(); ok {
		t.Error("an older backend reported a color scheme")
	}
	list := scene.UniformList(1, 20, scene.Text(2, "row").When(&scene.ConditionalStyle{
		ColorScheme: scene.ColorSchemeDark, Style: &scene.Style{Opacity: scene.Ptr[float32](0.5)},
	}))
	if err := window.Set(ctx, list); !errors.As(err, &protocolErr) || *sent != 1 {
		t.Errorf("conditional list row on an older backend = %v (sent %d)", err, *sent)
	}

	capable, sent := stateClient(t, []string{"conditional-styles"}, nil)
	if _, err := capable.OpenWindow(ctx, WindowOptions{Title: "w", Width: 10, Height: 10}, conditional); err != nil || *sent != 1 {
		t.Errorf("capable backend = %v (sent %d)", err, *sent)
	}
}
