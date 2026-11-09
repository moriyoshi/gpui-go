//go:build !gpui_e2e

package main

import (
	"context"
	"errors"
	"io"
	"time"
)

func runNative(context.Context, []string, time.Duration, io.Writer, string) error {
	return errors.New("native E2E execution requires make build-e2e; --check works without native libraries")
}
