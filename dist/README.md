# Native module publication

The [native release workflow](../.github/workflows/native-release.yml) builds, for every target in [native-targets.json](native-targets.json):

1. a consolidated static library holding the GPUI backend and the Sable runtime (`.a`, or `.lib` on Windows);
2. Bao-generated `.syso` objects and typed Go/assembly bindings, translated from that library;
3. a Go module ZIP containing the output of step 2, as a per-platform package.

It can upload them to a draft GitHub Release with per-asset checksums and a combined `SHA256SUMS`. The [metadata Worker](metadata-proxy/README.md) later serves module metadata and redirects Go module ZIP requests to those assets. Binaries are generated in the workflow and are not committed to Git. The opt-in GPL editor is not built or packaged. See the root [architecture](../ARCHITECTURE.md).

## Target table

`native-targets.json` is the single list of targets and the module prefix. The workflow matrix, asset names and the generated proxies in [internal/nativeshim](../internal/nativeshim) all derive from it. After changing it, run `make generate-nativeshim`; a default `go test` fails if the proxies are stale.

`modulePrefix` is `gpui-go.dev`, so modules sit at the domain root, as in `gpui-go.dev/darwin-arm64`. The release job refuses a placeholder prefix when `create_draft` is true.

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
| `bao_revision` | Exact 40-character commit in `moriyoshi/bao`; defaults to v0.0.2's `4ca57fab0ddfd368c8ea89b83bf355682671ee27` |
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
| `release` | `release:collect` | `SHA256SUMS`, release notes, catalog candidate; optional draft |

`release:smoke` downloads the actual ZIP through a local Go file proxy into a fresh module cache, runs `go mod verify`, and builds with `CGO_ENABLED=0` and unavailable C/C++/Rust compiler paths. It runs the [native initialization smoke](native-smoke/main.go.tmpl) at GOMAXPROCS 1 and 4, and on macOS signs the consumer ad hoc with Hardened Runtime and runs it again. Published libraries are built with the Rust `automation` feature. The smoke links `examples/uitest` with `-tags gpui,gpui_prebuilt,gpui_e2e` on every target and runs it on Linux, where GPUI needs no display. It also builds `examples/counter` with `-tags gpui,gpui_prebuilt,gpui_headless` against the same module and runs its headless smoke through `internal/nativeshim`. Apart from that Linux run it does not verify a native window loop or input, and nowhere does it verify rendered pixels or complete background TLS drainage.

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

The metadata JSON contains the module path, exact original `go.mod`, fixed publication time, GitHub asset mapping, ZIP size and SHA-256. The module ZIP includes Bao's binding declarations/provenance and a `publication.json` recording the source commit, Bao commit/executable hash, library hash and toolchain versions. These records describe the producer's build; they do not independently prove provenance or source correspondence.

Bao's build log and native execution logs are retained in a separate `native-evidence-*` artifact, and `native-release-<version>` holds the candidate with its catalog and notes. The Worker catalog stays empty until an actual candidate is approved and its metadata applied.

## Consume through the shim

`internal/nativeshim` exposes one function set to `host`. With `-tags gpui` it links a locally built Rust archive through cgo, as the Makefile targets do. With `-tags gpui,gpui_prebuilt` and `CGO_ENABLED=0` it forwards each call to the module for the current `GOOS`/`GOARCH`. Only the matching file is compiled, so an unsupported platform fails at build time with the missing proxy symbols.

Those modules are not in the root `go.mod` yet, because no version has been published and the required Go version would rise to the Bao profile's 1.26.10. Until then, `go mod tidy` cannot resolve the proxy imports. After the first release, add the published modules to `go.mod`.

## Sign and publish

The workflow intentionally creates drafts so the existing independent signing design can be completed before an immutable release is published. No publisher key is stored in this repository or generated by the workflow. On a separately trusted producer/signing environment, inspect the candidate, extract the Go module ZIP without changing its contents, and use the independently obtained Bao CLI:

```sh
bao sign -root /reviewed/native.example.org/gpui/darwin-arm64@v0.1.0 \
  -module native.example.org/gpui/darwin-arm64 \
  -target darwin/arm64 -release v0.1.0 \
  -key /secure/release-key.pem -o /release/bao-release-darwin-arm64.json
```

Replace the example module/version/paths with the actual candidate. Keep the envelope and private key outside the signed module tree. Consumers obtain their public keys and exact payload approvals through an independent trusted channel. `bao-release-darwin-arm64.json` is the agreed asset naming convention; Bao accepts explicit manifest paths and does not require that filename. Its signed payload authenticates the complete extracted module, including generated Go/assembly/native objects and provenance files.

Before finalizing the draft, review the actual native dependency and embedded-asset license obligations and include the remaining required texts/notices. The current preparation includes known project/GPUI/Bao/bootstrap notices and raw Cargo metadata, not a completed transitive notice bundle or license audit. If module contents change during this review, regenerate the ZIP, SHA-256 and metadata before signing. Draft replacement is a deliberate manual operation; the workflow refuses to overwrite release assets automatically.

Upload the independently signed envelope to the draft and finish artifact review before publishing with GitHub immutable releases enabled. Immutable releases cannot be extended with new signing assets afterward. The publication workflow does not itself finalize a draft or deploy the Worker.

## Activate metadata

Publishing a Release runs [Activate native release](../.github/workflows/activate-native-release.yml). It verifies the release metadata against `SHA256SUMS`, merges it into `catalog.ts` and opens a pull request. Merging that pull request runs [Deploy metadata proxy](../.github/workflows/deploy-metadata-proxy.yml), which deploys the Worker and checks the live routes and release assets. Releases without native module metadata are ignored, and `Activate native release` can also be dispatched for an existing tag.

Do not advertise draft assets: unauthenticated Go clients cannot download them. Test the public Go mirror and checksum database with an empty consumer cache before advertising a version, because the deployment check covers the Worker and GitHub only. See the [Worker guide](metadata-proxy/README.md#continuous-deployment) for setup.

Plain `go get` obtains the ZIP and Go checksum authentication. Additional Bao signature verification remains explicitly consumer-controlled and is not invoked automatically by Go. Its current restrictive `verified-build` profile has no `-tags` option, so verification of GPUI's existing tagged host build remains pending. The generated native module itself needs no GPUI build tags.

One module ZIP always has the same contents for its module/version. Per-target module paths do not guarantee target-only downloads during `go get` or `go mod tidy`, which may load dependencies across platforms.
