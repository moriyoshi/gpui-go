---
outline: [2, 3]
---

# Headless testing

Two different things in gpui-go are called headless, and only one of them can run `uitest`.

| | GPUI with no display | `gpui_headless` protocol backend |
| --- | --- | --- |
| What runs | The real GPUI renderer, drawing nothing | Your Go code and the Sable transport, with no GPUI at all |
| Layout and text shaping | Yes | No |
| Focus, hit testing, keymaps | Yes | No |
| `uitest` locators and actions | Yes | No |
| Platforms | Linux only | Any platform with a prebuilt module or that builds `rust/runtime` |
| Go build tags | `gpui,gpui_prebuilt,gpui_e2e` | `gpui,gpui_prebuilt,gpui_headless` |
| Good for | End-to-end tests on CI servers | Checking that a program starts, talks to the backend and shuts down |

Neither one produces pixels, so neither can catch a visual regression.

::: warning Not published yet
No [prebuilt](/guide/prebuilt-modules) version exists, so the `gpui_prebuilt` commands on this page cannot resolve today. Use [a source build](#build-from-source) for now.
:::

## Running uitest without a display

On Linux, GPUI starts without a display server when the environment names none. There is no flag or API for this, and no virtual display such as Xvfb is needed. The same [test program](./setup) runs unchanged.

GPUI checks `WAYLAND_DISPLAY` and then `DISPLAY`. On a server or in a container they are normally unset already. On a Linux desktop, clear them to stop windows from opening:

```sh
env -u DISPLAY -u WAYLAND_DISPLAY ./uitest-app
```

### What you need

Install Go, the system libraries the prebuilt module loads at run time, and at least one font, because layout does real text shaping. On Ubuntu 24.04:

```sh
sudo apt-get install -y fontconfig fonts-dejavu-core libfreetype6 libxcb1 \
  libxkbcommon0 libxkbcommon-x11-0
```

### In CI

```yaml
jobs:
  ui-tests:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      # Install Go, then the packages listed above.
      - run: CGO_ENABLED=0 go build -tags gpui,gpui_prebuilt,gpui_e2e -o uitest-app .
      - run: ./uitest-app
```

Hosted runners have no display, so nothing needs to be unset.

### What differs from a real display

GPUI lays out each window and handles input as usual, then discards the frame. `uitest` drives drawing and input dispatch directly, so it does not depend on the platform delivering frames or events.

| Behavior | Without a display |
| --- | --- |
| Layout, rendered bounds and scrolling | Real |
| Focus, Tab order and key bindings | Real |
| Pointer hit testing and click handlers | Real |
| Text input, selection and simulated composition | Real |
| Drawing | Discarded, so nothing can be captured |
| Clipboard | Absent. Writes are dropped and reads return nothing |
| Window active state | Windows are never active, so code that depends on it behaves differently |

The clipboard is the one gap that fails tests in practice. Any scenario that copies and pastes needs a real display, which is why `make e2e-no-display` skips `clipboard.star`. Do not assume other operating system services work without a display either.

Keyboard shortcuts resolve as they do on Linux, so `ControlOrMeta` means `ctrl`. An application developed on macOS that binds only `cmd-` shortcuts will fail here until it also binds the `ctrl-` form.

### Platform status

| Platform | Mode | State |
| --- | --- | --- |
| Linux arm64 | No display | `make uitest` and `make e2e-no-display` pass |
| Linux amd64 | No display | Configured in CI, not yet confirmed passing |
| Linux | Real display | Not validated |
| macOS arm64 | Real windows | `make uitest` and `make e2e` pass. Needs a logged-in graphical session |
| macOS | No display | Not available. GPUI's macOS windows have no headless mode |
| Windows | Any | Not validated, and GPUI refuses to open a window when headless |

To test on macOS, run in a graphical session. Windows appear on screen during the run, and the clipboard scenario temporarily replaces your clipboard text and then restores it. Rich clipboard formats are not preserved.

## The gpui_headless protocol backend

This backend replaces GPUI with a stub. Your program still runs through `host.Run`, the real Sable transport and the protobuf protocol, but opening, updating and closing a window do nothing beyond the protocol exchange. It needs no display.

```sh
CGO_ENABLED=0 go run -tags gpui,gpui_prebuilt,gpui_headless .
```

Use it to check that your program starts, opens its window, commits views, handles a `ViewUpdated` event and shuts down.

Keep these limits in mind:

- It does no layout, so element bounds, hit testing and focus do not exist.
- It advertises no features, so check `client.Supports` before relying on one.
- Automation and platform services such as the clipboard return an unsupported error.
- `Focus` and `ScrollTo` succeed and do nothing.
- Your code has no way to inject a click or a text change. Those hooks are private to gpui-go's own integration tests.

`uitest` cannot use this backend. `OpenPage` fails with `native archive lacks test-automation-v1`.

## Tests with no native code

Ordinary `go test` runs need neither of the above. Without the `gpui` tag, `host.Run` returns `ErrNativeUnavailable`, so keep application logic in functions you can call directly.

To test your own helpers that drive pages, `uitest.Target` is an interface. A small fake satisfies it, and `Page`, `Locator` and `Expect` run against the fake. That exercises your test code and `uitest`'s waiting and strictness rules, but says nothing about your interface, because no GPUI is involved.

To check Starlark scenarios for syntax and name errors without running them, call `Check` from [`uitest/starlark`](./starlark#reference), or use `make e2e-check` in a checkout.

## Build from source

A source build links a Rust archive built from a gpui-go checkout. You need Go, the pinned Rust toolchain and `protoc`, as in [Getting started](/guide/getting-started).

| | GPUI with no display | `gpui_headless` protocol backend |
| --- | --- | --- |
| Rust archive | `rust/native` with `automation` | `rust/runtime` |
| Go build tags | `gpui,gpui_e2e,sable_extern_lib,sable_portable` | `gpui,gpui_headless,sable_extern_lib,sable_portable` |

### uitest without a display

Install the native development libraries and at least one font. On Ubuntu 24.04:

```sh
sudo apt-get install -y pkg-config libclang-dev cmake fontconfig fonts-dejavu-core \
  libfontconfig1-dev libfreetype-dev libwayland-dev libx11-xcb-dev libxcb1-dev \
  libxcb-xkb-dev libxkbcommon-dev libxkbcommon-x11-dev libssl-dev libzstd-dev
```

From a checkout:

```sh
make uitest          # the Go example
make e2e-no-display  # the Starlark suite, minus the clipboard scenario
```

To build your own program in CI:

```yaml
jobs:
  ui-tests:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@v4
      # Install Go, Rust and protoc, then the packages listed above.
      - run: cargo build --locked --release --manifest-path "$GPUI_GO/rust/native/Cargo.toml" --features automation
      - run: cp "$GPUI_GO/target/release/libgpui_go_native.a" "$GPUI_GO/target/release/libgpui_go_e2e.a"
      - run: |
          CGO_LDFLAGS="-L$GPUI_GO/target/release -lgpui_go_e2e" \
            go build -a -mod=mod -tags gpui,gpui_e2e,sable_extern_lib,sable_portable -o uitest-app .
      - run: ./uitest-app
```

`GPUI_GO` is the path of your gpui-go checkout.

### gpui_headless

This backend needs no GPUI build, so it is much faster to compile.

```sh
cargo build --locked --release --manifest-path rust/runtime/Cargo.toml
CGO_LDFLAGS="-L/path/to/gpui-go/target/release -lgpui_go" \
  go run -a -mod=mod -tags gpui,gpui_headless,sable_extern_lib,sable_portable .
```

From a checkout, `make headless-smoke` and `make showcase-headless-smoke` run the examples this way, and `make integration` runs the Go tests against it.
