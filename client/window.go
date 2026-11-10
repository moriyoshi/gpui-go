package client

import (
	"context"
	"errors"
	"sync"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

type WindowOptions struct {
	Title         string
	Width, Height float32
}

// Window owns its root view. Its references become invalid when it closes.
type Window struct {
	client       *Client
	window, view *scene.Handle
	mu           sync.Mutex
	revision     uint64
	closed       bool
	size         WindowSize
	sized        bool
	scheme       scene.ColorScheme
	resized      observers[WindowSize]
	schemes      observers[scene.ColorScheme]
}

func (c *Client) OpenWindow(ctx context.Context, options WindowOptions, root *scene.Node) (*Window, error) {
	if root == nil {
		return nil, errors.New("gpui: missing root")
	}
	snapshot := root.Proto()
	if err := c.checkSnapshot(snapshot); err != nil {
		return nil, err
	}
	response, err := c.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_OpenWindow{OpenWindow: &pb.OpenWindow{
		Options:     &pb.WindowOptions{Title: options.Title, Size: &scene.Size{Width: options.Width, Height: options.Height}},
		InitialView: &pb.ViewSnapshot{Revision: 1, Root: snapshot},
	}}})
	if err != nil {
		return nil, err
	}
	opened := response.GetWindowOpened()
	if opened == nil || !validHandle(opened.Window) || !validHandle(opened.View) || opened.Revision != 1 {
		return nil, errors.New("gpui: invalid window-opened response")
	}
	w := &Window{client: c, window: opened.Window, view: opened.View, revision: opened.Revision}
	if viewport := opened.GetViewport(); viewport != nil {
		w.size, w.sized = WindowSize{viewport.Width, viewport.Height}, true
	}
	w.scheme = opened.GetColorScheme()
	c.windowsMu.Lock()
	c.windows[opened.Window.Id] = w
	c.windowsMu.Unlock()
	return w, nil
}

func (c *Client) forget(window *scene.Handle) {
	c.windowsMu.Lock()
	delete(c.windows, window.GetId())
	c.windowsMu.Unlock()
}

func validHandle(handle *scene.Handle) bool {
	return handle != nil && handle.Id != 0 && handle.Generation != 0
}

func (w *Window) Handle() *scene.Handle     { return proto.Clone(w.window).(*scene.Handle) }
func (w *Window) ViewHandle() *scene.Handle { return proto.Clone(w.view).(*scene.Handle) }

// EmbedTarget identifies this window's view to a host extension that creates
// embedded views in it.
func (w *Window) EmbedTarget() (uint64, uint32) { return w.view.Id, w.view.Generation }

// Set replaces a complete view. A successful response means installed, not
// necessarily painted. Revisions are consumed even on cancellation because an
// operation may commit immediately before cancellation wins the response race.
func (w *Window) Set(ctx context.Context, root *scene.Node) error {
	if root == nil {
		return errors.New("gpui: missing root")
	}
	snapshot := root.Proto()
	if err := w.client.checkSnapshot(snapshot); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return &ProtocolError{Code: ErrorStaleHandle, Message: "window is closed"}
	}
	w.revision++
	response, err := w.client.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_CommitView{CommitView: &pb.CommitView{
		View: w.view, Snapshot: &pb.ViewSnapshot{Revision: w.revision, Root: snapshot},
	}}})
	if err != nil {
		return err
	}
	committed := response.GetViewCommitted()
	if committed == nil || !proto.Equal(committed.View, w.view) || committed.Revision != w.revision {
		return errors.New("gpui: invalid view-committed response")
	}
	return nil
}

func (w *Window) Close(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	response, err := w.client.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_CloseWindow{CloseWindow: &pb.CloseWindow{Window: w.window}}})
	if err != nil {
		return err
	}
	if response.GetAck() == nil {
		return errors.New("gpui: invalid close response")
	}
	w.closed = true
	w.client.forget(w.window)
	return nil
}

func (c *Client) Quit(ctx context.Context) error {
	response, err := c.call(ctx, OperationRequest, &pb.Request{Body: &pb.Request_Quit{Quit: &pb.Quit{}}})
	if err != nil {
		return err
	}
	if response.GetAck() == nil {
		return errors.New("gpui: invalid quit response")
	}
	return nil
}

func (w *Window) target(element uint64) *scene.FocusTarget {
	return &scene.FocusTarget{View: w.ViewHandle(), ElementId: element}
}
func (w *Window) Focus(ctx context.Context, element uint64) error {
	_, err := w.client.command(ctx, &pb.Command{Kind: &pb.Command_Focus{Focus: w.target(element)}})
	return err
}
func (w *Window) ScrollTo(ctx context.Context, element uint64, item uint32) error {
	_, err := w.client.command(ctx, &pb.Command{Kind: &pb.Command_ScrollTo{ScrollTo: &pb.ScrollTo{Target: w.target(element), Item: item}}})
	return err
}
func (w *Window) DispatchAction(ctx context.Context, element uint64, name string, payload []byte) error {
	_, err := w.client.command(ctx, &pb.Command{Kind: &pb.Command_DispatchAction{DispatchAction: &pb.DispatchAction{Target: w.target(element), Action: &pb.Action{Name: name, Payload: append([]byte(nil), payload...)}}}})
	return err
}
func (w *Window) Command(ctx context.Context, operation WindowOperation, title string, size *scene.Size) error {
	_, err := w.client.command(ctx, &pb.Command{Kind: &pb.Command_Window{Window: &pb.WindowCommand{Window: w.Handle(), Operation: operation, Title: title, Size: size}}})
	return err
}
func (w *Window) SetTitle(ctx context.Context, title string) error {
	return w.Command(ctx, WindowSetTitle, title, nil)
}
func (w *Window) Resize(ctx context.Context, width, height float32) error {
	return w.Command(ctx, WindowResize, "", &scene.Size{Width: width, Height: height})
}
func (w *Window) Activate(ctx context.Context) error {
	return w.Command(ctx, WindowActivate, "", nil)
}
func (w *Window) Minimize(ctx context.Context) error {
	return w.Command(ctx, WindowMinimize, "", nil)
}
func (w *Window) Zoom(ctx context.Context) error {
	return w.Command(ctx, WindowZoom, "", nil)
}
func (w *Window) ToggleFullscreen(ctx context.Context) error {
	return w.Command(ctx, WindowFullscreen, "", nil)
}

// ActionTarget returns the serializable target used in native menu items.
func (w *Window) ActionTarget(element uint64) *scene.FocusTarget { return w.target(element) }
