package uitest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// CleanupTimeout bounds cleanup independently of a scenario's cancellation.
const CleanupTimeout = 5 * time.Second

// RunScenario executes any Go callback and guarantees target cleanup. Interpreter
// adapters keep their own evaluation/cancellation logic inside the callback.
func RunScenario(ctx context.Context, target Target, run func(context.Context) error) (err error) {
	if target == nil {
		return errors.New("uitest: missing target")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), CleanupTimeout)
		defer cancel()
		err = errors.Join(err, target.Cleanup(cleanup))
	}()
	if run == nil {
		return errors.New("uitest: missing scenario callback")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return errors.Join(run(ctx), ctx.Err())
}

// RunSuite applies per-scenario deadlines, continues after failures and returns
// all failures. Each callback owns its evaluation and should use RunScenario for
// cleanup. Source diagnostics belong to the interpreter adapter, if any.
func RunSuite(ctx context.Context, paths []string, timeout time.Duration, out io.Writer, run func(context.Context, string) error) error {
	if len(paths) == 0 || timeout <= 0 || run == nil {
		return errors.New("uitest: scenarios, positive timeout and runner are required")
	}
	if out == nil {
		out = io.Discard
	}
	var failures []error
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			failures = append(failures, err)
			break
		}
		scenario, cancel := context.WithTimeout(ctx, timeout)
		err := errors.Join(run(scenario, path), scenario.Err())
		cancel()
		if err != nil {
			fmt.Fprintf(out, "FAIL %s: %v\n", path, err)
			failures = append(failures, fmt.Errorf("%s: %w", path, err))
		} else {
			fmt.Fprintf(out, "PASS %s\n", path)
		}
	}
	return errors.Join(failures...)
}
