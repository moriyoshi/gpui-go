package gpui

import (
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
)

func Image(id uint64, resource *Resource) *Node {
	if resource == nil {
		panic("gpui: nil image resource")
	}
	return scene.Image(id, resource.Handle())
}
func Canvas(id uint64, commands ...*pb.DrawCommand) *Node { return scene.Canvas(id, commands...) }
func UniformList(id uint64, rowHeight float32, rows ...*Node) *Node {
	return scene.UniformList(id, rowHeight, rows...)
}
