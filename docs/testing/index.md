# Testing

The `uitest` package drives a running gpui-go application the way Playwright drives a browser: find elements with locators, act on them, and assert with retries. The application is real. Go, the Sable transport and the GPUI renderer all run, and each click or keystroke goes through GPUI's own hit testing, keymaps and text input handling.

```go
page, err := uitest.OpenPage(ctx, target, "main")
if err != nil {
	return err
}
if err := page.GetByRole("button", uitest.RoleOptions{Name: "Increment"}).Click(ctx); err != nil {
	return err
}
return uitest.Expect(page.GetByTestID("count")).ToHaveText(ctx, "Count: 1")
```

Start with [setting up your application](./setup), then see [Writing scenarios](./writing-scenarios). To run on a machine with no display, see [Headless testing](./headless). Every method is listed in the [API reference](./api).

## What it covers

| Exercised | Not exercised |
| --- | --- |
| Layout, text shaping and rendered bounds | Pixels. Nothing compares images |
| Pointer hit testing and click handlers | Physical OS mouse and keyboard events |
| Focus, Tab order, shortcuts and chords | An installed OS IME. Composition is simulated |
| Editing, selection and Unicode in native fields | Screen-reader output. Roles come from your snapshot, not the OS accessibility tree |
| Go event handling and the views it commits | Drag-and-drop gestures |

Input is synthesized inside the GPUI process, so a test cannot prove that the operating system would have delivered the same event.

## How a test run works

A test program runs as two processes. The parent starts a copy of itself as a worker, and the worker owns the GPUI main loop and runs the scenarios.

```text
parent: uitest.RunWorker
  worker: host.Run
    gpuitest.NewNativeTarget(client, factory)
      uitest.RunScenario
        your scenario: Page, Locator, Expect
      uitest.WriteWorkerReport
```

The split exists because an exit status alone cannot be trusted. On macOS, AppKit can terminate the process with status zero without returning to Go. The worker writes a report file before shutting down, and the parent requires both a successful exit and a complete report that records no failure. A crash, a missing report and a hang past the deadline all fail the run.
