//go:build !gpui

package host

import (
	"context"

	"github.com/moriyoshi/gpui-go/client"
)

func run(context.Context, func(context.Context, *client.Client) error) error {
	return ErrNativeUnavailable
}
