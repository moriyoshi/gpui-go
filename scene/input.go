package scene

import pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"

// InputState mirrors a native text field on the application's state-owning
// goroutine. Its zero value is an empty field. Reset is an explicit Go edit;
// Accept acknowledges native edits without disturbing selection or IME state.
type InputState struct {
	Value        string
	Epoch        uint64
	EditRevision uint64
}

func (s *InputState) Reset(value string) { s.Value = value; s.Epoch++; s.EditRevision = 0 }

func (s *InputState) Accept(change *TextChanged) bool {
	if change == nil || change.Epoch != s.Epoch || change.EditRevision <= s.EditRevision {
		return false
	}
	s.Value, s.EditRevision = change.Value, change.EditRevision
	return true
}

type TextInputOptions struct {
	Placeholder string
	OnChange    uint64
	Disabled    bool
	// Masked draws bullets instead of the text and blocks copy and cut. Change
	// events still carry the real value.
	Masked bool
}

// TextInput builds a single-line editable field. Style its container with
// width, padding, colors, and borders as for any other Node. Newlines are
// normalized to spaces for native typing/paste. Input values are limited to
// 64 KiB. Reuse the same element ID to retain focus and selection.
func TextInput(id uint64, value InputState, options TextInputOptions) *Node {
	input := &pb.TextInput{Value: value.Value, Epoch: value.Epoch, EditRevision: value.EditRevision, Placeholder: options.Placeholder, Disabled: options.Disabled, Masked: options.Masked}
	if options.OnChange != 0 {
		input.OnChange = &EventBinding{HandlerId: options.OnChange}
	}
	return &Node{element: &Element{Id: id, Kind: &pb.Element_TextInput{TextInput: input}}}
}
