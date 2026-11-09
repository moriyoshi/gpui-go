//go:build gpui && !gpui_prebuilt && gpui_headless

package nativeshim

// #include "gpui_go.h"
import "C"

import "unsafe"

func GPUIRunHeadless() int32 { return int32(C.gpui_go_run_headless()) }

func GPUIHeadlessClick(entity uint64, generation uint32, revision, element uint64) int32 {
	return int32(C.gpui_go_headless_click(C.uint64_t(entity), C.uint32_t(generation), C.uint64_t(revision), C.uint64_t(element)))
}

func GPUIHeadlessTextChange(entity uint64, generation uint32, element, epoch, revision uint64, data unsafe.Pointer, length uintptr) int32 {
	return int32(C.gpui_go_headless_text_change(C.uint64_t(entity), C.uint32_t(generation), C.uint64_t(element), C.uint64_t(epoch), C.uint64_t(revision), (*C.uint8_t)(data), C.size_t(length)))
}
