package client

import (
	"context"
	"errors"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

// OnClick registers a handler without exposing a Go pointer to Rust. IDs are
// never reused. Call RemoveHandler when the application no longer needs one.
func (c *Client) OnClick(handler func(context.Context, *Event) error) uint64 {
	return c.registerHandler(handler)
}

// OnTextChange registers a callback for native text edits, including IME
// composition. Use InputState.Accept to ignore obsolete edits after a reset.
func (c *Client) OnTextChange(handler func(context.Context, *Event) error) uint64 {
	return c.registerHandler(handler)
}

func (c *Client) registerHandler(handler func(context.Context, *Event) error) uint64 {
	if handler == nil {
		panic("gpui: nil event handler")
	}
	id := c.handlerID.Add(1)
	c.handlersMu.Lock()
	c.handlers[id] = handler
	c.handlersMu.Unlock()
	return id
}

func (c *Client) RemoveHandler(id uint64) {
	c.handlersMu.Lock()
	delete(c.handlers, id)
	c.handlersMu.Unlock()
}

// NextEvents acknowledges the preceding received batch. Cancellation can be
// retried: Rust retains unacknowledged events. Use one consumer per Client.
func (c *Client) NextEvents(ctx context.Context) ([]*Event, error) {
	c.eventsMu.Lock()
	defer c.eventsMu.Unlock()
	response, err := c.call(ctx, OperationEvents, &pb.Request{Body: &pb.Request_NextEvents{NextEvents: &pb.NextEvents{MaxEvents: 128, AfterSequence: c.cursor}}})
	if err != nil {
		return nil, err
	}
	batch := response.GetEvents()
	if batch == nil || len(batch.Events) == 0 || len(batch.Events) > 128 {
		return nil, errors.New("gpui: invalid event batch")
	}
	cursor := c.cursor
	for _, event := range batch.Events {
		if event == nil || event.Sequence <= cursor || event.Kind == nil {
			return nil, errors.New("gpui: invalid event sequence or kind")
		}
		cursor = event.Sequence
	}
	c.cursor = cursor
	return batch.Events, nil
}

// DispatchEvents runs handlers sequentially until canceled or a handler fails.
// Window lifecycle events can be consumed directly with NextEvents instead.
func (c *Client) DispatchEvents(ctx context.Context) error {
	for {
		events, err := c.NextEvents(ctx)
		if err != nil {
			return err
		}
		for _, event := range events {
			if err := c.Dispatch(ctx, event); err != nil {
				return err
			}
		}
	}
}

// Dispatch invokes the registered callback for a received event. Use it with
// NextEvents to combine UI input and background results on one state-owning
// goroutine. It does not commit or acknowledge an event by itself.
func (c *Client) Dispatch(ctx context.Context, event *Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event == nil {
		return errors.New("gpui: nil event")
	}
	switch event.Kind.(type) {
	case *pb.Event_WindowResized:
		viewport := event.GetWindowResized().GetViewport()
		return c.dispatchWindowState(ctx, event, func(w *Window) func(context.Context) error {
			size := WindowSize{viewport.GetWidth(), viewport.GetHeight()}
			if w.sized && w.size == size {
				return nil
			}
			w.size, w.sized = size, true
			return w.resized.notify(size)
		})
	case *pb.Event_ColorSchemeChanged:
		scheme := event.GetColorSchemeChanged().GetColorScheme()
		return c.dispatchWindowState(ctx, event, func(w *Window) func(context.Context) error {
			if scheme == scene.ColorSchemeUnspecified || w.scheme == scheme {
				return nil
			}
			w.scheme = scheme
			return w.schemes.notify(scheme)
		})
	case *pb.Event_WindowClosed:
		c.forget(event.GetWindow())
	}
	if event.HandlerId == 0 {
		return nil
	}
	c.handlersMu.RLock()
	handler := c.handlers[event.HandlerId]
	c.handlersMu.RUnlock()
	if handler == nil {
		return nil
	}
	return handler(ctx, event)
}
