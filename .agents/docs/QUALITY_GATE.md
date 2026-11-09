# Quality gate

Choose checks by the affected layer and record actual results. A docs-only change does not require native builds. Pure Go mocks cannot prove native editor behavior.

## Scope matrix

| Change | Required evidence |
| --- | --- |
| Harness/docs | `make harness-check`; review factual claims and commands against code |
| Go API/example | Format touched files; `go build ./...`; `go vet ./...`; `go test -race ./...` |
| Public value aliases | Existing generated callers remain assignable; optional zero/false presence and serialization survive; GoX typed attributes accept public aliases; generic examples contain no generated/helper imports |
| Reactive graph/node bindings | Dynamic dependencies and memo equality, diamond/batch behavior, failure recovery and owner cleanup; keyed identity/disposal and input epochs; affected native reactive scenario for native preservation claims |
| GoX template/tooling | Pinned runtime/generator agreement; source formatting and isolated generation check; installer regressions; compile checked-in output without generator/native dependencies; affected native reactive scenario |
| Generic protocol/validation | Regenerate bindings; core tests; Go contract tests; capability/validator agreement |
| Mailboxes/events/shutdown | Runtime tests and `make integration`; affected cancellation/overflow/release cases |
| Native rendering/input | Native build and affected smoke; `make test-native` for GPUI dispatch; report OS input testing separately |
| Starlark E2E | Harness race tests, `make e2e-check`, tagged Go tests, both native configurations, affected native scenarios and the deliberately failing assertion control |
| Shared UI testing | Public Go API/process/locator tests; dependency graph excludes Starlark; Go-only and Starlark consumers exercise the same core; strictness and timeout controls |
| Editor component/host | Component-only check; workspace Clippy; `make editor-smoke`; affected Go editor tests |
| Dependencies/build tags | Locked affected builds, shared revision agreement, actual configuration/license review |
| Package boundaries | Root facade interoperability and tagged transitive dependency checks; migrated client/scene tests; Sable integration/native consumers when startup or drivers move |

Use focused tests while iterating. Broaden checks when they observe another affected layer; do not repeat passing checks without a relevant change or unresolved concern.

## Go gate

Format changed files with `gofmt -w`; `gofmt -l` on them must produce nothing. Run build, vet, and race tests above. Keep default tests independent of a display, native archives, external services, and network once dependencies are cached. Test observable contracts, not a restatement of the implementation.

## Rust gate

There is no root Cargo manifest. Select the affected project:

```sh
cargo test --locked --manifest-path rust/core/Cargo.toml
cargo test --locked --manifest-path rust/runtime/Cargo.toml
cargo test --locked --manifest-path rust/native/Cargo.toml --lib
cargo clippy --locked --manifest-path rust/native/Cargo.toml --all-targets -- -D warnings
cargo check --locked --manifest-path rust/zed-editor/Cargo.toml
cargo clippy --locked --manifest-path rust/zed-editor/Cargo.toml --workspace -- -D warnings
```

Use `cargo fmt --manifest-path <affected-manifest> --check`, with `--all` for the editor workspace. `--workspace` on editor Clippy matters: the host is a separate member. Do not blindly copy an all-features gate that enables unavailable platform/shader configurations; name the supported features actually checked.

Keep lockfiles and compatibility patches; avoid unscoped dependency updates. Review FFI panic containment at the actual foreign callback boundary, main-thread execution, ownership, and shutdown ordering.

## Behavioral review

Read the [architecture invariants](../../ARCHITECTURE.md#invariants). Preserve request correlation, ID scope, stale-handle rejection, limits, validation-before-mutation, increasing revisions, generic event acknowledgement, and input epoch/edit-revision rules. Editor events have no replay guarantee. Editor buffers/undo histories must remain independent, closing one must not quit the host, and reopening after the last close must work.

Protobuf declarations do not prove capability implementation. Native command smoke does not prove mouse, key routing, composition, accessibility, or clipboard behavior. Report automated/manual coverage precisely.

Regenerate via `make generate`. `make check-generated` requires Git. For a source export without `.git`, generate into isolated output under `.agents-workspace/tmp/` and compare bytes with `gen/`; do not claim a Git diff passed.

## Documentation and completion

Run `make harness-check` for harness changes. It checks local link targets, required documents, skill frontmatter, and prose punctuation, not arbitrary Markdown fragment slugs, external URLs, or runtime claims.

Append durable findings and evidence to [JOURNAL](JOURNAL.md); track unresolved follow-ups in [TODO](TODO.md) and maintain [LTM](LTM/INDEX.md). Report passed, failed, and not-run checks separately. Fix introduced failures and state environmental limits without counting skipped checks as passed.
