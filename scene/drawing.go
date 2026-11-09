package scene

import (
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"google.golang.org/protobuf/proto"
)

func Image(id uint64, resource *Handle) *Node {
	if resource == nil {
		panic("gpui: nil image resource")
	}
	return &Node{element: &Element{Id: id, Kind: &pb.Element_Image{Image: &pb.Image{Resource: proto.Clone(resource).(*Handle)}}}}
}

// Embedded places a native view owned by a host extension, such as an embedded
// Zed editor. Each instance can appear once per view. The backend must support
// the embedded-views capability.
func Embedded(id, instance uint64) *Node {
	return &Node{element: &Element{Id: id, Kind: &pb.Element_Embedded{Embedded: &pb.Embedded{Instance: instance}}}}
}

// Canvas submits local-coordinate drawing commands to GPUI. Paths are filled
// when stroke_width is zero, otherwise stroked. No Go paint callback is used.
func Canvas(id uint64, commands ...*DrawCommand) *Node {
	drawing := &Drawing{Commands: commands}
	return &Node{element: &Element{Id: id, Kind: &pb.Element_Drawing{Drawing: proto.Clone(drawing).(*Drawing)}}}
}

// UniformList virtualizes native layout/painting. Go supplies all row snapshots
// up front; this is not a paging protocol. The normal snapshot limits apply.
func UniformList(id uint64, rowHeight float32, rows ...*Node) *Node {
	list := &pb.UniformList{RowHeight: rowHeight}
	for _, row := range rows {
		if row == nil {
			panic("gpui: nil list row")
		}
		list.Items = append(list.Items, row.Proto())
	}
	return &Node{element: &Element{Id: id, Kind: &pb.Element_UniformList{UniformList: list}}}
}
