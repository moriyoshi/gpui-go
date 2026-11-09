# Prebuilt modules

::: warning Not published yet
No prebuilt version exists, so the commands on this page cannot resolve today. Use the [source build](./getting-started#build-it) for now.
:::

Prebuilt native code will be distributed as ordinary Go modules, one per platform, under `gpui-go.dev/__binary/`. You will not need Rust or a C toolchain, and Go's normal checksum verification applies.

Import the package for your platform for its side effect. It registers that platform's module:

```go
import _ "github.com/moriyoshi/gpui-go/prebuilt/darwin/arm64"
```

Then build without cgo:

```sh
go mod tidy
CGO_ENABLED=0 go build -tags gpui,gpui_prebuilt -o hello .
```

Without the import the program still compiles, and `host.Run` returns an error naming the package to import.

## Choosing platforms

Each platform package brings in one module of 15 to 22 MB. `go mod tidy` downloads every module your imports reach, whatever the build tags, so import only the platforms you build. Each package is empty on other platforms, so one file can list several:

```go
//go:build gpui_prebuilt

package main

import (
	_ "github.com/moriyoshi/gpui-go/prebuilt/darwin/arm64"
	_ "github.com/moriyoshi/gpui-go/prebuilt/linux/amd64"
)
```

The build constraint keeps the imports out of source builds. `github.com/moriyoshi/gpui-go/prebuilt` imports all six platforms, at the cost of downloading all of them.

## Platform status

| Target | State |
| --- | --- |
| `darwin/arm64` | Native window, headless and end-to-end tests pass |
| `darwin/amd64` | Native window, headless and end-to-end tests pass, on Intel and under Rosetta |
| `linux/arm64` | Headless and end-to-end tests pass without a display. Needs `libxcb`, `libxkbcommon` and `libfreetype` at run time |
| `linux/amd64` | X11 window, headless and end-to-end tests pass. A window also needs `libvulkan1` and a Vulkan driver such as `mesa-vulkan-drivers` |
| `windows/amd64`, `windows/arm64` | Headless smoke test passes |

The modules are built with test automation, so [end-to-end tests](/testing/) only need the `gpui_e2e` tag. The optional GPL editor is never included in these modules.
