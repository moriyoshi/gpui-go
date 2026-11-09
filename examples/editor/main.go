// The native backend used by this example links GPL-3.0-or-later components.
package main

import (
	"context"
	"errors"
	"flag"
	"log"

	"github.com/moriyoshi/gpui-go/editor"
)

const sample = `// Zed's editor::Editor, hosted by GPUI and controlled from Go.
// Try typing, selection, copy/paste, Cmd-Z, and Japanese IME.
// Unicode: 日本語 · café · 🦀

package main

import "fmt"

func main() {
    fmt.Println("Hello from Go → protobuf → Sable → Zed!")
}
`
const notes = `An independent editor instance

Each window owns its own buffer, selection, and undo history.
This window hides its gutter and uses soft wrapping.
Edit either window; Go receives changes with the matching editor ID.
Close both windows to exit.
`

func main() {
	smoke := flag.Bool("smoke", false, "exercise native editor instances and their lifecycle")
	flag.Parse()
	err := editor.Run(context.Background(), func(ctx context.Context, app *editor.Client) error {
		first, err := app.Open(ctx, editor.Options{Title: "Editor example — code", Text: sample})
		if err != nil {
			return err
		}
		second, err := app.Open(ctx, editor.Options{Title: "Editor example — notes", Text: notes, HideGutter: true, Width: 650, Height: 500})
		if err != nil {
			return err
		}
		if *smoke {
			err := smokeEditors(ctx, app, first, second)
			if err != nil {
				// AppKit can end the process before the caller reports this.
				log.Printf("Editor smoke failed: %v", err)
			}
			return err
		}
		open := map[uint64]bool{first.ID(): true, second.ID(): true}
		for len(open) > 0 {
			event, err := app.NextEvent(ctx)
			if err != nil {
				if errors.Is(err, editor.ErrClosed) || ctx.Err() != nil {
					return nil
				}
				return err
			}
			switch event.Kind {
			case editor.Changed:
				log.Printf("editor %d: revision %d, %d bytes", event.EditorID, event.State.Revision, len(event.State.Text))
			case editor.EditorClosed:
				delete(open, event.EditorID)
			case editor.Failed:
				log.Printf("editor %d: %v", event.EditorID, event.Error)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
