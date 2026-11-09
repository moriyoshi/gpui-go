# Agent harness provenance

Adapted on 2026-11-07 from default-branch snapshots fetched from GitHub:

| Repository | Revision | Source material |
| --- | --- | --- |
| [moriyoshi/cornus](https://github.com/moriyoshi/cornus/tree/df47166a8739e02efa808d17f9e32ae82f1d57db) | `df47166a8739e02efa808d17f9e32ae82f1d57db` | AGENTS.md, .agents/docs/, .agents/skills/, .agents-workspace/.gitignore |
| [moriyoshi/yesnodb](https://github.com/moriyoshi/yesnodb/tree/de5d42acf3ef6625df9816688d929bfd0a3f4def) | `de5d42acf3ef6625df9816688d929bfd0a3f4def` | AGENTS.md, .agents/docs/, .agents/skills/ |

A local sibling Cornus checkout was inspected too; the fetched revisions above are the adoption sources. Source checkouts in ignored .agents-workspace/tmp/harness-sources/ are scratch, not maintained harness content.

## Structure

Root AGENTS.md routes work to overview, architecture, testing, quality gate, journal, TODO, and LTM. Root `./ARCHITECTURE.md` is canonical, following Cornus; architecture links point directly to that document. Journal is append-only outside explicit reconciliation. LTM distinguishes source topics and syntheses. TODOs require re-verification against code.

Skills live in .agents/skills/. Scratch tmp/worktrees live in ignored .agents-workspace/. Existing bin/target/cache directories remain supported to preserve Makefile behavior and avoid expensive rebuilds. The stdlib checker at .agents/scripts/check_harness.py validates the structure via make harness-check, which also runs in the protocol CI job.

## Skills and origins

| Skill | Source | Local adaptation |
| --- | --- | --- |
| good-sleep | Both | Journal to topic memory; unresolved work to TODO |
| deep-sleep | Both | Synthesis preserves source topics |
| distill-memories | Both | Promote verified facts to README, root architecture, overview, gates |
| reconcile-journal-ltm | Both | Audit coverage before explicit journal compaction |
| tackle-todos | Both | Scan Go/Rust/protobuf, verify stale work, respect delegation authorization |
| quality-gate | yesnodb | Layered Go/Rust/protocol/native gate |
| write-tests | yesnodb | Observable failure classes and isolated negative controls |
| assess-doc-quality | Cornus | Repository links/APIs/commands/evidence, without a website toolchain |
| audit-licenses | Cornus | Go/Rust configurations, preserving the explicitly chosen GPL editor path |

## Intentional adaptations

These are project-specific rewrites, not imported project histories. No database/kernel invariants, OCI deployment policies, prior audit baselines, notices, or upstream TODOs are copied.

Cornus's web-screenshot and translate-documents skills target its SPA/multilingual documentation site, which this native library does not have. They are not installed. No site glossary, frontend package manager, Claude wrapper, or browser workflow is invented.

Skills preserve the user's existing authorization and current execution/delegation constraints. They do not require reconfirmation for already requested routine work or authorize unrelated implementation, publication, external messages, or automatic agents.

Prose uses ASCII punctuation and avoids icon markers. Unlike blanket upstream normalization/emoji rules, this does not alter Unicode fixtures or examples: combining sequences, Japanese IME, and emoji are meaningful inputs here. License review reports the opted-in GPL configuration instead of treating it as an accidental forbidden dependency.
