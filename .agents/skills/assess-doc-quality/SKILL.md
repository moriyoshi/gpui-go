---
name: assess-doc-quality
description: Review gpui-go documentation for usable navigation, factual agreement with APIs/build commands, coverage limits, and appropriate audience; fix only when requested.
---

# Assess Documentation Quality

Determine the requested files/scope and read AGENTS.md plus .agents/docs/QUALITY_GATE.md. This is an assessment unless fixes are also requested.

Run make harness-check for maintained harness links and metadata. Review headings/fragments and external references separately; the checker does not implement a website renderer. Verify documented commands against Makefile, public methods against Go/Rust code, and scope/limits against protocols and native implementation.

Check that readers can follow complete workflows, find the intended document, and distinguish public usage from agent memory. Root `./ARCHITECTURE.md` is canonical; architecture links point directly to that document.

Pay particular attention to unsupported protobuf declarations, optional native tags/frameworks, Cargo manifest selection, GPL configuration, and claims confusing command smoke with typing/IME automation. Check current journal/TODO entries for known gaps without treating them as proof.

Report each actionable finding with severity, file/heading, the misleading claim, source evidence, and suggested correction. Do not grade by personal stylistic preference or invent a VitePress/translation toolchain.

If fixes are requested, edit within scope and rerun affected mechanical checks. Record a durable assessment under .agents/docs when requested or useful to future work.
