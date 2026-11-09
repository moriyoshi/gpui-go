# Local patch to accesskit_macos 0.26.3

This is the crates.io release of `accesskit_macos` 0.26.3 (AccessKit commit `c88605b96d04431f9c3c792464a0f2f253480e94`, `platforms/macos`), MIT OR Apache-2.0, with one backported change: `src/subclass.rs` is replaced by its version from AccessKit PR #835, "fix: Fully drop macOS SubclassingAdapter if KVO is detected" (merge commit `451885c185`, git blob `f4ebc196a`). That PR's base blob `a873f7c85` is identical to 0.26.3's file, so the patch applies exactly. Its integration tests are not vendored.

The fix shipped in `accesskit_macos` 0.27.2, but the pinned Zed revision's GPUI depends on the 0.26 series. Upstream issue #823 describes the crash: `SubclassingAdapter::drop` restored the view's original class unconditionally, discarding the `NSKVONotifying_` subclass that AppKit's Touch Bar responder-chain observation had installed, so AppKit's later `removeObserver:` threw. GPUI-go's prebuilt macOS modules hit this because Go's internal linker records SDK 12.0, which enables that observation (see the 2026-11-10 entries in `.agents/docs/JOURNAL.md`).

`rust/native/Cargo.toml` applies this through `[patch.crates-io]`. Remove the patch once the pinned GPUI moves to `accesskit_macos` 0.27.2 or later.
