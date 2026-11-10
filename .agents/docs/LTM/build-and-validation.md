# Build and validation

Toolchains: Go 1.26.10, Rust 1.99.1, protoc 35.1; the Makefile pins protoc-gen-go. Core/runtime/native are independent Cargo projects. Only the editor component/host share a workspace. Use explicit manifests and locked builds.

Existing build/cache directories are deliberate: bin/, target/, .cache/. The editor shares target/zed-editor with debug info disabled. Do not mint a target directory per session. Scratch output belongs in .agents-workspace/tmp/.

Go does not track external Rust archive contents in its cache; native and headless targets force a new link with `-a`. A cached executable can run old native code even after Cargo succeeds. Runtime shaders avoid the optional Metal compiler. Settings assets need embedding outside Zed's checkout.

Standalone generic startup now lives in `host`; the root `gpui.Run` forwards to it. `client`, `scene`, `ui` and the shared test core must not transitively import `host`, Sable or cgo. `architecture_test.go` selects native/automation tags while checking these dependencies. The explicit Go-only executable can import the host but must exclude Starlark. Avoid overlapping recursive `go ... ./...` package scans with Cargo writes under `target/`: transient Cargo metadata files can disappear during traversal. Sequence those phases if that setup error occurs.

Pure Go tests, Rust core/runtime tests, actual Sable integration, native command smoke, and native user input are different evidence. Do not equate a native RPC smoke with automated typing/IME. GUI service denials in a sandbox are environmental failures.

`make test-native` exercises GPUI's test-platform dispatch for keys/chords/predicates, keyboard activation, pointer hitboxes, Unicode input handling, menu actions and asynchronous picker completion/cancellation. The feature desktop smoke exercises real resource decoding, models, clipboard text round trips/restoration, menus, focus/list/window commands; it optionally uploads a supplied font. Neither establishes screen-reader, real OS IME, physical drag/drop or pixel behavior. CI builds generic examples and runs the GPUI unit tests; graphical smokes remain explicit.

The workspace may lack .git. For generated-binding verification, generate into isolated output and compare bytes rather than initializing Git to satisfy make check-generated.

The Starlark harness supports `make e2e-check` without native linkage and `make e2e` for eleven native scenarios. Its shared `uitest` core has no Starlark dependency; `make uitest` exercises the same Page/Locator/expect API in Go. Fixtures remain in `examples/e2eapp/`, `examples/reactiveapp/` and `examples/counter-gox/app/`. AppKit can terminate the worker with exit zero without returning through Go, so the public worker supervisor requires an atomic version-1 complete/successful report plus process success. Missing reports, assertions and crashes fail. Keep the failing assertion/strictness controls outside the passing glob; require status 1 and a subsequent passing scenario. Reports remain under `.agents-workspace/tmp/e2e-run-*/`.

Sources: [Makefile](../../../Makefile), [testing](../TESTING.md), [toolchain](../../../rust-toolchain.toml), [quality gate](../QUALITY_GATE.md).
