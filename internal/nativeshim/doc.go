// Package nativeshim is the single Go boundary to the native GPUI/Sable
// library. Build tags select how that library reaches the executable:
//
//   - gpui links a locally built Rust archive through cgo and Sable.
//   - gpui,gpui_prebuilt calls the published per-platform module for the
//     current GOOS/GOARCH and builds with CGO_ENABLED=0.
//
// Without gpui the package is empty, so default builds need no native code.
// The prebuilt_<goos>_<goarch>.go files are generated; see gen.go.
package nativeshim

//go:generate go run gen.go
