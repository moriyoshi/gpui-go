package main

import (
	"context"

	"github.com/doors-dev/gox"
	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

// goxState is one captured appearance of a template, reached by running before.
type goxState struct {
	name   string
	before func() error
}

type goxBuild func(context.Context, *ui.Document, *reactive.Scope) (gox.Elem, []goxState)

// nodeShot captures every state of one node tree in the same window.
func nodeShot(name string, width, height float32, build func(context.Context, *ui.Document, *reactive.Scope) (*ui.Node, []goxState)) shot {
	return shot{name, func(ctx context.Context, s *session) error {
		graph, document, owner := reactivePage(s)
		defer graph.Dispose()
		body, states := build(ctx, document, owner)
		if err := document.Err(); err != nil {
			return err
		}
		root := document.Div(owner, body).Style(pageStyle())
		mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: width, Height: height}, root)
		if err != nil {
			return err
		}
		defer mount.Close(context.WithoutCancel(ctx))
		for i, state := range states {
			if state.before != nil {
				if err := state.before(); err != nil {
					return err
				}
			}
			if i > 0 {
				id, err := s.onlyWindow(ctx)
				if err != nil {
					return err
				}
				delete(s.seen, id)
			}
			if err := s.capture(ctx, state.name); err != nil {
				return err
			}
		}
		return nil
	}}
}

func goxShot(name string, width, height float32, build goxBuild) shot {
	return nodeShot(name, width, height, func(ctx context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
		element, states := build(ctx, document, owner)
		return mustBuild(ctx, document, owner, element), states
	})
}

func one(name string, element gox.Elem) (gox.Elem, []goxState) {
	return element, []goxState{{name: name}}
}

func mustBuild(ctx context.Context, document *ui.Document, owner *reactive.Scope, element gox.Elem) *ui.Node {
	node, err := nativegox.Build(ctx, document, owner, element)
	if err != nil {
		panic(err)
	}
	return node
}

func goxShots() []shot {
	return []shot{
		goxShot("gox-components", 340, 90, func(context.Context, *ui.Document, *reactive.Scope) (gox.Elem, []goxState) {
			return one("gox-components", Tags())
		}),
		goxShot("gox-children", 320, 160, func(context.Context, *ui.Document, *reactive.Scope) (gox.Elem, []goxState) {
			return one("gox-children", Profile())
		}),
		goxShot("gox-roster", 300, 170, func(context.Context, *ui.Document, *reactive.Scope) (gox.Elem, []goxState) {
			return one("gox-roster", Roster([]Person{{"Ada", true}, {"Grace", false}, {"Linus", true}}))
		}),
		goxShot("gox-text-kinds", 340, 110, func(_ context.Context, _ *ui.Document, owner *reactive.Scope) (gox.Elem, []goxState) {
			count := reactive.NewSignal(owner, 0)
			return TextKinds(count), []goxState{{name: "gox-text-kinds", before: func() error { return count.Set(3) }}}
		}),
		goxShot("gox-mixed-text", 300, 170, func(context.Context, *ui.Document, *reactive.Scope) (gox.Elem, []goxState) {
			return one("gox-mixed-text", MixedText())
		}),
		goxShot("gox-form", 400, 130, func(_ context.Context, _ *ui.Document, owner *reactive.Scope) (gox.Elem, []goxState) {
			name := reactive.NewSignal(owner, "")
			locked := reactive.NewSignal(owner, false)
			return NameForm(name, locked), []goxState{{name: "gox-form-empty"}, {name: "gox-form", before: func() error { return name.Set("Ada Lovelace") }}}
		}),
		goxShot("gox-switch", 300, 100, func(_ context.Context, _ *ui.Document, owner *reactive.Scope) (gox.Elem, []goxState) {
			on := reactive.NewSignal(owner, false)
			flip := ui.Handler(func(context.Context, *client.Event) error {
				return on.Update(func(value bool) bool { return !value })
			})
			return Switch(on, flip), []goxState{{name: "gox-switch-off"}, {name: "gox-switch-on", before: func() error { return on.Set(true) }}}
		}),
		goxShot("gox-board", 300, 170, func(ctx context.Context, document *ui.Document, owner *reactive.Scope) (gox.Elem, []goxState) {
			open := reactive.NewSignal(owner, true)
			tasks := reactive.NewSignal(owner, []Task{{1, "Write docs"}, {2, "Take screenshots"}, {3, "Deploy"}})
			details := document.Show(owner, open.Get, func(scope *reactive.Scope) *ui.Node {
				return mustBuild(ctx, document, scope, Details())
			}, nil)
			rows := ui.For(document, owner, tasks.Get,
				func(task Task) int { return task.ID },
				func(scope *reactive.Scope, task *reactive.Signal[Task], index *reactive.Signal[int]) *ui.Node {
					return mustBuild(ctx, document, scope, TaskRow(task, index))
				},
			)
			return Board(details, rows), []goxState{{name: "gox-board"}, {name: "gox-board-updated", before: func() error {
				if err := open.Set(false); err != nil {
					return err
				}
				return tasks.Set([]Task{{3, "Deploy"}, {1, "Write docs"}, {2, "Take screenshots"}})
			}}}
		}),
	}
}
