// Adapted from GPUI's input.rs example, licensed Apache-2.0:
// https://github.com/zed-industries/zed/blob/e231594b6b8818ea8d668ee2eb73e1fb09300654/crates/gpui/examples/input.rs
// Changes add protobuf synchronization/events, disabled and masked fields, bounded
// single-line values, horizontal caret scrolling, and the bridge's persistent entity
// lifecycle.
use gpui::{
    App, Bounds, ClipboardItem, Context, CursorStyle, ElementId, ElementInputHandler, Entity,
    EntityInputHandler, FocusHandle, Focusable, GlobalElementId, KeyBinding, LayoutId, MouseButton,
    MouseDownEvent, MouseMoveEvent, MouseUpEvent, PaintQuad, Pixels, Point, ShapedLine,
    SharedString, Style, TextRun, UTF16Selection, UnderlineStyle, Window, actions, div, fill,
    point, prelude::*, px, relative, rgba, size,
};
use gpui_go::{
    Hub, Message,
    gpui_go_core::{pb, text},
};
use std::{ops::Range, sync::Arc};
use unicode_segmentation::UnicodeSegmentation;

const MASK: &str = "\u{2022}";

pub(crate) fn masked_text(content: &str) -> String {
    MASK.repeat(content.graphemes(true).count())
}

/// Maps a byte offset in the real text to one in `masked_text`.
pub(crate) fn masked_offset(content: &str, offset: usize) -> usize {
    content
        .grapheme_indices(true)
        .take_while(|(index, _)| *index < offset)
        .count()
        * MASK.len()
}

/// Maps a byte offset in `masked_text` back to one in the real text.
pub(crate) fn unmasked_offset(content: &str, offset: usize) -> usize {
    content
        .grapheme_indices(true)
        .nth(offset / MASK.len())
        .map_or(content.len(), |(index, _)| index)
}

pub fn bind_keys(cx: &mut App) {
    cx.bind_keys([
        KeyBinding::new("tab", FocusNext, None),
        KeyBinding::new("shift-tab", FocusPrevious, None),
        KeyBinding::new("backspace", Backspace, Some("GpuiGoTextInput")),
        KeyBinding::new("delete", Delete, Some("GpuiGoTextInput")),
        KeyBinding::new("left", Left, Some("GpuiGoTextInput")),
        KeyBinding::new("right", Right, Some("GpuiGoTextInput")),
        KeyBinding::new("shift-left", SelectLeft, Some("GpuiGoTextInput")),
        KeyBinding::new("shift-right", SelectRight, Some("GpuiGoTextInput")),
        KeyBinding::new("home", Home, Some("GpuiGoTextInput")),
        KeyBinding::new("end", End, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-left", Home, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-right", End, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-a", SelectAll, Some("GpuiGoTextInput")),
        KeyBinding::new("ctrl-a", SelectAll, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-v", Paste, Some("GpuiGoTextInput")),
        KeyBinding::new("ctrl-v", Paste, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-c", Copy, Some("GpuiGoTextInput")),
        KeyBinding::new("ctrl-c", Copy, Some("GpuiGoTextInput")),
        KeyBinding::new("cmd-x", Cut, Some("GpuiGoTextInput")),
        KeyBinding::new("ctrl-x", Cut, Some("GpuiGoTextInput")),
        KeyBinding::new(
            "ctrl-cmd-space",
            ShowCharacterPalette,
            Some("GpuiGoTextInput"),
        ),
        KeyBinding::new("tab", FocusNext, Some("GpuiGoTextInput")),
        KeyBinding::new("shift-tab", FocusPrevious, Some("GpuiGoTextInput")),
    ]);
}

actions!(
    text_input,
    [
        Backspace,
        Delete,
        Left,
        Right,
        SelectLeft,
        SelectRight,
        SelectAll,
        Home,
        End,
        ShowCharacterPalette,
        Paste,
        Cut,
        Copy,
        FocusNext,
        FocusPrevious,
    ]
);

pub(crate) struct TextInput {
    hub: Arc<Hub>,
    owner: pb::Handle,
    element_id: u64,
    epoch: u64,
    edit_revision: u64,
    disabled: bool,
    masked: bool,
    scroll_x: Pixels,
    focus_handle: FocusHandle,
    content: SharedString,
    placeholder: SharedString,
    selected_range: Range<usize>,
    selection_reversed: bool,
    marked_range: Option<Range<usize>>,
    last_layout: Option<ShapedLine>,
    last_bounds: Option<Bounds<Pixels>>,
    is_selecting: bool,
}

impl TextInput {
    #[cfg(feature = "automation")]
    pub(crate) fn automation_state(&self) -> pb::AutomationInputState {
        let selected = self.range_to_utf16(&self.selected_range);
        let marked = self.marked_range.as_ref().map(|r| self.range_to_utf16(r));
        pb::AutomationInputState {
            value: self.content.to_string(),
            epoch: self.epoch,
            edit_revision: self.edit_revision,
            selection_start: selected.start as u32,
            selection_end: selected.end as u32,
            marked_start: marked.as_ref().map(|r| r.start as u32),
            marked_end: marked.map(|r| r.end as u32),
            text_bounds: self.last_bounds.map(|b| pb::Bounds {
                origin: Some(pb::Point {
                    x: b.origin.x.into(),
                    y: b.origin.y.into(),
                }),
                size: Some(pb::Size {
                    width: b.size.width.into(),
                    height: b.size.height.into(),
                }),
            }),
        }
    }
    #[cfg(test)]
    pub(crate) fn value(&self) -> &str {
        &self.content
    }
    pub(crate) fn accessible_role(&self) -> gpui::Role {
        if self.masked {
            gpui::Role::PasswordInput
        } else {
            gpui::Role::TextInput
        }
    }
    pub(crate) fn accessible_value(&self) -> String {
        self.displayed_text().to_string()
    }
    #[cfg(test)]
    pub(crate) fn shaped_text(&self) -> Option<String> {
        self.last_layout.as_ref().map(|line| line.text.to_string())
    }
    fn displayed_text(&self) -> SharedString {
        if self.masked {
            masked_text(&self.content).into()
        } else {
            self.content.clone()
        }
    }
    fn display_offset(&self, offset: usize) -> usize {
        if self.masked {
            masked_offset(&self.content, offset)
        } else {
            offset
        }
    }
    pub(crate) fn display_range(&self, range: &Range<usize>) -> Range<usize> {
        self.display_offset(range.start)..self.display_offset(range.end)
    }
    /// The selection and caret as byte offsets in the shaped line.
    pub(crate) fn display_selection(&self) -> (Range<usize>, usize) {
        (
            self.display_range(&self.selected_range),
            self.display_offset(self.cursor_offset()),
        )
    }
    #[cfg(test)]
    pub(crate) fn shaped_x(&self, offset: usize) -> Option<Pixels> {
        self.last_layout
            .as_ref()
            .map(|line| line.x_for_index(offset))
    }
    fn content_offset(&self, offset: usize) -> usize {
        if self.masked {
            unmasked_offset(&self.content, offset)
        } else {
            offset
        }
    }
    pub fn new(
        props: &pb::TextInput,
        owner: pb::Handle,
        element_id: u64,
        hub: Arc<Hub>,
        cx: &mut Context<Self>,
    ) -> Self {
        Self {
            focus_handle: cx.focus_handle().tab_stop(!props.disabled),
            content: props.value.clone().into(),
            placeholder: props.placeholder.clone().into(),
            selected_range: props.value.len()..props.value.len(),
            selection_reversed: false,
            marked_range: None,
            last_layout: None,
            last_bounds: None,
            is_selecting: false,
            scroll_x: px(0.),
            epoch: props.epoch,
            edit_revision: props.edit_revision,
            disabled: props.disabled,
            masked: props.masked,
            hub,
            owner,
            element_id,
        }
    }

    pub fn sync(&mut self, props: &pb::TextInput, window: &mut Window, cx: &mut Context<Self>) {
        self.focus_handle = self.focus_handle.clone().tab_stop(!props.disabled);
        let mut changed = self.placeholder.as_ref() != props.placeholder
            || self.disabled != props.disabled
            || self.masked != props.masked;
        if self.masked != props.masked {
            self.last_layout = None;
        }
        self.placeholder = props.placeholder.clone().into();
        self.disabled = props.disabled;
        self.masked = props.masked;
        if text::accepts_snapshot(
            self.epoch,
            self.edit_revision,
            props.epoch,
            props.edit_revision,
        ) {
            if self.epoch != props.epoch || self.content.as_ref() != props.value {
                changed = true;
                self.content = props.value.clone().into();
                self.selected_range = self.content.len()..self.content.len();
                self.selection_reversed = false;
                self.marked_range = None;
                self.last_layout = None;
                self.is_selecting = false;
                window.notify_text_input_changed(&self.focus_handle);
            }
            self.epoch = props.epoch;
            self.edit_revision = props.edit_revision;
        }
        if changed {
            cx.notify();
        }
    }

    fn changed(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.edit_revision += 1;
        let change = pb::TextChanged {
            value: self.content.to_string(),
            epoch: self.epoch,
            edit_revision: self.edit_revision,
            composing: self.marked_range.is_some(),
        };
        if let Err(failure) = self.hub.input(Message::TextChanged {
            entity: self.owner.clone(),
            element: self.element_id,
            change,
        }) {
            self.hub.finish(failure);
            cx.quit();
        }
        window.notify_text_input_changed(&self.focus_handle);
        cx.notify();
    }

    fn focus_next(&mut self, _: &FocusNext, window: &mut Window, cx: &mut Context<Self>) {
        window.focus_next(cx);
    }
    fn focus_previous(&mut self, _: &FocusPrevious, window: &mut Window, cx: &mut Context<Self>) {
        window.focus_prev(cx);
    }

    fn left(&mut self, _: &Left, window: &mut Window, cx: &mut Context<Self>) {
        if self.selected_range.is_empty() {
            self.move_to(self.previous_boundary(self.cursor_offset()), window, cx);
        } else {
            self.move_to(self.selected_range.start, window, cx)
        }
    }

    fn right(&mut self, _: &Right, window: &mut Window, cx: &mut Context<Self>) {
        if self.selected_range.is_empty() {
            self.move_to(self.next_boundary(self.selected_range.end), window, cx);
        } else {
            self.move_to(self.selected_range.end, window, cx)
        }
    }

    fn select_left(&mut self, _: &SelectLeft, window: &mut Window, cx: &mut Context<Self>) {
        self.select_to(self.previous_boundary(self.cursor_offset()), window, cx);
    }

    fn select_right(&mut self, _: &SelectRight, window: &mut Window, cx: &mut Context<Self>) {
        self.select_to(self.next_boundary(self.cursor_offset()), window, cx);
    }

    fn select_all(&mut self, _: &SelectAll, window: &mut Window, cx: &mut Context<Self>) {
        self.move_to(0, window, cx);
        self.select_to(self.content.len(), window, cx)
    }

    fn home(&mut self, _: &Home, window: &mut Window, cx: &mut Context<Self>) {
        self.move_to(0, window, cx);
    }

    fn end(&mut self, _: &End, window: &mut Window, cx: &mut Context<Self>) {
        self.move_to(self.content.len(), window, cx);
    }

    fn backspace(&mut self, _: &Backspace, window: &mut Window, cx: &mut Context<Self>) {
        if self.selected_range.is_empty() {
            let prev = self.previous_boundary(self.cursor_offset());
            if self.cursor_offset() == prev {
                window.play_system_bell();
                return;
            }
            self.select_to(prev, window, cx)
        }
        self.replace_text_in_range(None, "", window, cx)
    }

    fn delete(&mut self, _: &Delete, window: &mut Window, cx: &mut Context<Self>) {
        if self.selected_range.is_empty() {
            let next = self.next_boundary(self.cursor_offset());
            if self.cursor_offset() == next {
                window.play_system_bell();
                return;
            }
            self.select_to(next, window, cx)
        }
        self.replace_text_in_range(None, "", window, cx)
    }

    fn on_mouse_down(
        &mut self,
        event: &MouseDownEvent,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        if self.disabled {
            return;
        }
        window.focus(&self.focus_handle, cx);
        self.is_selecting = true;
        window.request_virtual_keyboard();

        if event.modifiers.shift {
            self.select_to(self.index_for_mouse_position(event.position), window, cx);
        } else {
            self.move_to(self.index_for_mouse_position(event.position), window, cx)
        }
    }

    fn on_mouse_up(&mut self, _: &MouseUpEvent, _window: &mut Window, _: &mut Context<Self>) {
        self.is_selecting = false;
    }

    fn on_mouse_move(
        &mut self,
        event: &MouseMoveEvent,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        if self.is_selecting {
            self.select_to(self.index_for_mouse_position(event.position), window, cx);
        }
    }

    fn show_character_palette(
        &mut self,
        _: &ShowCharacterPalette,
        window: &mut Window,
        _: &mut Context<Self>,
    ) {
        window.show_character_palette();
    }

    fn paste(&mut self, _: &Paste, window: &mut Window, cx: &mut Context<Self>) {
        if let Some(text) = cx.read_from_clipboard().and_then(|item| item.text()) {
            self.replace_text_in_range(None, &text.replace("\n", " "), window, cx);
        }
    }

    fn copy(&mut self, _: &Copy, _: &mut Window, cx: &mut Context<Self>) {
        if !self.disabled && !self.masked && !self.selected_range.is_empty() {
            cx.write_to_clipboard(ClipboardItem::new_string(
                self.content[self.selected_range.clone()].to_string(),
            ));
        }
    }
    fn cut(&mut self, _: &Cut, window: &mut Window, cx: &mut Context<Self>) {
        if !self.disabled && !self.masked && !self.selected_range.is_empty() {
            cx.write_to_clipboard(ClipboardItem::new_string(
                self.content[self.selected_range.clone()].to_string(),
            ));
            self.replace_text_in_range(None, "", window, cx)
        }
    }

    fn move_to(&mut self, offset: usize, window: &Window, cx: &mut Context<Self>) {
        self.selected_range = offset..offset;
        self.selection_reversed = false;
        window.notify_text_selection_changed(&self.focus_handle);
        cx.notify()
    }

    fn cursor_offset(&self) -> usize {
        if self.selection_reversed {
            self.selected_range.start
        } else {
            self.selected_range.end
        }
    }

    fn index_for_mouse_position(&self, position: Point<Pixels>) -> usize {
        if self.content.is_empty() {
            return 0;
        }

        let (Some(bounds), Some(line)) = (self.last_bounds.as_ref(), self.last_layout.as_ref())
        else {
            return 0;
        };
        if position.y < bounds.top() {
            return 0;
        }
        if position.y > bounds.bottom() {
            return self.content.len();
        }
        self.content_offset(line.closest_index_for_x(position.x - bounds.left() + self.scroll_x))
    }

    fn select_to(&mut self, offset: usize, window: &Window, cx: &mut Context<Self>) {
        if self.selection_reversed {
            self.selected_range.start = offset
        } else {
            self.selected_range.end = offset
        };
        if self.selected_range.end < self.selected_range.start {
            self.selection_reversed = !self.selection_reversed;
            self.selected_range = self.selected_range.end..self.selected_range.start;
        }
        window.notify_text_selection_changed(&self.focus_handle);
        cx.notify()
    }

    fn offset_to_utf16(&self, offset: usize) -> usize {
        text::utf8_to_utf16(&self.content, offset)
    }

    fn range_to_utf16(&self, range: &Range<usize>) -> Range<usize> {
        self.offset_to_utf16(range.start)..self.offset_to_utf16(range.end)
    }

    fn range_from_utf16(&self, range_utf16: &Range<usize>) -> Range<usize> {
        text::range_from_utf16(&self.content, range_utf16)
    }

    fn previous_boundary(&self, offset: usize) -> usize {
        self.content
            .grapheme_indices(true)
            .rev()
            .find_map(|(idx, _)| (idx < offset).then_some(idx))
            .unwrap_or(0)
    }

    fn next_boundary(&self, offset: usize) -> usize {
        self.content
            .grapheme_indices(true)
            .find_map(|(idx, _)| (idx > offset).then_some(idx))
            .unwrap_or(self.content.len())
    }
}

impl EntityInputHandler for TextInput {
    fn text_for_range(
        &mut self,
        range_utf16: Range<usize>,
        actual_range: &mut Option<Range<usize>>,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<String> {
        if self.masked {
            return None;
        }
        let range = self.range_from_utf16(&range_utf16);
        actual_range.replace(self.range_to_utf16(&range));
        Some(self.content[range].to_string())
    }

    fn selected_text_range(
        &mut self,
        _ignore_disabled_input: bool,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<UTF16Selection> {
        if self.disabled {
            return None;
        }
        Some(UTF16Selection {
            range: self.range_to_utf16(&self.selected_range),
            reversed: self.selection_reversed,
        })
    }

    fn set_selected_text_range(
        &mut self,
        range_utf16: Range<usize>,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        self.selected_range = self.range_from_utf16(&range_utf16);
        self.selection_reversed = false;
        window.notify_text_selection_changed(&self.focus_handle);
        cx.notify();
    }

    fn text_length_utf16(
        &mut self,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<usize> {
        Some(self.content.encode_utf16().count())
    }

    fn marked_text_range(
        &self,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<Range<usize>> {
        self.marked_range
            .as_ref()
            .map(|range| self.range_to_utf16(range))
    }

    fn unmark_text(&mut self, window: &mut Window, cx: &mut Context<Self>) {
        self.marked_range = None;
        self.changed(window, cx);
    }

    fn replace_text_in_range(
        &mut self,
        range_utf16: Option<Range<usize>>,
        new_text: &str,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        let range = range_utf16
            .as_ref()
            .map(|range_utf16| self.range_from_utf16(range_utf16))
            .or(self.marked_range.clone())
            .unwrap_or(self.selected_range.clone());

        if self.disabled {
            return;
        }
        let normalized = text::single_line(new_text);
        let new_text = normalized.as_str();
        if self.content.len() - range.len() + new_text.len() > text::MAX_INPUT_BYTES {
            window.play_system_bell();
            return;
        }
        self.content =
            (self.content[0..range.start].to_owned() + new_text + &self.content[range.end..])
                .into();
        self.selected_range = range.start + new_text.len()..range.start + new_text.len();
        self.selection_reversed = false;
        self.marked_range.take();
        self.changed(window, cx);
    }

    fn replace_and_mark_text_in_range(
        &mut self,
        range_utf16: Option<Range<usize>>,
        new_text: &str,
        new_selected_range_utf16: Option<Range<usize>>,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        let range = range_utf16
            .as_ref()
            .map(|range_utf16| self.range_from_utf16(range_utf16))
            .or(self.marked_range.clone())
            .unwrap_or(self.selected_range.clone());

        if self.disabled {
            return;
        }
        let normalized = text::single_line(new_text);
        let new_text = normalized.as_str();
        if self.content.len() - range.len() + new_text.len() > text::MAX_INPUT_BYTES {
            window.play_system_bell();
            return;
        }
        self.content =
            (self.content[0..range.start].to_owned() + new_text + &self.content[range.end..])
                .into();
        if !new_text.is_empty() {
            self.marked_range = Some(range.start..range.start + new_text.len());
        } else {
            self.marked_range = None;
        }
        let insertion_start_utf16 = self.offset_to_utf16(range.start);
        let inserted_length_utf16 = new_text.encode_utf16().count();
        self.selected_range = new_selected_range_utf16
            .map(|selected| {
                self.range_from_utf16(
                    &(insertion_start_utf16 + selected.start.min(inserted_length_utf16)
                        ..insertion_start_utf16 + selected.end.min(inserted_length_utf16)),
                )
            })
            .unwrap_or_else(|| range.start + new_text.len()..range.start + new_text.len());
        self.selection_reversed = false;

        self.changed(window, cx);
    }

    fn bounds_for_range(
        &mut self,
        range_utf16: Range<usize>,
        bounds: Bounds<Pixels>,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<Bounds<Pixels>> {
        let last_layout = self.last_layout.as_ref()?;
        if last_layout.text != self.displayed_text() {
            return None;
        }
        let range = self.range_from_utf16(&range_utf16);
        Some(Bounds::from_corners(
            point(
                bounds.left() - self.scroll_x
                    + last_layout.x_for_index(self.display_offset(range.start)),
                bounds.top(),
            ),
            point(
                bounds.left() - self.scroll_x
                    + last_layout.x_for_index(self.display_offset(range.end)),
                bounds.bottom(),
            ),
        ))
    }

    fn selection_bounds_for_range(
        &mut self,
        range_utf16: Range<usize>,
        bounds: Bounds<Pixels>,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Vec<Bounds<Pixels>> {
        self.bounds_for_range(range_utf16, bounds, window, cx)
            .into_iter()
            .collect()
    }

    fn character_index_for_point(
        &mut self,
        point: gpui::Point<Pixels>,
        _window: &mut Window,
        _cx: &mut Context<Self>,
    ) -> Option<usize> {
        let line_point = self.last_bounds?.localize(&point)?;
        let last_layout = self.last_layout.as_ref()?;

        if last_layout.text != self.displayed_text() {
            return None;
        }
        let utf8_index =
            self.content_offset(last_layout.closest_index_for_x(line_point.x + self.scroll_x));
        Some(self.offset_to_utf16(utf8_index))
    }
}

struct TextElement {
    input: Entity<TextInput>,
}

struct PrepaintState {
    line: Option<ShapedLine>,
    cursor: Option<PaintQuad>,
    selection: Option<PaintQuad>,
    scroll_x: Pixels,
}

impl IntoElement for TextElement {
    type Element = Self;

    fn into_element(self) -> Self::Element {
        self
    }
}

impl Element for TextElement {
    type RequestLayoutState = ();
    type PrepaintState = PrepaintState;

    fn id(&self) -> Option<ElementId> {
        None
    }

    fn source_location(&self) -> Option<&'static core::panic::Location<'static>> {
        None
    }

    fn request_layout(
        &mut self,
        _id: Option<&GlobalElementId>,
        _inspector_id: Option<&gpui::InspectorElementId>,
        window: &mut Window,
        cx: &mut App,
    ) -> (LayoutId, Self::RequestLayoutState) {
        let mut style = Style::default();
        style.size.width = relative(1.).into();
        style.size.height = window.line_height().into();
        (window.request_layout(style, [], cx), ())
    }

    fn prepaint(
        &mut self,
        _id: Option<&GlobalElementId>,
        _inspector_id: Option<&gpui::InspectorElementId>,
        bounds: Bounds<Pixels>,
        _request_layout: &mut Self::RequestLayoutState,
        window: &mut Window,
        cx: &mut App,
    ) -> Self::PrepaintState {
        let input = self.input.read(cx);
        let content = input.displayed_text();
        let (selected_range, cursor) = input.display_selection();
        let marked_range = input
            .marked_range
            .as_ref()
            .map(|range| input.display_range(range));
        let style = window.text_style();

        let (display_text, text_color) = if content.is_empty() {
            {
                let mut color = style.color;
                color.a *= 0.45;
                (input.placeholder.clone(), color)
            }
        } else {
            (content, style.color)
        };

        let run = TextRun {
            len: display_text.len(),
            font: style.font(),
            color: text_color,
            background_color: None,
            underline: None,
            strikethrough: None,
        };
        let runs = if let Some(marked_range) = marked_range.as_ref() {
            vec![
                TextRun {
                    len: marked_range.start,
                    ..run.clone()
                },
                TextRun {
                    len: marked_range.end - marked_range.start,
                    underline: Some(UnderlineStyle {
                        color: Some(run.color),
                        thickness: px(1.0),
                        wavy: false,
                    }),
                    ..run.clone()
                },
                TextRun {
                    len: display_text.len() - marked_range.end,
                    ..run
                },
            ]
            .into_iter()
            .filter(|run| run.len > 0)
            .collect()
        } else {
            vec![run]
        };

        let font_size = style.font_size.to_pixels(window.rem_size());
        let line = window
            .text_system()
            .shape_line(display_text, font_size, &runs, None);

        let caret = line.x_for_index(cursor);
        let visible = (bounds.size.width - px(2.)).max(px(1.));
        let mut scroll_x = input.scroll_x.min((line.width - visible).max(px(0.)));
        if caret - scroll_x > visible {
            scroll_x = caret - visible;
        }
        if caret < scroll_x {
            scroll_x = caret;
        }
        let cursor_pos = caret - scroll_x;
        let (selection, cursor) = if selected_range.is_empty() {
            (
                None,
                Some(fill(
                    Bounds::new(
                        point(bounds.left() + cursor_pos, bounds.top()),
                        size(px(2.), bounds.bottom() - bounds.top()),
                    ),
                    style.color,
                )),
            )
        } else {
            (
                Some(fill(
                    Bounds::from_corners(
                        point(
                            bounds.left() - scroll_x + line.x_for_index(selected_range.start),
                            bounds.top(),
                        ),
                        point(
                            bounds.left() - scroll_x + line.x_for_index(selected_range.end),
                            bounds.bottom(),
                        ),
                    ),
                    rgba(0x647dee55),
                )),
                None,
            )
        };
        PrepaintState {
            line: Some(line),
            cursor,
            selection,
            scroll_x,
        }
    }

    fn paint(
        &mut self,
        _id: Option<&GlobalElementId>,
        _inspector_id: Option<&gpui::InspectorElementId>,
        bounds: Bounds<Pixels>,
        _request_layout: &mut Self::RequestLayoutState,
        prepaint: &mut Self::PrepaintState,
        window: &mut Window,
        cx: &mut App,
    ) {
        let focus_handle = self.input.read(cx).focus_handle.clone();
        if !self.input.read(cx).disabled {
            window.handle_input(
                &focus_handle,
                ElementInputHandler::new(bounds, self.input.clone()),
                cx,
            );
        }
        if let Some(selection) = prepaint.selection.take() {
            window.paint_quad(selection)
        }
        let line = prepaint.line.take().unwrap();
        line.paint(
            point(bounds.left() - prepaint.scroll_x, bounds.top()),
            window.line_height(),
            gpui::TextAlign::Left,
            None,
            window,
            cx,
        )
        .unwrap();

        if !self.input.read(cx).disabled
            && focus_handle.is_focused(window)
            && let Some(cursor) = prepaint.cursor.take()
        {
            window.paint_quad(cursor);
        }

        self.input.update(cx, |input, _cx| {
            input.last_layout = Some(line);
            input.last_bounds = Some(bounds);
            input.scroll_x = prepaint.scroll_x;
        });
    }
}

impl Render for TextInput {
    fn render(&mut self, _window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let input = cx.weak_entity();
        let disabled = self.disabled;
        div()
            .id("native-text-input")
            .role(self.accessible_role())
            .aria_value(self.accessible_value())
            .aria_placeholder(self.placeholder.to_string())
            .on_a11y_action(gpui::AccessibleAction::SetValue, move |data, window, cx| {
                if let Some(gpui::accesskit::ActionData::Value(value)) = data {
                    let _ = input.update(cx, |input, cx| {
                        let full = 0..input.content.encode_utf16().count();
                        input.replace_text_in_range(Some(full), value.as_ref(), window, cx);
                    });
                }
            })
            .flex()
            .w_full()
            .h(px(24.))
            .overflow_hidden()
            .key_context("GpuiGoTextInput")
            .track_focus(&self.focus_handle(cx))
            .capture_any_mouse_down(move |_, window, _| {
                if disabled {
                    window.prevent_default();
                }
            })
            .cursor(CursorStyle::IBeam)
            .on_action(cx.listener(Self::backspace))
            .on_action(cx.listener(Self::delete))
            .on_action(cx.listener(Self::left))
            .on_action(cx.listener(Self::right))
            .on_action(cx.listener(Self::select_left))
            .on_action(cx.listener(Self::select_right))
            .on_action(cx.listener(Self::select_all))
            .on_action(cx.listener(Self::home))
            .on_action(cx.listener(Self::end))
            .on_action(cx.listener(Self::show_character_palette))
            .on_action(cx.listener(Self::paste))
            .on_action(cx.listener(Self::cut))
            .on_action(cx.listener(Self::copy))
            .on_action(cx.listener(Self::focus_next))
            .on_action(cx.listener(Self::focus_previous))
            .on_mouse_down(MouseButton::Left, cx.listener(Self::on_mouse_down))
            .on_mouse_up(MouseButton::Left, cx.listener(Self::on_mouse_up))
            .on_mouse_up_out(MouseButton::Left, cx.listener(Self::on_mouse_up))
            .on_mouse_move(cx.listener(Self::on_mouse_move))
            .line_height(px(24.))
            .child(TextElement { input: cx.entity() })
    }
}

impl Focusable for TextInput {
    fn focus_handle(&self, _: &App) -> FocusHandle {
        self.focus_handle.clone()
    }
}
