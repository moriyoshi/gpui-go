# Native module publication

The [native release workflow](../.github/workflows/native-release.yml) builds, for every target in [native-targets.json](native-targets.json):

1. a consolidated static library holding the GPUI backend and the Sable runtime (`.a`, or `.lib` on Windows);
2. Bao-generated `.syso` objects and typed Go/assembly bindings, translated from that library;
3. a Go module ZIP containing the output of step 2, as a per-platform package.

It can upload them to a draft GitHub Release with per-asset checksums and a combined `SHA256SUMS`. The [metadata Worker](metadata-proxy/README.md) later serves module metadata and redirects Go module ZIP requests to those assets. Binaries are generated in the workflow and are not committed to Git. The opt-in GPL editor is not built or packaged. See the root [architecture](../ARCHITECTURE.md).

## Target table

`native-targets.json` is the single list of targets and the module prefix. The workflow matrix, asset names, the generated function table in [internal/nativeshim](../internal/nativeshim) and the registration packages under [prebuilt](../prebuilt) all derive from it. After changing it, run `make generate-nativeshim`; a default `go test` fails if the generated files are stale.

`modulePrefix` is `gpui-go.dev/__binary`, and each module path appends `<goos>/<goarch>`, as in `gpui-go.dev/__binary/darwin/arm64`. Consumers never import these modules directly; `host` reaches them through the registration packages. The release job refuses a placeholder prefix when `create_draft` is true.

| Target | Rust target | Library | Status |
| --- | --- | --- | --- |
| `darwin/arm64` | `aarch64-apple-darwin` | `.a` | Passes the hosted job; headless and native window smoke through the shim pass locally |
| `darwin/amd64` | `x86_64-apple-darwin` | `.a` | Passes the hosted job |
| `linux/arm64` | `aarch64-unknown-linux-gnu` | `.a` | Passes the hosted job, headless only; a native X11 window is unverified |
| `linux/amd64` | `x86_64-unknown-linux-gnu` | `.a` | Passes the hosted job, headless only; a native X11 window is unverified |
| `windows/amd64`, `windows/arm64` | `x86_64-pc-windows-gnu`, `aarch64-pc-windows-gnullvm` | `.lib` | Translates and links; not yet run on a hosted Windows runner |

With `targets: all` a failing target blocks the release job, so select a working subset until each target passes.

Bao options for each target are in [bao.config.json](bao.config.json). Windows names LLVM-MinGW import libraries and caps three 64-byte read-only data sections at 32 bytes with `max_data_alignment`, because Go's PE reader cannot provide more; whether that code relies on the larger alignment has not been audited.

Linux needs a prelink before Bao. `release:syso` prelinks the library to remove unreachable code, merge per-function sections and fold function aliases, because the archive has 334,676 sections and Go's ELF loader and function table have limits. The config also binds `libxcb`, `libxkbcommon` and `libxkbcommon-x11` dynamically, so consumers need those system libraries at run time.

Windows uses the GNU ABI because Bao does not support MSVC Rust targets. The `.lib` is a GNU-ABI COFF archive for LLVM-MinGW/MinGW linkers, not for MSVC `link.exe`.

Each target is built on its own runner. On macOS, `release:syso` rebinds several framework imports through `Cocoa` and `ApplicationServices` and fails if the package needs more than 14 dylibs: stock Go's Mach-O binder corrupts dylib ordinals of 16 or more, and a consumer reaching package `net` adds one. Bao's broader target list (LoongArch, PowerPC) is not enabled because GPUI has not been checked there.

## Prepare a candidate

Run `Prepare native release` manually from GitHub Actions, selecting a source revision that contains the desired workflow and release tooling. Provide:

| Input | Meaning |
| --- | --- |
| `version` | Canonical native module version, initially `v0.x.y` or `v1.x.y`, optionally a prerelease |
| `targets` | `all`, or a comma-separated subset such as `darwin/arm64,linux/amd64` |
| `bao_revision` | Exact 40-character commit in `moriyoshi/bao`; defaults to v0.0.3's `e05f1cfb2d2054457067f3268908992ca9b1df2c` |
| `create_draft` | Defaults to false, retaining Actions artifacts only; true creates a new draft Release |

The workflow pins third-party Actions to commit hashes, disables persisted checkout credentials, and gives only the release job permission to write releases. It never replaces an existing release or uses `latest` as a producer pin. Publication scripts check that the running Go version and Bao's verified linker profile are Go 1.26.10 and that Bao reports the selected commit. The shared pin is in `metadata-proxy/scripts/profile.ts`.

Jobs and their scripts, each under `dist/metadata-proxy`:

| Job | Script | Result |
| --- | --- | --- |
| `plan` | `release:targets` | Type-checks tools, checks the proxies are current, selects the matrix |
| `library` | `release:archive` | Static library, Cargo metadata |
| `syso-and-module` | `release:syso` | `-syso.zip` of the generated package |
| `syso-and-module` | `release:module` | Go module ZIP, publication record, release metadata |
| `syso-and-module` | `release:smoke` | Module download, checks and execution; host run through the shim |
| `release` | `release:collect` | `SHA256SUMS`, release notes and a check of every target's metadata; optional draft |

`release:smoke` downloads the actual ZIP through a local Go file proxy into a fresh module cache, runs `go mod verify`, and builds with `CGO_ENABLED=0` and unavailable C/C++/Rust compiler paths. It runs the [native initialization smoke](native-smoke/main.go.tmpl) at GOMAXPROCS 1 and 4, and on macOS signs the consumer ad hoc with Hardened Runtime and runs it again. Published libraries are built with the Rust `automation` feature. The smoke links `examples/uitest` with `-tags gpui_prebuilt,gpui_e2e` on every target and runs it on Linux, where GPUI needs no display. It also builds `examples/counter` with `-tags gpui_prebuilt,gpui_headless` against the same module and runs its headless smoke through `internal/nativeshim`. Apart from that Linux run it does not verify a native window loop or input, and nowhere does it verify rendered pixels or complete background TLS drainage.

The local file-proxy check disables the public checksum database only inside its subprocess environment, and only for the unpublished module, because that module cannot be in the database yet. Keep checksum verification enabled for production consumers.

For a local rehearsal, run each script with `--source-sha unverified-local`, which skips the comparison with `HEAD` and is recorded as such in `publication.json`. The release job always passes the real commit.

## Artifacts

Each target contributes:

```text
gpui-native-vX.Y.Z-<goos>-<goarch>.a          (.lib on Windows)
gpui-native-vX.Y.Z-<goos>-<goarch>-syso.zip
gpui-native-vX.Y.Z-<goos>-<goarch>.zip
native-release-<goos>-<goarch>.json
```

Each of the first three has a `.sha256` sidecar. `SHA256SUMS` lists every asset other than the sidecars and is checked with `sha256sum --check --strict` before the draft is created; the release job also refuses sidecars that disagree with it. Checksums detect corruption and substitution after the fact; they are not signatures.

The metadata JSON contains the module path, exact original `go.mod`, fixed publication time, GitHub asset mapping, ZIP size and SHA-256. The Worker reads it back from the published release to serve the version, so it is part of the published module's identity and must never be replaced. The module ZIP includes Bao's binding declarations/provenance and a `publication.json` recording the source commit, Bao commit/executable hash, library hash and toolchain versions. These records describe the producer's build; they do not independently prove provenance or source correspondence.

Bao's build log and native execution logs are retained in a separate `native-evidence-*` artifact, and `native-release-<version>` holds the candidate with its notes. Nothing reaches the Worker until the release is published.

## Consume through the shim

`internal/nativeshim` exposes one function set to `host`. With `-tags gpui` it links a locally built Rust archive through cgo, as the Makefile targets do. With `-tags gpui_prebuilt` and `CGO_ENABLED=0` it forwards each call through a table that a registration package fills. `github.com/moriyoshi/gpui-go/prebuilt/<goos>/<goarch>` imports that target's module and registers it from `init`; its registration file is constrained to the target, so the package is empty elsewhere. `github.com/moriyoshi/gpui-go/prebuilt` imports all six, and `host` imports it in a file constrained to `gpui_prebuilt`, so the tags alone select the module. The shim cannot import the registration packages itself, because they import the shim.

`go mod tidy` and `go get` load imports as if every build tag were set, so any consumer that imports `host` downloads all six modules on tidy, including source-build consumers. This is the cost of not asking programs for a platform-specific import. An ordinary `go build` downloads only the current target's module. On a GOOS/GOARCH without a module, a prebuilt build compiles and `host.Run` returns an error.

The root `go.mod` requires all six modules, which raises the repository's Go version to the Bao profile's 1.26.10. A consumer gets the native version that its gpui-go version requires, not whatever is latest. After publishing a release, update the requirements with `go get gpui-go.dev/__binary/<goos>/<goarch>@<version>` for each target. `release:smoke` overrides the requirement for the module it builds with a `-modfile`, because the new version is not resolvable before publication.

## Sign and publish

The workflow intentionally creates drafts so the existing independent signing design can be completed before an immutable release is published. No publisher key is stored in this repository or generated by the workflow. On a separately trusted producer/signing environment, inspect the candidate, extract the Go module ZIP without changing its contents, and use the independently obtained Bao CLI:

```sh
bao sign -root /reviewed/gpui-go.dev/__binary/darwin/arm64@v0.1.0 \
  -module gpui-go.dev/__binary/darwin/arm64 \
  -target darwin/arm64 -release v0.1.0 \
  -key /secure/release-key.pem -o /release/bao-release-darwin-arm64.json
```

Replace the example module/version/paths with the actual candidate. Keep the envelope and private key outside the signed module tree. Consumers obtain their public keys and exact payload approvals through an independent trusted channel. `bao-release-darwin-arm64.json` is the agreed asset naming convention; Bao accepts explicit manifest paths and does not require that filename. Its signed payload authenticates the complete extracted module, including generated Go/assembly/native objects and provenance files.

Before finalizing the draft, review the actual native dependency and embedded-asset license obligations and include the remaining required texts/notices. The current preparation includes known project/GPUI/Bao/bootstrap notices and raw Cargo metadata, not a completed transitive notice bundle or license audit. If module contents change during this review, regenerate the ZIP, SHA-256 and metadata before signing. Draft replacement is a deliberate manual operation; the workflow refuses to overwrite release assets automatically.

Upload the independently signed envelope to the draft and finish artifact review before publishing with GitHub immutable releases enabled. Immutable releases cannot be extended with new signing assets afterward. The publication workflow does not itself finalize a draft.

## Advertise the release

Publishing a Release runs [Deploy metadata proxy](../.github/workflows/deploy-metadata-proxy.yml). It picks Go's `@latest` among the published releases, checks that every target's metadata and ZIP are downloadable, deploys the Worker with `NATIVE_VERSION` set to that version and checks the live routes. Nothing is committed and no pull request is opened. Publishing a prerelease while a stable release exists leaves `@latest` on the stable one, but the prerelease still resolves when requested by version. To roll `@latest` back or forward, dispatch the workflow with `version`.

Draft assets are never advertised, because unauthenticated clients, the Worker included, cannot download them. Test the public Go mirror and checksum database with an empty consumer cache before advertising a version, because the deployment check covers the Worker and GitHub only. See the [Worker guide](metadata-proxy/README.md#continuous-deployment) for setup.

Plain `go get` obtains the ZIP and Go checksum authentication. Additional Bao signature verification remains explicitly consumer-controlled and is not invoked automatically by Go. Its current restrictive `verified-build` profile has no `-tags` option, so verification of GPUI's existing tagged host build remains pending. The generated native module itself needs no GPUI build tags.

One module ZIP always has the same contents for its module/version. Per-target modules keep an ordinary `go build` to one download, while `go mod tidy` and `go get` fetch every target.
