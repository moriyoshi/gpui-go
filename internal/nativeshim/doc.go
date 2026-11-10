// Package nativeshim is the single Go boundary to the native GPUI/Sable
// library. Build tags select how that library reaches the executable:
//
//   - gpui links a locally built Rust archive through cgo and Sable.
//   - gpui_prebuilt, alone or with gpui, calls a published per-platform
//     module and builds with CGO_ENABLED=0. Packages under prebuilt/ register
//     the module; host imports them, since importing them here would be a
//     cycle.
//
// Without either tag the package is empty, so default builds need no native code.
// prebuilt_backend.go and the packages under prebuilt/ are generated; see gen.go.
package nativeshim

//go:generate go run gen.go
