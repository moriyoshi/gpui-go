//go:build gpui && !gpui_prebuilt && !gpui_headless

package nativeshim

/*
#cgo darwin LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore -framework CoreVideo -framework CoreText -framework CoreGraphics -framework Security -framework SystemConfiguration -framework IOKit -framework CoreServices -framework Carbon -framework UserNotifications -framework UniformTypeIdentifiers -lc++
#cgo linux LDFLAGS: -lxcb -lxkbcommon -lxkbcommon-x11 -lxcb-xkb -lfreetype -lfontconfig -lz -lstdc++ -lgcc_s -lutil -lrt -lpthread -lm -ldl
#include "gpui_go.h"
*/
import "C"

// GPUIRunNative must remain an ordinary cgo call: it can block for the lifetime
// of the application, which is invalid for Sable's short asm crossings.
func GPUIRunNative() int32 { return int32(C.gpui_go_run_native()) }
