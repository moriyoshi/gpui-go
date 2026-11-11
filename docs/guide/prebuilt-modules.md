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

[Window size, color scheme](/reference/window-state) and [conditional styles](/reference/window-state#conditional-styles), including `md:` and `dark:` utilities, need native modules from v0.0.3 on. From v0.0.5, each gpui-go release requires the native modules of its own version. With the v0.0.1 modules, which gpui-go v0.0.1 through v0.0.4 require, `Window.Size` and `Window.ColorScheme` report nothing, `document.WindowSize()` and `document.ColorScheme()` stay unset, and a snapshot with conditional styles fails with an unsupported error. On those gpui-go versions, require the native modules at v0.0.4 in your own `go.mod` to get both.

## Troubleshooting

If `go get` or `go mod tidy` reports `502 Bad Gateway` or `not found` for a `gpui-go.dev/__binary/...` module, or a build reports `missing go.sum entry` for one, the public Go mirror and checksum database have probably cached a failed fetch of a newly published version. They retry by themselves, usually within an hour. `GOPROXY=direct` does not help in the meantime, because `go` still verifies the module against `sum.golang.org`, which has cached the same failure.

To build before then, skip the checksum database for the native modules only:

```sh
GONOSUMDB=gpui-go.dev/__binary go get github.com/moriyoshi/gpui-go@latest
```

Go then fetches the modules from `gpui-go.dev` without checking them against the public checksum database. Hashes already in your own `go.sum` are still checked; new ones are trusted as downloaded. Drop the setting once the error is gone, so that checksum verification applies again.
