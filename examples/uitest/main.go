//go:build gpui_e2e

// A Go-only consumer of the public uitest package. No Starlark wiring is used.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/examples/e2eapp"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/uitest"
	gpuitest "github.com/moriyoshi/gpui-go/uitest/gpui"
)

func main() {
	report := flag.String("worker-report", "", "internal native worker report")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, *report); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, report string) error {
	if report != "" {
		return host.Run(ctx, func(ctx context.Context, client *clientlib.Client) error {
			target := gpuitest.NewNativeTarget(client, func(ctx context.Context, client *clientlib.Client, fixture string) (gpuitest.Application, error) {
				return e2eapp.Open(ctx, client, fixture)
			})
			scenario, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			err := uitest.RunScenario(scenario, target, func(ctx context.Context) error { return checkWorkbench(ctx, target) })
			return uitest.WriteWorkerReport(report, err)
		})
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	report, err = uitest.RunWorker(ctx, uitest.WorkerOptions{
		Executable: binary, Args: func(path string) []string { return []string{"--worker-report", path} },
		ArtifactsDir: filepath.Join(".agents-workspace", "tmp"), Timeout: 45 * time.Second, Stdout: os.Stdout, Stderr: os.Stderr,
	})
	if err == nil {
		fmt.Printf("Go uitest consumer passed (report: %s)\n", report)
	}
	return err
}

func checkWorkbench(ctx context.Context, target uitest.Target) error {
	page, err := uitest.OpenPage(ctx, target, "workbench")
	if err != nil {
		return err
	}
	name := page.GetByRole("textbox", uitest.RoleOptions{Name: "Name", Exact: true})
	if err := name.Fill(ctx, "Go 日本語🙂"); err != nil {
		return err
	}
	if err := uitest.Expect(name).ToHaveValue(ctx, "Go 日本語🙂"); err != nil {
		return err
	}
	if err := page.Keyboard().Press(ctx, "ControlOrMeta+J"); err != nil {
		return err
	}
	if err := uitest.Expect(page.GetByTestID("count")).ToHaveText(ctx, "Count: 1"); err != nil {
		return err
	}
	notifications := page.GetByRole("checkbox", uitest.RoleOptions{Name: "Enable notifications"})
	if err := notifications.Check(ctx); err != nil {
		return err
	}
	return uitest.Expect(notifications).ToBeChecked(ctx)
}
