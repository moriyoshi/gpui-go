// Package countergox is the GoX version of the counter example.
package countergox

import (
	"context"
	"fmt"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

// App owns the counter's signals, document and native window.
type App struct {
	graph    *reactive.Graph
	document *ui.Document
	mount    *ui.Mount
	count    *reactive.Signal[int]
}

// Open creates one counter window. The caller owns Close and event dispatch.
func Open(ctx context.Context, c *client.Client, _ string) (_ *App, err error) {
	graph := reactive.NewGraph()
	a := &App{graph: graph, document: ui.New(graph.Root(), c)}
	defer func() {
		if err != nil {
			graph.Dispose()
		}
	}()
	a.count = reactive.NewSignal(a.document.Scope(), 0)
	root, err := nativegox.Build(ctx, a.document, a.document.Scope(), view(a.count, func(context.Context, *client.Event) error {
		return a.Increment()
	}))
	if err != nil {
		return nil, err
	}
	a.mount, err = a.document.OpenWindow(ctx, client.WindowOptions{Title: "Go + GPUI + Sable (GoX)", Width: 560, Height: 420}, root)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *App) Window() *client.Window             { return a.mount.Window }
func (a *App) Element(name string) (uint64, bool) { return a.document.Element(name) }
func (a *App) State() map[string]any              { return map[string]any{"count": a.count.Peek()} }
func (a *App) Increment() error                   { return a.count.Update(func(value int) int { return value + 1 }) }
func (a *App) Update(_ context.Context, operation, _ string) error {
	if operation != "increment" {
		return fmt.Errorf("unknown counter update %q", operation)
	}
	return a.Increment()
}
func (a *App) Close(ctx context.Context) error {
	defer a.graph.Dispose()
	return a.mount.Close(ctx)
}
