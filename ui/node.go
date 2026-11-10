package ui

import (
	"context"
	"errors"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/tw"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	// The element's styles are the compiled classes refined by the explicit
	// styles, as an inline style refines utility classes.
	style, hover *pb.Style
	classes      *tw.Classes
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
	if n.element != nil && n.element.GetContainer() == nil && n.element.GetUniformList() == nil {
		return errors.New("ui: only containers, uniform lists and fragments accept children")
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
	n.style = clone(style)
	n.restyle()
	return n
}
func (n *Node) BindStyle(read func() *scene.Style) *Node {
	n.document.bind(n.scope, func() error {
		value := read()
		if !proto.Equal(n.style, value) {
			n.Style(value)
		}
		return nil
	})
	return n
}

// Hover sets a partial style refinement GPUI applies while the pointer is over
// the element, without a Go event or another snapshot.
func (n *Node) Hover(style *scene.Style) *Node {
	n.hover = clone(style)
	n.restyle()
	return n
}

// TWClass sets Tailwind-style utility classes, compiled with the document's
// theme; hover: classes become the hover refinement. Style and Hover refine
// the classes field by field. A class that cannot be compiled is a
// construction error reported by Document.Err.
func (n *Node) TWClass(classes string) *Node {
	compiled, err := n.document.compile(classes)
	if err != nil {
		n.document.record(err)
		return n
	}
	n.classes = compiled
	n.restyle()
	return n
}

// BindTWClass recompiles the class list when the accessor's dependencies
// change. A class that cannot be compiled fails the computation.
func (n *Node) BindTWClass(read func() string) *Node {
	last := ""
	bound := false
	n.document.bind(n.scope, func() error {
		classes := read()
		if bound && classes == last {
			return nil
		}
		compiled, err := n.document.compile(classes)
		if err != nil {
			return err
		}
		last, bound = classes, true
		n.classes = compiled
		n.restyle()
		return nil
	})
	return n
}

func clone(style *scene.Style) *pb.Style {
	if style == nil {
		return nil
	}
	return proto.Clone(style).(*scene.Style)
}

// refine overlays explicit fields on a base style. Shadows are one value, so
// explicit shadows replace the base's rather than appending.
func refine(base, explicit *pb.Style) *pb.Style {
	if explicit == nil {
		return base
	}
	if base == nil {
		return clone(explicit)
	}
	if len(explicit.Shadows) != 0 {
		base.Shadows = nil
	}
	proto.Merge(base, explicit)
	return base
}

// without clears the fields that explicit sets, per side for edges and whole
// otherwise, returning nil when nothing is left.
func without(style, explicit *pb.Style) *pb.Style {
	if style == nil || explicit == nil {
		return style
	}
	var clearSet func(target, set protoreflect.Message)
	clearSet = func(target, set protoreflect.Message) {
		set.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
			if !field.IsList() && field.Message() != nil && field.Message().FullName() == "gpui.v1.Edges" && target.Has(field) {
				clearSet(target.Mutable(field).Message(), value.Message())
				if edges := target.Get(field).Message(); !edges.IsValid() || proto.Size(edges.Interface()) == 0 {
					target.Clear(field)
				}
			} else {
				target.Clear(field)
			}
			return true
		})
	}
	clearSet(style.ProtoReflect(), explicit.ProtoReflect())
	if proto.Size(style) == 0 {
		return nil
	}
	return style
}

func (n *Node) restyle() {
	var style, hover *pb.Style
	if n.classes != nil {
		style, hover = n.classes.Style(), n.classes.Hover()
	}
	n.element.Style, n.element.HoverStyle = refine(style, n.style), refine(hover, n.hover)
	n.element.ConditionalStyles = nil
	if n.classes != nil {
		for _, condition := range n.classes.Conditions() {
			// Explicit styles win over breakpoint and dark: utilities too, as
			// an inline style wins over a media query.
			condition.Style, condition.HoverStyle = without(condition.Style, n.style), without(condition.HoverStyle, n.hover)
			if condition.Style != nil || condition.HoverStyle != nil {
				n.element.ConditionalStyles = append(n.element.ConditionalStyles, condition)
			}
		}
	}
	n.document.changed()
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
