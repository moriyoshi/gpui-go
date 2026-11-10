//go:build (gpui || gpui_prebuilt) && gpui_headless

package host

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

func TestSableCounterRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	err := Run(ctx, func(ctx context.Context, client *clientlib.Client) error {
		var window *clientlib.Window
		var handler uint64
		count := 0
		dispatched := errors.New("click dispatched")
		handler = client.OnClick(func(ctx context.Context, event *pb.Event) error {
			if event.Revision != 1 || event.HandlerId != handler {
				t.Errorf("click metadata: %v", event)
			}
			count++
			if err := window.Set(ctx, scene.Text(1, "Count: 1").OnClick(handler)); err != nil {
				return err
			}
			return dispatched
		})
		defer client.RemoveHandler(handler)
		var err error
		window, err = client.OpenWindow(ctx, clientlib.WindowOptions{Title: "Go → Sable → Rust", Width: 560, Height: 420}, scene.Text(1, "Count: 0").OnClick(handler).
			Style(&pb.Style{FontWeight: proto.Uint32(600), OverflowY: pb.Overflow_OVERFLOW_SCROLL.Enum(), Cursor: pb.Cursor_CURSOR_POINTER.Enum()}).
			Hover(&pb.Style{Background: scene.RGB(0x5b5ce2)}))
		if err != nil {
			return err
		}
		// A real Sable task waits in Rust, gets canceled by Go, and releases
		// the sole event consumer before another pull is admitted.
		pullctx, stop := context.WithTimeout(ctx, 20*time.Millisecond)
		_, err = client.NextEvents(pullctx)
		stop()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("pending event pull: %v", err)
		}
		view := window.ViewHandle()
		if status := headlessClick(view.Id, view.Generation, 1, 1); status != 0 {
			t.Errorf("inject click: %d", status)
		}
		if err := client.DispatchEvents(ctx); !errors.Is(err, dispatched) {
			t.Errorf("click dispatcher: %v", err)
			return err
		}
		if count != 1 {
			t.Errorf("Go callback count: %d", count)
		}
		events, err := client.NextEvents(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 || events[0].Revision != 2 || events[0].GetViewUpdated() == nil {
			t.Errorf("update: %v", events)
		}
		// Invalid generations must fail in Rust, not just in the Go wrapper.
		_, err = callRaw(ctx, client, &pb.Request{Body: &pb.Request_CommitView{CommitView: &pb.CommitView{
			View: &pb.Handle{Id: view.Id, Generation: view.Generation + 1}, Snapshot: &pb.ViewSnapshot{Revision: 3, Root: scene.Text(1, "invalid").Proto()},
		}}})
		var protocolErr *clientlib.ProtocolError
		if !errors.As(err, &protocolErr) || protocolErr.Code != pb.ErrorCode_ERROR_CODE_STALE_HANDLE {
			t.Errorf("stale handle: %v", err)
		}
		if err := window.Close(ctx); err != nil {
			return err
		}
		events, err = client.NextEvents(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 || events[0].GetWindowClosed() == nil {
			t.Errorf("close: %v", events)
		}
		if err := exerciseTextInput(ctx, client); err != nil {
			return err
		}
		if err := exerciseCoreFeatures(ctx, client); err != nil {
			return err
		}
		if err := exerciseWindowState(ctx, client); err != nil {
			return err
		}
		// Quit also has to wake a Rust-side event pull, so Run's teardown can
		// wait for every Sable call before freeing the runtime.
		// Run cancels the application context as soon as the platform exits.
		// Use an independent deadline here to observe Rust's Closed response
		// rather than racing that equally valid Go context cancellation.
		shutdownctx, stopShutdown := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer stopShutdown()
		pending := make(chan error, 1)
		go func() { _, err := client.NextEvents(shutdownctx); pending <- err }()
		if err := client.Quit(shutdownctx); err != nil {
			return err
		}
		select {
		case err := <-pending:
			if !errors.As(err, &protocolErr) || protocolErr.Code != pb.ErrorCode_ERROR_CODE_CLOSED {
				t.Errorf("shutdown pull: %v", err)
			}
		case <-shutdownctx.Done():
			return shutdownctx.Err()
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The headless backend has no real window: it reports the requested size,
// applies resize commands at once, and accepts conditional styles.
func exerciseWindowState(ctx context.Context, client *clientlib.Client) error {
	for _, feature := range []string{"window-size", "window-color-scheme", "conditional-styles"} {
		if !client.Supports(feature) {
			return fmt.Errorf("backend lacks %s", feature)
		}
	}
	root := scene.Div(1, scene.Text(2, "responsive")).When(&scene.ConditionalStyle{
		MinWidth: scene.Ptr[float32](768), Style: &pb.Style{Padding: scene.Padding(16)},
	}, &scene.ConditionalStyle{
		ColorScheme: scene.ColorSchemeDark, HoverStyle: &pb.Style{Opacity: scene.Ptr[float32](0.5)},
	})
	window, err := client.OpenWindow(ctx, clientlib.WindowOptions{Title: "Window state", Width: 500, Height: 300}, root)
	if err != nil {
		return err
	}
	if got, ok := window.Size(); !ok || got != (clientlib.WindowSize{Width: 500, Height: 300}) {
		return fmt.Errorf("opened size: %v %v", got, ok)
	}
	if got, ok := window.ColorScheme(); !ok || got != scene.ColorSchemeLight {
		return fmt.Errorf("opened color scheme: %v %v", got, ok)
	}
	var seen []clientlib.WindowSize
	window.OnResized(func(_ context.Context, size clientlib.WindowSize) error {
		seen = append(seen, size)
		return nil
	})
	schemes := 0
	window.OnColorSchemeChanged(func(context.Context, scene.ColorScheme) error {
		schemes++
		return nil
	})
	if err := window.Resize(ctx, 900, 400); err != nil {
		return err
	}
	events, err := client.NextEvents(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := client.Dispatch(ctx, event); err != nil {
			return err
		}
	}
	want := clientlib.WindowSize{Width: 900, Height: 400}
	if got, _ := window.Size(); got != want || len(seen) != 1 || seen[0] != want || schemes != 0 {
		return fmt.Errorf("resized to %v, resize callbacks %v, color scheme callbacks %d, events %v", got, seen, schemes, events)
	}
	// The Rust validator, not only the Go guard, checks conditional styles.
	invalid := scene.Text(1, "x").When(&scene.ConditionalStyle{
		MinWidth: scene.Ptr[float32](900), MaxWidth: scene.Ptr[float32](100), Style: &pb.Style{Opacity: scene.Ptr[float32](1)},
	})
	var protocolErr *clientlib.ProtocolError
	if err := window.Set(ctx, invalid); !errors.As(err, &protocolErr) || protocolErr.Code != pb.ErrorCode_ERROR_CODE_INVALID_ARGUMENT {
		return fmt.Errorf("inverted width range: %v", err)
	}
	if err := window.Close(ctx); err != nil {
		return err
	}
	_, err = client.NextEvents(ctx)
	return err
}

func exerciseTextInput(ctx context.Context, client *clientlib.Client) error {
	state := scene.InputState{Value: "A🙂Z"}
	var window *clientlib.Window
	var handler uint64
	render := func() *scene.Node {
		return scene.TextInput(10, state, scene.TextInputOptions{Placeholder: "Type Unicode", OnChange: handler}).Style(&pb.Style{Width: scene.Pixels(300)})
	}
	handler = client.OnTextChange(func(ctx context.Context, event *pb.Event) error {
		if !state.Accept(event.GetTextChanged()) {
			return errors.New("valid native edit was rejected")
		}
		return window.Set(ctx, render())
	})
	defer client.RemoveHandler(handler)
	var err error
	window, err = client.OpenWindow(ctx, clientlib.WindowOptions{Title: "Text input round trip", Width: 500, Height: 200}, render())
	if err != nil {
		return err
	}
	view := window.ViewHandle()
	inject := func(epoch, revision uint64, value string) error {
		if status := headlessTextChange(view.Id, view.Generation, 10, epoch, revision, value); status != 0 {
			return fmt.Errorf("inject text change: %d", status)
		}
		return nil
	}
	commit := func(revision uint64) error {
		events, err := client.NextEvents(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 || events[0].GetViewUpdated() == nil || events[0].Revision != revision {
			return fmt.Errorf("unexpected input commit: %v", events)
		}
		return nil
	}
	edit := func(value string) error {
		events, err := client.NextEvents(ctx)
		if err != nil {
			return err
		}
		if len(events) != 1 || events[0].GetTextChanged().GetValue() != value || events[0].HandlerId != handler {
			return fmt.Errorf("unexpected text event: %v", events)
		}
		return client.Dispatch(ctx, events[0])
	}
	if err := inject(0, 1, "Aé🙂Z"); err != nil {
		return err
	}
	if err := edit("Aé🙂Z"); err != nil {
		return err
	}
	if err := commit(2); err != nil {
		return err
	}
	state.Reset("reset")
	if err := window.Set(ctx, render()); err != nil {
		return err
	}
	if err := commit(3); err != nil {
		return err
	}
	// An edit captured before the explicit reset must never reach the Go state.
	if err := inject(0, 99, "obsolete"); err != nil {
		return err
	}
	if err := inject(1, 1, "日本語🙂"); err != nil {
		return err
	}
	if err := edit("日本語🙂"); err != nil {
		return err
	}
	if state.Value != "日本語🙂" || state.Epoch != 1 {
		return errors.New("text edit/reset state mismatch")
	}
	if err := commit(4); err != nil {
		return err
	}
	if err := window.Close(ctx); err != nil {
		return err
	}
	events, err := client.NextEvents(ctx)
	if err != nil {
		return err
	}
	if len(events) != 1 || events[0].GetWindowClosed() == nil {
		return fmt.Errorf("input close: %v", events)
	}
	return nil
}

func exerciseCoreFeatures(ctx context.Context, client *clientlib.Client) error {
	model, err := client.CreateModel(ctx, []byte("first"))
	if err != nil {
		return err
	}
	received := ""
	subscription, err := model.Observe(ctx, func(_ context.Context, state *pb.ModelState) error { received = string(state.Value); return nil })
	if err != nil {
		return err
	}
	next, err := model.Update(ctx, 1, []byte("日本語"))
	if err != nil {
		return err
	}
	if next.Revision != 2 {
		return errors.New("model revision did not advance")
	}
	events, err := client.NextEvents(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := client.Dispatch(ctx, event); err != nil {
			return err
		}
	}
	if received != "日本語" {
		return errors.New("model observation failed through Sable")
	}
	if _, err := model.Update(ctx, 1, []byte("conflict")); err == nil {
		return errors.New("conflicting model write succeeded")
	}
	if err := subscription.Close(ctx); err != nil {
		return err
	}
	if err := model.Release(ctx); err != nil {
		return err
	}
	resource, err := client.UploadResource(ctx, pb.ResourceKind_RESOURCE_KIND_SVG, "image/svg+xml", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if err != nil {
		return err
	}
	action := client.OnEvent(func(context.Context, *pb.Event) error { received = "action"; return nil })
	defer client.RemoveHandler(action)
	root := scene.Div(1, scene.Image(2, resource.Handle()), scene.UniformList(3, 20, scene.Text(4, "row"))).Focusable(0).BindKey("cmd-k", &pb.Action{Name: "activate"}, action).Accessible("Integration root", &pb.Accessibility{Role: pb.AccessibleRole_ACCESSIBLE_ROLE_GENERIC})
	window, err := client.OpenWindow(ctx, clientlib.WindowOptions{Title: "Core features", Width: 200, Height: 200}, root)
	if err != nil {
		return err
	}
	if err := resource.Release(ctx); err == nil {
		return errors.New("released a live resource")
	}
	if err := window.Focus(ctx, 1); err != nil {
		return err
	}
	if err := window.ScrollTo(ctx, 3, 0); err != nil {
		return err
	}
	if err := window.DispatchAction(ctx, 1, "activate", nil); err != nil {
		return err
	}
	events, err = client.NextEvents(ctx)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := client.Dispatch(ctx, event); err != nil {
			return err
		}
	}
	if received != "action" {
		return errors.New("logical action failed through Sable")
	}
	if err := window.Close(ctx); err != nil {
		return err
	}
	if _, err := client.NextEvents(ctx); err != nil {
		return err
	}
	return resource.Release(ctx)
}

// callRaw bypasses the client validation to observe the actual Rust stale-handle
// rejection. No additional production client API is exposed for this test.
func callRaw(ctx context.Context, c *clientlib.Client, request *pb.Request) (*pb.Response, error) {
	request.SessionId = c.Capabilities().SessionId
	request.RequestId = 1000000
	data, err := proto.Marshal(request)
	if err != nil {
		return nil, err
	}
	data, err = new(sableTransport).Call(ctx, clientlib.OperationRequest, data)
	if err != nil {
		return nil, err
	}
	response := new(pb.Response)
	if err := proto.Unmarshal(data, response); err != nil {
		return nil, err
	}
	if failure := response.GetError(); failure != nil {
		return nil, &clientlib.ProtocolError{Code: failure.Code, Message: failure.Message}
	}
	return response, nil
}
