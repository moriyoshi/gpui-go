//! Display-independent rules shared by the native text editor and its tests.
use std::ops::Range;

pub const MAX_INPUT_BYTES: usize = 64 * 1024;

pub fn single_line(text: &str) -> String {
    text.replace(['\n', '\r', '\u{2028}', '\u{2029}'], " ")
}

pub fn utf16_to_utf8(text: &str, offset: usize) -> usize {
    let (mut bytes, mut units) = (0, 0);
    for ch in text.chars() {
        if units >= offset {
            break;
        }
        bytes += ch.len_utf8();
        units += ch.len_utf16();
    }
    bytes
}

pub fn utf8_to_utf16(text: &str, offset: usize) -> usize {
    let (mut bytes, mut units) = (0, 0);
    for ch in text.chars() {
        if bytes >= offset {
            break;
        }
        bytes += ch.len_utf8();
        units += ch.len_utf16();
    }
    units
}

pub fn range_from_utf16(text: &str, range: &Range<usize>) -> Range<usize> {
    let start = utf16_to_utf8(text, range.start);
    start..utf16_to_utf8(text, range.end).max(start)
}

pub fn accepts_snapshot(
    epoch: u64,
    revision: u64,
    incoming_epoch: u64,
    incoming_revision: u64,
) -> bool {
    incoming_epoch > epoch || (incoming_epoch == epoch && incoming_revision >= revision)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn native_ranges_handle_surrogate_pairs_and_out_of_bounds_selection() {
        let text = "e🙂漢";
        assert_eq!(range_from_utf16(text, &(1..3)), 1..5);
        assert_eq!(range_from_utf16(text, &(3..99)), 5..8);
        // This intentionally reversed range is input data, not an iterator.
        assert_eq!(range_from_utf16(text, &Range { start: 99, end: 0 }), 8..8);
        assert_eq!(utf8_to_utf16(text, 8), 4);
        assert_eq!(utf16_to_utf8(text, 2), 5);
    }
    #[test]
    fn stale_echoes_do_not_replace_native_edits_and_new_epochs_reset() {
        assert!(!accepts_snapshot(2, 4, 2, 3));
        assert!(!accepts_snapshot(2, 4, 1, 999));
        assert!(accepts_snapshot(2, 4, 2, 4));
        assert!(accepts_snapshot(2, 4, 3, 0));
        assert_eq!(single_line("hello\r\n日本語\u{2028}🙂"), "hello  日本語 🙂");
    }
}
