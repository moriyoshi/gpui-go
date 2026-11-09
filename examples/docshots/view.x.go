// Managed by GoX v0.3.3

//line view.gox:1
package main

import (
	"fmt"

	"github.com/doors-dev/gox"
	"github.com/moriyoshi/gpui-go/reactive"
	"github.com/moriyoshi/gpui-go/scene"
	"github.com/moriyoshi/gpui-go/ui"
	nativegox "github.com/moriyoshi/gpui-go/ui/gox"
)

var columnStyle = &scene.Style{FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(10)}
var rowStyle = &scene.Style{FlexDirection: scene.FlexRow.Enum(), Gap: scene.Pixels(10), AlignItems: scene.AlignCenter.Enum()}
var inputStyle = &scene.Style{Width: scene.Pixels(320), Height: scene.Pixels(40), Padding: scene.Padding(8), Background: scene.RGB(0x27272a), CornerRadius: scene.Ptr[float32](6)}
var cardStyle = &scene.Style{Width: scene.Pixels(260), Padding: scene.Padding(14), Gap: scene.Pixels(6), FlexDirection: scene.FlexColumn.Enum(), Background: scene.RGB(0x27272a), CornerRadius: scene.Ptr[float32](8)}
var titleStyle = &scene.Style{FontSize: scene.Ptr[float32](18), FontWeight: scene.Ptr[uint32](600)}

type Person struct {
	Name  string
	Admin bool
}

type Task struct {
	ID    int
	Title string
}

//line view.gox:29
func Badge(text string, color uint32) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("span")
		if __e != nil {
			return
		}
		{
//line view.gox:31
			__e = __c.Set("style", &scene.Style{
				Padding:      scene.Padding(6),
				Background:   scene.RGB(color),
				CornerRadius: scene.Ptr[float32](6),
			})
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:36
			__e = __c.Any(text)
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
//line view.gox:38
}

//line view.gox:40
func Tags() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:41
			__e = __c.Set("style", rowStyle)
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:42
			__e = __c.Any(Badge("Stable", 0x0d9488))
			if __e != nil {
				return
			}
//line view.gox:43
			__e = __c.Any(Badge("Beta", 0xb45309))
			if __e != nil {
				return
			}
//line view.gox:44
			__e = __c.Any(Badge("Deprecated", 0xbe123c))
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

//line view.gox:48
func Card(title string, body gox.Elem) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:49
			__e = __c.Set("style", cardStyle)
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
//line view.gox:50
				__e = __c.Set("style", titleStyle)
				if __e != nil {
					return
				}
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:50
				__e = __c.Any(title)
				if __e != nil {
					return
				}
			}
			__e = __c.Close()
			if __e != nil {
				return
			}
//line view.gox:51
			__e = __c.Any(body)
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
//line view.gox:53
}

//line view.gox:55
func Profile() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
//line view.gox:56
		__e = __c.Any(Card("Ada Lovelace", gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context()
			_ = ctx
			__e = __c.InitContainer()
			if __e != nil {
				return
			}
			{
				__e = __c.Init("span")
				if __e != nil {
					return
				}
				{
					__e = __c.Submit()
					if __e != nil {
						return
					}
					__e = __c.Text("Analyst")
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
					__e = __c.Text("London")
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
//line view.gox:59
			return
		})))
		if __e != nil {
			return
		}
		return
	})
//line view.gox:60
}

//line view.gox:62
func Roster(people []Person) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:63
			__e = __c.Set("style", columnStyle)
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:64
			for _, person := range people {
				__e = __c.Init("div")
				if __e != nil {
					return
				}
				{
//line view.gox:65
					__e = __c.Set("style", rowStyle)
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
						__e = __c.Submit()
						if __e != nil {
							return
						}
//line view.gox:66
						__e = __c.Any(person.Name)
						if __e != nil {
							return
						}
					}
					__e = __c.Close()
					if __e != nil {
						return
					}
//line view.gox:67
					if person.Admin {
//line view.gox:68
						__e = __c.Any(Badge("Admin", 0x4f46e5))
						if __e != nil {
							return
						}
					}
				}
				__e = __c.Close()
				if __e != nil {
					return
				}
			}
		}
		__e = __c.Close()
		if __e != nil {
			return
		}
		return
	})
//line view.gox:73
}

//line view.gox:75
func TextKinds(count *reactive.Signal[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:76
			__e = __c.Set("style", columnStyle)
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
				__e = __c.Submit()
				if __e != nil {
					return
				}
//line view.gox:77
				__e = __c.Any(fmt.Sprintf("Formatted once: %d", count.Get()))
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
//line view.gox:79
				__e = __c.Any(nativegox.Text(func() string {
					return fmt.Sprintf("Bound to the signal: %d", count.Get())
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
//line view.gox:84
}

//line view.gox:86
func MixedText() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:87
			__e = __c.Set("style", columnStyle)
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
				__e = __c.Submit()
				if __e != nil {
					return
				}
				__e = __c.Text("Hello, ")
				if __e != nil {
					return
				}
//line view.gox:88
				__e = __c.Any("Ada")
				if __e != nil {
					return
				}
				__e = __c.Text("!")
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
//line view.gox:89
				__e = __c.Any("Hello, Ada!")
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
//line view.gox:91
}

//line view.gox:93
func NameForm(name *reactive.Signal[string], locked *reactive.Signal[bool]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:94
			__e = __c.Set("style", columnStyle)
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
//line view.gox:96
				__e = __c.Set("testid", "name")
				if __e != nil {
					return
				}
//line view.gox:97
				__e = __c.Set("label", "Name")
				if __e != nil {
					return
				}
//line view.gox:98
				__e = __c.Set("value", name)
				if __e != nil {
					return
				}
//line view.gox:99
				__e = __c.Set("placeholder", "Your name")
				if __e != nil {
					return
				}
//line view.gox:100
				__e = __c.Set("disabled", locked.Get)
				if __e != nil {
					return
				}
//line view.gox:101
				__e = __c.Set("style", inputStyle)
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
//line view.gox:103
				__e = __c.Any(nativegox.Text(func() string {
					return "Hello, " + name.Get()
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
//line view.gox:108
}

//line view.gox:110
func Switch(on *reactive.Signal[bool], flip ui.Handler) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("button")
		if __e != nil {
			return
		}
		{
//line view.gox:112
			__e = __c.Set("label", "Notifications")
			if __e != nil {
				return
			}
//line view.gox:113
			__e = __c.Set("role", scene.RoleCheckbox)
			if __e != nil {
				return
			}
//line view.gox:114
			__e = __c.Set("checked", on.Get)
			if __e != nil {
				return
			}
//line view.gox:115
			__e = __c.Set("onClick", flip)
			if __e != nil {
				return
			}
//line view.gox:116
			__e = __c.Set("style", func() *scene.Style {
				background := scene.RGB(0x3f3f46)
				if on.Get() {
					background = scene.RGB(0x0d9488)
				}
				return &scene.Style{
					Padding:      scene.Padding(12),
					Background:   background,
					CornerRadius: scene.Ptr[float32](8),
				}
			})
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:127
			__e = __c.Any(nativegox.Text(func() string {
				if on.Get() {
					return "Notifications on"
				}
				return "Notifications off"
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
//line view.gox:134
}

//line view.gox:136
func Board(details, rows *ui.Node) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("div")
		if __e != nil {
			return
		}
		{
//line view.gox:137
			__e = __c.Set("style", columnStyle)
			if __e != nil {
				return
			}
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:138
			__e = __c.Any(details)
			if __e != nil {
				return
			}
//line view.gox:139
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
//line view.gox:141
}

//line view.gox:143
func Details() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
//line view.gox:144
		__e = __c.Any(Badge("Details are open", 0x0d9488))
		if __e != nil {
			return
		}
		return
	})
//line view.gox:145
}

//line view.gox:147
func TaskRow(task *reactive.Signal[Task], index *reactive.Signal[int]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context()
		_ = ctx
		__e = __c.Init("span")
		if __e != nil {
			return
		}
		{
			__e = __c.Submit()
			if __e != nil {
				return
			}
//line view.gox:149
			__e = __c.Any(nativegox.Text(func() string {
				return fmt.Sprintf("%d. %s", index.Get()+1, task.Get().Title)
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
//line view.gox:153
}
