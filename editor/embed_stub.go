//go:build !gpui_zed_editor_embed

package editor

import "context"

// Attach requires the gpui_zed_editor_embed tag and the combined native archive.
func Attach(context.Context) (*Client, error) { return nil, ErrNativeUnavailable }
