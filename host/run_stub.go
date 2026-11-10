//go:build !gpui && !gpui_prebuilt

package host

import (
	"context"

	"github.com/moriyoshi/gpui-go/client"
)

func run(context.Context, func(context.Context, *client.Client) error) error {
	return ErrNativeUnavailable
}
