//go:build gpui && !gpui_prebuilt

package nativeshim

/*
#cgo CFLAGS: -I${SRCDIR}/../../include
#include "gpui_go.h"
*/
import "C"

import (
	"context"

	"github.com/moriyoshi/sable"
)

func GPUIInit() int32 { return int32(C.gpui_go_init()) }
func GPUIStop()       { C.gpui_go_stop() }

// Start begins delivering Sable completions.
func Start() { sable.Init() }

// Call awaits one Rust operation.
func Call(ctx context.Context, op uint32, request []byte) ([]byte, error) {
	return sable.CallCtx(ctx, op, request)
}

// Shutdown frees Sable after every Call has returned.
func Shutdown() { sable.Shutdown() }

// FinalizeThread has nothing to do: the host C runtime owns native TLS here.
func FinalizeThread() {}
