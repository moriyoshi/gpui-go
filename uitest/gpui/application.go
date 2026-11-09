// Package gpui adapts the GPUI protocol client to the shared uitest harness.
// Driving requires the gpui_e2e tag and an instrumented native backend.
package gpui

import (
	"context"

	clientlib "github.com/moriyoshi/gpui-go/client"
)

// Application is supplied by an example or application under test. Presentation
// and fixture-specific state/updates stay outside the harness implementation.
type Application interface {
	Window() *clientlib.Window
	Element(string) (uint64, bool)
	State() map[string]any
	Update(context.Context, string, string) error
	Close(context.Context) error
}

// Factory opens one application and its primary window. Element("root") must
// resolve to a nonzero stable ID for event barriers. The application owns its
// presentation, handlers, models and any additional resources released by Close.
type Factory func(context.Context, *clientlib.Client, string) (Application, error)
