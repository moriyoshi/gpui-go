//! Reusable GPUI component wrapping Zed's local multiline editor.
//!
//! This GPL-3.0-or-later crate owns no application loop, windows, transport, or
//! application chrome. Initialize Zed settings/theme in your host, call [`init`],
//! then create an `Entity<EditorComponent>` and compose it as any GPUI child.
//! Subscribe to [`Change`] for native edits. The optional `host` workspace
//! member exposes the component to Go over Sable.

mod ambiguous;
mod column;

pub use editor::Editor as NativeEditor;
pub use gpui;

use column::{LineLayout, byte_character, byte_column, visual_column};

use editor::{
    Bias, EditorEvent, MultiBufferSnapshot, SelectionEffects, ToPoint, actions as edit,
    scroll::Autoscroll,
};
use gpui::{
    App, Context, Entity, EventEmitter, FocusHandle, Focusable, KeyBinding, Subscription, Window,
    div, prelude::*,
};
use language::{Point, language_settings::SoftWrap};

/// Initial configuration for one independent local buffer.
#[derive(Clone, Debug)]
pub struct EditorOptions {
    pub text: String,
    pub read_only: bool,
    pub show_gutter: bool,
    pub soft_wrap: bool,
    pub width_table: WidthTable,
}

/// How a [`Position`] counts East Asian Ambiguous characters. Zed draws glyphs
/// from the font either way, so choose the table that matches the font.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum WidthTable {
    /// Every Ambiguous character takes one column.
    #[default]
    NarrowAmbiguous,
    /// Every Ambiguous character with a width takes two columns.
    WideAmbiguous,
    /// `unicode-width`'s East Asian context: symbols, punctuation and numbers
    /// take two columns, and letters take one.
    WideSymbols,
}
impl Default for EditorOptions {
    fn default() -> Self {
        Self {
            text: String::new(),
            read_only: false,
            show_gutter: true,
            soft_wrap: true,
            width_table: WidthTable::NarrowAmbiguous,
        }
    }
}

/// A zero-based position. `line` is a buffer line, and `column` counts visual
/// cells from its start: a wide character takes two, a tab reaches its stop, and
/// a combining mark takes none. Soft wrapping does not restart the count.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Position {
    pub line: u32,
    pub column: u32,
    /// Only read when setting a selection. An alternative to `column`: Unicode
    /// code points from the start of the line. Setting both nonzero is invalid.
    pub character: u32,
    /// Only read when setting a selection. Reported positions leave it `None`.
    pub clamp: Clamp,
}

/// Where a [`Position`] inside a wide character or a tab goes.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub enum Clamp {
    /// Reject the selection.
    #[default]
    None,
    /// Moves before the character in text order, whatever its direction.
    Before,
    /// Moves after the character in text order.
    After,
}

/// `start` is never after `end`. Equal positions are a caret.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq)]
pub struct Selection {
    pub start: Position,
    pub end: Position,
}

/// An owned snapshot. Revisions are local to an editor and begin at zero.
/// `selection` is the newest selection; moving it does not change the revision.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct EditorState {
    pub revision: u64,
    pub text: String,
    pub read_only: bool,
    pub selection: Selection,
}

/// Emitted after a native or programmatic text/read-only change.
#[derive(Clone, Debug)]
pub struct Change(pub EditorState);

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct ReadOnly;
impl std::fmt::Display for ReadOnly {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("editor is read-only")
    }
}
impl std::error::Error for ReadOnly {}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct InvalidSelection;
impl std::fmt::Display for InvalidSelection {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str("selection is reversed or does not start a character in the buffer")
    }
}
impl std::error::Error for InvalidSelection {}

fn line_text(row: u32, snapshot: &MultiBufferSnapshot) -> String {
    let end = snapshot.clip_point(Point::new(row, u32::MAX), Bias::Left);
    snapshot.text_for_range(Point::new(row, 0)..end).collect()
}

fn position(point: Point, snapshot: &MultiBufferSnapshot, layout: LineLayout) -> Position {
    let line = line_text(point.row, snapshot);
    Position {
        line: point.row,
        column: visual_column(&line, point.column, layout),
        character: 0,
        clamp: Clamp::None,
    }
}

/// `None` when `position` is outside the buffer, or not at the start of a
/// character and not clamped.
fn point(position: Position, snapshot: &MultiBufferSnapshot, layout: LineLayout) -> Option<Point> {
    if position.line > snapshot.max_point().row {
        return None;
    }
    let line = line_text(position.line, snapshot);
    let column = match (position.character, position.column) {
        (0, column) => byte_column(&line, column, position.clamp, layout)?,
        (character, 0) => byte_character(&line, character, position.clamp)?,
        _ => return None,
    };
    Some(Point::new(position.line, column))
}

/// An embeddable view. Retaining its entity retains text, selection, and undo
/// history independently of the parent's render calls.
pub struct EditorComponent {
    editor: Entity<NativeEditor>,
    state: EditorState,
    width_table: WidthTable,
    _subscription: Subscription,
}
impl EventEmitter<Change> for EditorComponent {}
impl Focusable for EditorComponent {
    fn focus_handle(&self, cx: &App) -> FocusHandle {
        self.editor.focus_handle(cx)
    }
}
impl EditorComponent {
    /// Does not move keyboard focus; the host chooses when to focus this view.
    pub fn new(options: EditorOptions, window: &mut Window, cx: &mut Context<Self>) -> Self {
        let editor = cx.new(|cx| {
            let buffer = cx.new(|cx| language::Buffer::local(options.text.clone(), cx));
            let mut editor = NativeEditor::for_buffer(buffer, None, window, cx);
            editor.set_soft_wrap_mode(
                if options.soft_wrap {
                    SoftWrap::EditorWidth
                } else {
                    SoftWrap::None
                },
                cx,
            );
            editor.set_show_gutter(options.show_gutter, cx);
            editor.set_show_indent_guides(false, cx);
            editor.set_read_only(options.read_only);
            editor
        });
        let subscription = cx.subscribe_in(&editor, window, |this: &mut Self, _, event, _, cx| {
            if matches!(event, EditorEvent::BufferEdited) {
                this.refresh(cx);
            }
        });
        Self {
            editor,
            state: EditorState {
                revision: 0,
                text: options.text,
                read_only: options.read_only,
                selection: Selection::default(),
            },
            width_table: options.width_table,
            _subscription: subscription,
        }
    }
    /// Access the underlying Zed entity for native actions and extensions.
    pub fn native_editor(&self) -> &Entity<NativeEditor> {
        &self.editor
    }
    pub fn state(&mut self, cx: &mut Context<Self>) -> EditorState {
        self.refresh(cx);
        self.state.clone()
    }
    fn line_layout(&self, cx: &mut Context<Self>) -> LineLayout {
        let display = self
            .editor
            .update(cx, |editor, cx| editor.display_snapshot(cx));
        let tabs = display.tab_snapshot();
        LineLayout {
            tab_size: tabs.tab_size.get(),
            max_expansion_byte: tabs.max_expansion_column,
            width_table: self.width_table,
        }
    }
    fn refresh(&mut self, cx: &mut Context<Self>) {
        let layout = self.line_layout(cx);
        let editor = self.editor.read(cx);
        let text = editor.text(cx);
        let read_only = editor.read_only(cx);
        let snapshot = editor.buffer().read(cx).snapshot(cx);
        let newest = editor.selections.newest_anchor();
        let selection = Selection {
            start: position(newest.start.to_point(&snapshot), &snapshot, layout),
            end: position(newest.end.to_point(&snapshot), &snapshot, layout),
        };
        if text != self.state.text || read_only != self.state.read_only {
            self.state = EditorState {
                revision: self.state.revision + 1,
                text,
                read_only,
                selection,
            };
            cx.emit(Change(self.state.clone()));
            cx.notify();
        } else {
            self.state.selection = selection;
        }
    }
    /// Authoritative replacement; also permitted in read-only mode.
    pub fn set_text(
        &mut self,
        text: impl Into<String>,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> EditorState {
        self.editor.update(cx, |editor, cx| {
            editor.finalize_last_transaction(cx);
            editor.set_text(text.into(), window, cx);
            editor.finalize_last_transaction(cx);
        });
        self.state(cx)
    }
    pub fn insert_text(
        &mut self,
        text: &str,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Result<EditorState, ReadOnly> {
        if self.editor.read(cx).read_only(cx) {
            return Err(ReadOnly);
        }
        self.editor.update(cx, |editor, cx| {
            editor.finalize_last_transaction(cx);
            editor.insert(text, window, cx);
            editor.finalize_last_transaction(cx);
        });
        Ok(self.state(cx))
    }
    pub fn undo(
        &mut self,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Result<EditorState, ReadOnly> {
        if self.editor.read(cx).read_only(cx) {
            return Err(ReadOnly);
        }
        self.editor.update(cx, |editor, cx| {
            editor.undo(&Default::default(), window, cx)
        });
        Ok(self.state(cx))
    }
    pub fn redo(
        &mut self,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Result<EditorState, ReadOnly> {
        if self.editor.read(cx).read_only(cx) {
            return Err(ReadOnly);
        }
        self.editor.update(cx, |editor, cx| {
            editor.redo(&Default::default(), window, cx)
        });
        Ok(self.state(cx))
    }
    /// Replaces all selections with one and scrolls it into view. Also
    /// permitted in read-only mode. Does not move keyboard focus.
    pub fn set_selection(
        &mut self,
        selection: Selection,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Result<EditorState, InvalidSelection> {
        let layout = self.line_layout(cx);
        let snapshot = self.editor.read(cx).buffer().read(cx).snapshot(cx);
        let (Some(start), Some(end)) = (
            point(selection.start, &snapshot, layout),
            point(selection.end, &snapshot, layout),
        ) else {
            return Err(InvalidSelection);
        };
        // Compared here because clamping can reorder positions in one character.
        if start > end {
            return Err(InvalidSelection);
        }
        self.editor.update(cx, |editor, cx| {
            editor.change_selections(
                SelectionEffects::scroll(Autoscroll::fit()),
                window,
                cx,
                |selections| selections.select_ranges([start..end]),
            );
        });
        Ok(self.state(cx))
    }
    pub fn set_read_only(&mut self, value: bool, cx: &mut Context<Self>) -> EditorState {
        self.editor.update(cx, |editor, cx| {
            editor.set_read_only(value);
            cx.notify();
        });
        self.state(cx)
    }
    pub fn focus(&self, window: &mut Window, cx: &mut Context<Self>) {
        window.focus(&self.focus_handle(cx), cx);
    }
}
impl Render for EditorComponent {
    fn render(&mut self, _: &mut Window, _: &mut Context<Self>) -> impl IntoElement {
        div().size_full().child(self.editor.clone())
    }
}

/// Register Zed's editor integration once, after host settings/theme setup.
/// This does not replace settings, install key bindings, or open a window.
pub fn init(cx: &mut App) {
    editor::init(cx);
}

/// Optional conventional macOS bindings. Hosts may install their own keymap.
pub fn bind_default_keys(cx: &mut App) {
    use edit::*;
    cx.bind_keys([
        KeyBinding::new("left", MoveLeft, Some("Editor")),
        KeyBinding::new("right", MoveRight, Some("Editor")),
        KeyBinding::new("up", LineUp, Some("Editor")),
        KeyBinding::new("down", LineDown, Some("Editor")),
        KeyBinding::new("shift-left", SelectLeft, Some("Editor")),
        KeyBinding::new("shift-right", SelectRight, Some("Editor")),
        KeyBinding::new("shift-up", SelectUp, Some("Editor")),
        KeyBinding::new("shift-down", SelectDown, Some("Editor")),
        KeyBinding::new("enter", Newline, Some("Editor")),
        KeyBinding::new("tab", Tab, Some("Editor")),
        KeyBinding::new("backspace", Backspace, Some("Editor")),
        KeyBinding::new("delete", Delete, Some("Editor")),
        KeyBinding::new("cmd-a", SelectAll, Some("Editor")),
        KeyBinding::new("cmd-c", Copy, Some("Editor")),
        KeyBinding::new("cmd-v", Paste, Some("Editor")),
        KeyBinding::new("cmd-x", Cut, Some("Editor")),
        KeyBinding::new("cmd-z", Undo, Some("Editor")),
        KeyBinding::new("cmd-shift-z", Redo, Some("Editor")),
        KeyBinding::new("home", MoveToBeginningOfLine::default(), Some("Editor")),
        KeyBinding::new("end", MoveToEndOfLine::default(), Some("Editor")),
        KeyBinding::new("cmd-left", MoveToBeginningOfLine::default(), Some("Editor")),
        KeyBinding::new("cmd-right", MoveToEndOfLine::default(), Some("Editor")),
    ]);
}
