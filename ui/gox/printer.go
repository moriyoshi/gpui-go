// Package gox adapts GoX's typed job stream into a persistent GPUI node graph.
// It uses the GoX runtime only; its formatter/compiler is not linked into apps.
package gox

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/doors-dev/gox"
	clientlib "github.com/moriyoshi/gpui-go/client"
	pb "github.com/moriyoshi/gpui-go/gen/gpui/v1"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	"google.golang.org/protobuf/proto"
)

// Text preserves an accessor as a binding rather than formatting its value once.
type TextBinding struct{ Read func() string }

func Text(read func() string) TextBinding { return TextBinding{read} }

type frame struct {
	id   uint64
	node *ui.Node
}
type printer struct {
	document *ui.Document
	owner    *reactive.Scope
	stack    []frame
	roots    []*ui.Node
}

// Build executes a GoX component once. Bindings retain accessors; later signal
// changes do not rerun the template. Only supported native tags/typed attributes
// are accepted. Failed builds dispose their effects, nodes and event handlers.
func Build(ctx context.Context, document *ui.Document, owner *reactive.Scope, element gox.Elem) (*ui.Node, error) {
	scope := owner.Child()
	p := &printer{document: document, owner: scope}
	err := owner.Graph().Batch(func() error {
		if err := element.Print(ctx, p); err != nil {
			return err
		}
		if len(p.stack) != 0 {
			return errors.New("gox GPUI: unclosed head")
		}
		return nil
	})
	if err != nil {
		scope.Dispose()
		return nil, err
	}
	if len(p.roots) != 1 {
		scope.Dispose()
		return nil, fmt.Errorf("gox GPUI: expected one root, got %d", len(p.roots))
	}
	return p.roots[0], nil
}
func (p *printer) append(node *ui.Node) error {
	if len(p.stack) == 0 {
		p.roots = append(p.roots, node)
		return nil
	}
	return p.stack[len(p.stack)-1].node.Append(node)
}
func (p *printer) Send(job gox.Job) error {
	if pooled, ok := job.(gox.Releaser); ok {
		defer gox.Release(pooled)
	}
	if err := job.Context().Err(); err != nil {
		return err
	}
	switch j := job.(type) {
	case *gox.JobOpen:
		if j.ID == 0 {
			return errors.New("gox GPUI: missing head ID")
		}
		attrs := map[string]any{}
		if j.Attrs != nil {
			if err := j.Attrs.ApplyMods(j.Context(), j.Tag); err != nil {
				return err
			}
			for _, attr := range j.Attrs.List() {
				if attr.IsSet() {
					attrs[attr.Name()] = attr.Value()
				}
			}
		}
		node, err := p.open(j.Kind, j.Tag, attrs)
		if err != nil {
			return err
		}
		if err := p.append(node); err != nil {
			return err
		}
		if j.Kind != gox.KindVoid {
			p.stack = append(p.stack, frame{id: j.ID, node: node})
		}
		return nil
	case *gox.JobClose:
		if len(p.stack) == 0 || p.stack[len(p.stack)-1].id != j.ID {
			return errors.New("gox GPUI: mismatched closing head")
		}
		p.stack = p.stack[:len(p.stack)-1]
		return nil
	case *gox.JobText:
		return p.append(p.document.Text(p.owner, j.Text))
	case *gox.JobFprint:
		switch value := j.Any.(type) {
		case TextBinding:
			if value.Read == nil {
				return errors.New("gox GPUI: nil text accessor")
			}
			return p.append(p.document.BindText(p.owner, value.Read))
		case *ui.Node:
			return p.append(value)
		default:
			if value != nil && reflect.TypeOf(value).Kind() == reflect.Func {
				return errors.New("gox GPUI: wrap reactive text accessors with gox.Text")
			}
			return p.append(p.document.Text(p.owner, fmt.Sprint(value)))
		}
	case *gox.JobError:
		return j.Err
	default:
		return fmt.Errorf("gox GPUI: unsupported job %T; raw HTML and templ output are not native elements", job)
	}
}
func (p *printer) open(kind gox.HeadKind, tag string, attrs map[string]any) (*ui.Node, error) {
	for name := range attrs {
		switch name {
		case "testid", "label", "role", "twClass", "style", "onClick", "value", "placeholder", "disabled", "masked", "checked",
			"src", "commands", "rowHeight":
		default:
			return nil, fmt.Errorf("gox GPUI: unsupported attribute %q", name)
		}
	}
	var node *ui.Node
	if kind == gox.KindContainer {
		if len(attrs) > 0 {
			return nil, errors.New("gox GPUI: fragment attributes are unsupported")
		}
		return p.document.Fragment(p.owner), nil
	}
	role := pb.AccessibleRole_ACCESSIBLE_ROLE_GENERIC
	// Each tag-specific attribute belongs to exactly one tag.
	for name, owner := range map[string]string{"src": "img", "commands": "canvas", "rowHeight": "uniform-list"} {
		if _, ok := attrs[name]; ok && tag != owner {
			return nil, fmt.Errorf("gox GPUI: %s requires %s", name, owner)
		}
	}
	switch tag {
	case "div", "span", "text":
		node = p.document.Div(p.owner)
	case "img":
		switch src := attrs["src"].(type) {
		case *clientlib.Resource:
			node = p.document.Image(p.owner, src)
		case func() *clientlib.Resource:
			node = p.document.BindImage(p.owner, src)
		default:
			return nil, errors.New("gox GPUI: img src requires *client.Resource or an accessor")
		}
		role = pb.AccessibleRole_ACCESSIBLE_ROLE_IMAGE
	case "canvas":
		switch commands := attrs["commands"].(type) {
		case nil:
			node = p.document.Canvas(p.owner)
		case []*scene.DrawCommand:
			node = p.document.Canvas(p.owner, commands...)
		case func() []*scene.DrawCommand:
			node = p.document.BindCanvas(p.owner, commands)
		default:
			return nil, errors.New("gox GPUI: canvas commands require []*scene.DrawCommand or an accessor")
		}
	case "uniform-list":
		var height float32
		switch value := attrs["rowHeight"].(type) {
		case float32:
			height = value
		case float64:
			height = float32(value)
		case int:
			height = float32(value)
		default:
			return nil, errors.New("gox GPUI: uniform-list requires a numeric rowHeight")
		}
		if !(height >= 1 && height <= 16384) {
			return nil, errors.New("gox GPUI: uniform-list rowHeight must be in [1, 16384]")
		}
		node = p.document.UniformList(p.owner, height)
		role = pb.AccessibleRole_ACCESSIBLE_ROLE_LIST
	case "button":
		node = p.document.Div(p.owner).Focusable(0)
		role = pb.AccessibleRole_ACCESSIBLE_ROLE_BUTTON
	case "input":
		if kind != gox.KindVoid {
			return nil, errors.New("gox GPUI: input must be a void head")
		}
		value, ok := attrs["value"].(*reactive.Signal[string])
		if !ok {
			return nil, errors.New("gox GPUI: input value requires a string signal")
		}
		options := ui.InputOptions{}
		if raw, ok := attrs["placeholder"]; ok {
			var valid bool
			options.Placeholder, valid = raw.(string)
			if !valid {
				return nil, errors.New("gox GPUI: placeholder requires a string")
			}
		}
		if raw, ok := attrs["disabled"]; ok {
			switch value := raw.(type) {
			case bool:
				options.Disabled = func() bool { return value }
			case func() bool:
				options.Disabled = value
			default:
				return nil, errors.New("gox GPUI: disabled requires bool or an accessor")
			}
		}
		if raw, ok := attrs["masked"]; ok {
			switch value := raw.(type) {
			case bool:
				options.Masked = func() bool { return value }
			case func() bool:
				options.Masked = value
			default:
				return nil, errors.New("gox GPUI: masked requires bool or an accessor")
			}
		}
		node = p.document.Input(p.owner, value, options)
		role = pb.AccessibleRole_ACCESSIBLE_ROLE_TEXT_INPUT
	default:
		return nil, fmt.Errorf("gox GPUI: unsupported native tag %q", tag)
	}
	if value, ok := attrs["testid"]; ok {
		name, ok := value.(string)
		if !ok {
			return nil, errors.New("gox GPUI: testid requires a string")
		}
		node.Name(name)
	}
	if value, ok := attrs["twClass"]; ok {
		switch classes := value.(type) {
		case string:
			node.TWClass(classes)
		case func() string:
			node.BindTWClass(classes)
		default:
			return nil, errors.New("gox GPUI: twClass requires a string or an accessor")
		}
	}
	if value, ok := attrs["style"]; ok {
		switch style := value.(type) {
		case *scene.Style:
			node.Style(style)
		case func() *scene.Style:
			node.BindStyle(style)
		default:
			return nil, errors.New("gox GPUI: style requires *scene.Style or an accessor")
		}
	}
	if value, ok := attrs["onClick"]; ok {
		switch handler := value.(type) {
		case ui.Handler:
			node.OnClick(handler)
		case func(context.Context, *clientlib.Event) error:
			node.OnClick(ui.Handler(handler))
		default:
			return nil, errors.New("gox GPUI: onClick requires ui.Handler")
		}
	}
	label := ""
	if raw, ok := attrs["label"]; ok {
		var valid bool
		label, valid = raw.(string)
		if !valid {
			return nil, errors.New("gox GPUI: label requires a string")
		}
	}
	if raw, ok := attrs["role"]; ok {
		var valid bool
		role, valid = raw.(scene.AccessibleRole)
		if !valid || role == pb.AccessibleRole_ACCESSIBLE_ROLE_UNSPECIFIED {
			return nil, errors.New("gox GPUI: role requires scene.AccessibleRole")
		}
	}
	metadata := &scene.Accessibility{Role: role}
	node.Accessible(label, metadata)
	if raw, ok := attrs["checked"]; ok {
		switch value := raw.(type) {
		case bool:
			metadata.Checked = proto.Bool(value)
			node.Accessible(label, metadata)
		case func() bool:
			node.BindAccessibility(label, func() *scene.Accessibility { return &scene.Accessibility{Role: role, Checked: proto.Bool(value())} })
		default:
			return nil, errors.New("gox GPUI: checked requires bool or an accessor")
		}
	}
	if tag != "input" && (attrs["value"] != nil || attrs["placeholder"] != nil || attrs["disabled"] != nil || attrs["masked"] != nil) {
		return nil, errors.New("gox GPUI: value/placeholder/disabled/masked require input")
	}
	return node, nil
}
