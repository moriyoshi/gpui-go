---
name: write-tests
description: Design gpui-go regressions around observable failure classes and select the correct Go, Rust, Sable, native editor, or input-interaction layer.
---

# Write Tests

Read the relevant implementation, existing tests, architecture invariants, and .agents/docs/TESTING.md before choosing a test. State the concrete failure and observable result first; a test derived only from the intended fix can certify the same blind spot.

Choose the layer:
* Go client tests: encoding, negotiation, correlation, typed errors, revisions, event-consumer rules.
* Rust core/text tests: invalid trees/sessions, text ranges and UTF-16/grapheme boundaries.
* Runtime and actual Sable integration: bounded queues, cancellation, event acknowledgement, pending-call release.
* Real native editor smoke: native transactions, read-only behavior, instance isolation, close/stale handles/reopen.
* Actual input interaction: focus/key routing, composition, selection, clipboard, accessibility.

Use boundary-biased Unicode, size, revision, and lifecycle cases where relevant. Preserve intentional decomposed data and distinguish the generic acknowledged event stream from the editor's non-replayed stream. Mocks cannot prove Zed undo or platform input.

Prefer extending meaningful existing coverage. Avoid tests that mirror the implementation or assert incidental wording. For complicated changes, record the failure-to-layer map in the journal.

When sensitivity is uncertain, use a negative control in an isolated scratch fixture/copy and confirm a behavioral diagnostic. Do not deliberately break shared source; compile failure alone is not a valid control.

Run focused tests, then the applicable quality gate. Record newly found failure classes and remaining coverage gaps; do not weaken an oracle to turn a failure green.
