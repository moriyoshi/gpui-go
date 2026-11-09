package scene

import (
	"google.golang.org/protobuf/proto"
)

func (n *Node) Interact(options *Interaction) *Node {
	if options == nil {
		n.element.Interaction = nil
	} else {
		n.element.Interaction = proto.Clone(options).(*Interaction)
	}
	return n
}
func (n *Node) Accessible(label string, metadata *Accessibility) *Node {
	n.element.AccessibilityLabel = proto.String(label)
	if metadata == nil {
		n.element.Accessibility = nil
	} else {
		n.element.Accessibility = proto.Clone(metadata).(*Accessibility)
	}
	return n
}
func (n *Node) Animate(animation *OpacityAnimation) *Node {
	if animation == nil {
		n.element.Animation = nil
	} else {
		n.element.Animation = proto.Clone(animation).(*OpacityAnimation)
	}
	return n
}
func (n *Node) Focusable(tabIndex int32) *Node {
	if n.element.Interaction == nil {
		n.element.Interaction = &Interaction{}
	}
	n.element.Interaction.Focusable = true
	n.element.Interaction.TabIndex = tabIndex
	return n
}

// BindKey attaches a native shortcut to this node's subtree. Chords use GPUI's
// native multi-stroke matching. The node is made focusable automatically.
func (n *Node) BindKey(keystroke string, action *Action, handlerID uint64) *Node {
	return n.BindKeyWithContext(keystroke, "", action, handlerID)
}

// BindKeyWithContext adds a shortcut with a GPUI keymap predicate, evaluated
// against Interaction.KeyContext on the native focus path.
func (n *Node) BindKeyWithContext(keystroke, predicate string, action *Action, handlerID uint64) *Node {
	if action == nil {
		panic("gpui: nil action")
	}
	if n.element.Interaction == nil {
		n.element.Interaction = &Interaction{}
	}
	n.element.Interaction.Focusable = true
	n.element.KeyBindings = append(n.element.KeyBindings, &KeyBinding{Keystroke: keystroke, Context: predicate, Action: proto.Clone(action).(*Action), HandlerId: handlerID, StopPropagation: true})
	return n
}

// OnAction binds a logical action for programmatic/native menu dispatch.
func (n *Node) OnAction(name string, handlerID uint64) *Node {
	return n.BindKey("", &Action{Name: name}, handlerID)
}
