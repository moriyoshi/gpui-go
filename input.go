package gpui

import (
	"github.com/moriyoshi/gpui-go/scene"
)

type InputState = scene.InputState
type TextInputOptions = scene.TextInputOptions

func TextInput(id uint64, value InputState, options TextInputOptions) *Node {
	return scene.TextInput(id, value, options)
}
