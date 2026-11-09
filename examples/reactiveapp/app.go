// Package reactiveapp is a consumer of the reactive graph and GoX GPUI adapter.
// Template source is view.gox. Install the pinned tool with make gox-tools.
package reactiveapp

import (
	"context"
	"fmt"

	"github.com/doors-dev/gox"
	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

type item struct {
	ID    int
	Title *reactive.Signal[string]
}

// App owns the reactive example's graph, document and native window.
type App struct {
	graph                                    *reactive.Graph
	document                                 *ui.Document
	mount                                    *ui.Mount
	count                                    *reactive.Signal[int]
	name                                     *reactive.Signal[string]
	show                                     *reactive.Signal[bool]
	items                                    *reactive.Signal[[]*item]
	nextItem, builds, countRuns, detailsRuns int
}

// Open constructs the example. The caller owns Close and event dispatch.
func Open(ctx context.Context, client *clientlib.Client, _ string) (_ *App, err error) {
	g := reactive.NewGraph()
	a := &App{graph: g}
	a.document = ui.New(g.Root(), client)
	scope := a.document.Scope()
	defer func() {
		if err != nil {
			a.document.Dispose()
			g.Dispose()
		}
	}()
	a.count = reactive.NewSignal(scope, 0)
	a.name = reactive.NewSignal(scope, "")
	a.show = reactive.NewSignal(scope, true)
	a.items = reactive.NewSignal(scope, []*item{{ID: 1, Title: reactive.NewSignal(scope, "Alpha")}, {ID: 2, Title: reactive.NewSignal(scope, "Beta")}})
	a.nextItem = 2
	rows := ui.For(a.document, scope, a.items.Get, func(item *item) int { return item.ID }, func(owner *reactive.Scope, data *reactive.Signal[*item], index *reactive.Signal[int]) *ui.Node {
		return a.buildOwned(ctx, owner, rowTemplate(data, index))
	})
	details := a.document.Show(scope, a.show.Get, func(owner *reactive.Scope) *ui.Node {
		parity := reactive.NewMemo(owner, func() string {
			if a.count.Get()%2 == 0 {
				return "Even"
			}
			return "Odd"
		})
		return a.buildOwned(ctx, owner, a.detailsTemplate(parity))
	}, nil)
	root, err := nativegox.Build(ctx, a.document, scope, a.template(rows, details))
	if err != nil {
		return nil, err
	}
	a.mount, err = a.document.OpenWindow(ctx, clientlib.WindowOptions{Title: "Reactive GPUI node graph", Width: 840, Height: 850}, root)
	if err != nil {
		return nil, err
	}
	return a, nil
}
func (a *App) Window() *clientlib.Window          { return a.mount.Window }
func (a *App) Element(name string) (uint64, bool) { return a.document.Element(name) }
func (a *App) State() map[string]any {
	return map[string]any{"count": a.count.Peek(), "name": a.name.Peek(), "items": len(a.items.Peek()), "builds": a.builds, "count_runs": a.countRuns, "details_runs": a.detailsRuns, "computations": a.graph.Stats().Computations}
}
func (a *App) Close(ctx context.Context) error { defer a.graph.Dispose(); return a.mount.Close(ctx) }
func (a *App) Update(_ context.Context, operation, value string) error {
	switch operation {
	case "increment":
		return a.count.Update(func(v int) int { return v + 1 })
	case "batch":
		return a.graph.Batch(func() error {
			if err := a.count.Update(func(v int) int { return v + 1 }); err != nil {
				return err
			}
			return a.count.Update(func(v int) int { return v + 1 })
		})
	case "reverse":
		next := append([]*item(nil), a.items.Peek()...)
		for i, j := 0, len(next)-1; i < j; i, j = i+1, j-1 {
			next[i], next[j] = next[j], next[i]
		}
		return a.items.Set(next)
	case "toggle":
		return a.show.Update(func(value bool) bool { return !value })
	case "reset_name":
		return a.name.Set(value)
	case "add":
		a.nextItem++
		next := append([]*item(nil), a.items.Peek()...)
		next = append(next, &item{ID: a.nextItem, Title: reactive.NewSignal(a.document.Scope(), fmt.Sprintf("Item %d", a.nextItem))})
		return a.items.Set(next)
	case "remove":
		next := a.items.Peek()
		if len(next) > 0 {
			return a.items.Set(append([]*item(nil), next[1:]...))
		}
		return nil
	default:
		return fmt.Errorf("unknown reactive update %q", operation)
	}
}

// Structural factories run inside reactive computations, which convert template
// construction failures into errors and suppress incomplete snapshot publication.
func (a *App) buildOwned(ctx context.Context, owner *reactive.Scope, component gox.Elem) *ui.Node {
	node, err := nativegox.Build(ctx, a.document, owner, component)
	if err != nil {
		panic(err)
	}
	return node
}
