//go:build (gpui || gpui_prebuilt) && !gpui_headless

package host

import "github.com/moriyoshi/gpui-go/internal/nativeshim"

func runPlatform() int { return int(nativeshim.GPUIRunNative()) }
