// Package e2e forwards the original Starlark harness API.
// Deprecated: import github.com/moriyoshi/gpui-go/uitest/starlark instead.
package e2e

import (
	"io"

	"github.com/moriyoshi/gpui-go/uitest"
	starlarktest "github.com/moriyoshi/gpui-go/uitest/starlark"
)

type Harness = starlarktest.Harness

func New(target uitest.Target, out io.Writer) *Harness { return starlarktest.New(target, out) }
func Check(path string) error                          { return starlarktest.Check(path) }
