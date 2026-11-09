//go:build gpui_zed_editor_embed

package editor

/*
#cgo darwin LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore -framework CoreVideo -framework CoreText -framework CoreGraphics -framework Security -framework SystemConfiguration -framework IOKit -framework CoreServices -framework Carbon -framework UserNotifications -framework UniformTypeIdentifiers -lc++
int gpui_zed_embed_register(void);
*/
import "C"

import (
	"context"
	"fmt"
	"strings"

	"github.com/moriyoshi/sable"
)

// Registration has to precede host.Run, which starts the application.
func init() {
	if status := C.gpui_zed_embed_register(); status != 0 {
		panic(fmt.Sprintf("editor: register embedded editors: status %d", status))
	}
}

type embeddedTransport struct{}

func (embeddedTransport) Call(ctx context.Context, op Operation, request []byte) ([]byte, error) {
	result, err := sable.CallCtx(ctx, uint32(op), request)
	if err != nil && strings.Contains(err.Error(), "editor event stream closed") {
		return nil, ErrClosed
	}
	return result, err
}

// Attach connects to the editors in a process started by host.Run. Call it from
// the application callback, and do not also call Run.
func Attach(ctx context.Context) (*Client, error) { return NewClient(ctx, embeddedTransport{}) }
