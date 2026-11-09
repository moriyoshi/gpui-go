//go:build !gpui_zed_editor

package editor

import "context"

func run(context.Context, func(context.Context, *Client) error) error { return ErrNativeUnavailable }
