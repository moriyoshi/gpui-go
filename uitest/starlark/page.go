package starlark

import (
	"context"
	"fmt"
	"time"

	"github.com/moriyoshi/gpui-go/uitest"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func builtin(name string, fn func(starlark.Tuple, []starlark.Tuple) (starlark.Value, error)) *starlark.Builtin {
	return starlark.NewBuiltin(name, func(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return fn(args, kwargs)
	})
}
func timed(locator *uitest.Locator, ms int) (*uitest.Locator, error) {
	if ms < 0 {
		return nil, fmt.Errorf("timeout_ms must be positive")
	}
	if ms > 0 {
		return locator.WithTimeout(time.Duration(ms) * time.Millisecond), nil
	}
	return locator, nil
}
func (h *Harness) page(ctx context.Context, page *uitest.Page) starlark.Value {
	attrs := starlark.StringDict{}
	for _, name := range []string{"locator", "get_by_test_id", "get_by_text", "get_by_role"} {
		attrs[name] = builtin(name, func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var value string
			nameFilter := ""
			exact := false
			argument := map[string]string{"locator": "selector", "get_by_test_id": "test_id", "get_by_text": "text", "get_by_role": "role"}[name]
			fields := []any{argument, &value}
			if name == "get_by_role" {
				fields = append(fields, "name?", &nameFilter, "exact?", &exact)
			} else if name == "get_by_text" {
				fields = append(fields, "exact?", &exact)
			}
			if err := starlark.UnpackArgs(name, args, kwargs, fields...); err != nil {
				return nil, err
			}
			var locator *uitest.Locator
			switch name {
			case "get_by_role":
				locator = page.GetByRole(value, uitest.RoleOptions{Name: nameFilter, Exact: exact})
			case "get_by_text":
				locator = page.GetByText(value, uitest.TextOptions{Exact: exact})
			default:
				locator = page.GetByTestID(value)
			}
			return &locatorValue{ctx: ctx, locator: locator}, nil
		})
	}
	attrs["close"] = builtin("close", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackArgs("close", args, kwargs); err != nil {
			return nil, err
		}
		return starlark.None, page.Close(ctx)
	})
	attrs["state"] = builtin("state", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackArgs("state", args, kwargs); err != nil {
			return nil, err
		}
		value, err := page.State(ctx)
		if err != nil {
			return nil, err
		}
		return toStar(value)
	})
	attrs["update"] = builtin("update", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var operation string
		value := ""
		if err := starlark.UnpackArgs("update", args, kwargs, "operation", &operation, "value?", &value); err != nil {
			return nil, err
		}
		return starlark.None, page.Update(ctx, operation, value)
	})
	attrs["set_default_timeout"] = builtin("set_default_timeout", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var ms int
		if err := starlark.UnpackArgs("set_default_timeout", args, kwargs, "timeout_ms", &ms); err != nil {
			return nil, err
		}
		return starlark.None, page.SetDefaultTimeout(time.Duration(ms) * time.Millisecond)
	})
	keys := starlark.StringDict{}
	keys["press"] = builtin("press", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value string
		if err := starlark.UnpackArgs("press", args, kwargs, "key", &value); err != nil {
			return nil, err
		}
		return starlark.None, page.Keyboard().Press(ctx, value)
	})
	keys["insert_text"] = builtin("insert_text", func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var value string
		if err := starlark.UnpackArgs("insert_text", args, kwargs, "text", &value); err != nil {
			return nil, err
		}
		return starlark.None, page.Keyboard().InsertText(ctx, value)
	})
	attrs["keyboard"] = starlarkstruct.FromStringDict(starlark.String("Keyboard"), keys)
	return starlarkstruct.FromStringDict(starlark.String("Page"), attrs)
}

type locatorValue struct {
	ctx     context.Context
	locator *uitest.Locator
}

func (v *locatorValue) String() string      { return v.locator.String() }
func (*locatorValue) Type() string          { return "Locator" }
func (*locatorValue) Freeze()               {}
func (*locatorValue) Truth() starlark.Bool  { return starlark.True }
func (*locatorValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: Locator") }
func (*locatorValue) AttrNames() []string {
	return []string{"click", "fill", "press", "hover", "focus", "check", "uncheck", "input_value", "inspect", "count", "nth", "first", "last"}
}
func (v *locatorValue) Attr(name string) (starlark.Value, error) {
	if name == "first" {
		return &locatorValue{v.ctx, v.locator.First()}, nil
	}
	if name == "last" {
		return &locatorValue{v.ctx, v.locator.Last()}, nil
	}
	if name == "nth" {
		return builtin(name, func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var index int
			if err := starlark.UnpackArgs(name, args, kwargs, "index", &index); err != nil {
				return nil, err
			}
			if index < 0 {
				return nil, fmt.Errorf("nth index must be nonnegative")
			}
			return &locatorValue{v.ctx, v.locator.Nth(index)}, nil
		}), nil
	}
	for _, known := range []string{"click", "fill", "press", "hover", "focus", "check", "uncheck", "input_value", "inspect", "count"} {
		if name != known {
			continue
		}
		return builtin(name, func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			ms := 0
			value := ""
			fields := []any{}
			if name == "fill" || name == "press" {
				argument := "value"
				if name == "press" {
					argument = "key"
				}
				fields = append(fields, argument, &value)
			}
			fields = append(fields, "timeout_ms?", &ms)
			if err := starlark.UnpackArgs(name, args, kwargs, fields...); err != nil {
				return nil, err
			}
			locator, err := timed(v.locator, ms)
			if err != nil {
				return nil, err
			}
			switch name {
			case "click":
				err = locator.Click(v.ctx)
			case "fill":
				err = locator.Fill(v.ctx, value)
			case "press":
				err = locator.Press(v.ctx, value)
			case "hover":
				err = locator.Hover(v.ctx)
			case "focus":
				err = locator.Focus(v.ctx)
			case "check":
				err = locator.Check(v.ctx)
			case "uncheck":
				err = locator.Uncheck(v.ctx)
			case "count":
				count, err := locator.Count(v.ctx)
				return starlark.MakeInt(count), err
			case "input_value":
				value, err := locator.InputValue(v.ctx)
				return starlark.String(value), err
			case "inspect":
				value, err := locator.Inspect(v.ctx)
				if err != nil {
					return nil, err
				}
				return toStar(value)
			}
			return starlark.None, err
		}), nil
	}
	return nil, nil
}

func assertionValue(ctx context.Context, locator *uitest.Locator) starlark.Value {
	attrs := starlark.StringDict{}
	for _, name := range []string{"to_have_count", "to_have_text", "to_have_value", "to_be_visible", "to_be_hidden", "to_be_enabled", "to_be_disabled", "to_be_focused", "to_be_checked", "not_to_be_checked"} {
		attrs[name] = builtin(name, func(args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			ms := 0
			value := ""
			count := 0
			fields := []any{}
			if name == "to_have_count" {
				fields = append(fields, "count", &count)
			} else if name == "to_have_text" || name == "to_have_value" {
				fields = append(fields, "value", &value)
			}
			fields = append(fields, "timeout_ms?", &ms)
			if err := starlark.UnpackArgs(name, args, kwargs, fields...); err != nil {
				return nil, err
			}
			loc, err := timed(locator, ms)
			if err != nil {
				return nil, err
			}
			assert := uitest.Expect(loc)
			switch name {
			case "to_have_count":
				err = assert.ToHaveCount(ctx, count)
			case "to_have_text":
				err = assert.ToHaveText(ctx, value)
			case "to_have_value":
				err = assert.ToHaveValue(ctx, value)
			case "to_be_visible":
				err = assert.ToBeVisible(ctx)
			case "to_be_hidden":
				err = assert.ToBeHidden(ctx)
			case "to_be_enabled":
				err = assert.ToBeEnabled(ctx)
			case "to_be_disabled":
				err = assert.ToBeDisabled(ctx)
			case "to_be_focused":
				err = assert.ToBeFocused(ctx)
			case "to_be_checked":
				err = assert.ToBeChecked(ctx)
			case "not_to_be_checked":
				err = assert.NotToBeChecked(ctx)
			}
			return starlark.None, err
		})
	}
	return starlarkstruct.FromStringDict(starlark.String("Expect"), attrs)
}
