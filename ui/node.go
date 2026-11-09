package ui

import (
	"context"
	"errors"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

// Node keeps its ID across updates. Fragments are transparent ranges and do not
// add native layout containers. Nodes have one parent and one document.
type Node struct {
	document *Document
	scope    *reactive.Scope
	id       uint64
	element  *pb.Element
	parent   *Node
	children []*Node
	disposed bool
}

func (n *Node) ID() uint64             { return n.id }
func (n *Node) Scope() *reactive.Scope { return n.scope }
func (n *Node) Dispose()               { n.scope.Dispose() }
func (n *Node) Name(name string) *Node {
	if n.element == nil || name == "" {
		n.document.record(errors.New("ui: names require a nonempty element name"))
		return n
	}
	if old := n.document.names[name]; old != nil && old != n {
		n.document.record(errors.New("ui: duplicate element name"))
		return n
	}
	n.document.names[name] = n
	return n
}
func (n *Node) SetChildren(children ...*Node) error {
	if n.disposed {
		return reactive.ErrDisposed
	}
	if n.element != nil && n.element.GetContainer() == nil {
		return errors.New("ui: only containers/fragments accept children")
	}
	seen := map[*Node]bool{}
	for _, child := range children {
		if child == nil || child.disposed || child.document != n.document || seen[child] || (child.parent != nil && child.parent != n) {
			return errors.New("ui: invalid, duplicate or already-parented child")
		}
		for p := n; p != nil; p = p.parent {
			if p == child {
				return errors.New("ui: node cycle")
			}
		}
		seen[child] = true
	}
	old := append([]*Node(nil), n.children...)
	for _, child := range old {
		if !seen[child] {
			child.parent = nil
			child.Dispose()
		}
	}
	n.children = append([]*Node(nil), children...)
	for _, child := range n.children {
		child.parent = n
	}
	n.document.changed()
	return nil
}
func (n *Node) Append(children ...*Node) error {
	return n.SetChildren(append(append([]*Node(nil), n.children...), children...)...)
}
func (n *Node) Style(style *scene.Style) *Node {
	if style == nil {
		n.element.Style = nil
	} else {
		n.element.Style = proto.Clone(style).(*scene.Style)
	}
	n.document.changed()
	return n
}
func (n *Node) BindStyle(read func() *scene.Style) *Node {
	n.document.bind(n.scope, func() error {
		value := read()
		if !proto.Equal(n.element.Style, value) {
			n.Style(value)
		}
		return nil
	})
	return n
}
func (n *Node) Accessible(label string, metadata *scene.Accessibility) *Node {
	n.element.AccessibilityLabel = proto.String(label)
	if metadata == nil {
		n.element.Accessibility = nil
	} else {
		n.element.Accessibility = proto.Clone(metadata).(*scene.Accessibility)
	}
	n.document.changed()
	return n
}
func (n *Node) BindAccessibility(label string, read func() *scene.Accessibility) *Node {
	n.document.bind(n.scope, func() error {
		value := read()
		if n.element.AccessibilityLabel == nil || *n.element.AccessibilityLabel != label || !proto.Equal(n.element.Accessibility, value) {
			n.Accessible(label, value)
		}
		return nil
	})
	return n
}
func (n *Node) Focusable(index int32) *Node {
	if n.element.Interaction == nil {
		n.element.Interaction = &pb.Interaction{}
	}
	n.element.Interaction.Focusable = true
	n.element.Interaction.TabIndex = index
	n.document.changed()
	return n
}
func (n *Node) OnClick(handler Handler) *Node {
	d := n.document
	if d.client == nil {
		d.record(errors.New("ui: click handlers require a client"))
		return n
	}
	id := d.client.OnClick(func(ctx context.Context, event *clientlib.Event) error {
		return d.scope.Graph().Batch(func() error { return handler(ctx, event) })
	})
	n.scope.OnCleanup(func() { d.client.RemoveHandler(id) })
	n.element.OnClick = &pb.EventBinding{HandlerId: id}
	d.changed()
	return n
}
