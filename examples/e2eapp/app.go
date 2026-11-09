// Package e2eapp supplies application presentation and fixtures to gpui-e2e.
// The reusable harness knows only its Application interface and named elements.
package e2eapp

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/scene"
)

// App owns the workbench's presentation, state and native resources.
type App struct {
	client                                               *clientlib.Client
	window                                               *clientlib.Window
	model                                                *clientlib.Model
	subscription                                         *clientlib.Subscription
	handlers                                             []uint64
	increment, changeName, changeOther, reset, selectRow uint64
	count                                                int
	name, other                                          scene.InputState
	composing, disabled                                  bool
	selected                                             int
	checked                                              bool
	toggle                                               uint64
}

// Open constructs a workbench. The caller owns Close and event dispatch.
func Open(ctx context.Context, client *clientlib.Client, fixture string) (_ *App, err error) {
	if fixture != "workbench" {
		return nil, fmt.Errorf("unknown fixture %q", fixture)
	}
	a := &App{client: client, selected: -1}
	defer func() {
		if err != nil {
			_ = a.Close(context.WithoutCancel(ctx))
		}
	}()
	a.model, err = client.CreateModel(ctx, []byte("0"))
	if err != nil {
		return nil, err
	}
	a.increment = a.on(func(ctx context.Context, _ *clientlib.Event) error {
		state, err := a.model.Read(ctx)
		if err != nil {
			return err
		}
		value, err := strconv.Atoi(string(state.Value))
		if err != nil {
			return err
		}
		_, err = a.model.Update(ctx, state.Revision, []byte(strconv.Itoa(value+1)))
		return err
	})
	a.changeName = a.on(func(ctx context.Context, event *clientlib.Event) error {
		change := event.GetTextChanged()
		if a.name.Accept(change) {
			a.composing = change.Composing
			return a.commit(ctx)
		}
		return nil
	})
	a.changeOther = a.on(func(ctx context.Context, event *clientlib.Event) error {
		if a.other.Accept(event.GetTextChanged()) {
			return a.commit(ctx)
		}
		return nil
	})
	a.reset = a.on(func(ctx context.Context, _ *clientlib.Event) error {
		return a.Update(ctx, "replace", "Reset 日本語")
	})
	a.toggle = a.on(func(ctx context.Context, _ *clientlib.Event) error { a.checked = !a.checked; return a.commit(ctx) })
	a.selectRow = a.on(func(ctx context.Context, event *clientlib.Event) error {
		a.selected = int(event.ElementId) - 1000
		return a.commit(ctx)
	})
	a.window, err = client.OpenWindow(ctx, clientlib.WindowOptions{Title: "GPUI Starlark E2E workbench", Width: 760, Height: 800}, a.render())
	if err != nil {
		return nil, err
	}
	a.subscription, err = a.model.Observe(ctx, func(ctx context.Context, state *clientlib.ModelState) error {
		value, err := strconv.Atoi(string(state.Value))
		if err != nil {
			return err
		}
		a.count = value
		return a.commit(ctx)
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}
func (a *App) on(handler func(context.Context, *clientlib.Event) error) uint64 {
	id := a.client.OnEvent(handler)
	a.handlers = append(a.handlers, id)
	return id
}
func (a *App) Window() *clientlib.Window { return a.window }
func (a *App) Element(name string) (uint64, bool) {
	ids := map[string]uint64{"root": 1, "increment": 2, "count": 3, "name": 10, "mirror": 11, "reset": 12, "disabled": 13, "other": 14, "notifications": 20, "list": 50, "selected": 51}
	if id, ok := ids[name]; ok {
		return id, true
	}
	var row int
	if n, err := fmt.Sscanf(name, "row-%d", &row); err == nil && n == 1 && name == fmt.Sprintf("row-%d", row) && row >= 0 && row < 1000 {
		return uint64(1000 + row), true
	}
	return 0, false
}
func (a *App) State() map[string]any {
	return map[string]any{"count": a.count, "name": a.name.Value, "other": a.other.Value, "epoch": a.name.Epoch, "composing": a.composing, "disabled": a.disabled, "selected": a.selected, "checked": a.checked}
}
func (a *App) Update(ctx context.Context, operation, value string) error {
	switch operation {
	case "replace":
		a.name.Reset(value)
		a.composing = false
	case "rerender":
	case "disable":
		a.disabled = value != "false"
	case "scroll_last":
		return a.window.ScrollTo(ctx, 50, 999)
	default:
		return fmt.Errorf("unknown fixture update %q", operation)
	}
	return a.commit(ctx)
}
func (a *App) commit(ctx context.Context) error { return a.window.Set(ctx, a.render()) }
func (a *App) Close(ctx context.Context) error {
	var errs []error
	for _, id := range a.handlers {
		a.client.RemoveHandler(id)
	}
	a.handlers = nil
	if a.subscription != nil {
		errs = append(errs, a.subscription.Close(ctx))
		a.subscription = nil
	}
	if a.window != nil {
		errs = append(errs, a.window.Close(ctx))
		a.window = nil
	}
	if a.model != nil {
		errs = append(errs, a.model.Release(ctx))
		a.model = nil
	}
	return errors.Join(errs...)
}
func button(id uint64, text string, handler uint64) *scene.Node {
	return scene.Text(id, text).OnClick(handler).Focusable(0).Accessible(text, &scene.Accessibility{Role: scene.RoleButton}).Style(&scene.Style{Width: scene.Pixels(260), Height: scene.Pixels(36), Padding: scene.Padding(8), Background: scene.RGB(0x334155), Cursor: scene.CursorPointer.Enum()})
}
func (a *App) render() *scene.Node {
	rows := make([]*scene.Node, 1000)
	for i := range rows {
		rows[i] = scene.Text(uint64(1000+i), fmt.Sprintf("Row %d", i)).OnClick(a.selectRow).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Pixels(32), Padding: scene.Padding(6)})
	}
	field := func(id uint64, state scene.InputState, handler uint64, disabled bool) *scene.Node {
		label := map[uint64]string{10: "Name", 13: "Locked field", 14: "Other"}[id]
		return scene.TextInput(id, state, scene.TextInputOptions{OnChange: handler, Disabled: disabled, Placeholder: "Type here"}).Accessible(label, &scene.Accessibility{Role: scene.RoleTextInput}).Style(&scene.Style{Width: scene.Pixels(400), Height: scene.Pixels(42), Padding: scene.Padding(6), Background: scene.RGB(0x1e293b), BorderColor: scene.RGB(0x64748b), BorderWidth: scene.Ptr[float32](1)})
	}
	return scene.Div(1,
		scene.Text(4, "Starlark E2E workbench"),
		button(2, "Increment native model", a.increment),
		scene.Text(3, fmt.Sprintf("Count: %d", a.count)),
		field(10, a.name, a.changeName, a.disabled),
		scene.Text(11, "Name: "+a.name.Value),
		button(12, "Replace input", a.reset),
		field(13, scene.InputState{Value: "Locked"}, 0, true),
		field(14, a.other, a.changeOther, false),
		button(20, "Enable notifications", a.toggle).Accessible("Enable notifications", &scene.Accessibility{Role: scene.RoleCheckbox, Checked: scene.Ptr[bool](a.checked)}),
		scene.Text(51, fmt.Sprintf("Selected: %d", a.selected)),
		scene.UniformList(50, 32, rows...).Style(&scene.Style{Width: scene.Pixels(450), Height: scene.Pixels(220)}),
	).Interact(&scene.Interaction{Focusable: true, KeyContext: "Workbench"}).
		BindKeyWithContext("cmd-k cmd-s", "Workbench", &scene.Action{Name: "increment"}, a.increment).
		BindKey("cmd-j", &scene.Action{Name: "increment"}, a.increment).
		BindKey("ctrl-j", &scene.Action{Name: "increment"}, a.increment).
		Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100), FlexDirection: scene.FlexColumn.Enum(), Padding: scene.Padding(24), Gap: scene.Pixels(10), Background: scene.RGB(0x0f172a), TextColor: scene.RGB(0xe2e8f0), FontSize: scene.Ptr[float32](15)})
}
