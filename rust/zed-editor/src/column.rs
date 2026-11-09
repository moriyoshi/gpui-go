//! Conversions between the buffer's UTF-8 byte columns and the visual columns
//! of the public API. These mirror how the pinned Zed revision lays out a line.

use std::iter;

use editor::display_map::{is_invisible, replacement};
use unicode_segmentation::UnicodeSegmentation;
use unicode_width::UnicodeWidthStr;

use crate::{Clamp, WidthTable, ambiguous::is_ambiguous};

/// What the width of a line depends on. The tab fields come from Zed's display map.
#[derive(Clone, Copy, Debug)]
pub(crate) struct LineLayout {
    pub tab_size: u32,
    /// A tab starting at or after this byte is painted one cell wide.
    pub max_expansion_byte: u32,
    pub width_table: WidthTable,
}

/// Each grapheme boundary of `line`, and its end, as `(byte, visual column)`.
fn boundaries(line: &str, layout: LineLayout) -> impl Iterator<Item = (u32, u32)> + '_ {
    let (mut cells, mut chars) = (0, 0);
    line.grapheme_indices(true)
        .chain(iter::once((line.len(), "")))
        .map(move |(byte, grapheme)| {
            let boundary = (byte as u32, cells);
            if grapheme == "\t" {
                // Zed finds the next stop by counting characters, so wide
                // characters before a tab do not move the stop.
                let size = if (byte as u32) < layout.max_expansion_byte {
                    layout.tab_size
                } else {
                    1
                };
                let advance = size - chars % size;
                cells += advance;
                chars += advance;
            } else {
                cells += grapheme_cells(grapheme, layout.width_table);
                chars += grapheme.chars().count() as u32;
            }
            boundary
        })
}

/// `unicode-width` applies the East Asian Width table, gives combining marks no
/// width and gives emoji presentation sequences two cells.
fn grapheme_cells(grapheme: &str, table: WidthTable) -> u32 {
    let mut chars = grapheme.chars();
    let first = chars.next();
    match (first, chars.next()) {
        // Zed paints a one-cell glyph in place of a lone invisible character.
        (Some(c), None) if is_invisible(c) && replacement(c).is_some() => 1,
        _ => match table {
            WidthTable::NarrowAmbiguous => grapheme.width() as u32,
            WidthTable::WideSymbols => grapheme.width_cjk() as u32,
            WidthTable::WideAmbiguous => match grapheme.width() {
                1 if first.is_some_and(is_ambiguous) => 2,
                width => width as u32,
            },
        },
    }
}

/// The visual column of `byte`. A byte inside a grapheme reports its start.
pub(crate) fn visual_column(line: &str, byte: u32, layout: LineLayout) -> u32 {
    boundaries(line, layout)
        .take_while(|&(boundary, _)| boundary <= byte)
        .last()
        .map_or(0, |(_, column)| column)
}

/// The byte at `column`. A column inside a tab or a wide character moves as
/// `clamp` says. `None` when `clamp` is [`Clamp::None`] there, or when the column
/// is past the end of the line.
pub(crate) fn byte_column(
    line: &str,
    column: u32,
    clamp: Clamp,
    layout: LineLayout,
) -> Option<u32> {
    resolve(boundaries(line, layout), column, clamp)
}

/// The byte `character` code points into `line`. A count inside a grapheme
/// cluster moves as `clamp` says, and one past the end is `None`.
pub(crate) fn byte_character(line: &str, character: u32, clamp: Clamp) -> Option<u32> {
    let mut chars = 0;
    let stops = line
        .grapheme_indices(true)
        .chain(iter::once((line.len(), "")))
        .map(move |(byte, grapheme)| {
            let stop = (byte as u32, chars);
            chars += grapheme.chars().count() as u32;
            stop
        });
    resolve(stops, character, clamp)
}

/// Finds `target` in ascending `(byte, measure)` boundaries.
fn resolve(boundaries: impl Iterator<Item = (u32, u32)>, target: u32, clamp: Clamp) -> Option<u32> {
    let mut previous = None;
    for (byte, measure) in boundaries {
        if measure == target {
            return Some(byte);
        }
        if measure > target {
            return match clamp {
                Clamp::None => None,
                Clamp::Before => previous,
                Clamp::After => Some(byte),
            };
        }
        previous = Some(byte);
    }
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    const LAYOUT: LineLayout = LineLayout {
        tab_size: 4,
        max_expansion_byte: 256,
        width_table: WidthTable::NarrowAmbiguous,
    };
    const WIDE: LineLayout = LineLayout {
        width_table: WidthTable::WideAmbiguous,
        ..LAYOUT
    };
    const SYMBOLS: LineLayout = LineLayout {
        width_table: WidthTable::WideSymbols,
        ..LAYOUT
    };

    fn round_trip(line: &str, pairs: &[(u32, u32)]) {
        for &(byte, column) in pairs {
            assert_eq!(visual_column(line, byte, LAYOUT), column, "byte {byte}");
            assert_eq!(
                byte_column(line, column, Clamp::None, LAYOUT),
                Some(byte),
                "column {column}"
            );
        }
    }

    #[test]
    fn ascii_columns_are_bytes() {
        round_trip("abc", &[(0, 0), (1, 1), (3, 3)]);
        assert_eq!(byte_column("abc", 4, Clamp::None, LAYOUT), None);
        assert_eq!(byte_column("", 0, Clamp::None, LAYOUT), Some(0));
    }

    #[test]
    fn wide_characters_take_two_cells() {
        round_trip("日本x", &[(0, 0), (3, 2), (6, 4), (7, 5)]);
        assert_eq!(byte_column("日本x", 1, Clamp::None, LAYOUT), None);
        assert_eq!(byte_column("日本x", 3, Clamp::None, LAYOUT), None);
    }

    #[test]
    fn tabs_advance_to_the_next_stop() {
        round_trip("\tx", &[(0, 0), (1, 4), (2, 5)]);
        round_trip("ab\tx", &[(2, 2), (3, 4)]);
        round_trip("abcd\tx", &[(4, 4), (5, 8)]);
        for inside in 1..4 {
            assert_eq!(byte_column("\tx", inside, Clamp::None, LAYOUT), None);
        }
    }

    #[test]
    fn tab_stops_count_characters_like_zed() {
        // 日 is one character: the tab advances three, from character 1 to 4.
        round_trip("日\tz", &[(3, 2), (4, 5), (5, 6)]);
    }

    #[test]
    fn tabs_past_the_expansion_limit_take_one_cell() {
        let line = format!("{}\tx", "a".repeat(256));
        round_trip(&line, &[(256, 256), (257, 257)]);
    }

    #[test]
    fn combining_marks_share_their_base_cell() {
        round_trip("e\u{301}x", &[(0, 0), (3, 1), (4, 2)]);
        // A byte inside the cluster reports the cluster's start.
        assert_eq!(visual_column("e\u{301}x", 1, LAYOUT), 0);
    }

    #[test]
    fn emoji_sequences_are_one_wide_cluster() {
        let line = "\u{1f469}\u{200d}\u{1f4bb}x";
        round_trip(line, &[(0, 0), (11, 2), (12, 3)]);
    }

    #[test]
    fn invisible_characters_paint_a_cell() {
        round_trip("\u{1}x", &[(0, 0), (1, 1), (2, 2)]);
        round_trip("a\u{200b}x", &[(1, 1), (4, 2), (5, 3)]);
    }

    #[test]
    fn clamping_moves_out_of_wide_characters() {
        let line = "日本x";
        assert_eq!(byte_column(line, 1, Clamp::Before, LAYOUT), Some(0));
        assert_eq!(byte_column(line, 1, Clamp::After, LAYOUT), Some(3));
        assert_eq!(byte_column(line, 3, Clamp::Before, LAYOUT), Some(3));
        assert_eq!(byte_column(line, 3, Clamp::After, LAYOUT), Some(6));
    }

    #[test]
    fn clamping_moves_out_of_tabs() {
        for inside in 1..4 {
            assert_eq!(byte_column("\tx", inside, Clamp::Before, LAYOUT), Some(0));
            assert_eq!(byte_column("\tx", inside, Clamp::After, LAYOUT), Some(1));
        }
    }

    #[test]
    fn clamping_leaves_exact_columns_alone() {
        for clamp in [Clamp::Before, Clamp::After] {
            assert_eq!(byte_column("日本x", 2, clamp, LAYOUT), Some(3));
        }
    }

    #[test]
    fn clamping_does_not_rescue_columns_past_the_end() {
        for clamp in [Clamp::None, Clamp::Before, Clamp::After] {
            assert_eq!(byte_column("ab", 3, clamp, LAYOUT), None);
        }
    }

    fn width(text: &str, layout: LineLayout) -> u32 {
        visual_column(text, text.len() as u32, layout)
    }

    #[test]
    fn each_table_counts_ambiguous_letters_and_symbols() {
        for (text, narrow, wide, symbols) in [
            ("α", 1, 2, 1),
            ("Я", 1, 2, 1),
            ("é", 1, 2, 1),
            ("●", 1, 2, 2),
            ("→", 1, 2, 2),
            ("①", 1, 2, 2),
        ] {
            assert_eq!(width(text, LAYOUT), narrow, "narrow {text}");
            assert_eq!(width(text, WIDE), wide, "wide {text}");
            assert_eq!(width(text, SYMBOLS), symbols, "symbols {text}");
        }
    }

    #[test]
    fn strict_table_leaves_neutral_characters_narrow() {
        // Neutral in the table, though the symbols rule widens them.
        for text in ["⅐", "↚"] {
            assert_eq!(width(text, WIDE), 1, "wide {text}");
            assert_eq!(width(text, SYMBOLS), 2, "symbols {text}");
        }
    }

    #[test]
    fn table_applies_in_both_directions() {
        for byte_and_column in [(0, 0), (2, 2), (5, 4), (6, 5)] {
            let (byte, column) = byte_and_column;
            assert_eq!(visual_column("α→x", byte, WIDE), column);
            assert_eq!(byte_column("α→x", column, Clamp::None, WIDE), Some(byte));
        }
        assert_eq!(byte_column("α→x", 1, Clamp::None, WIDE), None);
        assert_eq!(byte_column("α→x", 1, Clamp::After, WIDE), Some(2));
        assert_eq!(byte_column("α→x", 3, Clamp::Before, SYMBOLS), Some(5));
    }

    #[test]
    fn combining_marks_stay_on_their_base_in_every_table() {
        for layout in [LAYOUT, WIDE, SYMBOLS] {
            assert_eq!(width("e\u{301}", layout), 1);
        }
        assert_eq!(width("\u{3b1}\u{301}", WIDE), 2);
    }

    #[test]
    fn unambiguous_widths_ignore_the_table() {
        for layout in [LAYOUT, WIDE, SYMBOLS] {
            for (text, cells) in [("Ａ", 2), ("日", 2), ("ｱ", 1), ("a", 1)] {
                assert_eq!(width(text, layout), cells, "{text}");
            }
        }
    }

    #[test]
    fn characters_count_code_points() {
        // Neither a tab nor a wide character counts as more than one.
        let line = "\t日\tz";
        for (character, byte) in [(0, 0), (1, 1), (2, 4), (3, 5), (4, 6)] {
            assert_eq!(byte_character(line, character, Clamp::None), Some(byte));
        }
        assert_eq!(byte_character(line, 5, Clamp::None), None);
        assert_eq!(byte_character(line, 5, Clamp::After), None);
    }

    #[test]
    fn characters_inside_a_cluster_follow_the_clamp() {
        let line = "e\u{301}x";
        assert_eq!(byte_character(line, 1, Clamp::None), None);
        assert_eq!(byte_character(line, 1, Clamp::Before), Some(0));
        assert_eq!(byte_character(line, 1, Clamp::After), Some(3));
        assert_eq!(byte_character(line, 2, Clamp::None), Some(3));
    }

    #[test]
    fn right_to_left_text_is_measured_in_text_order() {
        // א, a tab to column 4, then ב: bytes 0..2, 2..3 and 3..5.
        let line = "א\tב";
        round_trip(line, &[(0, 0), (2, 1), (3, 4), (5, 5)]);
        assert_eq!(byte_column(line, 2, Clamp::Before, LAYOUT), Some(2));
        assert_eq!(byte_column(line, 2, Clamp::After, LAYOUT), Some(3));
    }
}
