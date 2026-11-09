package scene

import (
	"testing"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
)

func TestSnapshotOwnsItsData(t *testing.T) {
	style := &pb.Style{Background: RGB(0xff0000)}
	child := Text(2, "original").Style(style)
	root := Div(1, child)
	style.Background.Red = 0
	child.element.GetText().Content = "changed"
	copy := root.Proto()
	if got := copy.GetContainer().Children[0]; got.GetText().Content != "original" || got.Style.Background.Red != 1 {
		t.Fatalf("snapshot aliases builder data: %v", got)
	}
	copy.GetContainer().Children[0].GetText().Content = "also changed"
	if root.Proto().GetContainer().Children[0].GetText().Content != "original" {
		t.Fatal("Proto returned shared memory")
	}
}

func TestTextInputCarriesMasked(t *testing.T) {
	if !TextInput(1, InputState{}, TextInputOptions{Masked: true}).Proto().GetTextInput().Masked {
		t.Fatal("masked option was dropped")
	}
	if TextInput(1, InputState{}, TextInputOptions{}).Proto().GetTextInput().Masked {
		t.Fatal("inputs must not be masked by default")
	}
}

func TestHoverStyleOwnsItsData(t *testing.T) {
	style := &pb.Style{Background: RGB(0x5b5ce2)}
	node := Text(1, "Hover me").Hover(style)
	style.Background.Red = 0
	snapshot := node.Proto()
	if snapshot.HoverStyle.Background.Red == 0 {
		t.Fatal("hover aliases caller style")
	}
	snapshot.HoverStyle.Background.Red = 0
	if node.Proto().HoverStyle.Background.Red == 0 {
		t.Fatal("hover aliases returned snapshot")
	}
}
