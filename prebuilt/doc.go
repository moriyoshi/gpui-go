// Package prebuilt registers the published native module for whichever
// platform the program is built for. Build with -tags gpui,gpui_prebuilt and
// CGO_ENABLED=0.
//
// It imports every platform's registration package, so `go mod tidy` downloads
// every platform's module. A program that targets fewer platforms should import
// only the matching packages, such as prebuilt/darwin/arm64.
package prebuilt
