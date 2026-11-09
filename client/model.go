package client

import (
	"context"
	"errors"
	"sync"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

// Model is a session-owned serializable entity, backed by a GPUI Entity on the
// native backend. Values are opaque bytes (for example, an application protobuf).
// Updates use compare-and-swap revisions; canceled writes are never retried.
type Model struct {
	client *Client
	handle *scene.Handle
}

func (c *Client) CreateModel(ctx context.Context, value []byte) (*Model, error) {
	result, err := c.command(ctx, &pb.Command{Kind: &pb.Command_CreateModel{CreateModel: &pb.CreateModel{Value: append([]byte(nil), value...)}}})
	if err != nil {
		return nil, err
	}
	if result.Model == nil || !validHandle(result.Model.Entity) || result.Model.Revision != 1 {
		return nil, errors.New("gpui: invalid model creation result")
	}
	return &Model{client: c, handle: proto.Clone(result.Model.Entity).(*scene.Handle)}, nil
}
func (m *Model) Handle() *scene.Handle { return proto.Clone(m.handle).(*scene.Handle) }
func (m *Model) Read(ctx context.Context) (*ModelState, error) {
	result, err := m.client.command(ctx, &pb.Command{Kind: &pb.Command_ReadModel{ReadModel: &pb.ModelTarget{Entity: m.handle}}})
	if err != nil {
		return nil, err
	}
	return m.validate(result.Model)
}
func (m *Model) Update(ctx context.Context, expectedRevision uint64, value []byte) (*ModelState, error) {
	result, err := m.client.command(ctx, &pb.Command{Kind: &pb.Command_UpdateModel{UpdateModel: &pb.UpdateModel{Entity: m.handle, ExpectedRevision: expectedRevision, Value: append([]byte(nil), value...)}}})
	if err != nil {
		return nil, err
	}
	return m.validate(result.Model)
}
func (m *Model) validate(state *ModelState) (*ModelState, error) {
	if state == nil || !proto.Equal(state.Entity, m.handle) || state.Revision == 0 || len(state.Value) > 65536 {
		return nil, errors.New("gpui: invalid model state")
	}
	return state, nil
}
func (m *Model) Release(ctx context.Context) error {
	_, err := m.client.command(ctx, &pb.Command{Kind: &pb.Command_ReleaseModel{ReleaseModel: &pb.ModelTarget{Entity: m.handle}}})
	return err
}

// Subscription owns an observer callback. Close it when finished. Delivery uses
// the client's ordinary acknowledged event stream and requires event dispatch.
type Subscription struct {
	model   *Model
	handler uint64
	mu      sync.Mutex
	closed  bool
}

func (m *Model) Observe(ctx context.Context, handler func(context.Context, *ModelState) error) (*Subscription, error) {
	if handler == nil {
		return nil, errors.New("gpui: nil model observer")
	}
	id := m.client.OnEvent(func(ctx context.Context, event *Event) error {
		state, err := m.validate(event.GetModelChanged())
		if err != nil {
			return err
		}
		return handler(ctx, state)
	})
	_, err := m.client.command(ctx, &pb.Command{Kind: &pb.Command_ObserveModel{ObserveModel: &pb.ObserveModel{Entity: m.handle, HandlerId: id}}})
	if err != nil {
		m.client.RemoveHandler(id)
		return nil, err
	}
	return &Subscription{model: m, handler: id}, nil
}
func (s *Subscription) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	_, err := s.model.client.command(ctx, &pb.Command{Kind: &pb.Command_UnobserveModel{UnobserveModel: &pb.ObserveModel{Entity: s.model.handle, HandlerId: s.handler}}})
	var protocolErr *ProtocolError
	if err != nil && !(errors.As(err, &protocolErr) && protocolErr.Code == ErrorStaleHandle) {
		return err
	}
	s.model.client.RemoveHandler(s.handler)
	s.closed = true
	return nil
}
