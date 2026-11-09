// Package nativeshim is the single Go boundary to the native GPUI/Sable
// library. Build tags select how that library reaches the executable:
//
//   - gpui links a locally built Rust archive through cgo and Sable.
//   - gpui,gpui_prebuilt calls a published per-platform module and builds
//     with CGO_ENABLED=0. The module is registered by importing a package
//     under prebuilt/, so this package imports no native module itself.
//
// Without gpui the package is empty, so default builds need no native code.
// prebuilt_backend.go and the packages under prebuilt/ are generated; see gen.go.
package nativeshim

//go:generate go run gen.go
