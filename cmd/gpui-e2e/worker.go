package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/moriyoshi/gpui-go/uitest"
)

func runWorker(ctx context.Context, paths []string, timeout time.Duration, out, stderr io.Writer) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	report, err := uitest.RunWorker(ctx, uitest.WorkerOptions{
		Executable: binary,
		Args: func(report string) []string {
			args := []string{"--worker-report", report, "--timeout", timeout.String()}
			return append(args, paths...)
		},
		ArtifactsDir: filepath.Join(".agents-workspace", "tmp"),
		Timeout:      time.Duration(len(paths))*(timeout+uitest.CleanupTimeout) + 15*time.Second,
		Stdout:       out, Stderr: stderr,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "All %d scenarios passed (report: %s)\n", len(paths), report)
	return nil
}
