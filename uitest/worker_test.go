package uitest_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moriyoshi/gpui-go/uitest"
)

func TestReportsRejectIncompleteFailedAndUnsupportedResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.json")
	if err := uitest.ReadWorkerReport(path); err == nil {
		t.Fatal("missing report accepted")
	}
	for _, data := range []string{"{}", "not json", `{"version":2,"complete":true}`, `{"version":1,"complete":true,"error":"native assertion failed"}`} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := uitest.ReadWorkerReport(path); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
	if err := uitest.WriteWorkerReport(path, errors.New("wrong count")); err != nil {
		t.Fatal(err)
	}
	if err := uitest.ReadWorkerReport(path); err == nil || !strings.Contains(err.Error(), "wrong count") {
		t.Fatalf("lost failure: %v", err)
	}
	if err := uitest.WriteWorkerReport(path, nil); err != nil {
		t.Fatal(err)
	}
	if err := uitest.ReadWorkerReport(path); err != nil {
		t.Fatal(err)
	}
}

// This subprocess fixture exercises actual process exit and report handling.
func TestWorkerProcess(t *testing.T) {
	if os.Getenv("GPUI_UITEST_WORKER_HELPER") != "1" {
		return
	}
	args := os.Args[len(os.Args)-2:]
	path, mode := args[0], args[1]
	switch mode {
	case "missing":
		os.Exit(0)
	case "timeout":
		time.Sleep(time.Minute)
		os.Exit(0)
	case "failed":
		if err := uitest.WriteWorkerReport(path, errors.New("wrong value")); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	case "crashed":
		if err := uitest.WriteWorkerReport(path, nil); err != nil {
			os.Exit(4)
		}
		os.Exit(3)
	case "success":
		if err := uitest.WriteWorkerReport(path, nil); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	default:
		os.Exit(5)
	}
}

func TestWorkerRequiresBothProcessAndReportSuccess(t *testing.T) {
	for _, mode := range []string{"success", "failed", "missing", "crashed", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			timeout := 5 * time.Second
			if mode == "timeout" {
				timeout = 30 * time.Millisecond
			}
			report, err := uitest.RunWorker(context.Background(), uitest.WorkerOptions{
				Executable: os.Args[0], Args: func(path string) []string { return []string{"-test.run=^TestWorkerProcess$", "--", path, mode} },
				ArtifactsDir: t.TempDir(), Timeout: timeout, Env: append(os.Environ(), "GPUI_UITEST_WORKER_HELPER=1"),
			})
			if (err == nil) != (mode == "success") || report == "" {
				t.Fatalf("mode %s: report=%s err=%v", mode, report, err)
			}
			if mode == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("missing deadline error: %v", err)
			}
		})
	}
}
