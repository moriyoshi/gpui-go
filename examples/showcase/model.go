package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/moriyoshi/gpui-go/scene"
)

type effect uint8

const (
	noEffect effect = iota
	startJob
	stopJob
)

type activity struct{ when, title, detail string }
type build struct{ name, branch, status, duration string }
type model struct {
	page, accent, region, density, filter                              string
	dark, notifications, retry, showCompleted, dropdown, dialog, toast bool
	concurrency, progress                                              int
	running                                                            bool
	toastText                                                          string
	history                                                            []activity
	name, email, query                                                 scene.InputState
	inputChanges                                                       int
	composing                                                          bool
}

var builds = []build{
	{"Website production", "main", "Completed", "42s"},
	{"Desktop application", "feature/gpui", "Running", "18s"},
	{"API integration tests", "main", "Completed", "1m 12s"},
	{"Documentation preview", "docs/components", "Queued", "—"},
}

func newModel() model {
	m := model{page: "Overview", accent: "Indigo", region: "Tokyo", density: "Comfortable", filter: "All", notifications: true, retry: true, showCompleted: true, concurrency: 2}
	m.name.Value = "Ada Lovelace"
	m.record("Workspace ready", "All systems are operational")
	m.record("Website deployed", "Production build completed in 42 seconds")
	m.record("Review requested", "Desktop component library is ready for review")
	return m
}

func (m *model) record(title, detail string) {
	m.history = append([]activity{{time.Now().Format("15:04:05"), title, detail}}, m.history...)
	if len(m.history) > 60 {
		m.history = m.history[:60]
	}
}

func (m *model) apply(action string) (effect, error) {
	result := noEffect
	switch {
	case strings.HasPrefix(action, "page:"):
		page := strings.TrimPrefix(action, "page:")
		if page != "Overview" && page != "Controls" && page != "Inputs" && page != "Activity" {
			return noEffect, fmt.Errorf("unknown page %q", page)
		}
		m.page = page
		m.dropdown = false
		m.composing = false
		return noEffect, nil
	case strings.HasPrefix(action, "accent:"):
		accent := strings.TrimPrefix(action, "accent:")
		if accent != "Indigo" && accent != "Emerald" && accent != "Amber" {
			return noEffect, fmt.Errorf("unknown accent %q", accent)
		}
		m.accent = accent
	case strings.HasPrefix(action, "region:"):
		region := strings.TrimPrefix(action, "region:")
		if region != "Tokyo" && region != "London" && region != "San Francisco" {
			return noEffect, fmt.Errorf("unknown region %q", region)
		}
		m.region, m.dropdown = region, false
	case strings.HasPrefix(action, "density:"):
		density := strings.TrimPrefix(action, "density:")
		if density != "Comfortable" && density != "Compact" {
			return noEffect, fmt.Errorf("unknown density %q", density)
		}
		m.density = density
	case strings.HasPrefix(action, "filter:"):
		filter := strings.TrimPrefix(action, "filter:")
		if filter != "All" && filter != "Running" && filter != "Completed" {
			return noEffect, fmt.Errorf("unknown filter %q", filter)
		}
		m.filter = filter
	case action == "theme":
		m.dark = !m.dark
	case action == "notifications":
		m.notifications = !m.notifications
	case action == "retry":
		m.retry = !m.retry
	case action == "completed":
		m.showCompleted = !m.showCompleted
	case action == "dropdown":
		m.dropdown = !m.dropdown
		return noEffect, nil
	case action == "workers:more":
		if m.concurrency < 8 {
			m.concurrency++
		}
	case action == "workers:less":
		if m.concurrency > 1 {
			m.concurrency--
		}
	case action == "start":
		if m.running {
			return noEffect, nil
		}
		if m.progress == 100 {
			m.progress = 0
		}
		m.running = true
		result = startJob
	case action == "pause":
		m.running = false
		result = stopJob
	case action == "reset-job":
		m.progress, m.running = 0, false
		result = stopJob
	case action == "dialog:open":
		m.dialog = true
		return noEffect, nil
	case action == "dialog:cancel":
		m.dialog = false
		return noEffect, nil
	case action == "dialog:confirm":
		next := newModel()
		next.name.Epoch, next.email.Epoch, next.query.Epoch = m.name.Epoch+1, m.email.Epoch+1, m.query.Epoch+1
		*m = next
		m.page = "Controls"
		result = stopJob
	case action == "save":
		m.toast, m.toastText = true, "Preferences saved for this session"
	case action == "toast:close":
		m.toast = false
		return noEffect, nil
	case action == "inputs:clear":
		m.name.Reset("")
		m.email.Reset("")
		m.query.Reset("")
		m.inputChanges, m.composing = 0, false
	case action == "inputs:sample":
		m.name.Reset("森吉 🙂")
		m.email.Reset("hello@example.com")
		m.query.Reset("desktop")
		m.inputChanges, m.composing = 0, false
	case action == "noop":
		return noEffect, nil
	default:
		return noEffect, fmt.Errorf("unknown action %q", action)
	}
	m.record(actionTitle(action), fmt.Sprintf("%s region · %d workers · %s layout", m.region, m.concurrency, strings.ToLower(m.density)))
	return result, nil
}

func actionTitle(action string) string {
	switch action {
	case "theme":
		return "Appearance changed"
	case "start":
		return "Build started"
	case "pause":
		return "Build paused"
	case "reset-job":
		return "Build reset"
	case "save":
		return "Preferences saved"
	case "dialog:confirm":
		return "Workspace reset"
	default:
		return "Updated " + strings.ReplaceAll(action, ":", " / ")
	}
}

func (m *model) advance(progress int) {
	if !m.running || progress <= m.progress {
		return
	}
	m.progress = min(progress, 100)
	if m.progress == 100 {
		m.running = false
		m.record("Build completed", "All 20 steps finished successfully")
		if m.notifications {
			m.toast, m.toastText = true, "Build completed successfully"
		}
	}
}

func (m *model) visibleBuilds() []build {
	var visible []build
	for _, build := range builds {
		if !strings.Contains(strings.ToLower(build.name+" "+build.branch), strings.ToLower(m.query.Value)) {
			continue
		}
		if !m.showCompleted && build.status == "Completed" {
			continue
		}
		if m.filter != "All" && build.status != m.filter {
			continue
		}
		visible = append(visible, build)
	}
	return visible
}
