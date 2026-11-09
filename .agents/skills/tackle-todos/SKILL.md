---
name: tackle-todos
description: Reverify documented and source TODOs, prioritize a requested subset, implement authorized work, and record outcomes without expanding the task to the entire backlog.
---

# Tackle TODOs

Read .agents/docs/TODO.md and search first-party Go, Rust, and protobuf sources for TODO/FIXME comments using rg. Exclude generated bindings, caches, build artifacts, upstream scratch checkouts, and vendored code.

Deduplicate equivalent items and verify every candidate against current code before acting. Record stale/resolved entries with evidence. Classify correctness, validation, serialization, testing, and design work; a missing feature is not automatically an approved bug fix.

Apply the user's filter/scope. Explain a concrete work list and proceed with already authorized items. Keep proposals requiring new scope as follow-ups rather than implementing them implicitly.

Work sequentially unless delegation is authorized by the current user/session or applicable instructions. When authorized, give agents independent file ownership and the relevant project gate; this skill does not override runtime delegation constraints or create automatic permission for agents/worktrees.

Verify each change in the layer that observes it. Reuse existing caches/target directories. Update TODO status, preserve unresolved reasons, and append a journal entry with evidence. Do not commit or push unless requested.
