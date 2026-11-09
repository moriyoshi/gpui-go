---
name: distill-memories
description: Promote verified journal and LTM knowledge into the canonical overview, architecture, testing, quality gate, or public API documentation.
---

# Distill Memories

Read .agents/docs/OVERVIEW.md, root ARCHITECTURE.md, .agents/docs/QUALITY_GATE.md, recent journal entries, and LTM/INDEX.md. Open only topic notes relevant to missing or stale canonical guidance.

Classify each candidate:
* Public API/run/behavior guidance: README.md or rust/zed-editor/README.md.
* Scope and orientation: .agents/docs/OVERVIEW.md.
* Ownership, structure, invariants: root ARCHITECTURE.md.
* Evidence and test-layer selection: .agents/docs/TESTING.md.
* Recurring verification requirements: .agents/docs/QUALITY_GATE.md.
* Incident detail or temporary investigation: retain in journal/LTM.

Verify claims against current code, manifests, and Makefile. Preserve uncertainty instead of promoting a stale note as fact. Merge concise guidance into existing sections; do not duplicate the agent architecture pointer or import another project's website layout.

Preserve distinctions between generic snapshots and editor instances, between native command smoke and actual input automation, and between the Apache backend and the opted-in GPL editor.

Append a record of meaningful promotions to JOURNAL.md. Check links and commands, then run make harness-check. Do not compact earlier journal entries here.
