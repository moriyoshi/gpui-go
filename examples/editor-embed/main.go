// An editor placed inside an ordinary view, with a header and a status line.
//
// The native backend used by this example links GPL-3.0-or-later components.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/editor"
	"github.com/moriyoshi/gpui-go/host"
	"github.com/moriyoshi/gpui-go/scene"
)

const notes = `Each line is an ordinary Zed buffer.

The header and the status line below are generic elements,
and this editor sits between them in a scene.Div.
`

func render(field *editor.Editor, status string) *scene.Node {
	return scene.Div(1,
		scene.Text(2, "Project notes").Style(&scene.Style{FontSize: scene.Ptr[float32](20), FontWeight: scene.Ptr[uint32](600)}),
		scene.Embedded(3, field.ID()).Style(&scene.Style{
			Width: scene.Percent(100), FlexGrow: scene.Ptr[float32](1),
		}),
		scene.Text(4, status).Style(&scene.Style{FontSize: scene.Ptr[float32](13), TextColor: scene.RGB(0xa1a1aa)}),
	).Style(&scene.Style{
		Width: scene.Percent(100), Height: scene.Percent(100), Padding: scene.Padding(16), Gap: scene.Pixels(12),
		FlexDirection: scene.FlexColumn.Enum(), Background: scene.RGB(0x18181b), TextColor: scene.RGB(0xf4f4f5),
		FontSize: scene.Ptr[float32](15),
	})
}

func main() {
	smoke := flag.Bool("smoke", false, "exercise the embedded editor and exit")
	out := flag.String("out", "", "save a PNG of the window, using bin/docs-window-ids")
	flag.Parse()
	err := host.Run(context.Background(), func(ctx context.Context, app *client.Client) error {
		// AppKit can end the process before host.Run returns an error to main.
		err := run(ctx, app, *smoke, *out)
		if err != nil {
			log.Print("failed: ", err)
		}
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, app *client.Client, smoke bool, out string) error {
	window, err := app.OpenWindow(ctx, client.WindowOptions{Title: "Embedded editor", Width: 640, Height: 400}, scene.Div(1))
	if err != nil {
		return err
	}
	editors, err := editor.Attach(ctx)
	if err != nil {
		return err
	}
	field, err := editors.OpenEmbedded(ctx, window, editor.EmbedOptions{Text: notes})
	if err != nil {
		return err
	}
	if err := window.Set(ctx, render(field, "0 edits")); err != nil {
		return err
	}
	if smoke || out != "" {
		return exercise(ctx, app, editors, window, field, out)
	}
	return interact(ctx, app, editors, window, field)
}

// interact combines GPUI and editor events on this goroutine, which owns the view.
func interact(ctx context.Context, app *client.Client, editors *editor.Client, window *client.Window, field *editor.Editor) error {
	type received struct {
		event *editor.Event
		err   error
	}
	events := make(chan received, 1)
	go func() {
		for {
			event, err := editors.NextEvent(ctx)
			select {
			case events <- received{event, err}:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	go func() { _ = app.DispatchEvents(ctx) }()
	for {
		select {
		case <-ctx.Done():
			return nil
		case item := <-events:
			if item.err != nil {
				return item.err
			}
			if item.event.Kind == editor.Changed && item.event.EditorID == field.ID() {
				if err := window.Set(ctx, render(field, fmt.Sprintf("%d edits", item.event.State.Revision))); err != nil {
					return err
				}
			}
		}
	}
}

func exercise(ctx context.Context, app *client.Client, editors *editor.Client, window *client.Window, field *editor.Editor, out string) error {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	log.Print("smoke: start")
	if !app.Supports("embedded-views") {
		return errors.New("backend does not advertise embedded-views")
	}
	inserted, err := field.InsertText(ctx, "Edited from Go. ")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(inserted.Text, "Edited from Go. Each line") || inserted.Revision != 1 {
		return fmt.Errorf("insert: %+v", inserted)
	}
	event, err := editors.NextEvent(ctx)
	if err != nil {
		return err
	}
	if event.Kind != editor.Changed || event.EditorID != field.ID() || *event.State != *inserted {
		return fmt.Errorf("change event: %+v", event)
	}
	log.Print("smoke: edit observed")
	// A new snapshot must keep the same native editor and its state.
	start, end := editor.Position{Line: 2, Column: 4}, editor.Position{Line: 2, Column: 10}
	selected, err := field.SetSelection(ctx, start, end)
	if err != nil {
		return err
	}
	if err := window.Set(ctx, render(field, "1 edit, selection kept across a new snapshot")); err != nil {
		return err
	}
	kept, err := field.State(ctx)
	if err != nil {
		return err
	}
	if *kept != *selected {
		return fmt.Errorf("new snapshot changed the editor: %+v", kept)
	}
	log.Print("smoke: snapshot kept the editor")
	if out != "" {
		if err := capture(ctx, out); err != nil {
			return err
		}
	}
	return checks(ctx, app, editors, window, field)
}

func capture(ctx context.Context, path string) error {
	time.Sleep(4 * time.Second)
	output, err := exec.CommandContext(ctx, "bin/docs-window-ids", strconv.Itoa(os.Getpid())).Output()
	if err != nil {
		return err
	}
	ids := strings.Fields(string(output))
	if len(ids) == 0 {
		return errors.New("no window to capture")
	}
	if output, err := exec.CommandContext(ctx, "screencapture", "-x", "-o", "-l"+ids[0], path).CombinedOutput(); err != nil {
		return fmt.Errorf("screencapture: %w: %s", err, output)
	}
	return nil
}

func rejected(ctx context.Context, window *client.Window, what string, root *scene.Node) error {
	if err := window.Set(ctx, root); err == nil {
		return fmt.Errorf("%s was accepted", what)
	}
	return nil
}

func checks(ctx context.Context, app *client.Client, editors *editor.Client, window *client.Window, field *editor.Editor) error {
	log.Print("smoke: checks")
	if err := rejected(ctx, window, "an unknown instance", scene.Div(1, scene.Embedded(2, 999999))); err != nil {
		return err
	}
	if err := rejected(ctx, window, "a repeated instance", scene.Div(1, scene.Embedded(2, field.ID()), scene.Embedded(3, field.ID()))); err != nil {
		return err
	}
	log.Print("smoke: bad placements rejected")
	// Leaving the tree does not destroy the editor.
	before, err := field.State(ctx)
	if err != nil {
		return err
	}
	if err := window.Set(ctx, scene.Div(1)); err != nil {
		return err
	}
	if err := window.Set(ctx, render(field, "back in the tree")); err != nil {
		return err
	}
	after, err := field.State(ctx)
	if err != nil {
		return err
	}
	if *after != *before {
		return fmt.Errorf("editor changed while out of the tree: %+v", after)
	}

	log.Print("smoke: editor survived leaving the tree")
	other, err := app.OpenWindow(ctx, client.WindowOptions{Title: "Second window", Width: 320, Height: 200}, scene.Div(1))
	if err != nil {
		return err
	}
	foreign, err := editors.OpenEmbedded(ctx, other, editor.EmbedOptions{Text: "second window"})
	if err != nil {
		return err
	}
	if err := rejected(ctx, window, "another window's instance", scene.Div(1, scene.Embedded(2, foreign.ID()))); err != nil {
		return err
	}
	if err := other.Set(ctx, scene.Div(1, scene.Embedded(2, foreign.ID()).Style(&scene.Style{Width: scene.Percent(100), Height: scene.Percent(100)}))); err != nil {
		return err
	}
	if _, err := foreign.InsertText(ctx, "independent "); err != nil {
		return err
	}
	if err := expectChanged(ctx, editors, foreign, "independent second window"); err != nil {
		return err
	}
	untouched, err := field.State(ctx)
	if err != nil {
		return err
	}
	if untouched.Text != before.Text {
		return errors.New("an edit crossed embedded editors")
	}
	log.Print("smoke: second editor independent")
	// Closing a window closes the editors inside it.
	if err := other.Close(ctx); err != nil {
		return err
	}
	if err := expectClosed(ctx, editors, foreign); err != nil {
		return err
	}
	if _, err := foreign.State(ctx); !errors.Is(err, editor.ErrNotFound) {
		return fmt.Errorf("editor in a closed window: %v", err)
	}
	if _, err := editors.OpenEmbedded(ctx, other, editor.EmbedOptions{}); !errors.Is(err, editor.ErrNotFound) {
		return fmt.Errorf("embedding in a closed window: %v", err)
	}
	log.Print("smoke: window close released its editor")
	if err := window.Set(ctx, scene.Div(1)); err != nil {
		return err
	}
	if err := field.Close(ctx); err != nil {
		return err
	}
	if err := expectClosed(ctx, editors, field); err != nil {
		return err
	}
	if _, err := field.State(ctx); !errors.Is(err, editor.ErrNotFound) {
		return fmt.Errorf("closed editor: %v", err)
	}
	if err := rejected(ctx, window, "a closed instance", scene.Div(1, scene.Embedded(2, field.ID()))); err != nil {
		return err
	}
	log.Print("Embedded editor smoke passed: placement, snapshot persistence, independent editors, rejected placements, close paths")
	return nil
}

func expectChanged(ctx context.Context, editors *editor.Client, field *editor.Editor, text string) error {
	event, err := editors.NextEvent(ctx)
	if err != nil {
		return err
	}
	if event.Kind != editor.Changed || event.EditorID != field.ID() || event.State.Text != text {
		return fmt.Errorf("change event: %+v", event)
	}
	return nil
}

func expectClosed(ctx context.Context, editors *editor.Client, field *editor.Editor) error {
	for {
		event, err := editors.NextEvent(ctx)
		if err != nil {
			return err
		}
		if event.Kind == editor.EditorClosed && event.EditorID == field.ID() {
			return nil
		}
	}
}
