# Native module metadata proxy

This Cloudflare Worker lets stock Go discover native modules whose ZIP files are uploaded to public GitHub Releases, and serves the user documentation built from [docs/](../../docs/index.md) from the same domain. It serves small metadata responses and redirects ZIP requests to GitHub. It never downloads, executes, or relays the binary payload itself. The Go/Rust runtime architecture is described in the root [architecture document](../../ARCHITECTURE.md).

The bundled [catalog](catalog.ts) starts empty. No native module versions or assets are published by this change, and GPUI-go does not yet depend on a hosted native module.

The [publication guide](../README.md) covers producing module ZIPs, preparing draft GitHub Releases, independent Bao signing and merging generated metadata into this catalog.

## Release catalog

Add entries to `catalog.ts` after uploading the corresponding module ZIPs. The catalog interfaces live in [types.ts](types.ts). This illustrative entry uses a placeholder domain and release:

```ts
import type { ModuleEntry } from "./types.ts";

const catalog: readonly ModuleEntry[] = [
  {
    path: "native.example.org/gpui/darwin-arm64",
    repository: "moriyoshi/gpui-go",
    versions: {
      "v0.1.0": {
        time: "2026-11-08T00:00:00Z",
        tag: "v0.1.0",
        asset: "gpui-native-v0.1.0-darwin-arm64.zip",
        goMod: "module native.example.org/gpui/darwin-arm64\n\ngo 1.26.10\n",
      },
    },
  },
];

export default catalog;
```

`path` is the full native module path, including the domain serving discovery. Configure a separate entry for each platform module, or one entry for a combined module. `repository` is the public GitHub owner/repository. Each version explicitly records the release `tag` and `asset`; asset names are not inferred from a client's platform. Simple tags without slashes are supported. Upload assets before advertising their versions here.

`goMod` must contain the exact bytes of the published module's `go.mod`, including dependencies, toolchain directives and retractions. The Worker checks the module declaration but does not validate the entire Go module file or inspect the ZIP. `time` is the fixed publication time in UTC RFC 3339 form. Never regenerate it from the current request time. Versions must be canonical semantic release versions, including optional prerelease identifiers. Versions v2 and later require the matching `/vN` module suffix. Pseudo-versions, abbreviated versions, branch names and commit queries are not supported.

The release ZIP must use the Go module ZIP format. For the example above, its entries start with `native.example.org/gpui/darwin-arm64@v0.1.0/`. Include the Go/assembly bindings, native objects, licenses and notices. Go limits both compressed and total uncompressed module size to 500 MiB. Files can be generated in CI and uploaded as release assets without being committed to Git.

Treat every published version as immutable: preserve its ZIP, `goMod`, publication time, tag and asset mapping across deployments. Enable GitHub immutable releases. Keep old versions available, and use a new version for changed contents. The Worker does not keep a historical database that could prevent a maintainer from replacing catalog entries.

## Routes

For the example entry, deployed on `https://native.example.org`:

| Request | Response |
| --- | --- |
| `/gpui/darwin-arm64?go-get=1` | HTML with `go-import` pointing to `https://native.example.org/proxy` using the `mod` transport |
| `/gpui/darwin-arm64/bindings?go-get=1` | The same discovery prefix for subpackages |
| `/proxy/native.example.org/gpui/darwin-arm64/@v/list` | Known versions, one per line |
| `/proxy/native.example.org/gpui/darwin-arm64/@latest` | Highest stable version, or highest prerelease when there are no stable versions |
| `/proxy/native.example.org/gpui/darwin-arm64/@v/v0.1.0.info` | Go version/time JSON |
| `/proxy/native.example.org/gpui/darwin-arm64/@v/v0.1.0.mod` | The original `go.mod` |
| `/proxy/native.example.org/gpui/darwin-arm64/@v/v0.1.0.zip` | HTTP 302 to the exact GitHub Release asset |

The production prefix is the domain root, so module paths and documentation paths share one namespace. `/proxy/` and any path under a catalog module go to the metadata handler, as does any method other than GET and HEAD. Everything else goes to the VitePress output in `docs/.vitepress/dist`, including its 404 page for missing paths. Keep documentation pages under paths such as `/guide/`, because a module route like `/__binary/darwin/arm64` would hide a page with the same path. The native modules all sit under `/__binary/`.

Module paths and prerelease versions with uppercase ASCII use Go's `!` case encoding in proxy requests. Discovery paths use their original case. The Worker supports GET and HEAD, returns 405 for other methods, and returns uncached 404 responses for unlisted modules, versions and unsupported routes. It only redirects catalog-listed releases in their configured GitHub repositories. It makes no GitHub API calls and needs no GitHub token, KV namespace or storage binding. Versioned metadata/redirects are cached for one year; discovery, lists and latest responses for five minutes.

## Check and deploy

Use Node.js 22.18 or newer. Install the pinned development dependencies from the lockfile, then type-check the Worker, catalog and tests and run the tests:

```sh
cd dist/metadata-proxy
npm ci
npm run check
npm test
```

`npm run check` runs TypeScript with strict checks and no output files. `npm test` uses Node's native TypeScript stripping, so no compiled JavaScript or test loader is needed. The Worker itself has no runtime package dependencies. Build the documentation first, because Wrangler uploads its output as static assets (`npm ci --prefix ../../docs && npm run build --prefix ../../docs`). Use Cloudflare Wrangler 4 to bundle the TypeScript entry point and run or deploy the Worker:

```sh
wrangler dev
wrangler deploy --dry-run
wrangler deploy
```

The checked-in Wrangler configuration also enables a `workers.dev` address for inspection. Production uses the custom domain `gpui-go.dev`, whose hostname must match the module paths in the catalog. Pass it on every deployment so the domain stays attached, which is what the workflow below does:

```sh
wrangler deploy --domain gpui-go.dev
```

Discovery requires that hostname; proxy routes can also be exercised through the development address. Once an actual release and domain are configured, a consumer can use ordinary `go get native.example.org/gpui/darwin-arm64@v0.1.0`. GPUI-go can later require the native module transitively so consumers retain its existing public import path. No custom GOPROXY setting is required for a publicly discoverable module with normal Go proxy/checksum availability. Validate the deployed discovery, public module mirror and checksum database before advertising a release. Keep Go checksum verification enabled.

## Continuous deployment

[Deploy metadata proxy](../../.github/workflows/deploy-metadata-proxy.yml) runs on pushes to `main` that change the Worker, its catalog, the documentation or the target table, and can be dispatched manually. It checks the Worker, builds the documentation, bundles both with the Wrangler pinned in [deploy/](deploy/package.json), deploys it to the host named by `modulePrefix` in [native-targets.json](../native-targets.json), then runs `release:check-deployment` against the live domain. While `modulePrefix` is a placeholder it stops after bundling.

Setup, once per repository:

1. Keep `modulePrefix` on a domain whose zone is in your Cloudflare account, and run `make generate-nativeshim` after changing it.
2. Create a GitHub environment named `metadata-proxy`, with the secrets `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`. The token needs Workers Scripts edit, and Workers Routes and DNS edit on that zone so Wrangler can attach the custom domain.
3. Allow GitHub Actions to create pull requests in the repository settings, so `Activate native release` can open catalog updates.

One module ZIP always has the same contents regardless of request headers or query parameters. Putting every platform in one module downloads them all. Separate modules keep downloads to the targets a consumer imports, because `go get` and `go mod tidy` only download modules reachable through an import, under any build tags.

This Worker handles distribution, not independent artifact authorization. Stock Go checks module hashes; it does not run Bao's proposed additional signature verifier. Signature keys and the consumer's trust policy must be managed independently of the GitHub publishing account.

References: [Go proxy protocol](https://go.dev/ref/mod#goproxy-protocol), [proxy-backed module discovery](https://go.dev/ref/mod#serving-modules-directly-from-a-proxy), [module ZIP format](https://go.dev/ref/mod#zip-files), [GitHub immutable releases](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases), [Cloudflare Wrangler configuration](https://developers.cloudflare.com/workers/wrangler/configuration/).
