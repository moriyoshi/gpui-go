# Native module metadata proxy

This Cloudflare Worker lets stock Go discover native modules whose ZIP files are uploaded to public GitHub Releases, and serves the user documentation built from [docs/](../../docs/index.md) from the same domain. It serves small metadata responses and redirects ZIP requests to GitHub. It never downloads, executes, or relays the binary payload itself. The Go/Rust runtime architecture is described in the root [architecture document](../../ARCHITECTURE.md).

Nothing about individual releases is checked in. The module list comes from `modulePrefix`, `repository` and `targets` in [native-targets.json](../native-targets.json), which the Worker bundles. Each version's metadata is read from that release's own `native-release-<goos>-<goarch>.json` asset, and the root `go.mod` requires the published modules.

The [publication guide](../README.md) covers producing module ZIPs, preparing draft GitHub Releases and independent Bao signing.

## Release metadata

For a request about version `vX.Y.Z` of `<modulePrefix>/<goos>/<goarch>`, the Worker fetches

```text
https://github.com/<repository>/releases/download/vX.Y.Z/native-release-<goos>-<goarch>.json
```

and serves the version only if that file is schema-1 metadata for exactly that target, module path, repository and version, with tag `vX.Y.Z` and asset `gpui-native-vX.Y.Z-<goos>-<goarch>.zip`. The release workflow writes the file (`release:module`), with the shapes in [types.ts](types.ts). Release v0.0.1's file for darwin/arm64 is:

```json
{
  "schema": 1,
  "target": "darwin/arm64",
  "entry": {
    "path": "gpui-go.dev/__binary/darwin/arm64",
    "repository": "moriyoshi/gpui-go",
    "versions": {
      "v0.0.1": {
        "time": "2026-11-10T08:53:52Z",
        "tag": "v0.0.1",
        "asset": "gpui-native-v0.0.1-darwin-arm64.zip",
        "goMod": "module gpui-go.dev/__binary/darwin/arm64\n\ngo 1.26.10\n"
      }
    }
  },
  "zip": { "name": "gpui-native-v0.0.1-darwin-arm64.zip", "sha256": "40d1cc42...", "size": 17693760 }
}
```

`goMod` holds the exact bytes of the published module's `go.mod`, so an old version keeps serving its own `go.mod` after a later release changes the template, as Go's checksum database requires. The Worker checks the module declaration but does not validate the entire Go module file or inspect the ZIP. `time` is the fixed publication time in UTC RFC 3339 form. Versions must be canonical semantic release versions, including optional prerelease identifiers. Versions v2 and later would require a matching `/vN` module suffix. Pseudo-versions, abbreviated versions, branch names and commit queries are not supported.

Only `NATIVE_VERSION` is listed. The deploy workflow sets this Worker variable to the release it advertises, and `@v/list` and `@latest` report that version alone. Any other published version still resolves when requested explicitly, as with `go get <module>@v0.0.1`, so a pinned `go.mod` keeps working. Without `NATIVE_VERSION`, as under a bare `wrangler dev`, the list is empty and `@latest` is missing.

The release ZIP must use the Go module ZIP format, with entries under `<module>@<version>/`. Go limits both compressed and total uncompressed module size to 500 MiB.

The Worker's answers are only as stable as the release assets. Treat every published version as immutable: never replace its ZIP or metadata, and use a new version for changed contents. Enable GitHub immutable releases, which enforces this.

## Routes

For `gpui-go.dev/__binary/darwin/arm64`, with `NATIVE_VERSION=v0.0.1`:

| Request | Response |
| --- | --- |
| `/__binary/darwin/arm64?go-get=1` | HTML with `go-import` pointing to `https://gpui-go.dev/proxy` using the `mod` transport |
| `/__binary/darwin/arm64/bindings?go-get=1` | The same discovery prefix for subpackages |
| `/proxy/gpui-go.dev/__binary/darwin/arm64/@v/list` | `v0.0.1` |
| `/proxy/gpui-go.dev/__binary/darwin/arm64/@latest` | Version/time JSON for `v0.0.1` |
| `/proxy/gpui-go.dev/__binary/darwin/arm64/@v/v0.0.1.info` | Version/time JSON from the release metadata |
| `/proxy/gpui-go.dev/__binary/darwin/arm64/@v/v0.0.1.mod` | The original `go.mod` from the release metadata |
| `/proxy/gpui-go.dev/__binary/darwin/arm64/@v/v0.0.1.zip` | HTTP 302 to `https://github.com/moriyoshi/gpui-go/releases/download/v0.0.1/gpui-native-v0.0.1-darwin-arm64.zip` |

The production prefix is the domain root, so module paths and documentation paths share one namespace. `/proxy/` and any path under a module go to the metadata handler, as does any method other than GET and HEAD. Everything else goes to the VitePress output in `docs/.vitepress/dist`, including its 404 page for missing paths. Keep documentation pages under paths such as `/guide/`, because a module route like `/__binary/darwin/arm64` would hide a page with the same path. The native modules all sit under `/__binary/`.

Prerelease versions with uppercase ASCII use Go's `!` case encoding in proxy requests. The Worker supports GET and HEAD and returns 405 for other methods. It returns uncached 404 responses for unknown modules, malformed versions, versions without release metadata and unsupported routes, and an uncached 502 when the metadata cannot be fetched or fails validation, with the reason in the body; it never redirects a version it has not validated. A network error, 429 or 5xx from GitHub is retried three times over about four seconds first, because the public Go mirror caches a 502 as a failed version for a while. Workers observability is enabled in `wrangler.jsonc`, so the logged reason for each 502 is kept in the Cloudflare dashboard. Malformed versions are rejected without a fetch. Release asset downloads need no GitHub API call or token, and the Worker needs no KV namespace or storage binding. A validated release is kept in memory for the isolate's lifetime, and misses and failures are retried. Versioned metadata and redirects are cached for one year; discovery, lists and latest responses for five minutes.

## Check and deploy

Use Node.js 22.18 or newer. Install the pinned development dependencies from the lockfile, then type-check the Worker, scripts and tests and run the tests:

```sh
cd dist/metadata-proxy
npm ci
npm run check
npm test
```

`npm run check` runs TypeScript with strict checks and no output files. `npm test` uses Node's native TypeScript stripping, so no compiled JavaScript or test loader is needed. The Worker itself has no runtime package dependencies. Build the documentation first, because Wrangler uploads its output as static assets (`npm ci --prefix ../../docs && npm run build --prefix ../../docs`). Use Cloudflare Wrangler 4 to bundle the TypeScript entry point and run or deploy the Worker:

```sh
wrangler dev --var NATIVE_VERSION:v0.0.1
wrangler deploy --dry-run
```

The checked-in Wrangler configuration also enables a `workers.dev` address for inspection. Production uses the custom domain `gpui-go.dev`, whose hostname must match `modulePrefix`. Pass it and the advertised version on every deployment, so the domain stays attached and `@latest` stays set, which is what the workflow below does. A `--var` lasts only for the deployment that sets it:

```sh
npm run --silent release:select -- v0.0.1
wrangler deploy --domain gpui-go.dev --var NATIVE_VERSION:v0.0.1
```

`release:select` with a version checks that every target's metadata is valid and every ZIP is downloadable, then prints the version. Without one it reads release tags on standard input and picks Go's `@latest` among them first.

Discovery requires that hostname; proxy routes can also be exercised through the development address. A consumer can use ordinary `go get gpui-go.dev/__binary/darwin/arm64@v0.0.1`, though GPUI-go's own `go.mod` already requires the modules. No custom GOPROXY setting is required for a publicly discoverable module with normal Go proxy/checksum availability. Validate the deployed discovery, public module mirror and checksum database before advertising a release. Keep Go checksum verification enabled.

## Continuous deployment

[Deploy metadata proxy](../../.github/workflows/deploy-metadata-proxy.yml) runs when a release is published, on pushes to `main` that change the Worker, the documentation or the target table, and on manual dispatch. It always deploys the default branch, including for a release event. It checks the Worker, builds the documentation, bundles both with the Wrangler pinned in [deploy/](deploy/package.json), selects the version to advertise, deploys to the host named by `modulePrefix` in [native-targets.json](../native-targets.json) with `NATIVE_VERSION` set to it, then runs `release:check-deployment` against the live domain. The version is the dispatch input `version` when given, and otherwise Go's `@latest` among the published, non-draft releases: the highest stable version, or the highest prerelease while there is none. Every deploy recomputes it, so a docs-only push keeps the current version, and publishing a release is the only step needed to advertise it. The selection fails, and nothing deploys, if any target's metadata or ZIP is missing from that release. Nothing in the repository changes. While `modulePrefix` is a placeholder it stops after bundling.

Setup, once per repository:

1. Keep `modulePrefix` on a domain whose zone is in your Cloudflare account, and run `make generate-nativeshim` after changing it.
2. Create a GitHub environment named `metadata-proxy`, with the secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. The token needs Workers Scripts edit, and Workers Routes and DNS edit on that zone so Wrangler can attach the custom domain. A release event runs with the release tag as its ref, so if the environment restricts deployment branches, also allow the version tags.
3. Enable immutable releases in the repository settings. The Worker serves whatever the release assets say.

One module ZIP always has the same contents regardless of request headers or query parameters. Putting every platform in one module downloads them all. Separate modules keep an ordinary `go build` to the target's module, but `go get` and `go mod tidy` resolve imports under every build tag, so GPUI-go consumers download every platform's module on tidy.

This Worker handles distribution, not independent artifact authorization. Stock Go checks module hashes; it does not run Bao's proposed additional signature verifier. Signature keys and the consumer's trust policy must be managed independently of the GitHub publishing account.

References: [Go proxy protocol](https://go.dev/ref/mod#goproxy-protocol), [proxy-backed module discovery](https://go.dev/ref/mod#serving-modules-directly-from-a-proxy), [module ZIP format](https://go.dev/ref/mod#zip-files), [GitHub immutable releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases), [Cloudflare Wrangler configuration](https://developers.cloudflare.com/workers/wrangler/configuration/).
