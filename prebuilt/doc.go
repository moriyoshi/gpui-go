// Package prebuilt registers the published native module for whichever
// platform the program is built for. host imports it when built with
// -tags gpui_prebuilt, so programs do not import it themselves.
//
// It imports every platform's registration package, so `go mod tidy` in any
// module that imports host downloads every platform's module.
package prebuilt
