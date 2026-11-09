---
name: reconcile-journal-ltm
description: Audit journal coverage against LTM and TODO, fill missing durable knowledge, and compact covered entries when journal reconciliation is requested.
---

# Reconcile Journal and LTM

Read .agents/docs/JOURNAL.md, LTM/INDEX.md, TODO.md, and ../good-sleep/SKILL.md relative to this skill. Separate substantive entries from older consolidation metadata.

Audit each substantive entry for decisions, invariants, pitfalls, reproductions, test evidence, and follow-ups. Existing record tables are hints, not proof: open the destination notes and verify essential coverage.

Fill gaps using the good-sleep workflow. Extend existing topics, extract unresolved work into TODO, and refresh the index. Recheck coverage after editing. An audit-only request stops at the coverage report and any explicitly requested repairs.

For requested reconciliation/compaction, create one canonical journal section headed "LTM Consolidation Record". Deduplicate mappings from source entry headings to actual topic notes, synthesis/source relationships, and intentionally standalone references.

Only after verifying coverage may this workflow remove superseded consolidation records and substantive entries whose durable content is preserved. Keep uncovered/partially covered entries and the canonical mapping. This is the explicit exception to ordinary journal append-only behavior, not general permission to delete project files.

Run make harness-check. Report what was retained, consolidated, or removed and where the durable information now lives.
