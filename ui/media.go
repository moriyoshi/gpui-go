package ui

import (
	"errors"

	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

var errNilImage = errors.New("ui: images require a resource")

func handle(resource *clientlib.Resource) *pb.Handle {
	if resource == nil {
		return nil
	}
	return resource.Handle()
}

// Image displays an uploaded image or SVG resource. The resource must stay
// uploaded while any mounted snapshot references it.
func (d *Document) Image(owner *reactive.Scope, resource *clientlib.Resource) *Node {
	n := d.create(owner, &pb.Element{Kind: &pb.Element_Image{Image: &pb.Image{Resource: handle(resource)}}})
	if resource == nil {
		d.record(errNilImage)
	}
	return n
}

// BindImage displays the resource the accessor returns, switching when its
// dependencies change. A nil resource fails the computation.
func (d *Document) BindImage(owner *reactive.Scope, read func() *clientlib.Resource) *Node {
	n := d.create(owner, &pb.Element{Kind: &pb.Element_Image{Image: &pb.Image{}}})
	d.bind(n.scope, func() error {
		value := handle(read())
		if value == nil {
			return errNilImage
		}
		if image := n.element.GetImage(); !proto.Equal(image.Resource, value) {
			image.Resource = value
			d.changed()
		}
		return nil
	})
	return n
}

func drawing(commands []*scene.DrawCommand) *pb.Drawing {
	return proto.Clone(&pb.Drawing{Commands: commands}).(*pb.Drawing)
}

// Canvas paints drawing commands in local coordinates. Give it a size: it
// fills its box and has no children.
func (d *Document) Canvas(owner *reactive.Scope, commands ...*scene.DrawCommand) *Node {
	return d.create(owner, &pb.Element{Kind: &pb.Element_Drawing{Drawing: drawing(commands)}})
}

// BindCanvas repaints with the commands the accessor returns when its
// dependencies change. The commands are copied; unchanged commands publish
// nothing.
func (d *Document) BindCanvas(owner *reactive.Scope, read func() []*scene.DrawCommand) *Node {
	n := d.Canvas(owner)
	d.bind(n.scope, func() error {
		value := drawing(read())
		if element := n.element.GetKind().(*pb.Element_Drawing); !proto.Equal(element.Drawing, value) {
			element.Drawing = value
			d.changed()
		}
		return nil
	})
	return n
}

// UniformList virtualizes its children as rows of one height, in logical
// pixels from 1 to 16384. Children are managed like a Div's, so For and Show
// ranges can supply rows; each serialized child is one row. Every row is still
// sent in every snapshot. Scroll with Mount.Window().ScrollTo and the list's ID.
func (d *Document) UniformList(owner *reactive.Scope, rowHeight float32, rows ...*Node) *Node {
	n := d.create(owner, &pb.Element{Kind: &pb.Element_UniformList{UniformList: &pb.UniformList{RowHeight: rowHeight}}})
	if !(rowHeight >= 1 && rowHeight <= 16384) {
		d.record(errors.New("ui: uniform list row height must be in [1, 16384]"))
	}
	d.record(n.SetChildren(rows...))
	return n
}
