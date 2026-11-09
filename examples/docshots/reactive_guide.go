package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
)

// The functions below appear verbatim in docs/guide/reactive.md.

func stackStyle() *scene.Style {
	return &scene.Style{FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(10)}
}

func buttonStyle() *scene.Style {
	return &scene.Style{Padding: scene.Padding(10), Background: scene.RGB(indigo), CornerRadius: scene.Ptr[float32](6)}
}

func Counter(document *ui.Document, owner *reactive.Scope, count *reactive.Signal[int]) *ui.Node {
	label := document.BindText(owner, func() string {
		return fmt.Sprintf("Count: %d", count.Get())
	})
	button := document.Div(owner, document.Text(owner, "Increment")).
		Accessible("Increment", &scene.Accessibility{Role: scene.RoleButton}).
		Focusable(0).
		OnClick(func(context.Context, *client.Event) error {
			return count.Update(func(value int) int { return value + 1 })
		}).
		Style(buttonStyle())
	return document.Div(owner, label, button).Style(stackStyle())
}

func TwoLabels(document *ui.Document, owner *reactive.Scope, a, b *reactive.Signal[int]) *ui.Node {
	runsA, runsB := 0, 0
	labelA := document.BindText(owner, func() string {
		runsA++
		return fmt.Sprintf("A is %d (binding runs: %d)", a.Get(), runsA)
	})
	labelB := document.BindText(owner, func() string {
		runsB++
		return fmt.Sprintf("B is %d (binding runs: %d)", b.Get(), runsB)
	})
	return document.Div(owner, labelA, labelB).Style(stackStyle())
}

func ParityLabels(document *ui.Document, owner *reactive.Scope, count *reactive.Signal[int]) *ui.Node {
	parity := reactive.NewMemo(owner, func() string {
		if count.Get()%2 == 0 {
			return "even"
		}
		return "odd"
	})
	countRuns, parityRuns := 0, 0
	countLabel := document.BindText(owner, func() string {
		countRuns++
		return fmt.Sprintf("Count is %d (binding runs: %d)", count.Get(), countRuns)
	})
	parityLabel := document.BindText(owner, func() string {
		parityRuns++
		return fmt.Sprintf("Parity is %s (binding runs: %d)", parity.Get(), parityRuns)
	})
	return document.Div(owner, countLabel, parityLabel).Style(stackStyle())
}

func FullName(document *ui.Document, owner *reactive.Scope, caption string, first, last *reactive.Signal[string]) *ui.Node {
	runs := 0
	return document.BindText(owner, func() string {
		runs++
		return fmt.Sprintf("%s: %s %s (binding runs: %d)", caption, first.Get(), last.Get(), runs)
	})
}

func RenameSeparately(first, last *reactive.Signal[string]) error {
	if err := first.Set("Grace"); err != nil {
		return err
	}
	return last.Set("Hopper")
}

func RenameInBatch(graph *reactive.Graph, first, last *reactive.Signal[string]) error {
	return graph.Batch(func() error {
		if err := first.Set("Grace"); err != nil {
			return err
		}
		return last.Set("Hopper")
	})
}

func StatusTile(document *ui.Document, owner *reactive.Scope, healthy *reactive.Signal[bool]) *ui.Node {
	text := document.BindText(owner, func() string {
		if healthy.Get() {
			return "All systems normal"
		}
		return "Service degraded"
	})
	return document.Div(owner, text).BindStyle(func() *scene.Style {
		background := scene.RGB(rose)
		if healthy.Get() {
			background = scene.RGB(teal)
		}
		return &scene.Style{Padding: scene.Padding(14), Background: background, CornerRadius: scene.Ptr[float32](8)}
	})
}

func SignupForm(document *ui.Document, owner *reactive.Scope, name *reactive.Signal[string]) *ui.Node {
	valid := reactive.NewMemo(owner, func() bool {
		return len(strings.TrimSpace(name.Get())) >= 2
	})
	field := document.Input(owner, name, ui.InputOptions{Placeholder: "Your name"}).
		Accessible("Name", &scene.Accessibility{Role: scene.RoleTextInput}).
		Style(fieldStyle())
	message := document.BindText(owner, func() string {
		if valid.Get() {
			return "Hello, " + strings.TrimSpace(name.Get())
		}
		return "Enter at least 2 characters"
	}).BindStyle(func() *scene.Style {
		if valid.Get() {
			return &scene.Style{TextColor: scene.RGB(0x5eead4)}
		}
		return &scene.Style{TextColor: scene.RGB(0xfda4af)}
	})
	return document.Div(owner, field, message).Style(stackStyle())
}

func Disclosure(document *ui.Document, owner *reactive.Scope, open *reactive.Signal[bool]) *ui.Node {
	heading := document.BindText(owner, func() string {
		if open.Get() {
			return "Hide details"
		}
		return "Show details"
	})
	toggle := document.Div(owner, heading).
		Accessible("Details", &scene.Accessibility{Role: scene.RoleButton}).
		Focusable(0).
		OnClick(func(context.Context, *client.Event) error {
			return open.Update(func(value bool) bool { return !value })
		}).
		Style(buttonStyle())
	details := document.Show(owner, open.Get, func(scope *reactive.Scope) *ui.Node {
		scope.OnCleanup(func() {
			// Release anything this branch started.
		})
		return document.Div(scope,
			document.Text(scope, "Signals drive this branch."),
			document.Text(scope, "Closing it disposes these nodes."),
		).Style(stackStyle())
	}, nil)
	return document.Div(owner, toggle, details).Style(stackStyle())
}

type Todo struct {
	ID    int
	Title string
	Done  bool
}

func TodoList(document *ui.Document, owner *reactive.Scope, todos *reactive.Signal[[]Todo]) *ui.Node {
	builds := 0
	rows := ui.For(document, owner, todos.Get,
		func(todo Todo) int { return todo.ID },
		func(scope *reactive.Scope, todo *reactive.Signal[Todo], index *reactive.Signal[int]) *ui.Node {
			builds++
			build := builds
			return document.BindText(scope, func() string {
				mark := " "
				if todo.Get().Done {
					mark = "x"
				}
				return fmt.Sprintf("%d. [%s] %s (built as row %d)", index.Get()+1, mark, todo.Get().Title, build)
			})
		},
	)
	return document.Div(owner, rows).Style(stackStyle())
}

func CompleteTodo(todos *reactive.Signal[[]Todo], id int) error {
	next := slices.Clone(todos.Peek())
	for i := range next {
		if next[i].ID == id {
			next[i].Done = true
		}
	}
	return todos.Set(next)
}

func SyncTitle(ctx context.Context, owner *reactive.Scope, window *client.Window, unread *reactive.Signal[int]) error {
	_, err := reactive.NewEffect(owner, func(*reactive.Scope) error {
		return window.SetTitle(ctx, fmt.Sprintf("Inbox (%d unread)", unread.Get()))
	})
	return err
}

func RunWithResults(ctx context.Context, app *client.Client, results <-chan int, progress *reactive.Signal[int]) error {
	type received struct {
		events []*client.Event
		err    error
	}
	batches := make(chan received, 1)
	go func() {
		for {
			events, err := app.NextEvents(ctx)
			select {
			case batches <- received{events, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case value := <-results:
			if err := progress.Set(value); err != nil {
				return err
			}
		case batch := <-batches:
			if batch.err != nil {
				return batch.err
			}
			for _, event := range batch.events {
				if err := app.Dispatch(ctx, event); err != nil {
					return err
				}
			}
		}
	}
}

// reactiveGuideShots captures the functions above.
func reactiveGuideShots() []shot {
	return []shot{
		nodeShot("guide-counter", 300, 130, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			count := reactive.NewSignal(owner, 0)
			return Counter(document, owner, count), []goxState{{name: "guide-counter", before: func() error { return count.Set(3) }}}
		}),
		nodeShot("guide-two-labels", 340, 110, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			a := reactive.NewSignal(owner, 0)
			b := reactive.NewSignal(owner, 0)
			return TwoLabels(document, owner, a, b), []goxState{{name: "guide-two-labels", before: func() error {
				for value := 1; value <= 3; value++ {
					if err := a.Set(value); err != nil {
						return err
					}
				}
				return nil
			}}}
		}),
		nodeShot("guide-memo", 400, 110, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			count := reactive.NewSignal(owner, 0)
			return ParityLabels(document, owner, count), []goxState{{name: "guide-memo", before: func() error {
				for _, value := range []int{2, 4, 6} {
					if err := count.Set(value); err != nil {
						return err
					}
				}
				return nil
			}}}
		}),
		nodeShot("guide-batch", 480, 110, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			first1, last1 := reactive.NewSignal(owner, "Ada"), reactive.NewSignal(owner, "Lovelace")
			first2, last2 := reactive.NewSignal(owner, "Ada"), reactive.NewSignal(owner, "Lovelace")
			root := document.Div(owner, FullName(document, owner, "Separate", first1, last1), FullName(document, owner, "Batched", first2, last2)).Style(stackStyle())
			return root, []goxState{{name: "guide-batch", before: func() error {
				if err := RenameSeparately(first1, last1); err != nil {
					return err
				}
				return RenameInBatch(owner.Graph(), first2, last2)
			}}}
		}),
		nodeShot("guide-status", 300, 100, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			healthy := reactive.NewSignal(owner, true)
			return StatusTile(document, owner, healthy), []goxState{{name: "guide-status-ok"}, {name: "guide-status-bad", before: func() error { return healthy.Set(false) }}}
		}),
		nodeShot("guide-form", 400, 130, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			name := reactive.NewSignal(owner, "A")
			return SignupForm(document, owner, name), []goxState{{name: "guide-form-invalid"}, {name: "guide-form-valid", before: func() error { return name.Set("Ada Lovelace") }}}
		}),
		nodeShot("guide-disclosure", 320, 170, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			open := reactive.NewSignal(owner, false)
			return Disclosure(document, owner, open), []goxState{{name: "guide-disclosure-closed"}, {name: "guide-disclosure-open", before: func() error { return open.Set(true) }}}
		}),
		nodeShot("guide-todos", 420, 190, func(_ context.Context, document *ui.Document, owner *reactive.Scope) (*ui.Node, []goxState) {
			todos := reactive.NewSignal(owner, []Todo{{ID: 1, Title: "Write docs"}, {ID: 2, Title: "Take screenshots"}, {ID: 3, Title: "Deploy"}})
			return TodoList(document, owner, todos), []goxState{{name: "guide-todos"}, {name: "guide-todos-updated", before: func() error {
				if err := CompleteTodo(todos, 2); err != nil {
					return err
				}
				reordered := slices.Clone(todos.Peek())
				slices.Reverse(reordered)
				return todos.Set(append(reordered, Todo{ID: 4, Title: "Celebrate"}))
			}}}
		}),
		{name: "guide-effect", run: effectShot},
	}
}

func effectShot(ctx context.Context, s *session) error {
	graph, document, owner := reactivePage(s)
	defer graph.Dispose()
	unread := reactive.NewSignal(owner, 0)
	label := document.BindText(owner, func() string { return fmt.Sprintf("%d messages waiting", unread.Get()) })
	mount, err := document.OpenWindow(ctx, client.WindowOptions{Title: "gpui-go", Width: 320, Height: 80}, document.Div(owner, label).Style(pageStyle()))
	if err != nil {
		return err
	}
	defer mount.Close(context.WithoutCancel(ctx))
	if err := SyncTitle(ctx, owner, mount.Window, unread); err != nil {
		return err
	}
	if err := unread.Set(3); err != nil {
		return err
	}
	return s.capture(ctx, "guide-effect")
}
