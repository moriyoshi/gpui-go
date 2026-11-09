---
name: quality-gate
description: Evaluate and repair the checks applicable to a requested gpui-go change using the Go, Rust, protocol, native, and documentation quality gates.
---

# Quality Gate

Read .agents/docs/QUALITY_GATE.md, TESTING.md, and the relevant root ARCHITECTURE.md invariants. Determine scope from the request and available working-tree evidence. Use git status/diff only when .git is present; a source export is not an empty change set.

Select gate rows by affected layer. List checks and run the required commands with explicit Cargo manifests and --locked. Include --workspace for editor Clippy. Keep platform/display/feature prerequisites visible.

Review ownership, stale handles, revisions, validation-before-mutation, event semantics, bounded queues, and FFI/shutdown constraints where affected. Check regression coverage in the layer that can observe the failure. Read tests rather than inferring coverage from file names.

Report PASS, FAIL, or NOT RUN per applicable check with actual evidence. Fix introduced failures within the authorized scope, then rerun the focused proof and any other check affected by that fix. Do not label a skipped GUI run as passed or widen work to unrelated defects.

For documentation/harness changes, run make harness-check and verify runtime claims manually against code. Do not require native builds for prose-only work.

Append durable findings/results to JOURNAL.md and put unresolved follow-ups in TODO. Keep the report concise; do not invent a passing historical baseline.
