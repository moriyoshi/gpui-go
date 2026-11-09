package main

import (
	"fmt"
	"hash/fnv"
	"unicode/utf8"

	"github.com/moriyoshi/gpui-go/scene"
)

// Keys describe component identity, not a render-time counter. GPUI retains
// hover and scroll state for these IDs when Go replaces the surrounding tree.
func id(key string) uint64 { h := fnv.New64a(); _, _ = h.Write([]byte(key)); return h.Sum64() }
func box(key string, style *scene.Style, children ...*scene.Node) *scene.Node {
	return scene.Div(id(key), children...).Style(style)
}
func rowStyle(gap float32) *scene.Style {
	return &scene.Style{FlexDirection: scene.FlexRow.Enum(), AlignItems: scene.AlignCenter.Enum(), Gap: scene.Pixels(gap), FlexShrink: scene.Ptr[float32](0)}
}
func colStyle(gap float32) *scene.Style {
	return &scene.Style{FlexDirection: scene.FlexColumn.Enum(), Gap: scene.Pixels(gap), FlexShrink: scene.Ptr[float32](0)}
}
func row(key string, gap float32, children ...*scene.Node) *scene.Node {
	return box(key, rowStyle(gap), children...)
}
func column(key string, gap float32, children ...*scene.Node) *scene.Node {
	return box(key, colStyle(gap), children...)
}
func spacer(key string) *scene.Node { return box(key, &scene.Style{FlexGrow: scene.Ptr[float32](1)}) }
func inset(vertical, horizontal float32) *scene.Edges {
	return &scene.Edges{Top: scene.Pixels(vertical), Bottom: scene.Pixels(vertical), Left: scene.Pixels(horizontal), Right: scene.Pixels(horizontal)}
}
func label(key, value string, size float32, weight uint32, color uint32) *scene.Node {
	return scene.Text(id(key), value).Style(&scene.Style{FontSize: scene.Ptr[float32](size), FontWeight: scene.Ptr[uint32](weight), TextColor: scene.RGB(color), FlexShrink: scene.Ptr[float32](0)})
}

type colors struct{ background, panel, text, muted, border, hover, accent, accentHover, tint uint32 }

func (g *gallery) colors() colors {
	p := colors{0xf4f6fa, 0xffffff, 0x202a3c, 0x748096, 0xe5e9f0, 0xf0f3f8, 0x5b5ce2, 0x4849c5, 0xeeeeff}
	if g.state.dark {
		p.background, p.panel, p.text, p.muted, p.border, p.hover, p.tint = 0x101521, 0x1a2130, 0xe9edf7, 0x9aa7bd, 0x2c3548, 0x252e40, 0x2b2b4e
	}
	switch g.state.accent {
	case "Emerald":
		p.accent, p.accentHover = 0x168b70, 0x10765d
		if !g.state.dark {
			p.tint = 0xe9f7f1
		}
	case "Amber":
		p.accent, p.accentHover = 0xb97a16, 0x99610e
		if !g.state.dark {
			p.tint = 0xfff5df
		}
	}
	return p
}

func (g *gallery) button(key, title, action, variant string, disabled bool) *scene.Node {
	p := g.colors()
	background, foreground, border, hover := p.panel, p.text, p.border, p.hover
	switch variant {
	case "primary":
		background, foreground, border, hover = p.accent, 0xffffff, p.accent, p.accentHover
	case "danger":
		background, foreground, border, hover = 0xb13d47, 0xffffff, 0xb13d47, 0x97313a
	case "ghost":
		background, foreground, border, hover = p.panel, p.muted, p.panel, p.hover
	}
	style := rowStyle(6)
	style.JustifyContent = scene.JustifyCenter.Enum()
	style.Padding, style.Background, style.TextColor = inset(10, 14), scene.RGB(background), scene.RGB(foreground)
	style.BorderWidth, style.BorderColor, style.CornerRadius = scene.Ptr[float32](1), scene.RGB(border), scene.Ptr[float32](8)
	if disabled {
		style.Background, style.TextColor, style.BorderColor = scene.RGB(p.hover), scene.RGB(p.muted), scene.RGB(p.border)
	}
	style.Cursor = scene.CursorArrow.Enum()
	if !disabled {
		style.Cursor = scene.CursorPointer.Enum()
	}
	node := box(key, style, label(key+"/label", title, 13, 600, foreground))
	if disabled {
		return box(key, style, label(key+"/label", title, 13, 600, p.muted))
	}
	return node.Focusable(0).Accessible(title, &scene.Accessibility{Role: scene.RoleButton}).Hover(&scene.Style{Background: scene.RGB(hover)}).OnClick(g.bindings[action]).StopPropagation()
}

func (g *gallery) card(key, title, subtitle string, children ...*scene.Node) *scene.Node {
	p := g.colors()
	header := []*scene.Node{label(key+"/title", title, 16, 600, p.text)}
	if subtitle != "" {
		header = append(header, label(key+"/subtitle", subtitle, 12, 400, p.muted))
	}
	contents := []*scene.Node{column(key+"/heading", 5, header...)}
	contents = append(contents, children...)
	style := colStyle(18)
	style.Width, style.Padding, style.Background = scene.Percent(100), scene.Padding(22), scene.RGB(p.panel)
	style.BorderWidth, style.BorderColor, style.CornerRadius = scene.Ptr[float32](1), scene.RGB(p.border), scene.Ptr[float32](14)
	return box(key, style, contents...)
}

func (g *gallery) badge(key, title string, positive bool) *scene.Node {
	p := g.colors()
	foreground, background := p.muted, p.hover
	if positive {
		foreground, background = p.accent, p.tint
	}
	style := rowStyle(5)
	style.Padding, style.Background, style.CornerRadius = inset(5, 9), scene.RGB(background), scene.Ptr[float32](20)
	return box(key, style, label(key+"/label", title, 11, 600, foreground))
}

func (g *gallery) choiceRow(key, title, description, action string, checked, toggle bool) *scene.Node {
	p := g.colors()
	var indicator *scene.Node
	if toggle {
		track := rowStyle(0)
		track.Width, track.Height, track.Padding, track.CornerRadius = scene.Pixels(38), scene.Pixels(22), scene.Padding(3), scene.Ptr[float32](11)
		track.Background = scene.RGB(p.border)
		if checked {
			track.Background, track.JustifyContent = scene.RGB(p.accent), scene.JustifyEnd.Enum()
		}
		indicator = box(key+"/track", track, box(key+"/thumb", &scene.Style{Width: scene.Pixels(16), Height: scene.Pixels(16), Background: scene.RGB(0xffffff), CornerRadius: scene.Ptr[float32](8)}))
	} else {
		check := rowStyle(0)
		check.Width, check.Height, check.CornerRadius = scene.Pixels(19), scene.Pixels(19), scene.Ptr[float32](5)
		check.JustifyContent, check.BorderWidth, check.BorderColor = scene.JustifyCenter.Enum(), scene.Ptr[float32](1), scene.RGB(p.border)
		if checked {
			check.Background, check.BorderColor = scene.RGB(p.accent), scene.RGB(p.accent)
		}
		mark := ""
		if checked {
			mark = "✓"
		}
		indicator = box(key+"/check", check, label(key+"/mark", mark, 13, 700, 0xffffff))
	}
	text := colStyle(4)
	text.FlexGrow = scene.Ptr[float32](1)
	style := rowStyle(12)
	style.Width, style.Padding, style.Cursor, style.CornerRadius = scene.Percent(100), inset(7, 0), scene.CursorPointer.Enum(), scene.Ptr[float32](8)
	return box(key, style,
		box(key+"/text", text, label(key+"/title", title, 13, 500, p.text), label(key+"/description", description, 11, 400, p.muted)), indicator,
	).Focusable(0).Accessible(title, &scene.Accessibility{Role: scene.RoleCheckbox, Checked: scene.Ptr[bool](checked)}).Hover(&scene.Style{Background: scene.RGB(p.hover)}).OnClick(g.bindings[action]).StopPropagation()
}

func (g *gallery) segmented(key string, options []string, selected, prefix string) *scene.Node {
	p := g.colors()
	var children []*scene.Node
	for _, option := range options {
		style := rowStyle(0)
		style.Padding, style.CornerRadius, style.Cursor = inset(8, 12), scene.Ptr[float32](7), scene.CursorPointer.Enum()
		color := p.muted
		if option == selected {
			style.Background, color = scene.RGB(p.panel), p.accent
		}
		children = append(children, box(key+"/"+option, style, label(key+"/"+option+"/label", option, 12, 600, color)).Focusable(0).Accessible(option, &scene.Accessibility{Role: scene.RoleButton, Selected: scene.Ptr[bool](option == selected)}).Hover(&scene.Style{Background: scene.RGB(p.panel)}).OnClick(g.bindings[prefix+option]).StopPropagation())
	}
	style := rowStyle(3)
	style.Padding, style.Background, style.CornerRadius = scene.Padding(4), scene.RGB(p.hover), scene.Ptr[float32](10)
	return box(key, style, children...)
}

func (g *gallery) render() *scene.Node {
	p := g.colors()
	sidebar := colStyle(12)
	sidebar.Width, sidebar.Height, sidebar.Padding = scene.Pixels(210), scene.Percent(100), scene.Padding(22)
	sidebar.Background = scene.RGB(0x161d2d)
	logo := row("nav/logo", 10,
		box("nav/logo/icon", &scene.Style{Width: scene.Pixels(30), Height: scene.Pixels(30), Background: scene.RGB(p.accent), CornerRadius: scene.Ptr[float32](9)}),
		label("nav/logo/title", "Studio", 23, 700, 0xffffff))
	nav := []*scene.Node{logo, label("nav/caption", "COMPONENT SHOWCASE", 9, 600, 0x8793ac), box("nav/gap", &scene.Style{Height: scene.Pixels(22)})}
	for index, page := range []string{"Overview", "Controls", "Inputs", "Activity"} {
		style := rowStyle(12)
		style.Width, style.Padding, style.CornerRadius, style.Cursor = scene.Percent(100), inset(12, 12), scene.Ptr[float32](9), scene.CursorPointer.Enum()
		foreground := uint32(0x9ba7be)
		if g.state.page == page {
			style.Background, foreground = scene.RGB(0x2b3350), 0xffffff
		}
		nav = append(nav, box("nav/"+page, style, label("nav/"+page+"/number", fmt.Sprintf("0%d", index+1), 10, 600, 0x8998b8), label("nav/"+page+"/label", page, 13, 500, foreground)).Focusable(0).Accessible(page, &scene.Accessibility{Role: scene.RoleButton, Selected: scene.Ptr[bool](g.state.page == page)}).Hover(&scene.Style{Background: scene.RGB(0x252e43)}).OnClick(g.bindings["page:"+page]).StopPropagation())
	}
	nav = append(nav, spacer("nav/spacer"), label("nav/footer/title", "Made with GPUI", 12, 500, 0xe2e8f4), label("nav/footer/detail", "State and controls in Go", 10, 400, 0x8793ac))
	main := colStyle(0)
	main.FlexGrow, main.FlexShrink, main.MinWidth, main.Height = scene.Ptr[float32](1), scene.Ptr[float32](1), scene.Pixels(0), scene.Percent(100)
	header := rowStyle(16)
	header.Padding, header.Height, header.Width = inset(18, 28), scene.Pixels(88), scene.Percent(100)
	header.Background = scene.RGB(p.panel)
	themeLabel := "Dark mode"
	if g.state.dark {
		themeLabel = "Light mode"
	}
	var content []*scene.Node
	switch g.state.page {
	case "Overview":
		content = g.overview()
	case "Controls":
		content = g.controls()
	case "Inputs":
		content = g.inputs()
	case "Activity":
		content = g.activityPage()
	}
	viewport := colStyle(22)
	viewport.Width, viewport.MinHeight, viewport.Padding, viewport.FlexGrow, viewport.FlexShrink = scene.Percent(100), scene.Pixels(0), scene.Padding(28), scene.Ptr[float32](1), scene.Ptr[float32](1)
	viewport.OverflowY, viewport.OverflowX = scene.OverflowScroll.Enum(), scene.OverflowHidden.Enum()
	children := []*scene.Node{
		box("nav", sidebar, nav...),
		box("main", main,
			box("header", header, column("header/heading", 5, label("header/title", g.state.page, 23, 700, p.text), label("header/subtitle", "A workspace full of small, useful interactions.", 12, 400, p.muted)), spacer("header/spacer"), g.badge("header/status", "● All systems online", true), g.button("header/theme", themeLabel, "theme", "secondary", false)),
			box("viewport/"+g.state.page, viewport, content...),
		),
	}
	if g.state.toast {
		children = append(children, g.toast())
	}
	if g.state.dialog {
		children = append(children, g.dialog())
	}
	style := rowStyle(0)
	style.Width, style.Height, style.Background, style.TextColor = scene.Percent(100), scene.Percent(100), scene.RGB(p.background), scene.RGB(p.text)
	style.Position, style.OverflowX, style.OverflowY = scene.PositionRelative.Enum(), scene.OverflowHidden.Enum(), scene.OverflowHidden.Enum()
	return box("app", style, children...)
}

func (g *gallery) overview() []*scene.Node {
	p := g.colors()
	var metrics []*scene.Node
	for index, stat := range []struct{ name, value, detail string }{{"Builds today", "24", "+6 since yesterday"}, {"Success rate", "98.6%", "Healthy over the last 7 days"}, {"Active workers", fmt.Sprint(g.state.concurrency), "Configured in Controls"}} {
		key := fmt.Sprintf("overview/metric/%d", index)
		style := colStyle(12)
		style.Padding, style.Background, style.BorderWidth, style.BorderColor, style.CornerRadius, style.FlexGrow = scene.Padding(22), scene.RGB(p.panel), scene.Ptr[float32](1), scene.RGB(p.border), scene.Ptr[float32](14), scene.Ptr[float32](1)
		metrics = append(metrics, box(key, style, label(key+"/name", stat.name, 12, 500, p.muted), label(key+"/value", stat.value, 30, 700, p.text), label(key+"/detail", stat.detail, 10, 400, p.accent)))
	}
	status := "Ready to start"
	if g.state.running {
		status = "Building application…"
	} else if g.state.progress == 100 {
		status = "Build complete"
	} else if g.state.progress > 0 {
		status = "Paused — resume when ready"
	}
	track := rowStyle(0)
	track.Width, track.Height, track.Background, track.CornerRadius, track.OverflowX = scene.Percent(100), scene.Pixels(9), scene.RGB(p.hover), scene.Ptr[float32](5), scene.OverflowHidden.Enum()
	bar := box("overview/build/progress", track, box("overview/build/fill", &scene.Style{Width: scene.Percent(float32(g.state.progress)), Height: scene.Percent(100), Background: scene.RGB(p.accent), CornerRadius: scene.Ptr[float32](5)}))
	primary := "Start build"
	if g.state.progress > 0 && g.state.progress < 100 {
		primary = "Resume build"
	}
	if g.state.progress == 100 {
		primary = "Run again"
	}
	pipeline := g.card("overview/build", "Build pipeline", "A background Go task updates this progress bar.",
		row("overview/build/status", 10, label("overview/build/message", status, 13, 500, p.text), spacer("overview/build/spacer"), label("overview/build/percent", fmt.Sprintf("%d%%", g.state.progress), 14, 700, p.accent)), bar,
		row("overview/build/buttons", 8, g.button("overview/build/start", primary, "start", "primary", g.state.running), g.button("overview/build/pause", "Pause", "pause", "secondary", !g.state.running), g.button("overview/build/reset", "Reset", "reset-job", "ghost", g.state.progress == 0 && !g.state.running)),
	)
	return []*scene.Node{
		row("overview/metrics", 16, metrics...), pipeline,
		g.card("overview/recent", "Recent builds", "Use the segmented filter or hide completed builds.",
			row("overview/recent/toolbar", 10, g.segmented("overview/recent/filter", []string{"All", "Running", "Completed"}, g.state.filter, "filter:"), spacer("overview/recent/spacer"), g.button("overview/recent/completed", map[bool]string{true: "Hide completed", false: "Show completed"}[g.state.showCompleted], "completed", "ghost", false)),
			g.buildTable(),
		),
		g.card("overview/notes", "Make it yours", "Every control changes the same workspace state.",
			label("overview/notes/text", "Change the accent, choose a region, or tune the worker count in Controls.", 13, 400, p.muted),
			row("overview/notes/buttons", 8, g.button("overview/notes/controls", "Explore controls →", "page:Controls", "secondary", false), g.button("overview/notes/activity", "View activity", "page:Activity", "ghost", false)),
		),
	}
}

func (g *gallery) buildTable() *scene.Node {
	p := g.colors()
	cell := func(key, value string, width float32, color uint32) *scene.Node {
		return box(key, &scene.Style{Width: scene.Pixels(width), FlexShrink: scene.Ptr[float32](0)}, label(key+"/text", value, 12, 500, color))
	}
	var rows []*scene.Node
	header := rowStyle(12)
	header.Padding = inset(8, 0)
	rows = append(rows, box("builds/header", header, cell("builds/header/name", "PROJECT / BRANCH", 300, p.muted), spacer("builds/header/spacer"), cell("builds/header/status", "STATUS", 110, p.muted), cell("builds/header/time", "DURATION", 70, p.muted)))
	for _, build := range g.state.visibleBuilds() {
		key := "builds/" + build.name
		style := rowStyle(12)
		style.Padding, style.CornerRadius = inset(14, 0), scene.Ptr[float32](8)
		if g.state.density == "Compact" {
			style.Padding = inset(7, 0)
		}
		name := colStyle(5)
		name.Width = scene.Pixels(300)
		status := &scene.Style{Width: scene.Pixels(110), FlexShrink: scene.Ptr[float32](0)}
		rows = append(rows, box(key, style, box(key+"/name", name, label(key+"/title", build.name, 13, 500, p.text), label(key+"/branch", build.branch, 11, 400, p.muted)), spacer(key+"/spacer"), box(key+"/status", status, g.badge(key+"/badge", build.status, build.status != "Queued")), cell(key+"/duration", build.duration, 70, p.muted)).Hover(&scene.Style{Background: scene.RGB(p.hover)}))
	}
	if len(rows) == 1 {
		rows = append(rows, label("builds/empty", "No builds match these filters. Try All or show completed builds.", 12, 400, p.muted))
	}
	return column("builds", 3, rows...)
}

func (g *gallery) controls() []*scene.Node {
	p := g.colors()
	var radios []*scene.Node
	for _, accent := range []string{"Indigo", "Emerald", "Amber"} {
		key := "controls/accent/" + accent
		outer := rowStyle(0)
		outer.Width, outer.Height, outer.BorderWidth, outer.BorderColor, outer.CornerRadius, outer.JustifyContent = scene.Pixels(18), scene.Pixels(18), scene.Ptr[float32](1), scene.RGB(p.border), scene.Ptr[float32](9), scene.JustifyCenter.Enum()
		var dot []*scene.Node
		if accent == g.state.accent {
			outer.BorderColor = scene.RGB(p.accent)
			dot = append(dot, box(key+"/dot", &scene.Style{Width: scene.Pixels(9), Height: scene.Pixels(9), Background: scene.RGB(p.accent), CornerRadius: scene.Ptr[float32](5)}))
		}
		style := rowStyle(8)
		style.Padding, style.Cursor, style.CornerRadius = inset(8, 10), scene.CursorPointer.Enum(), scene.Ptr[float32](8)
		radios = append(radios, box(key, style, box(key+"/radio", outer, dot...), label(key+"/label", accent, 13, 500, p.text)).Hover(&scene.Style{Background: scene.RGB(p.hover)}).OnClick(g.bindings["accent:"+accent]).StopPropagation())
	}
	var regionOptions []*scene.Node
	for _, region := range []string{"Tokyo", "London", "San Francisco"} {
		regionOptions = append(regionOptions, g.button("controls/region/"+region, region, "region:"+region, "ghost", false))
	}
	region := []*scene.Node{g.button("controls/region/trigger", g.state.region+"   ▾", "dropdown", "secondary", false)}
	if g.state.dropdown {
		region = append(region, column("controls/region/menu", 3, regionOptions...))
	}
	return []*scene.Node{
		g.card("controls/preferences", "Workspace preferences", "Switches and checkboxes retain their state between pages.",
			g.choiceRow("controls/notifications", "Build notifications", "Show a toast when the background build finishes", "notifications", g.state.notifications, true),
			g.choiceRow("controls/retry", "Retry failed tasks", "Remember your preferred retry policy", "retry", g.state.retry, false),
			g.choiceRow("controls/completed", "Include completed builds", "Apply this filter to the overview table", "completed", g.state.showCompleted, false),
		),
		g.card("controls/appearance", "Appearance", "Try the same controls with different colors and spacing.",
			column("controls/appearance/accent", 8, label("controls/accent/label", "Accent color", 12, 600, p.text), row("controls/accent/options", 12, radios...)),
			column("controls/appearance/density", 8, label("controls/density/label", "Table density", 12, 600, p.text), g.segmented("controls/density", []string{"Comfortable", "Compact"}, g.state.density, "density:")),
		),
		g.card("controls/environment", "Environment", "A dropdown and a bounded stepper configure the workspace.",
			column("controls/region", 8, label("controls/region/label", "Build region", 12, 600, p.text), column("controls/region/options", 4, region...)),
			row("controls/workers", 12, column("controls/workers/labels", 4, label("controls/workers/title", "Concurrent workers", 13, 500, p.text), label("controls/workers/detail", "Choose between 1 and 8", 11, 400, p.muted)), spacer("controls/workers/spacer"), g.button("controls/workers/minus", "−", "workers:less", "secondary", g.state.concurrency == 1), label("controls/workers/count", fmt.Sprint(g.state.concurrency), 16, 700, p.text), g.button("controls/workers/plus", "+", "workers:more", "secondary", g.state.concurrency == 8)),
		),
		g.card("controls/actions", "Actions & feedback", "Primary, secondary, disabled, and destructive button states.",
			row("controls/actions/buttons", 8, g.button("controls/save", "Save preferences", "save", "primary", false), g.button("controls/cancel", "Reset workspace…", "dialog:open", "secondary", false), g.button("controls/disabled", "Unavailable", "noop", "secondary", true)),
			label("controls/session-note", "Preferences are kept in memory for this demo session.", 11, 400, p.muted),
		),
	}
}

func (g *gallery) activityPage() []*scene.Node {
	p := g.colors()
	var rows []*scene.Node
	for index, event := range g.state.history {
		key := fmt.Sprintf("activity/row/%d", index)
		style := rowStyle(16)
		style.Padding, style.CornerRadius = inset(14, 8), scene.Ptr[float32](8)
		body := colStyle(5)
		body.FlexGrow = scene.Ptr[float32](1)
		rows = append(rows, box(key, style, label(key+"/dot", "●", 10, 600, p.accent), box(key+"/body", body, label(key+"/title", event.title, 13, 500, p.text), label(key+"/detail", event.detail, 11, 400, p.muted)), label(key+"/time", event.when, 11, 400, p.muted)).Hover(&scene.Style{Background: scene.RGB(p.hover)}))
	}
	return []*scene.Node{g.card("activity", "Workspace activity", fmt.Sprintf("%d recent events · scroll to see earlier changes", len(g.state.history)), rows...)}
}

func (g *gallery) textField(key, title, placeholder, field string, disabled bool) *scene.Node {
	p := g.colors()
	state := *g.inputState(field)
	handler := g.bindings["input:"+field]
	if disabled {
		state = scene.InputState{Value: "Managed by your workspace"}
		handler = 0
	}
	style := colStyle(0)
	style.Width, style.Padding, style.Background = scene.Percent(100), inset(9, 12), scene.RGB(p.panel)
	style.BorderWidth, style.BorderColor, style.CornerRadius = scene.Ptr[float32](1), scene.RGB(p.border), scene.Ptr[float32](8)
	style.TextColor, style.FontSize = scene.RGB(p.text), scene.Ptr[float32](14)
	if disabled {
		style.Background, style.TextColor = scene.RGB(p.hover), scene.RGB(p.muted)
	}
	input := scene.TextInput(id(key+"/editor"), state, scene.TextInputOptions{Placeholder: placeholder, OnChange: handler, Disabled: disabled}).Style(style)
	if !disabled {
		input = input.Hover(&scene.Style{BorderColor: scene.RGB(p.accent)})
	}
	return column(key, 7, label(key+"/label", title, 12, 600, p.text), input)
}

func (g *gallery) inputs() []*scene.Node {
	p := g.colors()
	preview := func(value string) string {
		runes := []rune(value)
		if len(runes) > 65 {
			value = string(runes[:65]) + "…"
		}
		if value == "" {
			return "(empty)"
		}
		return value
	}
	status := "Committed text"
	if g.state.composing {
		status = "IME composition in progress"
	}
	return []*scene.Node{
		g.card("inputs/editing", "Try text editing", "Click a field, type, select, copy/paste, or switch input methods.",
			g.textField("inputs/name", "Display name", "Type your name…", "name", false),
			g.textField("inputs/email", "Email", "you@example.com", "email", false),
			row("inputs/actions", 8, g.button("inputs/sample", "Load Unicode sample", "inputs:sample", "secondary", false), g.button("inputs/clear", "Clear fields", "inputs:clear", "ghost", false)),
			label("inputs/help", "Tab / Shift+Tab · Shift+arrows · Cmd+A / C / X / V · Home / End", 11, 400, p.muted),
		),
		g.card("inputs/preview", "Live values in Go", fmt.Sprintf("%d native edits received · %s", g.state.inputChanges, status),
			label("inputs/preview/name", "Name: "+preview(g.state.name.Value), 14, 500, p.text),
			label("inputs/preview/email", "Email: "+preview(g.state.email.Value), 13, 400, p.muted),
			label("inputs/preview/count", fmt.Sprintf("Name: %d Unicode code points / %d UTF-8 bytes", utf8.RuneCountInString(g.state.name.Value), len(g.state.name.Value)), 11, 400, p.accent),
		),
		g.card("inputs/search", "Filter a real list", "The query updates these rows as you type.",
			g.textField("inputs/query", "Build search", "Try desktop, main, or docs…", "query", false),
			g.button("inputs/all", "Show all build statuses", "filter:All", "ghost", false),
			g.buildTable(),
		),
		g.card("inputs/disabled", "Disabled field", "This input is excluded from editing and keyboard tab stops.",
			g.textField("inputs/readonly", "Workspace policy", "", "name", true),
		),
	}
}

func (g *gallery) toast() *scene.Node {
	p := g.colors()
	style := rowStyle(12)
	style.Position, style.Inset, style.Width, style.Padding = scene.PositionAbsolute.Enum(), &scene.Edges{Right: scene.Pixels(24), Bottom: scene.Pixels(24)}, scene.Pixels(390), scene.Padding(16)
	style.Background, style.BorderWidth, style.BorderColor, style.CornerRadius = scene.RGB(p.panel), scene.Ptr[float32](1), scene.RGB(p.accent), scene.Ptr[float32](12)
	text := colStyle(5)
	text.FlexGrow = scene.Ptr[float32](1)
	return box("toast", style, box("toast/body", text, label("toast/title", "✓  Success", 13, 600, p.accent), label("toast/message", g.state.toastText, 11, 400, p.text)), g.button("toast/close", "×", "toast:close", "ghost", false))
}

func (g *gallery) dialog() *scene.Node {
	p := g.colors()
	style := colStyle(18)
	style.Width, style.Padding, style.Background, style.CornerRadius = scene.Pixels(470), scene.Padding(28), scene.RGB(p.panel), scene.Ptr[float32](16)
	content := box("dialog/content", style,
		label("dialog/eyebrow", "WORKSPACE SETTINGS", 10, 600, p.accent),
		label("dialog/title", "Reset your workspace?", 24, 700, p.text),
		label("dialog/body/1", "This resets the demo's preferences and stops the build.", 13, 400, p.muted),
		label("dialog/body/2", "No files or saved projects are affected.", 13, 400, p.muted),
		row("dialog/buttons", 8, spacer("dialog/spacer"), g.button("dialog/cancel", "Keep settings", "dialog:cancel", "secondary", false), g.button("dialog/confirm", "Reset workspace", "dialog:confirm", "danger", false)),
	).OnClick(g.bindings["noop"]).StopPropagation()
	overlay := rowStyle(0)
	overlay.Position, overlay.Inset, overlay.Width, overlay.Height = scene.PositionAbsolute.Enum(), scene.Padding(0), scene.Percent(100), scene.Percent(100)
	overlay.JustifyContent = scene.JustifyCenter.Enum()
	overlay.Background = &scene.Color{Red: 0.03, Green: 0.05, Blue: 0.1, Alpha: 0.65}
	return box("dialog", overlay, content).OnClick(g.bindings["dialog:cancel"]).StopPropagation()
}
