# Documents for humans and coding agents

* [README.md](README.md): public APIs, build commands, examples, and supported behavior.
* [ARCHITECTURE.md](ARCHITECTURE.md): canonical subsystem map, ownership, and invariants.
* [Rust editor embedding guide](rust/zed-editor/README.md): reusable component and standalone host.

# Documents for coding agents

* [Overview](.agents/docs/OVERVIEW.md): repository orientation and current scope.
* [Quality gate](.agents/docs/QUALITY_GATE.md): verification required for the affected layer.
* [Testing](.agents/docs/TESTING.md): test layers, native prerequisites, and commands.
* [Journal](.agents/docs/JOURNAL.md): append-only findings, decisions, and verification evidence.
* [Long-term memory index](.agents/docs/LTM/INDEX.md): durable topic references and syntheses.
* [TODO](.agents/docs/TODO.md): open work; verify each item against current code before acting.
* [Harness provenance](.agents/docs/HARNESS.md): upstream revisions and intentional adaptations.
* [Repository skills](.agents/skills/): maintenance, quality, testing, and audit workflows. Read the applicable `SKILL.md` when using one; the provenance document lists their origins.

# Rules and protocols

## Scope and ownership

* Read the overview and relevant architecture section before changing subsystem boundaries. Read the testing guide before modifying the protocol, runtime, input handling, or editor lifecycle.
* Keep reusable library behavior in the libraries. Application presentation, sample content, and smoke scenarios belong under `examples/`.
* Keep Go/Rust boundaries as versioned protobuf messages and opaque IDs. Never pass Go pointers or synchronous Go callbacks into GPUI rendering.
* GPUI runs on the process main thread. Native entities own focus, selection, composition, and undo history. Do not replace persistent editor entities on every Go update.
* Keep the GPL Zed editor component and its host opt-in and separate from the Apache GPUI backend. Preserve matching pinned Zed and Sable revisions across languages. Do not run an unscoped `cargo update`.

## File management

* Put durable work summaries in `.agents/docs/`, preferably the existing journal or relevant LTM topic. Put scratch scripts, logs, inventories, and investigation checkouts in `.agents-workspace/tmp/`.
* Existing build locations are deliberate exceptions: Go executables in ignored `bin/`, Rust artifacts in ignored `target/`, and local caches in ignored `.cache/`. Use the existing Makefile targets; do not scatter binaries at the repository root or create a new target directory per session.
* Preserve unrelated edits and user files. Editing, moving, or removing files within the user's requested change is authorized by that request; it is not permission to discard other work. Clean only scratch artifacts you created, and ask before deleting unrelated user data.
* Investigative code belongs in scratch space unless it becomes a maintained test, tool, or public feature. Record the finding and its reproduction, not an unused production API for the experiment.

## Building and verification

* Use the toolchains pinned in `go.mod` and `rust-toolchain.toml`, plus the generator versions in the Makefile. Discover installed tools; do not assume another project's tool installation paths.
* Format changed Go files with `gofmt`. Use `cargo fmt --manifest-path ...`; bare `rustfmt` can follow module declarations into unrelated files.
* Follow the scope-specific [quality gate](.agents/docs/QUALITY_GATE.md). Default Go tests stay independent of displays, native archives, and network services. Native validation is explicit.
* The Rust crates are separate Cargo projects; the editor component and its `host/` member form their own workspace. A root-level `cargo test` cannot validate this repository. Use explicit manifest paths and `--locked`.
* Go's cache does not track changes inside an external Rust archive. Preserve forced relinking in the editor build target; do not cite a cached executable as evidence for a new native build.
* Read the implementation and identify the failure class before adding a test. Prefer an existing test layer that observes the behavior. Check how a test could pass while the claim is false. When sensitivity is uncertain, use an isolated negative control; a compile error does not prove runtime behavior.
* Distinguish headless protocol tests, native command smoke tests, and actual mouse/keyboard/IME testing. Report exactly what ran and any unavailable layer. Do not weaken assertions or label skipped checks as passed.

## Git and collaboration

* Do not use `git checkout` or `git restore` to discard working-tree changes. Commit or push only when explicitly requested.
* Some workspaces are source exports without `.git`. Inspect available files and report that limitation; do not initialize a repository merely to run a check.
* Delegate only when requested or otherwise authorized by applicable instructions. Skills describe workflows, not additional permission to spawn agents, publish, send messages, or mutate external services.

## Documentation and memory

* Keep user workflows in README and the editor embedding guide, architecture in root `./ARCHITECTURE.md`, and operational agent knowledge under `.agents/docs/`. Link directly to the root architecture document.
* Append to `JOURNAL.md`; do not rewrite earlier entries during ordinary work. The `reconcile-journal-ltm` skill is the explicit maintenance exception after coverage has been verified.
* Maintain `LTM/INDEX.md` when adding or reorganizing topic notes. Keep unresolved work in TODO, with evidence and completion criteria; a backlog entry is not authorization to implement it.
* Use ASCII parentheses and colons in authored documentation, with ordinary surrounding spaces in prose. Avoid emoji as documentation markers. Preserve exact code, URLs, Unicode input examples, and test fixtures, including intentionally decomposed sequences; do not normalize or remove their data to satisfy a prose convention.

## Shell pitfalls

* The interactive zsh environment may alias copy/remove commands or enable `NO_CLOBBER`. Prefer patch tools or explicit Python file writes for repository edits. Inspect aliases/options if a command behaves unexpectedly.
* Do not delete an existing file merely to work around redirection, and do not use a broad process kill to refresh a binary. Respect running applications and unrelated sessions.
