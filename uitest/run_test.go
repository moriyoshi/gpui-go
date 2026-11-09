package uitest_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/uitest"
)

type target struct {
	requests        []*pb.AutomationRequest
	cleaned         bool
	cleanupCanceled bool
	cleanupErr      error
}

func (*target) Open(context.Context, string) (string, error) { return "app", nil }
func (*target) Close(context.Context, string) error          { return nil }
func (t *target) Input(_ context.Context, _, _ string, request *pb.AutomationRequest) error {
	t.requests = append(t.requests, request)
	return nil
}
func (*target) Inspect(context.Context, string, string) (map[string]any, error) { return nil, nil }
func (*target) State(context.Context, string) (map[string]any, error)           { return nil, nil }
func (*target) Update(context.Context, string, string, string) error            { return nil }
func (*target) Clipboard(context.Context, *string) (string, error)              { return "", nil }
func (t *target) Cleanup(ctx context.Context) error {
	t.cleaned = true
	t.cleanupCanceled = ctx.Err() != nil
	return t.cleanupErr
}

func TestGoConsumerUsesInputHelpersWithoutAnInterpreter(t *testing.T) {
	ctx := context.Background()
	driver := &target{}
	err := uitest.RunScenario(ctx, driver, func(ctx context.Context) error {
		if err := uitest.Click(ctx, driver, "app", "name"); err != nil {
			return err
		}
		if err := uitest.TypeText(ctx, driver, "app", "e\u0301🙂"); err != nil {
			return err
		}
		if err := uitest.Key(ctx, driver, "app", "cmd-k cmd-s"); err != nil {
			return err
		}
		return uitest.Compose(ctx, driver, "app", "name", "日本語", true)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !driver.cleaned || len(driver.requests) != 5 {
		t.Fatalf("cleanup/dispatch: %+v", driver)
	}
	if driver.requests[0].GetPointer().Phase != pb.AutomationPointer_PHASE_CLICK || driver.requests[1].GetText().Text != "e\u0301🙂" || driver.requests[2].GetKey().Keystroke != "cmd-k" || driver.requests[3].GetKey().Keystroke != "cmd-s" || !driver.requests[4].GetComposition().Commit {
		t.Fatalf("wrong inputs: %v", driver.requests)
	}
	pointer := &pb.AutomationPointer{Phase: pb.AutomationPointer_PHASE_MOVE}
	if err := uitest.Pointer(ctx, driver, "app", "name", pointer); err != nil {
		t.Fatal(err)
	}
	driver.requests[5].GetPointer().Phase = pb.AutomationPointer_PHASE_UP
	if pointer.Phase != pb.AutomationPointer_PHASE_MOVE {
		t.Fatal("input retained caller-owned metadata")
	}
	before := len(driver.requests)
	for _, keys := range []string{"", strings.Repeat("x ", 65)} {
		if err := uitest.Key(ctx, driver, "app", keys); err == nil {
			t.Fatal("invalid key sequence accepted")
		}
	}
	if len(driver.requests) != before {
		t.Fatal("invalid sequence reached target")
	}
}

func TestFailureAndCancellationPreserveCleanupErrors(t *testing.T) {
	cleanupFailure := errors.New("cleanup failed")
	driver := &target{cleanupErr: cleanupFailure}
	ctx, cancel := context.WithCancel(context.Background())
	err := uitest.RunScenario(ctx, driver, func(context.Context) error { cancel(); return errors.New("scenario failed") })
	if !errors.Is(err, cleanupFailure) || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "scenario failed") {
		t.Fatalf("lost error: %v", err)
	}
	if !driver.cleaned || driver.cleanupCanceled {
		t.Fatalf("invalid cleanup context: %+v", driver)
	}
}

func TestSuiteContinuesAfterFailureAndRejectsMissedDeadlines(t *testing.T) {
	var out bytes.Buffer
	var ran []string
	err := uitest.RunSuite(context.Background(), []string{"bad", "good"}, time.Second, &out, func(_ context.Context, name string) error {
		ran = append(ran, name)
		if name == "bad" {
			return errors.New("bad result")
		}
		return nil
	})
	if err == nil || len(ran) != 2 || !strings.Contains(out.String(), "FAIL bad") || !strings.Contains(out.String(), "PASS good") {
		t.Fatalf("suite result: %v %v %s", err, ran, out.String())
	}
	out.Reset()
	err = uitest.RunSuite(context.Background(), []string{"timeout"}, 10*time.Millisecond, &out, func(ctx context.Context, _ string) error { <-ctx.Done(); return nil })
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(out.String(), "PASS") {
		t.Fatalf("expired callback reported success: %v %s", err, out.String())
	}
}
