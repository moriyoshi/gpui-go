// Managed by GoX v0.3.3

//line view.gox:1
package reactiveapp

import (
	"context"
	"fmt"

	"github.com/doors-dev/gox"
	clientlib "github.com/moriyoshi/gpui-go/client"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

//line view.gox:15
func (a *App) button(id, label, operation string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("button")
		if __e != nil {
			return
		}
		{
//line view.gox:17
			__e = __c.Set("testid", id)
			if __e != nil {
				return
			}
//line view.gox:18
			__e = __c.Set("label", label)
			if __e != nil {
				return
			}
//line view.gox:19
			__e = __c.Set("style", &scene.Style{Padding: scene.Padding(8), Background: scene.RGB(0x334155), Height: scene.Pixels(36)})
			if __e != nil {
				return
			}
//line view.gox:20
			__e = __c.Set("onClick", ui.Handler(func(ctx context.Context, _ *clientlib.Event) error {
				return a.Update(ctx, operation, "")
			}))
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:23
			__e = __c.Any(label)
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
//line view.gox:25
}

//line view.gox:27
func (a *App) template(rows, details *ui.Node) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
//line view.gox:29
		a.builds++

		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:32
			__e = __c.Set("testid", "root")
			if __e != nil {
				return
			}
//line view.gox:33
			__e = __c.Set("style", &scene.Style{
				Width:         scene.Percent(100),
				Height:        scene.Percent(100),
				FlexDirection: scene.FlexColumn.Enum(),
				Padding:       scene.Padding(24),
				Gap:           scene.Pixels(10),
				Background:    scene.RGB(0x0f172a),
				TextColor:     scene.RGB(0xe2e8f0),
				FontSize:      scene.Ptr[float32](15),
			})
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
			__e = __c.Init("div")
			if __e != nil {
				return
			}
			{
				__e = __c.Submit()
				if __e != nil {
					return
				}
				__e = __c.Text("Fine-grained reactive GPUI graph")
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
//line view.gox:44
				__e = __c.Set("testid", "counter")
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:45
				__e = __c.Any(nativegox.Text(func() string {
					a.countRuns++
					return fmt.Sprintf("Count: %d", a.count.Get())
				}))
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
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:51
				__e = __c.Any(nativegox.Text(func() string {
					_ = a.count.Get()
					return fmt.Sprintf("Counter binding executions: %d; template builds: %d", a.countRuns, a.builds)
				}))
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
			__e = __c.Init("div")
			if __e != nil {
				return
			}
			{
//line view.gox:56
				__e = __c.Set("style", &scene.Style{Gap: scene.Pixels(10)})
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:57
				__e = __c.Any(a.button("increment", "Increment", "increment"))
				if __e != nil {
					return
				}
//line view.gox:58
				__e = __c.Any(a.button("batch", "Batch +2", "batch"))
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
			__e = __c.InitVoid("input")
			if __e != nil {
				return
			}
			{
//line view.gox:61
				__e = __c.Set("testid", "name")
				if __e != nil {
					return
				}
//line view.gox:62
				__e = __c.Set("label", "Name")
				if __e != nil {
					return
				}
//line view.gox:63
				__e = __c.Set("value", a.name)
				if __e != nil {
					return
				}
//line view.gox:64
				__e = __c.Set("placeholder", "Your name")
				if __e != nil {
					return
				}
//line view.gox:65
				__e = __c.Set("style", &scene.Style{Width: scene.Pixels(400), Height: scene.Pixels(42), Padding: scene.Padding(8), Background: scene.RGB(0x1e293b)})
				if __e != nil {
					return
				}
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
//line view.gox:66
				__e = __c.Set("testid", "greeting")
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:67
				__e = __c.Any(nativegox.Text(func() string {
					return "Hello, " + a.name.Get()
				}))
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
//line view.gox:71
			__e = __c.Any(a.button("toggle", "Toggle details", "toggle"))
			if __e != nil {
				return
			}
//line view.gox:72
			__e = __c.Any(details)
			if __e != nil {
				return
			}
			__e = __c.Init("div")
			if __e != nil {
				return
			}
			{
//line view.gox:73
				__e = __c.Set("style", &scene.Style{Gap: scene.Pixels(10)})
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:74
				__e = __c.Any(a.button("reverse", "Reverse rows", "reverse"))
				if __e != nil {
					return
				}
//line view.gox:75
				__e = __c.Any(a.button("add", "Add row", "add"))
				if __e != nil {
					return
				}
//line view.gox:76
				__e = __c.Any(a.button("remove", "Remove first row", "remove"))
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
//line view.gox:78
			__e = __c.Any(rows)
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
//line view.gox:80
}

//line view.gox:82
func rowTemplate(data *reactive.Signal[*item], index *reactive.Signal[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
//line view.gox:84
		value := data.Peek()

		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:86
			__e = __c.Set("style", &scene.Style{FlexDirection: scene.FlexRow.Enum(), Gap: scene.Pixels(14), Height: scene.Pixels(40)})
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
			__e = __c.InitVoid("input")
			if __e != nil {
				return
			}
			{
//line view.gox:88
				__e = __c.Set("testid", fmt.Sprintf("item-%d", value.ID))
				if __e != nil {
					return
				}
//line view.gox:89
				__e = __c.Set("label", fmt.Sprintf("Item %d", value.ID))
				if __e != nil {
					return
				}
//line view.gox:90
				__e = __c.Set("value", value.Title)
				if __e != nil {
					return
				}
//line view.gox:91
				__e = __c.Set("placeholder", "Item title")
				if __e != nil {
					return
				}
//line view.gox:92
				__e = __c.Set("style", &scene.Style{Width: scene.Pixels(250), Height: scene.Pixels(38), Padding: scene.Padding(6), Background: scene.RGB(0x1e293b)})
				if __e != nil {
					return
				}
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
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:94
				__e = __c.Any(nativegox.Text(func() string {
					return fmt.Sprintf("Row %d: %s", index.Get()+1, data.Get().Title.Get())
				}))
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
//line view.gox:99
}

//line view.gox:101
func (a *App) detailsTemplate(parity *reactive.Memo[string]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("span")
		if __e != nil {
			return
		}
		{
//line view.gox:102
			__e = __c.Set("testid", "details")
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:103
			__e = __c.Any(nativegox.Text(func() string {
				a.detailsRuns++
				return "Details: " + parity.Get()
			}))
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
//line view.gox:108
}
