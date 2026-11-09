# Introduction

gpui-go lets a Go program open windows and draw interfaces with [GPUI](https://gpui.rs), the Rust UI framework behind the Zed editor. Your application logic stays in ordinary Go, with goroutines and `context.Context`.

## How it works

You describe a window's contents as a tree of elements. Go sends that tree to Rust as a versioned protobuf message, and GPUI lays out and paints it. Clicks and keystrokes come back as events that your Go handlers process.

<figure class="concept-diagram">
<svg viewBox="0 0 680 236" role="img" aria-label="An element tree on the left: Div 1 contains Text 2 reading Count: 1 and Div 3, which contains Text 4 reading Increment. An arrow labeled Window.Set leads to the native window it produces, showing the count above a purple Increment button.">
<defs><marker id="tree-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path class="head" d="M0 0L10 5L0 10z"/></marker></defs>
<text class="heading" x="120" y="22" text-anchor="middle">Element tree, built in Go</text>
<text class="heading" x="516" y="22" text-anchor="middle">Native window, drawn by GPUI</text>
<rect class="box " x="16" y="40" width="120" height="34" rx="6"/><text class="label" x="76.0" y="57.0">Div 1</text>
<rect class="box " x="52" y="88" width="180" height="34" rx="6"/><text class="label" x="142.0" y="105.0">Text 2 &quot;Count: 1&quot;</text>
<rect class="box " x="52" y="136" width="120" height="34" rx="6"/><text class="label" x="112.0" y="153.0">Div 3</text>
<rect class="box " x="88" y="184" width="180" height="34" rx="6"/><text class="label" x="178.0" y="201.0">Text 4 &quot;Increment&quot;</text>
<line class="link" x1="30" y1="74" x2="30" y2="153"/>
<line class="link" x1="30" y1="105" x2="52" y2="105"/>
<line class="link" x1="30" y1="153" x2="52" y2="153"/>
<line class="link" x1="66" y1="170" x2="66" y2="201"/>
<line class="link" x1="66" y1="201" x2="88" y2="201"/>
<line class="arrow accent" x1="286" y1="128" x2="358" y2="128" marker-end="url(#tree-arrow)"/>
<text class="code" x="322" y="118" text-anchor="middle">Window.Set</text>
<image href="../reference/screenshots/intro-counter.png" x="368" y="44" width="296" height="165"/>
</svg>
</figure>

*The tree on the left produces the window on the right. Each element has a stable ID that ties it to its native state.*

<figure class="concept-diagram">
<svg viewBox="0 0 680 144" role="img" aria-label="Five stages in a row: your Go app, the Go client, the Sable transport, the Rust runtime and GPUI on the main thread. View snapshots and commands travel to the right as protobuf, and events travel back to the left.">
<defs><marker id="path-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path class="head" d="M0 0L10 5L0 10z"/></marker></defs>
<text class="note" x="340" y="18" text-anchor="middle">View snapshots and commands, as protobuf</text>
<line class="arrow accent" x1="16" y1="30" x2="664" y2="30" marker-end="url(#path-arrow)"/>
<rect class="box " x="16" y="44" width="108" height="56" rx="6"/><text class="label" x="70.0" y="63">Your Go app</text><text class="note" x="70.0" y="84" text-anchor="middle">your logic</text>
<rect class="box " x="151" y="44" width="108" height="56" rx="6"/><text class="label" x="205.0" y="63">Go client</text><text class="note" x="205.0" y="84" text-anchor="middle">scene, client</text>
<rect class="box accent" x="286" y="44" width="108" height="56" rx="6"/><text class="label" x="340.0" y="63">Sable</text><text class="note" x="340.0" y="84" text-anchor="middle">transport</text>
<rect class="box " x="421" y="44" width="108" height="56" rx="6"/><text class="label" x="475.0" y="63">Rust runtime</text><text class="note" x="475.0" y="84" text-anchor="middle">validates</text>
<rect class="box " x="556" y="44" width="108" height="56" rx="6"/><text class="label" x="610.0" y="63">GPUI</text><text class="note" x="610.0" y="84" text-anchor="middle">main thread</text>
<line class="link" x1="124" y1="72" x2="151" y2="72"/>
<line class="link" x1="259" y1="72" x2="286" y2="72"/>
<line class="link" x1="394" y1="72" x2="421" y2="72"/>
<line class="link" x1="529" y1="72" x2="556" y2="72"/>
<line class="arrow " x1="664" y1="114" x2="16" y2="114" marker-end="url(#path-arrow)"/>
<text class="note" x="340" y="134" text-anchor="middle">Clicks, key presses and other events</text>
</svg>
</figure>

*Every update takes this path. Your code stays in ordinary Go, and GPUI runs on the process main thread.*

[Sable](https://github.com/moriyoshi/sable) is the library that connects the Go and Rust runtimes. It lets a goroutine await an asynchronous Rust operation much as it would wait on a channel: the goroutine parks until Rust delivers the result, no OS thread stays blocked on the call, and canceling the `context.Context` reaches the pending operation. gpui-go uses it as a request and response transport for protobuf bytes, so commands and event pulls are ordinary awaited calls, and no Go pointers or callbacks cross into rendering.

Sable can also merge the Go and Rust schedulers around a single event loop, but gpui-go does not use that mode. It selects Sable's portable backend (the `sable_portable` build tag), which relies only on supported Go APIs, and GPUI keeps its own platform event loop and executors. You don't call Sable directly: `host.Run` starts and stops it for you.

GPUI renders the latest snapshot entirely in Rust and never waits for Go during a frame, so a busy goroutine does not stall painting. Interactive state that has to feel native, such as focus, text selection, IME composition and scroll position, is owned by Rust and survives each snapshot you send.

<figure class="concept-diagram">
<svg viewBox="0 0 680 236" role="img" aria-label="Two panels. Go owns application state, the element tree it sends, event handlers, and goroutines and cancellation. Rust owns layout and painting, focus and hover, text selection and IME composition, and scroll position.">
<rect class="panel" x="16" y="8" width="314" height="220" rx="10"/><text class="heading" x="173.0" y="30">Go</text><text class="note" x="173.0" y="48" text-anchor="middle">decides what to show</text>
<rect class="panel" x="350" y="8" width="314" height="220" rx="10"/><text class="heading" x="507.0" y="30">Rust</text><text class="note" x="507.0" y="48" text-anchor="middle">makes it feel native</text>
<rect class="box " x="34" y="62" width="278" height="32" rx="6"/><text class="label" x="173.0" y="78.0">Application state</text>
<rect class="box " x="34" y="102" width="278" height="32" rx="6"/><text class="label" x="173.0" y="118.0">The element tree you send</text>
<rect class="box " x="34" y="142" width="278" height="32" rx="6"/><text class="label" x="173.0" y="158.0">Event handlers</text>
<rect class="box " x="34" y="182" width="278" height="32" rx="6"/><text class="label" x="173.0" y="198.0">Goroutines and cancellation</text>
<rect class="box " x="368" y="62" width="278" height="32" rx="6"/><text class="label" x="507.0" y="78.0">Layout and painting</text>
<rect class="box " x="368" y="102" width="278" height="32" rx="6"/><text class="label" x="507.0" y="118.0">Focus and hover</text>
<rect class="box " x="368" y="142" width="278" height="32" rx="6"/><text class="label" x="507.0" y="158.0">Text selection and IME composition</text>
<rect class="box " x="368" y="182" width="278" height="32" rx="6"/><text class="label" x="507.0" y="198.0">Scroll position</text>
</svg>
</figure>

*Go decides what the view contains. Rust owns everything that has to respond within a frame, and that state survives each new snapshot.*

See [Concepts](./concepts) for how snapshots, element IDs and events work in practice.

## Packages

Everything lives in the module `github.com/moriyoshi/gpui-go`.

| Package | Use it for |
| --- | --- |
| `host` | Starting the native runtime with `host.Run` |
| `client` | Windows, events, resources, models and platform commands |
| `scene` | Building element trees and styles |
| `reactive`, `ui` | Signals, memos, effects and persistent component nodes |
| `ui/gox` | Writing views as [GoX](https://github.com/doors-dev/gox) templates |
| `uitest`, `uitest/gpui` | Page, Locator and expect-style [end-to-end tests](/testing/) |
| `editor` | The optional, GPL-licensed Zed editor component |

The root package `gpui` keeps compatibility aliases for older code. New code should import `client` and `scene` directly.

## Status

Headless protocol tests exercise the real Sable transport. Native builds and desktop smoke tests are verified on macOS/arm64. On Linux, the [end-to-end suites pass without a display](/testing/headless#platform-status). Linux with a real display and Windows have partial build work for [prebuilt modules](./prebuilt-modules), but their native runtime behavior is not validated.

Not yet validated on any platform: real OS IME composition, drag-and-drop gestures, screen-reader output and pixel-level rendering.

Outside the current API: arbitrary Rust `Render` implementations, custom shaders, variable-height or paged lists, styled text runs and incremental snapshot patches. Generic text inputs are single-line, with no undo/redo.

## License

gpui-go is licensed under the [Apache License, Version 2.0](https://github.com/moriyoshi/gpui-go/blob/main/LICENSE), including the Go packages, protobuf schemas, examples and the generic Rust backend. The optional Rust Zed editor component and host are [GPL-3.0-or-later](https://github.com/moriyoshi/gpui-go/blob/main/rust/zed-editor/LICENSE-GPL). Third-party code and assets keep their own licenses.
