package scene

import (
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

// Node is a builder for a protobuf element snapshot. Builders are mutable and
// should remain on their owner's goroutine. Proto returns an independent copy.
type Node struct{ element *Element }

func Div(id uint64, children ...*Node) *Node {
	n := &Node{element: &Element{Id: id, Kind: &pb.Element_Container{Container: &pb.Container{}}}}
	for _, child := range children {
		if child == nil {
			panic("gpui: nil child")
		}
		n.element.GetContainer().Children = append(n.element.GetContainer().Children, child.Proto())
	}
	return n
}

func Text(id uint64, text string) *Node {
	return &Node{element: &Element{Id: id, Kind: &pb.Element_Text{Text: &pb.Text{Content: text}}}}
}

// FromProto copies a caller-supplied element. Unsupported models are rejected
// by the bridge, rather than silently discarded.
func FromProto(element *Element) *Node {
	if element == nil {
		panic("gpui: nil element")
	}
	return &Node{element: proto.Clone(element).(*Element)}
}

func (n *Node) Proto() *Element { return proto.Clone(n.element).(*Element) }
func (n *Node) Style(style *Style) *Node {
	if style == nil {
		n.element.Style = nil
	} else {
		n.element.Style = proto.Clone(style).(*Style)
	}
	return n
}
func (n *Node) OnClick(handlerID uint64) *Node {
	n.element.OnClick = &EventBinding{HandlerId: handlerID}
	return n
}

// Hover adds a partial style refinement applied by GPUI while hovered. It does
// not require a Go event or another view snapshot.
func (n *Node) Hover(style *Style) *Node {
	if style == nil {
		n.element.HoverStyle = nil
	} else {
		n.element.HoverStyle = proto.Clone(style).(*Style)
	}
	return n
}

// When sets refinements that GPUI applies while the window matches each one's
// width range and color scheme, without a Go event or another snapshot. The
// backend needs the conditional-styles capability.
func (n *Node) When(conditions ...*ConditionalStyle) *Node {
	n.element.ConditionalStyles = nil
	for _, condition := range conditions {
		if condition == nil {
			panic("gpui: nil conditional style")
		}
		n.element.ConditionalStyles = append(n.element.ConditionalStyles, proto.Clone(condition).(*ConditionalStyle))
	}
	return n
}
func (n *Node) StopPropagation() *Node {
	if n.element.OnClick == nil {
		panic("gpui: StopPropagation requires OnClick")
	}
	n.element.OnClick.StopPropagation = true
	return n
}

func Pixels(value float32) *Length  { return &Length{Value: &pb.Length_Pixels{Pixels: value}} }
func Percent(value float32) *Length { return &Length{Value: &pb.Length_Percent{Percent: value}} }
func Rems(value float32) *Length    { return &Length{Value: &pb.Length_Rems{Rems: value}} }
func Auto() *Length                 { return &Length{Value: &pb.Length_Auto{Auto: true}} }
func RGB(value uint32) *Color {
	return &Color{Red: float32(value>>16&255) / 255, Green: float32(value>>8&255) / 255, Blue: float32(value&255) / 255, Alpha: 1}
}
func Padding(value float32) *Edges {
	return &Edges{Top: Pixels(value), Right: Pixels(value), Bottom: Pixels(value), Left: Pixels(value)}
}
