package uitest_test

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/uitest"
)

type locatorTarget struct {
	target
	matches                            []uitest.Match
	queries, inspections               int
	appearAfter                        int
	disabled, hidden, focused, checked bool
	value                              string
	changeValueAfter                   int
	inputErr                           error
}

func (t *locatorTarget) Query(context.Context, string, uitest.Selector) ([]uitest.Match, error) {
	t.queries++
	if t.queries < t.appearAfter {
		return nil, nil
	}
	return t.matches, nil
}
func (t *locatorTarget) Inspect(_ context.Context, _, element string) (map[string]any, error) {
	t.inspections++
	id := uint64(1)
	if strings.HasPrefix(element, "#") {
		id, _ = strconv.ParseUint(element[1:], 10, 64)
	}
	if t.changeValueAfter > 0 && t.inspections >= t.changeValueAfter {
		t.value = "new"
	}
	focused := uint64(0)
	if t.focused {
		focused = id
	}
	return map[string]any{"element_id": id, "focused_element_id": focused, "rendered": !t.hidden, "enabled": !t.disabled, "value": t.value, "text": t.value, "checked": t.checked}, nil
}
func (t *locatorTarget) Input(ctx context.Context, app, element string, request *pb.AutomationRequest) error {
	_ = t.target.Input(ctx, app, element, request)
	if t.inputErr != nil {
		return t.inputErr
	}
	if text := request.GetText(); text != nil {
		t.value = text.Text
	}
	if p := request.GetPointer(); p != nil && p.Phase == pb.AutomationPointer_PHASE_CLICK {
		t.checked = !t.checked
	}
	return nil
}
func (t *locatorTarget) Focus(context.Context, string, string) error  { t.focused = true; return nil }
func (t *locatorTarget) Reveal(context.Context, string, string) error { t.hidden = false; return nil }
func located() *locatorTarget                                         { return &locatorTarget{matches: []uitest.Match{{ElementID: 1}}} }

func TestLocatorsWaitForAppearanceAndReadCurrentState(t *testing.T) {
	driver := located()
	driver.appearAfter = 3
	locator := uitest.NewPage(driver, "app").GetByRole("textbox").WithTimeout(time.Second)
	if err := locator.Fill(context.Background(), "typed"); err != nil {
		t.Fatal(err)
	}
	if driver.queries < 3 || !driver.focused || driver.value != "typed" {
		t.Fatalf("appearance/focus/input: %+v", driver)
	}
	driver.changeValueAfter = driver.inspections + 3
	if err := uitest.Expect(locator).ToHaveValue(context.Background(), "new"); err != nil {
		t.Fatal(err)
	}
	if driver.value != "new" {
		t.Fatal("assertion observed a captured snapshot")
	}
}
func TestStrictLocatorsAndExplicitNthSelection(t *testing.T) {
	driver := located()
	driver.matches = append(driver.matches, uitest.Match{ElementID: 2})
	locator := uitest.NewPage(driver, "app").GetByRole("button")
	err := locator.Click(context.Background())
	if err == nil || !strings.Contains(err.Error(), "strict mode violation") || len(driver.requests) != 0 {
		t.Fatalf("ambiguous action: %v %v", err, driver.requests)
	}
	if err := locator.Nth(1).Click(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(driver.requests) != 1 {
		t.Fatal("explicit selection did not dispatch once")
	}
	if err := uitest.Expect(locator).ToHaveCount(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
}
func TestDisabledActionTimesOutAndDispatchedActionsAreNeverRetried(t *testing.T) {
	driver := located()
	driver.disabled = true
	locator := uitest.NewPage(driver, "app").Locator("name").WithTimeout(25 * time.Millisecond)
	err := locator.Fill(context.Background(), "forbidden")
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "disabled") || len(driver.requests) != 0 {
		t.Fatalf("disabled field mutated: %v %+v", err, driver.requests)
	}
	driver.disabled = false
	driver.inputErr = errors.New("reply lost after commitment")
	err = locator.Click(context.Background())
	if !errors.Is(err, driver.inputErr) || len(driver.requests) != 1 {
		t.Fatalf("action retried: %v %+v", err, driver.requests)
	}
}
func TestRevealCheckboxIdempotenceAndKeyboardAliases(t *testing.T) {
	driver := located()
	driver.hidden = true
	page := uitest.NewPage(driver, "app")
	checkbox := page.GetByRole("checkbox")
	if err := checkbox.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := checkbox.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	if driver.hidden || !driver.checked || len(driver.requests) != 1 {
		t.Fatalf("reveal/check was not idempotent: %+v", driver)
	}
	if err := checkbox.Uncheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if driver.checked || len(driver.requests) != 2 {
		t.Fatal("uncheck failed")
	}
	driver.requests = nil
	if err := page.Keyboard().Press(context.Background(), "ControlOrMeta+A ArrowLeft Shift+ArrowRight"); err != nil {
		t.Fatal(err)
	}
	first := "ctrl-a"
	if runtime.GOOS == "darwin" {
		first = "cmd-a"
	}
	for i, want := range []string{first, "left", "shift-right"} {
		if got := driver.requests[i].GetKey().Keystroke; got != want {
			t.Fatalf("key %d: %s, want %s", i, got, want)
		}
	}
}
func TestHiddenAssertionAcceptsNoMatches(t *testing.T) {
	driver := located()
	driver.matches = nil
	if err := uitest.Expect(uitest.NewPage(driver, "app").GetByText("absent")).ToBeHidden(context.Background()); err != nil {
		t.Fatal(err)
	}
}
