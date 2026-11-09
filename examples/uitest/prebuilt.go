//go:build gpui && gpui_prebuilt

package main

// Register the published native module for the target platform.
import _ "github.com/moriyoshi/gpui-go/prebuilt"
