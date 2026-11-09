// Managed by GoX v0.3.3

//line view.gox:1
package countergox

import (
	"fmt"

	"github.com/doors-dev/gox"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

//line view.gox:13
func view(count *reactive.Signal[int], increment ui.Handler) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:15
			__e = __c.Set("testid", "root")
			if __e != nil {
				return
			}
//line view.gox:16
			__e = __c.Set("style", &scene.Style{
				Width:          scene.Percent(100),
				Height:         scene.Percent(100),
				Padding:        scene.Padding(32),
				Gap:            scene.Pixels(20),
				FlexDirection:  scene.FlexColumn.Enum(),
				AlignItems:     scene.AlignCenter.Enum(),
				JustifyContent: scene.JustifyCenter.Enum(),
				Background:     scene.RGB(0x18181b),
				TextColor:      scene.RGB(0xf4f4f5),
				FontSize:       scene.Ptr[float32](20),
			})
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
			__e = __c.Init("span")
			if __e != nil {
				return
			}
			{
//line view.gox:28
				__e = __c.Set("style", &scene.Style{FontSize: scene.Ptr[float32](30)})
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
				__e = __c.Text("GPUI from Go")
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
			__e = __c.Init("span")
			if __e != nil {
				return
			}
			{
//line view.gox:29
				__e = __c.Set("testid", "count")
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:30
				__e = __c.Any(nativegox.Text(func() string {
					return fmt.Sprintf("Count: %d", count.Get())
				}))
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
			__e = __c.Init("button")
			if __e != nil {
				return
			}
			{
//line view.gox:35
				__e = __c.Set("testid", "increment")
				if __e != nil {
					return
				}
//line view.gox:36
				__e = __c.Set("label", "Increment")
				if __e != nil {
					return
				}
//line view.gox:37
				__e = __c.Set("onClick", increment)
				if __e != nil {
					return
				}
//line view.gox:38
				__e = __c.Set("style", &scene.Style{
					Padding:      scene.Padding(12),
					Background:   scene.RGB(0x4f46e5),
					CornerRadius: scene.Ptr[float32](8),
				})
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
				__e = __c.Text("Increment")
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
		}
		__e = __c.Close()
		if __e != nil {
			return
		}
		return
	})
//line view.gox:46
}
