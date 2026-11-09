---
name: audit-licenses
description: Inspect actual Go/Rust dependency and asset configurations for license/notice obligations, keeping the Apache backend and explicitly opted-in GPL editor distribution distinct.
---

# Audit Licenses

Determine the artifact and build tags/features being audited before inventorying dependencies. Read Cargo manifests/lockfiles, Go module metadata, and relevant license/notice files. A default untagged Go dependency list does not inventory native Rust archives.

Treat the generic Apache GPUI backend and opt-in GPL-3.0-or-later editor component/host as separate configurations. The user has explicitly selected the GPL editor path; report its requirements and boundaries rather than applying Cornus's blanket prohibition on strong copyleft or removing that feature.

Verify each conclusion against actual license text, including dual-license alternatives and modified assets/dependencies. Mark unknown or missing evidence for review. Do not infer a dependency license solely from the repository's top-level license or invent a legal conclusion.

Keep inventories and scanner experiments under .agents-workspace/tmp; put durable findings under .agents/docs. Use existing tools when available, but do not claim an automated inventory covers languages/features it never inspected.

Do not copy Cornus-specific scanners, npm baselines, notices, or distribution policy into this project. Do not add CI release blockers, relicense files, remove dependencies, or publish artifacts merely because an audit ran. Make only changes within the requested scope.

Report the exact configuration, source versions, confirmed licenses, unknowns, and notice follow-ups. Preserve rust/native attribution and the editor GPL license.
