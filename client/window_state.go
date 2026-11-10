package client

import (
	"context"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

// WindowSize is a window's drawable area in logical pixels.
type WindowSize struct{ Width, Height float32 }

// Size returns the window's drawable area, as reported when it opened and
// updated as Dispatch receives resizes. ok is false when the backend lacks the
// window-size capability.
func (w *Window) Size() (size WindowSize, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size, w.sized
}

// ColorScheme returns whether the window is drawn light or dark, as reported
// when it opened and updated as Dispatch receives changes. ok is false when
// the backend lacks the window-color-scheme capability.
func (w *Window) ColorScheme() (scheme scene.ColorScheme, ok bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.scheme, w.scheme != scene.ColorSchemeUnspecified
}

// OnResized registers a callback that Dispatch runs, on the dispatching
// goroutine, after the window's drawable area changes. A live resize is
// coalesced, so the callback sees the latest size rather than every step. The
// returned function removes the callback.
func (w *Window) OnResized(handler func(context.Context, WindowSize) error) (remove func()) {
	return w.observe(func() func() { return w.resized.add(handler) })
}

// OnColorSchemeChanged registers a callback that Dispatch runs, on the
// dispatching goroutine, after the window switches between the light and dark
// color schemes. The returned function removes the callback.
func (w *Window) OnColorSchemeChanged(handler func(context.Context, scene.ColorScheme) error) (remove func()) {
	return w.observe(func() func() { return w.schemes.add(handler) })
}

func (w *Window) observe(add func() func()) func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	remove := add()
	return func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		remove()
	}
}

// observers holds one kind of window callback; the window's mutex guards it.
type observers[T any] struct {
	next     uint64
	handlers map[uint64]func(context.Context, T) error
}

func (o *observers[T]) add(handler func(context.Context, T) error) (remove func()) {
	o.next++
	id := o.next
	if o.handlers == nil {
		o.handlers = map[uint64]func(context.Context, T) error{}
	}
	o.handlers[id] = handler
	return func() { delete(o.handlers, id) }
}

// notify snapshots the callbacks so they run after the window is unlocked.
func (o *observers[T]) notify(value T) func(context.Context) error {
	handlers := make([]func(context.Context, T) error, 0, len(o.handlers))
	for _, handler := range o.handlers {
		handlers = append(handlers, handler)
	}
	return func(ctx context.Context) error {
		for _, handler := range handlers {
			if err := handler(ctx, value); err != nil {
				return err
			}
		}
		return nil
	}
}

// dispatchWindowState applies a resize or color scheme event to the window it
// names, under the window's lock, then runs the callbacks the update returns.
// Events for unknown or closed windows are ignored.
func (c *Client) dispatchWindowState(ctx context.Context, event *Event, update func(*Window) func(context.Context) error) error {
	c.windowsMu.Lock()
	w := c.windows[event.GetWindow().GetId()]
	c.windowsMu.Unlock()
	if w == nil {
		return nil
	}
	w.mu.Lock()
	var run func(context.Context) error
	if !w.closed {
		run = update(w)
	}
	w.mu.Unlock()
	if run == nil {
		return nil
	}
	return run(ctx)
}

// usesConditionalStyles reports whether a snapshot needs the
// conditional-styles capability.
func usesConditionalStyles(element *pb.Element) bool {
	if len(element.GetConditionalStyles()) != 0 {
		return true
	}
	for _, child := range element.GetContainer().GetChildren() {
		if usesConditionalStyles(child) {
			return true
		}
	}
	for _, item := range element.GetUniformList().GetItems() {
		if usesConditionalStyles(item) {
			return true
		}
	}
	return false
}

// checkSnapshot refuses conditional styles that an older backend would drop
// as an unknown field.
func (c *Client) checkSnapshot(root *pb.Element) error {
	if usesConditionalStyles(root) && !c.Supports("conditional-styles") {
		return &ProtocolError{Code: ErrorUnsupported, Message: "this backend does not support conditional styles"}
	}
	return nil
}
