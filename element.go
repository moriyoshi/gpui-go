package gpui

import (
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

type Node = scene.Node

func Div(id uint64, children ...*Node) *Node { return scene.Div(id, children...) }
func Text(id uint64, text string) *Node      { return scene.Text(id, text) }
func FromProto(element *pb.Element) *Node    { return scene.FromProto(element) }
func Pixels(value float32) *pb.Length        { return scene.Pixels(value) }
func Percent(value float32) *pb.Length       { return scene.Percent(value) }
func Rems(value float32) *pb.Length          { return scene.Rems(value) }
func Auto() *pb.Length                       { return scene.Auto() }
func RGB(value uint32) *pb.Color             { return scene.RGB(value) }
func Padding(value float32) *pb.Edges        { return scene.Padding(value) }
