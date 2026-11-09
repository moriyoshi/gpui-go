// The showcase composes controls from GPUI layout, text and click primitives.
// Go owns the state; only snapshots and event IDs cross the Rust boundary.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
)

type jobStep struct {
	generation uint64
	value      int
}
type input struct {
	events []*clientlib.Event
	err    error
}
type gallery struct {
	state      model
	app        *clientlib.Client
	window     *clientlib.Window
	ctx        context.Context
	bindings   map[string]uint64
	steps      chan jobStep
	generation uint64
	cancelJob  context.CancelFunc
}

var actions = []string{
	"page:Overview", "page:Controls", "page:Activity", "theme", "notifications", "retry", "completed",
	"page:Inputs", "inputs:clear", "inputs:sample",
	"density:Comfortable", "density:Compact", "accent:Indigo", "accent:Emerald", "accent:Amber",
	"filter:All", "filter:Running", "filter:Completed", "dropdown", "region:Tokyo", "region:London", "region:San Francisco",
	"workers:more", "workers:less", "start", "pause", "reset-job", "dialog:open", "dialog:cancel", "dialog:confirm", "save", "toast:close", "noop",
}

func main() {
	smoke := flag.Bool("smoke", false, "exercise the gallery's controls and background job, then exit")
	page := flag.String("page", "Overview", "initial page: Overview, Controls, Inputs, or Activity")
	flag.Parse()
	if *page != "Overview" && *page != "Controls" && *page != "Inputs" && *page != "Activity" {
		log.Fatalf("unknown page %q", *page)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *smoke {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
	}
	err := host.Run(ctx, func(ctx context.Context, app *clientlib.Client) error {
		for _, feature := range []string{"hover", "scroll", "positioning", "font-weight", "text-input"} {
			found := false
			for _, available := range app.Capabilities().Features {
				found = found || available == feature
			}
			if !found {
				return fmt.Errorf("showcase requires bridge capability %q; rebuild the Rust library", feature)
			}
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		g := &gallery{state: newModel(), app: app, ctx: ctx, bindings: make(map[string]uint64), steps: make(chan jobStep, 1)}
		g.state.page = *page
		g.bind()
		defer g.close()
		window, err := app.OpenWindow(ctx, clientlib.WindowOptions{Title: "Studio — GPUI component showcase", Width: 1240, Height: 860}, g.render())
		if err != nil {
			return err
		}
		g.window = window
		if *smoke {
			return g.smoke()
		}
		return g.run()
	})
	if err != nil {
		log.Fatal(err)
	}
}

func (g *gallery) bind() {
	for _, action := range actions {
		g.bindings[action] = g.app.OnClick(func(context.Context, *clientlib.Event) error { return g.act(action) })
	}
	for _, field := range []string{"name", "email", "query"} {
		g.bindings["input:"+field] = g.app.OnTextChange(func(_ context.Context, event *clientlib.Event) error {
			if g.inputState(field).Accept(event.GetTextChanged()) {
				g.state.inputChanges++
				g.state.composing = event.GetTextChanged().Composing
				return g.window.Set(g.ctx, g.render())
			}
			return nil
		})
	}
}

func (g *gallery) inputState(field string) *scene.InputState {
	switch field {
	case "name":
		return &g.state.name
	case "email":
		return &g.state.email
	case "query":
		return &g.state.query
	default:
		panic("unknown input field")
	}
}

func (g *gallery) close() {
	if g.cancelJob != nil {
		g.cancelJob()
	}
	for _, handler := range g.bindings {
		g.app.RemoveHandler(handler)
	}
}

func (g *gallery) act(action string) error {
	change, err := g.state.apply(action)
	if err != nil {
		return err
	}
	if change != noEffect {
		if g.cancelJob != nil {
			g.cancelJob()
		}
		g.generation++ // Queued results from previous jobs cannot update this one.
		if change == startJob {
			ctx, cancel := context.WithCancel(g.ctx)
			g.cancelJob = cancel
			generation, progress := g.generation, g.state.progress
			go func() {
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
				for value := progress + 5; value <= 100; value += 5 {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
					}
					select {
					case <-ctx.Done():
						return
					case g.steps <- jobStep{generation, value}:
					}
				}
			}()
		}
	}
	return g.window.Set(g.ctx, g.render())
}

func (g *gallery) advance(step jobStep) error {
	if step.generation != g.generation || !g.state.running {
		return nil
	}
	g.state.advance(step.value)
	return g.window.Set(g.ctx, g.render())
}

func (g *gallery) run() error {
	// This goroutine only receives immutable event batches. All application
	// state and snapshots stay on the select loop below, including job results.
	ctx, cancel := context.WithCancel(g.ctx)
	defer cancel()
	inputs := make(chan input, 1)
	pumpDone := make(chan struct{})
	defer func() { cancel(); <-pumpDone }()
	go func() {
		defer close(pumpDone)
		for {
			events, err := g.app.NextEvents(ctx)
			select {
			case <-ctx.Done():
				return
			case inputs <- input{events, err}:
			}
			if err != nil {
				return
			}
		}
	}()
	for {
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
		case batch := <-inputs:
			if batch.err != nil {
				return batch.err
			}
			for _, event := range batch.events {
				if event.GetWindowClosed() != nil {
					return nil
				}
				if err := g.app.Dispatch(g.ctx, event); err != nil {
					return err
				}
			}
		case step := <-g.steps:
			if err := g.advance(step); err != nil {
				return err
			}
		}
	}
}

// Smoke invokes the same registered callbacks as real UI input. Each action
// commits through Sable, and its revision is acknowledged through protobuf.
// It does not synthesize native mouse input.
func (g *gallery) smoke() error {
	sequence := []struct {
		action string
		check  func(model) bool
	}{
		{"page:Inputs", func(m model) bool { return m.page == "Inputs" }},
		{"input:name", func(m model) bool { return m.name.Value == "日本語 🙂" }},
		{"input:email", func(m model) bool { return m.email.Value == "hello@example.com" }},
		{"input:query", func(m model) bool { return len(m.visibleBuilds()) == 1 }},
		{"inputs:sample", func(m model) bool { return m.name.Value == "森吉 🙂" && m.name.Epoch == 1 }},
		{"inputs:clear", func(m model) bool { return m.name.Value == "" && m.query.Value == "" }},
		{"page:Controls", func(m model) bool { return m.page == "Controls" }},
		{"notifications", func(m model) bool { return !m.notifications }},
		{"retry", func(m model) bool { return !m.retry }},
		{"density:Compact", func(m model) bool { return m.density == "Compact" }},
		{"accent:Emerald", func(m model) bool { return m.accent == "Emerald" }},
		{"dropdown", func(m model) bool { return m.dropdown }},
		{"region:London", func(m model) bool { return m.region == "London" && !m.dropdown }},
		{"workers:more", func(m model) bool { return m.concurrency == 3 }},
		{"theme", func(m model) bool { return m.dark }},
		{"save", func(m model) bool { return m.toast }},
		{"toast:close", func(m model) bool { return !m.toast }},
		{"dialog:open", func(m model) bool { return m.dialog }},
		{"dialog:cancel", func(m model) bool { return !m.dialog && m.region == "London" }},
		{"dialog:open", func(m model) bool { return m.dialog }},
		{"dialog:confirm", func(m model) bool { return !m.dialog && m.region == "Tokyo" && m.concurrency == 2 }},
		{"page:Overview", func(m model) bool { return m.page == "Overview" }},
		{"filter:Running", func(m model) bool { return len(m.visibleBuilds()) == 1 }},
		{"filter:All", func(m model) bool { return len(m.visibleBuilds()) == 4 }},
		{"start", func(m model) bool { return m.running }},
	}
	revision := uint64(1)
	wait := func() error {
		events, err := g.app.NextEvents(g.ctx)
		if err != nil {
			return err
		}
		revision++
		if len(events) != 1 || events[0].GetViewUpdated() == nil || events[0].Revision != revision {
			return fmt.Errorf("expected revision %d, got %v", revision, events)
		}
		// Allow native rendering to run as we visit each page and overlay.
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
		case <-time.After(40 * time.Millisecond):
		}
		return nil
	}
	for _, step := range sequence {
		event := &clientlib.Event{HandlerId: g.bindings[step.action], Kind: &clientlib.EventClick{Click: &clientlib.Click{}}}
		if strings.HasPrefix(step.action, "input:") {
			field := strings.TrimPrefix(step.action, "input:")
			state := g.inputState(field)
			value := map[string]string{"name": "日本語 🙂", "email": "hello@example.com", "query": "desktop"}[field]
			event.Kind = &clientlib.EventTextChanged{TextChanged: &clientlib.TextChanged{Value: value, Epoch: state.Epoch, EditRevision: state.EditRevision + 1}}
		}
		if err := g.app.Dispatch(g.ctx, event); err != nil {
			return err
		}
		if !step.check(g.state) {
			return fmt.Errorf("control %q did not update its state", step.action)
		}
		if err := wait(); err != nil {
			return err
		}
	}
	for g.state.running {
		select {
		case <-g.ctx.Done():
			return g.ctx.Err()
		case step := <-g.steps:
			if err := g.advance(step); err != nil {
				return err
			}
			if err := wait(); err != nil {
				return err
			}
		}
	}
	if g.state.progress != 100 {
		return fmt.Errorf("background job did not finish")
	}
	if err := g.act("page:Activity"); err != nil {
		return err
	}
	if err := wait(); err != nil {
		return err
	}
	log.Printf("showcase: %d controls, background progress, and %d protobuf view commits: OK", len(sequence), revision-1)
	return nil
}
