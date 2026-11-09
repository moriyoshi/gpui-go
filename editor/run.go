package editor

import (
	"context"
	"errors"
)

// Run enters the standalone native host on the process main thread. Call from
// main, once per process. The callback runs on a goroutine and must return when
// its context is canceled; returning requests host shutdown. The native host
// owns Sable and cannot share a process with another Sable runtime owner.
// Rust applications embedding EditorComponent can provide their own Transport
// and use NewClient instead of this standalone host.
func Run(ctx context.Context, application func(context.Context, *Client) error) error {
	if application == nil {
		return errors.New("editor: nil application callback")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return run(ctx, application)
}
