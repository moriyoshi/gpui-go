//go:build gpui_zed_editor

package editor

/*
#cgo darwin LDFLAGS: -framework Cocoa -framework Metal -framework QuartzCore -framework CoreVideo -framework CoreText -framework CoreGraphics -framework Security -framework SystemConfiguration -framework IOKit -framework CoreServices -framework Carbon -framework UserNotifications -framework UniformTypeIdentifiers -lc++
int gpui_zed_editor_init(void);
int gpui_zed_editor_run(void);
void gpui_zed_editor_stop(void);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moriyoshi/sable"
)

func init() { runtime.LockOSThread() }

var runtimeUsed atomic.Bool

type sableTransport struct {
	mu     sync.RWMutex
	closed bool
}

func (t *sableTransport) Call(ctx context.Context, op Operation, request []byte) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return nil, ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result, err := sable.CallCtx(ctx, uint32(op), request)
	if err != nil && strings.Contains(err.Error(), "editor event stream closed") {
		return nil, ErrClosed
	}
	return result, err
}
func run(ctx context.Context, application func(context.Context, *Client) error) error {
	if !runtimeUsed.CompareAndSwap(false, true) {
		return errors.New("editor: Run is supported once per process")
	}
	if status := C.gpui_zed_editor_init(); status != 0 {
		return fmt.Errorf("editor: initialize host: status %d", status)
	}
	sable.Init()
	transport := new(sableTransport)
	appctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		client, err := NewClient(appctx, transport)
		if err == nil {
			err = application(appctx, client)
		}
		if client != nil {
			quitctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			quitErr := client.Quit(quitctx)
			stop()
			if err == nil && appctx.Err() == nil && !errors.Is(quitErr, ErrClosed) {
				err = quitErr
			}
		}
		C.gpui_zed_editor_stop()
		finished <- err
	}()
	status := C.gpui_zed_editor_run()
	C.gpui_zed_editor_stop()
	cancel()
	err := <-finished
	transport.mu.Lock()
	transport.closed = true
	sable.Shutdown()
	transport.mu.Unlock()
	if status != 0 {
		return fmt.Errorf("editor: native host failed: status %d (requires the process main thread)", status)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, ErrClosed) {
		return nil
	}
	return err
}
