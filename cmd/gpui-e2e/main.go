package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/moriyoshi/gpui-go/uitest"
	e2e "github.com/moriyoshi/gpui-go/uitest/starlark"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("gpui-e2e", flag.ContinueOnError)
	flags.SetOutput(stderr)
	check := flags.Bool("check", false, "parse and resolve scenarios without GPUI")
	report := flags.String("worker-report", "", "internal native worker report path")
	timeout := flags.Duration("timeout", 30*time.Second, "deadline per scenario")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() == 0 {
		return errors.New("usage: gpui-e2e [--check] [--timeout 30s] e2e/scenarios/*.star")
	}
	if *timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	var errs []error
	for _, path := range flags.Args() {
		if err := e2e.Check(path); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
		} else if *check {
			fmt.Fprintf(out, "CHECK %s\n", path)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if *check {
		return nil
	}
	if *report != "" {
		return runNative(ctx, flags.Args(), *timeout, out, *report)
	}
	return runWorker(ctx, flags.Args(), *timeout, out, stderr)
}
func runScenarios(ctx context.Context, target uitest.Target, paths []string, timeout time.Duration, out io.Writer) error {
	return uitest.RunSuite(ctx, paths, timeout, out, func(ctx context.Context, path string) error {
		return e2e.New(target, out).RunFile(ctx, path)
	})
}
