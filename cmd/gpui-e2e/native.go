//go:build gpui_e2e

package main

import (
	"context"
	"errors"
	"io"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	countergox "github.com/moriyoshi/gpui-go/examples/counter-gox/app"
	"github.com/moriyoshi/gpui-go/examples/e2eapp"
	"github.com/moriyoshi/gpui-go/examples/reactiveapp"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/uitest"
	gpuitest "github.com/moriyoshi/gpui-go/uitest/gpui"
)

func runNative(ctx context.Context, paths []string, timeout time.Duration, out io.Writer, report string) error {
	var scenariosErr error
	runtimeErr := host.Run(ctx, func(ctx context.Context, client *clientlib.Client) error {
		factory := func(ctx context.Context, client *clientlib.Client, name string) (gpuitest.Application, error) {
			if name == "counter-gox" {
				return countergox.Open(ctx, client, name)
			}
			if name == "reactive" {
				return reactiveapp.Open(ctx, client, name)
			}
			return e2eapp.Open(ctx, client, name)
		}
		scenariosErr = runScenarios(ctx, gpuitest.NewNativeTarget(client, factory), paths, timeout, out)
		return uitest.WriteWorkerReport(report, scenariosErr)
	})
	// AppKit may terminate the process from GPUI quit without returning here.
	// The parent owns the exit code and requires the report written above.
	return errors.Join(scenariosErr, runtimeErr)
}
