package scene_test

import (
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/scene"
	"google.golang.org/protobuf/proto"
)

func TestPublicValuesPreserveWireIdentityPresenceAndOwnership(t *testing.T) {
	style := &scene.Style{
		FlexDirection: scene.FlexColumn.Enum(),
		Display:       scene.DisplayGrid.Enum(),
		FontSize:      scene.Ptr[float32](0),
		FlexWrap:      scene.Ptr(false),
		Opacity:       scene.Ptr[float32](0),
	}
	var legacy *pb.Style = style
	root := scene.Div(1, scene.Canvas(2, &scene.DrawCommand{
		Kind: &scene.DrawCommandPath{Path: &scene.DrawPath{
			Steps: []*scene.PathStep{{Verb: scene.PathMove, Point: &scene.Point{X: 3}}},
			Color: scene.RGB(0x123456),
		}},
	})).Style(legacy).Accessible("Drawing", &scene.Accessibility{Role: scene.RoleImage, Checked: scene.Ptr(false)})
	*style.FontSize = 99
	data, err := proto.Marshal(root.Proto())
	if err != nil {
		t.Fatal(err)
	}
	decoded := new(pb.Element)
	if err := proto.Unmarshal(data, decoded); err != nil {
		t.Fatal(err)
	}
	got := decoded.Style
	if got.FontSize == nil || *got.FontSize != 0 || got.FlexWrap == nil || *got.FlexWrap || got.Opacity == nil || *got.Opacity != 0 {
		t.Fatalf("optional zero/false presence or copied style lost: %v", got)
	}
	if got.GetFlexDirection() != scene.FlexColumn || got.GetDisplay() != scene.DisplayGrid || decoded.Accessibility.GetRole() != scene.RoleImage || decoded.Accessibility.Checked == nil {
		t.Fatalf("public enum/accessibility values did not survive serialization: %v", decoded)
	}
	path := decoded.GetContainer().Children[0].GetDrawing().Commands[0].GetPath()
	if path.Steps[0].Verb != scene.PathMove || path.Steps[0].Point.X != 3 {
		t.Fatalf("public drawing oneof lost: %v", path)
	}
}
