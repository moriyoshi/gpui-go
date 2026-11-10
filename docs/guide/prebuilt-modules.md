# Prebuilt modules

Prebuilt native code is distributed as ordinary Go modules, one per platform, under `gpui-go.dev/__binary/`. You do not need Rust or a C toolchain, and Go's normal checksum verification applies. Your code stays the same as for a source build; the build tags select the module for the target platform:

```sh
go mod tidy
CGO_ENABLED=0 go build -tags gpui_prebuilt -o hello .
```

`gpui_prebuilt` implies `gpui`, so you do not need both. Cross-compiling works the same way, for example with `GOOS=linux GOARCH=amd64`.

## Downloads

`go mod tidy` and `go get` resolve imports for every platform and build tag at once, so they download all six platform modules, about 110 MB in total, even for a source build. Go keeps them in its module cache, so this happens once per version on a machine. An ordinary `go build` uses only the target platform's module.

## Platform status

| Target | State |
| --- | --- |
| `darwin/arm64` | Native window, headless and end-to-end tests pass |
| `darwin/amd64` | Native window, headless and end-to-end tests pass, on Intel and under Rosetta |
| `linux/arm64` | Headless and end-to-end tests pass without a display. Needs `libxcb`, `libxkbcommon` and `libfreetype` at run time |
| `linux/amd64` | X11 window, headless and end-to-end tests pass. A window also needs `libvulkan1` and a Vulkan driver such as `mesa-vulkan-drivers` |
| `windows/amd64` | Runs on Windows (checked by hand); headless smoke test passes |
| `windows/arm64` | Headless smoke test passes |

The modules are built with test automation, so [end-to-end tests](/testing/) only need the `gpui_e2e` tag. The optional GPL editor is never included in these modules.

The v0.0.1 modules predate protocol minor 4. With them, `Window.Size` and `Window.ColorScheme` report nothing, `document.WindowSize()` and `document.ColorScheme()` stay unset, and a snapshot with [conditional styles](/reference/window-state#conditional-styles), including `md:` and `dark:` utilities, fails with an unsupported error. A later release adds both.
