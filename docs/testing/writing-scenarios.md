# Writing scenarios

**Prefer roles and names.** `GetByRole` finds what a user would identify, and it fails if the accessible label is lost. Use `GetByTestID` for text that has no role of its own, such as a status line.

**Assert with `Expect`.** Assertions poll until the condition holds or the timeout passes, five seconds by default. A one-off read with `Inspect` or `Count` does not wait for your condition.

**Resolve ambiguity deliberately.** Locators are strict, so an action on a locator with several matches fails rather than picking one. Narrow it with a name, or choose with `First`, `Last` or `Nth`.

```text
uitest: role("textbox") to be ready: uitest: strict mode violation: role("textbox") matched 3 elements; use Nth, First or Last
```

**Check what the action caused.** Readiness waits for an element to be attached, have rendered bounds and, for actions on a field, be enabled. It does not check whether another element covers the target or whether an animation is running. GPUI still does real hit testing, so a covered target's click lands on whatever is on top, and only the following assertion will reveal it.

**Compare Go and native state when it matters.** `page.State` shows what Go holds, while `ToHaveValue` and `ToHaveText` read the native side. A bug in synchronization can leave one of them right.

**Write shortcuts for both platforms.** `ControlOrMeta+J` presses `cmd-j` on macOS and `ctrl-j` elsewhere, so an application that binds only `cmd-j` fails on Linux.

**Do not retry an action that failed.** A failed action may already have been applied by the time the error comes back. `uitest` never resends one, and your scenarios should not either.

## Failures

A timeout names the locator and the condition. When the deadline passes between polls, it also reports the last observation:

```text
uitest: role("button") to be ready (last: 0 elements): context deadline exceeded
```

`RunScenario` always cleans up, with its own five-second deadline, so a failed scenario still closes its windows and restores the clipboard. It returns the scenario error and any cleanup error together.

For several scenarios in one worker, use `uitest.RunSuite`. It gives each scenario its own deadline, prints `PASS` or `FAIL` lines and continues after a failure. See [Running scenarios](./api#running-scenarios).
