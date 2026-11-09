# Prebuilt modules

::: warning Not published yet
No prebuilt version exists, so the commands on this page cannot resolve today. Use the [source build](./getting-started#build-it) for now.
:::

Prebuilt native code will be distributed as ordinary Go modules, one per platform, under this domain: `gpui-go.dev/darwin-arm64`, `gpui-go.dev/linux-amd64` and so on. You will not need Rust or a C toolchain, and Go's normal checksum verification applies.

```sh
CGO_ENABLED=0 go build -tags gpui,gpui_prebuilt -o hello .
```

## Platform status

| Target | State |
| --- | --- |
| `darwin/arm64` | Headless and native window smoke tests pass |
| `darwin/amd64` | Builds, but aborts at launch |
| `linux/arm64` | Headless smoke test passes. Needs `libxcb`, `libxkbcommon` and `libfreetype` at run time |
| `linux/amd64` | Library builds, packaging not yet run |
| `windows/amd64`, `windows/arm64` | Library builds, packaging blocked |

The modules are built with test automation, so [end-to-end tests](/testing/) only need the `gpui_e2e` tag. The optional GPL editor is never included in these modules.
