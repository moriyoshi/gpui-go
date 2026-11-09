package gpui

import (
	"context"

	"github.com/moriyoshi/gpui-go/host"
)

var ErrNativeUnavailable = host.ErrNativeUnavailable

// Run enters the standalone host. New applications may call host.Run directly.
// Call once from main; the callback must honor context cancellation.
func Run(ctx context.Context, application func(context.Context, *Client) error) error {
	return host.Run(ctx, application)
}
