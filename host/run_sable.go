//go:build gpui

package host

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/internal/nativeshim"
)

// init runs on Go's startup thread. Keeping it locked preserves the process's
// actual main thread until main invokes Run; locking a later goroutine alone
// would not satisfy AppKit's main-thread requirement.
func init() { runtime.LockOSThread() }

var usedRuntime atomic.Bool

type sableTransport struct {
	mu     sync.RWMutex
	closed bool
}

func (t *sableTransport) Call(ctx context.Context, op client.Operation, request []byte) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.closed {
		return nil, errors.New("gpui: transport is closed")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nativeshim.Call(ctx, uint32(op), request)
}

func run(ctx context.Context, application func(context.Context, *client.Client) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !usedRuntime.CompareAndSwap(false, true) {
		return errors.New("gpui: Run is supported once per process")
	}
	if status := nativeshim.GPUIInit(); status != 0 {
		return fmt.Errorf("gpui: initialize bridge: status %d", status)
	}
	nativeshim.Start()
	transport := new(sableTransport)
	appctx, cancel := context.WithCancel(ctx)
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		client, err := client.NewClient(appctx, transport)
		if err == nil {
			err = application(appctx, client)
		}
		// Use an independent deadline so callback cancellation can still quit
		// the UI. Sable cancellation releases a request if startup failed.
		if client != nil {
			quitctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			quitErr := client.Quit(quitctx)
			stop()
			if err == nil && appctx.Err() == nil {
				err = quitErr
			}
		}
		nativeshim.GPUIStop()
		finished <- err
	}()
	status := runPlatform()
	nativeshim.GPUIStop()
	cancel()
	applicationErr := <-finished
	// Stop accepting calls and wait for all calls holding the read lock before
	// freeing Sable's runtime. Rust stop has already released pending requests.
	transport.mu.Lock()
	transport.closed = true
	nativeshim.Shutdown()
	transport.mu.Unlock()
	nativeshim.FinalizeThread()
	if status != 0 {
		return fmt.Errorf("gpui: platform run failed: status %d (macOS requires the process main thread)", status)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if applicationErr != nil && !errors.Is(applicationErr, context.Canceled) {
		var protocolErr *client.ProtocolError
		if !errors.As(applicationErr, &protocolErr) || protocolErr.Code != pb.ErrorCode_ERROR_CODE_CLOSED {
			return applicationErr
		}
	}
	return nil
}
