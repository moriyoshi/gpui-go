//go:build gpui_prebuilt

package host

// Register the published native module for the target platform, so the
// gpui_prebuilt tag alone selects it.
import _ "github.com/moriyoshi/gpui-go/prebuilt"
