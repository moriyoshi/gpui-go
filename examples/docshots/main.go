// Command docshots renders the element reference samples as native windows and
// saves each one as a PNG. It is macOS-only; run it with make docs-screenshots.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
)

// A shot opens its windows through the capture helpers, which close them again.
type shot struct {
	name string
	run  func(ctx context.Context, s *session) error
}

type session struct {
	app       *client.Client
	out       string
	windowIDs string
	settle    time.Duration
	seen      map[string]bool
	images    *resources
}

func main() {
	out := flag.String("out", "docs/reference/screenshots", "directory for PNG files")
	windowIDs := flag.String("window-ids", "bin/docs-window-ids", "window listing helper built from tools/docs_window_ids.swift")
	only := flag.String("only", "", "capture only shots whose name contains this text")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	err := host.Run(context.Background(), func(ctx context.Context, app *client.Client) error {
		images, err := uploadResources(ctx, app)
		if err != nil {
			return err
		}
		// The first window compiles GPUI's shaders, so it needs longer to paint.
		s := &session{app: app, out: *out, windowIDs: *windowIDs, settle: 3 * time.Second, seen: map[string]bool{}, images: images}
		for _, item := range allShots() {
			if !strings.Contains(item.name, *only) {
				continue
			}
			if err := item.run(ctx, s); err != nil {
				return fmt.Errorf("%s: %w", item.name, err)
			}
			log.Printf("captured %s", item.name)
			s.settle = 700 * time.Millisecond
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

// capture saves the single window this process opened since the last capture.
func (s *session) capture(ctx context.Context, name string) error {
	time.Sleep(s.settle)
	path := filepath.Join(s.out, name+".png")
	var lastErr error
	for range 20 {
		id, err := s.newWindow(ctx)
		if err != nil {
			return err
		}
		if id != "" {
			output, err := exec.CommandContext(ctx, "screencapture", "-x", "-o", "-l"+id, path).CombinedOutput()
			if err == nil {
				s.seen[id] = true
				return nil
			}
			lastErr = fmt.Errorf("screencapture: %w: %s", err, output)
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no new window appeared")
	}
	return lastErr
}

// onlyWindow returns the window just captured, so it can be captured again.
func (s *session) onlyWindow(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, s.windowIDs, strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return "", fmt.Errorf("list windows: %w", err)
	}
	ids := strings.Fields(string(output))
	if len(ids) == 0 {
		return "", fmt.Errorf("no window is open")
	}
	return ids[0], nil
}

func (s *session) newWindow(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, s.windowIDs, strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return "", fmt.Errorf("list windows: %w", err)
	}
	for _, id := range strings.Fields(string(output)) {
		if !s.seen[id] {
			return id, nil
		}
	}
	return "", nil
}
