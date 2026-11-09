//go:build gpui && gpui_headless

package host

import (
	"runtime"
	"unsafe"

	"github.com/moriyoshi/gpui-go/internal/nativeshim"
)

func runPlatform() int { return int(nativeshim.GPUIRunHeadless()) }

func headlessClick(entity uint64, generation uint32, revision, element uint64) int {
	return int(nativeshim.GPUIHeadlessClick(entity, generation, revision, element))
}

func headlessTextChange(entity uint64, generation uint32, element, epoch, revision uint64, value string) int {
	data := []byte(value)
	var pinner runtime.Pinner
	defer pinner.Unpin()
	var ptr unsafe.Pointer
	if len(data) != 0 {
		ptr = unsafe.Pointer(unsafe.SliceData(data))
		pinner.Pin(ptr)
	}
	return int(nativeshim.GPUIHeadlessTextChange(entity, generation, element, epoch, revision, ptr, uintptr(len(data))))
}
