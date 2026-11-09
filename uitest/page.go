package uitest

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
)

// Selector describes a live query, rather than a captured native element.
type Selector struct {
	Kind, Value, Name string
	Exact             bool
}
type Match struct {
	ElementID uint64
	Element   string
}

// QueryTarget is an optional extension for live semantic selectors.
type QueryTarget interface {
	Query(context.Context, string, Selector) ([]Match, error)
}
type FocusTarget interface {
	Focus(context.Context, string, string) error
}
type RevealTarget interface {
	Reveal(context.Context, string, string) error
}
type RoleOptions struct {
	Name  string
	Exact bool
}
type TextOptions struct{ Exact bool }

// Page drives one application through live locators. Use it on the target's
// state-owning goroutine. Default action/assertion timeout is five seconds.
type Page struct {
	target  Target
	app     string
	timeout time.Duration
}

func NewPage(target Target, app string) *Page {
	return &Page{target: target, app: app, timeout: 5 * time.Second}
}
func OpenPage(ctx context.Context, target Target, fixture string) (*Page, error) {
	app, err := target.Open(ctx, fixture)
	if err != nil {
		return nil, err
	}
	return NewPage(target, app), nil
}
func (p *Page) ID() string                                        { return p.app }
func (p *Page) Close(ctx context.Context) error                   { return p.target.Close(ctx, p.app) }
func (p *Page) State(ctx context.Context) (map[string]any, error) { return p.target.State(ctx, p.app) }
func (p *Page) Update(ctx context.Context, operation, value string) error {
	return p.target.Update(ctx, p.app, operation, value)
}
func (p *Page) SetDefaultTimeout(timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("uitest: timeout must be positive")
	}
	p.timeout = timeout
	return nil
}
func (p *Page) Locator(name string) *Locator {
	return &Locator{page: p, selector: Selector{Kind: "testid", Value: name}}
}
func (p *Page) GetByTestID(name string) *Locator { return p.Locator(name) }
func (p *Page) GetByRole(role string, options ...RoleOptions) *Locator {
	s := Selector{Kind: "role", Value: role}
	if len(options) > 0 {
		s.Name = options[0].Name
		s.Exact = options[0].Exact
	}
	return &Locator{page: p, selector: s}
}
func (p *Page) GetByText(text string, options ...TextOptions) *Locator {
	s := Selector{Kind: "text", Value: text}
	if len(options) > 0 {
		s.Exact = options[0].Exact
	}
	return &Locator{page: p, selector: s}
}
func (p *Page) Keyboard() *Keyboard { return &Keyboard{page: p} }

type Keyboard struct{ page *Page }

func (k *Keyboard) Press(ctx context.Context, key string) error {
	return Key(ctx, k.page.target, k.page.app, normalizeKeys(key))
}
func (k *Keyboard) InsertText(ctx context.Context, text string) error {
	return TypeText(ctx, k.page.target, k.page.app, text)
}

var roles = map[string]pb.AccessibleRole{"generic": pb.AccessibleRole_ACCESSIBLE_ROLE_GENERIC, "button": pb.AccessibleRole_ACCESSIBLE_ROLE_BUTTON, "checkbox": pb.AccessibleRole_ACCESSIBLE_ROLE_CHECKBOX, "textbox": pb.AccessibleRole_ACCESSIBLE_ROLE_TEXT_INPUT, "text": pb.AccessibleRole_ACCESSIBLE_ROLE_TEXT, "list": pb.AccessibleRole_ACCESSIBLE_ROLE_LIST, "listitem": pb.AccessibleRole_ACCESSIBLE_ROLE_LIST_ITEM, "link": pb.AccessibleRole_ACCESSIBLE_ROLE_LINK, "img": pb.AccessibleRole_ACCESSIBLE_ROLE_IMAGE, "slider": pb.AccessibleRole_ACCESSIBLE_ROLE_SLIDER, "dialog": pb.AccessibleRole_ACCESSIBLE_ROLE_DIALOG}

// ParseRole resolves the shared locator vocabulary for a target adapter.
// Unknown names are rejected rather than silently querying a generic role.
func ParseRole(name string) (pb.AccessibleRole, bool) {
	role, ok := roles[name]
	return role, ok
}

// Locator re-resolves before every action/assertion. Actions require one match,
// visible geometry and (where applicable) enabled state. Dispatched mutations
// are never retried: cancellation may race commitment at the Sable boundary.
type Locator struct {
	page     *Page
	selector Selector
	ordinal  *int
	timeout  time.Duration
}

func (l *Locator) String() string         { return fmt.Sprintf("%s(%q)", l.selector.Kind, l.selector.Value) }
func (l *Locator) Nth(index int) *Locator { result := *l; result.ordinal = &index; return &result }
func (l *Locator) First() *Locator        { return l.Nth(0) }
func (l *Locator) Last() *Locator         { return l.Nth(-1) }
func (l *Locator) WithTimeout(timeout time.Duration) *Locator {
	result := *l
	result.timeout = timeout
	return &result
}
func (l *Locator) matches(ctx context.Context) ([]Match, error) {
	if l.page == nil || l.page.target == nil {
		return nil, errors.New("uitest: missing page target")
	}
	if l.selector.Kind == "role" {
		if _, ok := ParseRole(l.selector.Value); !ok {
			return nil, fmt.Errorf("uitest: unsupported role %q", l.selector.Value)
		}
	}
	var values []Match
	if query, ok := l.page.target.(QueryTarget); ok {
		var err error
		values, err = query.Query(ctx, l.page.app, l.selector)
		if err != nil {
			return nil, err
		}
	} else if l.selector.Kind == "testid" {
		state, err := l.page.target.Inspect(ctx, l.page.app, l.selector.Value)
		if err != nil {
			return nil, err
		}
		values = []Match{{Element: l.selector.Value, ElementID: id(state["element_id"])}}
	} else {
		return nil, errors.New("uitest: target does not support semantic selectors")
	}
	if l.ordinal != nil {
		index := *l.ordinal
		if index == -1 {
			index = len(values) - 1
		}
		if index < 0 || index >= len(values) {
			return nil, nil
		}
		values = values[index : index+1]
	}
	for i := range values {
		if values[i].Element == "" {
			values[i].Element = fmt.Sprintf("#%d", values[i].ElementID)
		}
	}
	return values, nil
}
func id(value any) uint64 {
	switch v := value.(type) {
	case uint64:
		return v
	case uint32:
		return uint64(v)
	case int:
		if v >= 0 {
			return uint64(v)
		}
	}
	return 0
}
func (l *Locator) one(ctx context.Context) (Match, map[string]any, bool, error) {
	values, err := l.matches(ctx)
	if err != nil {
		return Match{}, nil, false, err
	}
	if len(values) > 1 {
		return Match{}, nil, false, fmt.Errorf("uitest: strict mode violation: %s matched %d elements; use Nth, First or Last", l, len(values))
	}
	if len(values) == 0 {
		return Match{}, nil, false, nil
	}
	state, err := l.page.target.Inspect(ctx, l.page.app, values[0].Element)
	return values[0], state, true, err
}
func (l *Locator) wait(ctx context.Context, description string, condition func(context.Context) (bool, string, error)) error {
	timeout := l.timeout
	if timeout == 0 {
		timeout = l.page.timeout
	}
	if timeout <= 0 {
		return errors.New("uitest: timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	last := "not observed"
	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("uitest: %s %s (last: %s): %w", l, description, last, err)
		}
		ok, detail, err := condition(ctx)
		if err != nil {
			return fmt.Errorf("uitest: %s %s: %w", l, description, err)
		}
		if ok {
			return nil
		}
		last = detail
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
func enabled(state map[string]any) bool { value, ok := state["enabled"].(bool); return !ok || value }
func visible(state map[string]any) bool { value, _ := state["rendered"].(bool); return value }
func (l *Locator) ready(ctx context.Context, requireEnabled bool) (Match, map[string]any, error) {
	var match Match
	var state map[string]any
	err := l.wait(ctx, "to be ready", func(ctx context.Context) (bool, string, error) {
		m, s, found, err := l.one(ctx)
		if err != nil {
			return false, "", err
		}
		if !found {
			return false, "0 elements", nil
		}
		if !visible(s) {
			if reveal, ok := l.page.target.(RevealTarget); ok {
				if err := reveal.Reveal(ctx, l.page.app, m.Element); err != nil {
					return false, "", err
				}
				s, err = l.page.target.Inspect(ctx, l.page.app, m.Element)
				if err != nil {
					return false, "", err
				}
			}
		}
		if !visible(s) {
			return false, "not visible", nil
		}
		if requireEnabled && !enabled(s) {
			return false, "disabled", nil
		}
		match = m
		state = s
		return true, "", nil
	})
	return match, state, err
}
func (l *Locator) Count(ctx context.Context) (int, error) {
	m, err := l.matches(ctx)
	return len(m), err
}
func (l *Locator) Inspect(ctx context.Context) (map[string]any, error) {
	var state map[string]any
	err := l.wait(ctx, "to be attached", func(ctx context.Context) (bool, string, error) {
		_, s, found, err := l.one(ctx)
		state = s
		return found, "0 elements", err
	})
	return state, err
}
func (l *Locator) Click(ctx context.Context) error {
	m, _, err := l.ready(ctx, true)
	if err != nil {
		return err
	}
	return Click(ctx, l.page.target, l.page.app, m.Element)
}
func (l *Locator) Hover(ctx context.Context) error {
	m, _, err := l.ready(ctx, false)
	if err != nil {
		return err
	}
	return Pointer(ctx, l.page.target, l.page.app, m.Element, &pb.AutomationPointer{Phase: pb.AutomationPointer_PHASE_MOVE})
}
func (l *Locator) focus(ctx context.Context, m Match) error {
	focus, ok := l.page.target.(FocusTarget)
	if !ok {
		return errors.New("uitest: target does not support programmatic focus")
	}
	return focus.Focus(ctx, l.page.app, m.Element)
}
func (l *Locator) Focus(ctx context.Context) error {
	m, _, err := l.ready(ctx, true)
	if err != nil {
		return err
	}
	return l.focus(ctx, m)
}
func (l *Locator) Fill(ctx context.Context, value string) error {
	m, s, err := l.ready(ctx, true)
	if err != nil {
		return err
	}
	if _, ok := s["value"]; !ok {
		return errors.New("uitest: fill requires a native text input")
	}
	if err := l.focus(ctx, m); err != nil {
		return err
	}
	if err := Key(ctx, l.page.target, l.page.app, "ctrl-a"); err != nil {
		return err
	}
	return TypeText(ctx, l.page.target, l.page.app, value)
}
func (l *Locator) Press(ctx context.Context, key string) error {
	m, _, err := l.ready(ctx, true)
	if err != nil {
		return err
	}
	if err := l.focus(ctx, m); err != nil {
		return err
	}
	return l.page.Keyboard().Press(ctx, key)
}
func (l *Locator) InputValue(ctx context.Context) (string, error) {
	s, err := l.Inspect(ctx)
	if err != nil {
		return "", err
	}
	v, ok := s["value"].(string)
	if !ok {
		return "", errors.New("uitest: input_value requires a native text input")
	}
	return v, nil
}
func (l *Locator) Check(ctx context.Context) error   { return l.SetChecked(ctx, true) }
func (l *Locator) Uncheck(ctx context.Context) error { return l.SetChecked(ctx, false) }
func (l *Locator) SetChecked(ctx context.Context, want bool) error {
	m, s, err := l.ready(ctx, true)
	if err != nil {
		return err
	}
	got, ok := s["checked"].(bool)
	if !ok {
		return errors.New("uitest: check requires checkbox metadata")
	}
	if got != want {
		if err := Click(ctx, l.page.target, l.page.app, m.Element); err != nil {
			return err
		}
	}
	return Expect(l).Checked(ctx, want)
}

// Assertions poll current native state and retain the last observation in timeout errors.
type Assertions struct{ locator *Locator }

func Expect(locator *Locator) *Assertions { return &Assertions{locator: locator} }
func (a *Assertions) ToHaveCount(ctx context.Context, want int) error {
	return a.locator.wait(ctx, fmt.Sprintf("to have count %d", want), func(ctx context.Context) (bool, string, error) {
		count, err := a.locator.Count(ctx)
		return count == want, fmt.Sprintf("count %d", count), err
	})
}
func (a *Assertions) state(ctx context.Context, description string, missingOK bool, test func(map[string]any) (bool, string, error)) error {
	return a.locator.wait(ctx, description, func(ctx context.Context) (bool, string, error) {
		_, s, found, err := a.locator.one(ctx)
		if err != nil {
			return false, "", err
		}
		if !found {
			return missingOK, "0 elements", nil
		}
		return test(s)
	})
}
func (a *Assertions) ToBeVisible(ctx context.Context) error {
	return a.state(ctx, "to be visible", false, func(s map[string]any) (bool, string, error) {
		return visible(s), fmt.Sprintf("visible=%v", visible(s)), nil
	})
}
func (a *Assertions) ToBeHidden(ctx context.Context) error {
	return a.state(ctx, "to be hidden", true, func(s map[string]any) (bool, string, error) {
		return !visible(s), fmt.Sprintf("visible=%v", visible(s)), nil
	})
}
func (a *Assertions) Enabled(ctx context.Context, want bool) error {
	return a.state(ctx, fmt.Sprintf("enabled=%v", want), false, func(s map[string]any) (bool, string, error) {
		return enabled(s) == want, fmt.Sprintf("enabled=%v", enabled(s)), nil
	})
}
func (a *Assertions) ToBeEnabled(ctx context.Context) error  { return a.Enabled(ctx, true) }
func (a *Assertions) ToBeDisabled(ctx context.Context) error { return a.Enabled(ctx, false) }
func (a *Assertions) ToBeFocused(ctx context.Context) error {
	return a.state(ctx, "to be focused", false, func(s map[string]any) (bool, string, error) {
		got := id(s["focused_element_id"])
		return got == id(s["element_id"]) && got != 0, fmt.Sprintf("focused element %d", got), nil
	})
}
func (a *Assertions) ToHaveText(ctx context.Context, want string) error {
	return a.state(ctx, fmt.Sprintf("to have text %q", want), false, func(s map[string]any) (bool, string, error) {
		got, _ := s["text"].(string)
		return strings.Join(strings.Fields(got), " ") == strings.Join(strings.Fields(want), " "), fmt.Sprintf("text %q", got), nil
	})
}
func (a *Assertions) ToHaveValue(ctx context.Context, want string) error {
	return a.state(ctx, fmt.Sprintf("to have value %q", want), false, func(s map[string]any) (bool, string, error) {
		got, ok := s["value"].(string)
		if !ok {
			return false, "", errors.New("uitest: value assertion requires a native text input")
		}
		return got == want, fmt.Sprintf("value %q", got), nil
	})
}
func (a *Assertions) Checked(ctx context.Context, want bool) error {
	return a.state(ctx, fmt.Sprintf("checked=%v", want), false, func(s map[string]any) (bool, string, error) {
		got, ok := s["checked"].(bool)
		if !ok {
			return false, "", errors.New("uitest: checked assertion requires checkbox metadata")
		}
		return got == want, fmt.Sprintf("checked=%v", got), nil
	})
}
func (a *Assertions) ToBeChecked(ctx context.Context) error    { return a.Checked(ctx, true) }
func (a *Assertions) NotToBeChecked(ctx context.Context) error { return a.Checked(ctx, false) }

func normalizeKeys(keys string) string {
	aliases := map[string]string{"ArrowLeft": "left", "ArrowRight": "right", "ArrowUp": "up", "ArrowDown": "down", "Enter": "enter", "Backspace": "backspace", "Delete": "delete", "Home": "home", "End": "end", "Tab": "tab", "Escape": "escape", "Space": "space"}
	strokes := strings.Fields(keys)
	for i, stroke := range strokes {
		parts := strings.Split(stroke, "+")
		for j, part := range parts {
			switch part {
			case "ControlOrMeta":
				if runtime.GOOS == "darwin" {
					part = "cmd"
				} else {
					part = "ctrl"
				}
			case "Control":
				part = "ctrl"
			case "Meta":
				part = "cmd"
			case "Shift":
				part = "shift"
			case "Alt":
				part = "alt"
			}
			if alias, ok := aliases[part]; ok {
				part = alias
			}
			if len(parts) > 1 && j == len(parts)-1 && len(part) == 1 {
				part = strings.ToLower(part)
			}
			parts[j] = part
		}
		strokes[i] = strings.Join(parts, "-")
	}
	return strings.Join(strokes, " ")
}
