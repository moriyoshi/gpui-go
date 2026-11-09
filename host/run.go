// Package host owns the standalone GPUI/Sable runtime and platform loop.
// Import client and scene instead when an application supplies its own host.
package host

import (
	"context"
	"errors"

	"github.com/moriyoshi/gpui-go/client"
)

var ErrNativeUnavailable = errors.New("gpui: build with make run, or with -tags gpui,sable_extern_lib,sable_portable and the Rust static library")

// Run enters the desktop event loop on the main OS thread. Invoke it from
// main, once per process. The callback runs on a goroutine and must return
// when ctx is canceled. Returning from the callback requests application quit.
// The gpui_headless build tag runs the same protocol without a display.
func Run(ctx context.Context, application func(context.Context, *client.Client) error) error {
	if application == nil {
		return errors.New("gpui: nil application callback")
	}
	return run(ctx, application)
}
