// Package starlark runs scenarios through a replaceable uitest target.
// Its check mode and execution use the same builtins and language options.
package starlark

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/uitest"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

var options = &syntax.FileOptions{TopLevelControl: true, GlobalReassign: true}

type Harness struct {
	target uitest.Target
	out    io.Writer
}

func New(target uitest.Target, out io.Writer) *Harness {
	if out == nil {
		out = io.Discard
	}
	return &Harness{target: target, out: out}
}

// Check parses and resolves without starting GPUI or invoking any builtin.
func Check(path string) error {
	names := New(nil, nil).builtins(context.Background())
	_, _, err := starlark.SourceProgramOptions(options, path, nil, func(name string) bool { _, ok := names[name]; return ok })
	return err
}

func (h *Harness) RunFile(ctx context.Context, path string) error {
	return uitest.RunScenario(ctx, h.target, func(ctx context.Context) error {
		thread := &starlark.Thread{Name: path, Print: func(_ *starlark.Thread, message string) { fmt.Fprintln(h.out, message) }}
		thread.SetMaxExecutionSteps(1_000_000)
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-ctx.Done():
				thread.Cancel(ctx.Err().Error())
			case <-done:
			}
		}()
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := starlark.ExecFileOptions(options, thread, path, nil, h.builtins(ctx))
		var eval *starlark.EvalError
		if errors.As(err, &eval) {
			fmt.Fprintln(h.out, eval.Backtrace())
		}
		return err
	})
}

func (h *Harness) builtins(ctx context.Context) starlark.StringDict {
	d := starlark.StringDict{}
	d["open_page"] = builtin("open_page", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		fixture := "workbench"
		if err := starlark.UnpackArgs("open_page", args, kwargs, "fixture?", &fixture); err != nil {
			return nil, err
		}
		page, err := uitest.OpenPage(ctx, h.target, fixture)
		if err != nil {
			return nil, err
		}
		return h.page(ctx, page), nil
	})
	d["expect"] = builtin("expect", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value starlark.Value
		ms := 0
		if err := starlark.UnpackArgs("expect", args, kwargs, "locator", &value, "timeout_ms?", &ms); err != nil {
			return nil, err
		}
		locator, ok := value.(*locatorValue)
		if !ok {
			return nil, fmt.Errorf("expect requires a Locator")
		}
		loc, err := timed(locator.locator, ms)
		if err != nil {
			return nil, err
		}
		return assertionValue(ctx, loc), nil
	})
	add := func(name string, fn func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error)) {
		d[name] = starlark.NewBuiltin(name, fn)
	}
	add("log", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var message string
		if err := starlark.UnpackArgs("log", args, kwargs, "message", &message); err != nil {
			return nil, err
		}
		fmt.Fprintln(h.out, message)
		return starlark.None, nil
	})
	add("open_app", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		fixture := "workbench"
		if err := starlark.UnpackArgs("open_app", args, kwargs, "fixture?", &fixture); err != nil {
			return nil, err
		}
		id, err := h.target.Open(ctx, fixture)
		return starlark.String(id), err
	})
	add("close_app", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app string
		if err := starlark.UnpackArgs("close_app", args, kwargs, "app", &app); err != nil {
			return nil, err
		}
		return starlark.None, h.target.Close(ctx, app)
	})
	for _, name := range []string{"state", "inspect"} {
		add(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var app string
			element := "root"
			fields := []any{"app", &app}
			if b.Name() == "inspect" {
				fields = append(fields, "element?", &element)
			}
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, fields...); err != nil {
				return nil, err
			}
			var value map[string]any
			var err error
			if b.Name() == "state" {
				value, err = h.target.State(ctx, app)
			} else {
				value, err = h.target.Inspect(ctx, app, element)
			}
			if err != nil {
				return nil, err
			}
			return toStar(value)
		})
	}
	for _, name := range []string{"click", "mouse_down", "mouse_up", "mouse_move"} {
		add(name, func(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var app, element string
			var x, y starlark.Value = starlark.None, starlark.None
			pressed := false
			if err := starlark.UnpackArgs(b.Name(), args, kwargs, "app", &app, "element", &element, "x?", &x, "y?", &y, "pressed?", &pressed); err != nil {
				return nil, err
			}
			phase := map[string]pb.AutomationPointer_Phase{"click": pb.AutomationPointer_PHASE_CLICK, "mouse_down": pb.AutomationPointer_PHASE_DOWN, "mouse_up": pb.AutomationPointer_PHASE_UP, "mouse_move": pb.AutomationPointer_PHASE_MOVE}[b.Name()]
			p := &pb.AutomationPointer{Phase: phase, Pressed: pressed}
			if x != starlark.None || y != starlark.None {
				fx, okx := starlark.AsFloat(x)
				fy, oky := starlark.AsFloat(y)
				if !okx || !oky {
					return nil, fmt.Errorf("%s: x and y must both be numbers", b.Name())
				}
				p.Position = &pb.Point{X: float32(fx), Y: float32(fy)}
			}
			return starlark.None, uitest.Pointer(ctx, h.target, app, element, p)
		})
	}
	add("key", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, keys string
		if err := starlark.UnpackArgs("key", args, kwargs, "app", &app, "keystrokes", &keys); err != nil {
			return nil, err
		}
		return starlark.None, uitest.Key(ctx, h.target, app, keys)
	})
	add("type_text", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, text string
		if err := starlark.UnpackArgs("type_text", args, kwargs, "app", &app, "text", &text); err != nil {
			return nil, err
		}
		return starlark.None, uitest.TypeText(ctx, h.target, app, text)
	})
	add("compose", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, element, text string
		commit := false
		if err := starlark.UnpackArgs("compose", args, kwargs, "app", &app, "element", &element, "text", &text, "commit?", &commit); err != nil {
			return nil, err
		}
		return starlark.None, uitest.Compose(ctx, h.target, app, element, text, commit)
	})
	add("update", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var app, operation string
		value := ""
		if err := starlark.UnpackArgs("update", args, kwargs, "app", &app, "operation", &operation, "value?", &value); err != nil {
			return nil, err
		}
		return starlark.None, h.target.Update(ctx, app, operation, value)
	})
	add("clipboard", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value starlark.Value = starlark.None
		if err := starlark.UnpackArgs("clipboard", args, kwargs, "text?", &value); err != nil {
			return nil, err
		}
		var text *string
		if value != starlark.None {
			s, ok := starlark.AsString(value)
			if !ok {
				return nil, errors.New("clipboard: expected text")
			}
			text = &s
		}
		result, err := h.target.Clipboard(ctx, text)
		return starlark.String(result), err
	})
	add("assert_eq", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var got, want starlark.Value
		message := "values differ"
		if err := starlark.UnpackArgs("assert_eq", args, kwargs, "got", &got, "want", &want, "message?", &message); err != nil {
			return nil, err
		}
		equal, err := starlark.Equal(got, want)
		if err != nil {
			return nil, err
		}
		if !equal {
			return nil, fmt.Errorf("%s: got %s, want %s", message, got, want)
		}
		return starlark.None, nil
	})
	add("assert_true", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value starlark.Value
		message := "condition is false"
		if err := starlark.UnpackArgs("assert_true", args, kwargs, "condition", &value, "message?", &message); err != nil {
			return nil, err
		}
		if !value.Truth() {
			return nil, errors.New(message)
		}
		return starlark.None, nil
	})
	add("assert_contains", func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var text, substring string
		message := "substring missing"
		if err := starlark.UnpackArgs("assert_contains", args, kwargs, "text", &text, "substring", &substring, "message?", &message); err != nil {
			return nil, err
		}
		if !strings.Contains(text, substring) {
			return nil, fmt.Errorf("%s: %q does not contain %q", message, text, substring)
		}
		return starlark.None, nil
	})
	return d
}

func toStar(value any) (starlark.Value, error) {
	switch v := value.(type) {
	case nil:
		return starlark.None, nil
	case string:
		return starlark.String(v), nil
	case bool:
		return starlark.Bool(v), nil
	case int:
		return starlark.MakeInt(v), nil
	case uint64:
		return starlark.MakeUint64(v), nil
	case uint32:
		return starlark.MakeUint64(uint64(v)), nil
	case float32:
		return starlark.Float(v), nil
	case map[string]any:
		d := starlark.NewDict(len(v))
		for key, value := range v {
			s, err := toStar(value)
			if err != nil {
				return nil, err
			}
			if err := d.SetKey(starlark.String(key), s); err != nil {
				return nil, err
			}
		}
		return d, nil
	case []any:
		values := make([]starlark.Value, len(v))
		for i, value := range v {
			var err error
			values[i], err = toStar(value)
			if err != nil {
				return nil, err
			}
		}
		return starlark.NewList(values), nil
	default:
		return nil, fmt.Errorf("e2e: unsupported state value %T", value)
	}
}
