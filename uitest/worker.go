package uitest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type workerReport struct {
	Version  uint32 `json:"version"`
	Complete bool   `json:"complete"`
	Error    string `json:"error,omitempty"`
}

// WriteWorkerReport atomically records completion before native shutdown, which
// may terminate the process without returning through Go on macOS.
func WriteWorkerReport(path string, scenarioErr error) error {
	value := workerReport{Version: 1, Complete: true}
	if scenarioErr != nil {
		value.Error = scenarioErr.Error()
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// ReadWorkerReport rejects missing, unsupported, incomplete and failed reports.
func ReadWorkerReport(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("native worker did not complete its report: %w", err)
	}
	var result workerReport
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("invalid native worker report: %w", err)
	}
	if result.Version != 1 || !result.Complete {
		return errors.New("native worker report has an unsupported version or is incomplete")
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	return nil
}

// WorkerOptions separates process supervision from a consumer's CLI flags.
type WorkerOptions struct {
	Executable string
	// Args receives the absolute report path to pass to the worker.
	Args func(reportPath string) []string
	// ArtifactsDir defaults to the system temporary directory. Reports remain
	// available to the caller after both successful and failed runs.
	ArtifactsDir   string
	Timeout        time.Duration
	Stdout, Stderr io.Writer
	// Env defaults to the parent environment when nil.
	Env []string
}

// RunWorker requires process success AND a complete successful report. A zero
// AppKit exit alone cannot turn an assertion, early quit or crash into success.
// The returned report path is available even when the worker fails.
func RunWorker(ctx context.Context, options WorkerOptions) (string, error) {
	if options.Executable == "" || options.Args == nil || options.Timeout <= 0 {
		return "", errors.New("uitest: executable, arguments and positive worker timeout are required")
	}
	base := options.ArtifactsDir
	if base == "" {
		base = os.TempDir()
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(base, "e2e-run-")
	if err != nil {
		return "", err
	}
	report, err := filepath.Abs(filepath.Join(dir, "result.json"))
	if err != nil {
		return "", err
	}
	worker, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()
	cmd := exec.CommandContext(worker, options.Executable, options.Args(report)...)
	cmd.Stdout = options.Stdout
	cmd.Stderr = options.Stderr
	cmd.Env = options.Env
	cmd.WaitDelay = 3 * time.Second
	processErr := cmd.Run()
	resultErr := ReadWorkerReport(report)
	if err := errors.Join(processErr, resultErr, worker.Err()); err != nil {
		return report, fmt.Errorf("uitest worker failed (report: %s): %w", report, err)
	}
	return report, nil
}
