package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/moriyoshi/gpui-go/editor"
)

func smokeEditors(ctx context.Context, app *editor.Client, first, second *editor.Editor) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	original, err := first.State(ctx)
	if err != nil {
		return err
	}
	if original.Text != sample || original.Revision != 0 || first.ID() == second.ID() {
		return errors.New("invalid initial editor state or IDs")
	}
	unchanged, err := second.Undo(ctx)
	if err != nil {
		return err
	}
	if unchanged.Text != notes || unchanged.Revision != 0 {
		return errors.New("initial text should not create an undo entry")
	}
	before, err := first.SetText(ctx, sample+"\n// Replaced from Go\n")
	if err != nil {
		return err
	}
	event, err := app.NextEvent(ctx)
	if err != nil {
		return err
	}
	if event.Kind != editor.Changed || event.EditorID != first.ID() || *event.State != *before {
		return errors.New("native change event did not match the targeted editor")
	}
	insertion := "// Inserted by Go — 日本語 🦀\n"
	after, err := first.InsertText(ctx, insertion)
	if err != nil {
		return err
	}
	if !strings.Contains(after.Text, insertion) || after.Revision <= before.Revision {
		return errors.New("insert did not update native text")
	}
	undo, err := first.Undo(ctx)
	if err != nil {
		return err
	}
	if undo.Text != before.Text {
		return errors.New("undo did not restore text")
	}
	redo, err := first.Redo(ctx)
	if err != nil {
		return err
	}
	if redo.Text != after.Text {
		return errors.New("redo did not restore inserted text")
	}
	if _, err = first.SetReadOnly(ctx, true); err != nil {
		return err
	}
	if _, err = first.InsertText(ctx, insertion); !errors.Is(err, editor.ErrReadOnly) {
		return fmt.Errorf("expected typed read-only rejection, got %v", err)
	}
	readonly, err := first.State(ctx)
	if err != nil {
		return err
	}
	if !readonly.ReadOnly || readonly.Text != after.Text {
		return errors.New("read-only rejection changed text")
	}
	reset, err := first.SetText(ctx, "authoritative replacement 日本語")
	if err != nil {
		return err
	}
	if !reset.ReadOnly || reset.Text != "authoritative replacement 日本語" {
		return errors.New("authoritative replacement failed")
	}
	untouched, err := second.State(ctx)
	if err != nil {
		return err
	}
	if untouched.Text != notes || untouched.ReadOnly || untouched.Revision != 0 {
		return errors.New("commands leaked across editor instances")
	}
	if err = first.Close(ctx); err != nil {
		return err
	}
	if err = waitClosed(ctx, app, first.ID()); err != nil {
		return err
	}
	if _, err = first.State(ctx); !errors.Is(err, editor.ErrNotFound) {
		return fmt.Errorf("expected closed handle rejection, got %v", err)
	}
	if err = second.Focus(ctx); err != nil {
		return err
	}
	if _, err = second.InsertText(ctx, "still alive\n"); err != nil {
		return err
	}
	if err = second.Close(ctx); err != nil {
		return err
	}
	if err = waitClosed(ctx, app, second.ID()); err != nil {
		return err
	}
	third, err := app.Open(ctx, editor.Options{Title: "Lifecycle smoke", Text: "reopened"})
	if err != nil {
		return err
	}
	if third.ID() <= second.ID() {
		return errors.New("editor IDs were recycled")
	}
	if err = third.Close(ctx); err != nil {
		return err
	}
	if err = waitClosed(ctx, app, third.ID()); err != nil {
		return err
	}
	if err = smokeSelection(ctx, app); err != nil {
		return err
	}
	log.Print("Editor smoke passed: independent buffers, Unicode, events, undo/redo, read-only, focus, close, stale handles, reopen after last close")
	return nil
}

// smokeSelection observes where edits land, not only the reported selection.
func smokeSelection(ctx context.Context, app *editor.Client) error {
	const text = "first\n日本語 text\nlast\n\t日\tz\n"
	field, err := app.Open(ctx, editor.Options{Title: "Selection smoke", Text: text})
	if err != nil {
		return err
	}
	initial, err := field.State(ctx)
	if err != nil {
		return err
	}
	if initial.Selection != (editor.Selection{}) {
		return fmt.Errorf("initial selection: %+v", initial.Selection)
	}
	start, end := editor.Position{Line: 1, Column: 2}, editor.Position{Line: 1, Column: 6}
	selected, err := field.SetSelection(ctx, start, end)
	if err != nil {
		return err
	}
	if selected.Selection != (editor.Selection{Start: start, End: end}) || selected.Text != text || selected.Revision != initial.Revision {
		return fmt.Errorf("selection state: %+v", selected)
	}
	for _, invalid := range [][2]editor.Position{
		{{Line: 1, Column: 1}, {Line: 1, Column: 1}}, // Inside 日.
		{{Line: 1, Column: 2}, {Line: 1, Column: 3}}, // End inside 本.
		{{Line: 0, Column: 6}, {Line: 0, Column: 6}}, // Past the end of "first".
		{{Line: 3, Column: 2}, {Line: 3, Column: 2}}, // Inside the first tab.
		{{Line: 3, Column: 7}, {Line: 3, Column: 7}}, // Inside the second tab.
		{{Line: 5}, {Line: 5}},                       // Past the last line.
	} {
		_, err := field.SetSelection(ctx, invalid[0], invalid[1])
		var protocol *editor.ProtocolError
		if !errors.As(err, &protocol) || protocol.Code != editor.InvalidArgument {
			return fmt.Errorf("invalid selection %v: %v", invalid, err)
		}
	}
	unchanged, err := field.State(ctx)
	if err != nil {
		return err
	}
	if unchanged.Selection != selected.Selection {
		return fmt.Errorf("rejected selection moved the caret: %+v", unchanged.Selection)
	}
	inserted, err := field.InsertText(ctx, "X")
	if err != nil {
		return err
	}
	caret := editor.Position{Line: 1, Column: 3}
	if inserted.Text != "first\n日X text\nlast\n\t日\tz\n" || inserted.Selection != (editor.Selection{Start: caret, End: caret}) {
		return fmt.Errorf("insert at selection: %+v", inserted)
	}
	// The first event must be this edit: moving a selection emits nothing.
	event, err := app.NextEvent(ctx)
	if err != nil {
		return err
	}
	if event.Kind != editor.Changed || event.EditorID != field.ID() || *event.State != *inserted {
		return fmt.Errorf("change event: %+v", event)
	}
	if err = smokeClamp(ctx, field); err != nil {
		return err
	}
	// A tab to 4, 日 to 6, then a tab of 3 because Zed's stops count characters.
	z := editor.Position{Line: 3, Column: 9}
	if _, err = field.SetSelection(ctx, z, z); err != nil {
		return err
	}
	typed, err := field.InsertText(ctx, "Y")
	if err != nil {
		return err
	}
	caret = editor.Position{Line: 3, Column: 10}
	if typed.Text != "first\n日X text\nlast\n\t日\tYz\n" || typed.Selection != (editor.Selection{Start: caret, End: caret}) {
		return fmt.Errorf("insert after tabs: %+v", typed)
	}
	if event, err = app.NextEvent(ctx); err != nil {
		return err
	}
	if event.Kind != editor.Changed || *event.State != *typed {
		return fmt.Errorf("change event after tabs: %+v", event)
	}
	if _, err = field.SetReadOnly(ctx, true); err != nil {
		return err
	}
	last := editor.Position{Line: 4}
	locked, err := field.SetSelection(ctx, last, last)
	if err != nil {
		return err
	}
	if locked.Selection != (editor.Selection{Start: last, End: last}) || !locked.ReadOnly {
		return fmt.Errorf("read-only selection at the end of the buffer: %+v", locked)
	}
	if err = field.Close(ctx); err != nil {
		return err
	}
	if err = waitClosed(ctx, app, field.ID()); err != nil {
		return err
	}
	if _, err = field.SetSelection(ctx, last, last); !errors.Is(err, editor.ErrNotFound) {
		return fmt.Errorf("selection on a closed editor: %v", err)
	}
	for table, column := range map[editor.WidthTable]uint32{
		editor.WidthNarrowAmbiguous: 2,
		editor.WidthWideAmbiguous:   4,
		editor.WidthWideSymbols:     3,
	} {
		if err = smokeWidthTable(ctx, app, table, column); err != nil {
			return err
		}
	}
	return smokeCharacter(ctx, app)
}

// smokeCharacter observes where a character count lands, by inserting there.
func smokeCharacter(ctx context.Context, app *editor.Client) error {
	field, err := app.Open(ctx, editor.Options{Title: "Character smoke", Text: "e\u0301x\n\t日\tz"})
	if err != nil {
		return err
	}
	at := func(line, character uint32, clamp editor.Clamp) editor.Position {
		return editor.Position{Line: line, Character: character, Clamp: clamp}
	}
	column := func(line, value uint32) editor.Position { return editor.Position{Line: line, Column: value} }
	for _, check := range []struct {
		position, want editor.Position
	}{
		{at(0, 2, editor.ClampNone), column(0, 1)},
		{at(0, 1, editor.ClampBefore), column(0, 0)}, // Inside the cluster.
		{at(0, 1, editor.ClampAfter), column(0, 1)},  // Inside the cluster.
		{at(1, 1, editor.ClampNone), column(1, 4)},
		{at(1, 3, editor.ClampNone), column(1, 9)},
	} {
		state, err := field.SetSelection(ctx, check.position, check.position)
		if err != nil {
			return fmt.Errorf("character %+v: %w", check.position, err)
		}
		if want := (editor.Selection{Start: check.want, End: check.want}); state.Selection != want {
			return fmt.Errorf("character %+v: got %+v", check.position, state.Selection)
		}
	}
	for _, invalid := range []editor.Position{
		at(0, 1, editor.ClampNone),         // Inside the cluster.
		at(0, 4, editor.ClampBefore),       // Past the end of the line.
		{Line: 1, Column: 4, Character: 1}, // Both forms at once.
	} {
		_, err := field.SetSelection(ctx, invalid, invalid)
		var protocol *editor.ProtocolError
		if !errors.As(err, &protocol) || protocol.Code != editor.InvalidArgument {
			return fmt.Errorf("invalid character %+v: %v", invalid, err)
		}
	}
	z := at(1, 3, editor.ClampNone)
	if _, err = field.SetSelection(ctx, z, z); err != nil {
		return err
	}
	typed, err := field.InsertText(ctx, "Y")
	if err != nil {
		return err
	}
	if typed.Text != "e\u0301x\n\t日\tYz" {
		return fmt.Errorf("insert at character: %q", typed.Text)
	}
	if err = field.Close(ctx); err != nil {
		return err
	}
	return waitClosed(ctx, app, field.ID())
}

// smokeWidthTable checks that x in "α→x" is at column, by inserting before it.
func smokeWidthTable(ctx context.Context, app *editor.Client, table editor.WidthTable, column uint32) error {
	field, err := app.Open(ctx, editor.Options{Title: "Width table smoke", Text: "α→x", WidthTable: table})
	if err != nil {
		return err
	}
	x := editor.Position{Line: 0, Column: column}
	if _, err = field.SetSelection(ctx, x, x); err != nil {
		return fmt.Errorf("width table %d: %w", table, err)
	}
	typed, err := field.InsertText(ctx, "Y")
	if err != nil {
		return err
	}
	caret := editor.Position{Line: 0, Column: column + 1}
	if typed.Text != "α→Yx" || typed.Selection != (editor.Selection{Start: caret, End: caret}) {
		return fmt.Errorf("width table %d: %+v", table, typed)
	}
	if err = field.Close(ctx); err != nil {
		return err
	}
	return waitClosed(ctx, app, field.ID())
}

// smokeClamp expects line 1 to start with 日 and line 3 with a tab.
func smokeClamp(ctx context.Context, field *editor.Editor) error {
	at := func(line, column uint32, clamp editor.Clamp) editor.Position {
		return editor.Position{Line: line, Column: column, Clamp: clamp}
	}
	for _, check := range []struct {
		start, end editor.Position
		want       editor.Selection
	}{
		{at(1, 1, editor.ClampBefore), at(1, 1, editor.ClampBefore), editor.Selection{Start: at(1, 0, 0), End: at(1, 0, 0)}},
		{at(1, 1, editor.ClampAfter), at(1, 1, editor.ClampAfter), editor.Selection{Start: at(1, 2, 0), End: at(1, 2, 0)}},
		{at(1, 1, editor.ClampBefore), at(1, 1, editor.ClampAfter), editor.Selection{Start: at(1, 0, 0), End: at(1, 2, 0)}},
		{at(3, 2, editor.ClampBefore), at(3, 2, editor.ClampAfter), editor.Selection{Start: at(3, 0, 0), End: at(3, 4, 0)}},
		{at(1, 2, editor.ClampAfter), at(1, 2, editor.ClampBefore), editor.Selection{Start: at(1, 2, 0), End: at(1, 2, 0)}}, // An exact column does not move.
	} {
		state, err := field.SetSelection(ctx, check.start, check.end)
		if err != nil {
			return fmt.Errorf("clamp %+v: %w", check, err)
		}
		if state.Selection != check.want {
			return fmt.Errorf("clamp %+v: got %+v", check, state.Selection)
		}
	}
	for _, invalid := range [][2]editor.Position{
		{at(1, 1, editor.ClampAfter), at(1, 1, editor.ClampBefore)},  // Crosses over after clamping.
		{at(0, 6, editor.ClampBefore), at(0, 6, editor.ClampBefore)}, // Past the end of "first".
		{at(1, 1, editor.ClampBefore), at(1, 1, editor.ClampNone)},   // Only one end clamped.
	} {
		_, err := field.SetSelection(ctx, invalid[0], invalid[1])
		var protocol *editor.ProtocolError
		if !errors.As(err, &protocol) || protocol.Code != editor.InvalidArgument {
			return fmt.Errorf("invalid clamp %v: %v", invalid, err)
		}
	}
	return nil
}

func waitClosed(ctx context.Context, app *editor.Client, id uint64) error {
	for {
		event, err := app.NextEvent(ctx)
		if err != nil {
			return err
		}
		if event.Kind == editor.Failed {
			return event.Error
		}
		if event.Kind == editor.EditorClosed && event.EditorID == id {
			return nil
		}
	}
}
