package ui

import (
	"context"
	"errors"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
)

type InputOptions struct {
	Placeholder string
	Disabled    func() bool
	Masked      func() bool
}

// Input mirrors a signal through the bridge's epoch/edit-revision protocol.
// Native edits echo their acknowledgement; authoritative signal replacements
// advance the epoch. The persistent node/entity is never replaced per keystroke.
func (d *Document) Input(owner *reactive.Scope, value *reactive.Signal[string], options InputOptions) *Node {
	state := scene.InputState{Value: value.Peek()}
	n := d.create(owner, &pb.Element{Kind: &pb.Element_TextInput{TextInput: &pb.TextInput{Value: state.Value, Placeholder: options.Placeholder}}})
	copyState := func() {
		input := n.element.GetTextInput()
		input.Value = state.Value
		input.Epoch = state.Epoch
		input.EditRevision = state.EditRevision
		d.changed()
	}
	if d.client == nil {
		d.record(errors.New("ui: controlled input requires a client"))
		return n
	}
	id := d.client.OnTextChange(func(_ context.Context, event *clientlib.Event) error {
		return owner.Graph().Batch(func() error {
			if state.Accept(event.GetTextChanged()) {
				copyState()
				return value.Set(state.Value)
			}
			return nil
		})
	})
	n.element.GetTextInput().OnChange = &pb.EventBinding{HandlerId: id}
	n.scope.OnCleanup(func() { d.client.RemoveHandler(id) })
	d.bind(n.scope, func() error {
		next := value.Get()
		if next != state.Value {
			state.Reset(next)
			copyState()
		}
		if options.Disabled != nil {
			disabled := options.Disabled()
			if n.element.GetTextInput().Disabled != disabled {
				n.element.GetTextInput().Disabled = disabled
				d.changed()
			}
		}
		if options.Masked != nil {
			masked := options.Masked()
			if n.element.GetTextInput().Masked != masked {
				n.element.GetTextInput().Masked = masked
				d.changed()
			}
		}
		return nil
	})
	return n
}
