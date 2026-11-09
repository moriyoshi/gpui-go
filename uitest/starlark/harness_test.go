package starlark

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
)

type testTarget struct {
	opened, cleaned, cleanupCanceled bool
	clicks                           int
	text                             string
	block                            bool
	cleanupErr                       error
}

func (t *testTarget) Open(context.Context, string) (string, error) {
	t.opened = true
	return "app-1", nil
}
func (t *testTarget) Close(context.Context, string) error { t.opened = false; return nil }
func (t *testTarget) Input(ctx context.Context, _, _ string, request *pb.AutomationRequest) error {
	if t.block {
		<-ctx.Done()
		return ctx.Err()
	}
	if request.GetPointer() != nil {
		t.clicks++
	}
	if request.GetText() != nil {
		t.text = request.GetText().Text
	}
	return nil
}
func (t *testTarget) Inspect(context.Context, string, string) (map[string]any, error) {
	return map[string]any{"value": t.text, "selection": []any{uint32(0), uint32(2)}}, nil
}
func (t *testTarget) State(context.Context, string) (map[string]any, error) {
	return map[string]any{"clicks": t.clicks, "text": t.text}, nil
}
func (t *testTarget) Update(context.Context, string, string, string) error { return nil }
func (t *testTarget) Clipboard(context.Context, *string) (string, error)   { return "clipboard", nil }
func (t *testTarget) Cleanup(ctx context.Context) error {
	t.cleaned = true
	t.opened = false
	t.cleanupCanceled = ctx.Err() != nil
	return t.cleanupErr
}
func script(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.star")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestCheckResolvesAndExecutionUsesTheSameLanguage(t *testing.T) {
	path := script(t, `app = open_app()
for i in range(3):
    click(app, "increment")
app = app
type_text(app, "e\u0301🙂")
assert_eq(state(app)["clicks"], 3)
assert_eq(inspect(app, "name")["value"], "e\u0301🙂")
assert_eq(inspect(app, "name")["selection"], [0, 2])
assert_contains(clipboard(), "board")
log("scenario completed")
`)
	if err := Check(path); err != nil {
		t.Fatal(err)
	}
	target := &testTarget{}
	var out bytes.Buffer
	if err := New(target, &out).RunFile(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if !target.cleaned || target.opened || !strings.Contains(out.String(), "scenario completed") {
		t.Fatalf("cleanup/logging failed: %+v %q", target, out.String())
	}
	bad := script(t, `app = opne_app()`)
	if err := Check(bad); err == nil || !strings.Contains(err.Error(), "opne_app") {
		t.Fatalf("unknown builtin accepted: %v", err)
	}
}
func TestAssertionFailureStopsActionsAndStillCleansUp(t *testing.T) {
	cleanupErr := errors.New("cleanup failure")
	target := &testTarget{cleanupErr: cleanupErr}
	path := script(t, `app = open_app()
click(app, "increment")
assert_eq(state(app)["clicks"], 2, "deliberately wrong counter")
click(app, "increment")
`)
	err := New(target, nil).RunFile(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "deliberately wrong counter") || !errors.Is(err, cleanupErr) {
		t.Fatalf("missing assertion/cleanup error: %v", err)
	}
	if target.clicks != 1 || !target.cleaned || target.opened {
		t.Fatalf("actions continued or cleanup omitted: %+v", target)
	}
}
func TestCanceledBuiltinGetsIndependentCleanupDeadline(t *testing.T) {
	target := &testTarget{block: true}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := New(target, nil).RunFile(ctx, script(t, `app = open_app()
click(app, "blocked")
`))
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("missing timeout: %v", err)
	}
	if !target.cleaned || target.cleanupCanceled {
		t.Fatalf("cleanup inherited canceled context: %+v", target)
	}
}
func TestInterpreterStepBudgetAndCleanup(t *testing.T) {
	target := &testTarget{}
	err := New(target, nil).RunFile(context.Background(), script(t, `for i in range(1000000000):
    x = i
`))
	if err == nil || !strings.Contains(err.Error(), "too many steps") || !target.cleaned {
		t.Fatalf("step budget/cleanup: %v %+v", err, target)
	}
}
func TestMaintainedScenariosResolve(t *testing.T) {
	paths, err := filepath.Glob("../../e2e/scenarios/*.star")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no scenarios")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			if err := Check(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}
