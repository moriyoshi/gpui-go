import assert from "node:assert/strict";
import test from "node:test";
import catalog from "./catalog.ts";
import defaultWorker, { createSite, createWorker } from "./worker.ts";
import type { MetadataWorker, ModuleEntry, ModuleRelease, SiteEnv } from "./types.ts";

const path = "native.example.org/gpui/darwin-arm64";
const goMod = `module ${path}\n\ngo 1.26.5\n`;
const versions = {
  "v0.2.0": { time: "2026-11-01T00:00:00Z", tag: "v0.2.0", asset: "gpui-darwin-arm64.zip", goMod },
  "v0.10.0": { time: "2026-11-02T00:00:00Z", tag: "v0.10.0", asset: "gpui-darwin-arm64.zip", goMod },
  "v1.0.0-rc.1": { time: "2026-11-03T00:00:00Z", tag: "v1.0.0-rc.1", asset: "gpui-darwin-arm64.zip", goMod },
} satisfies Record<string, ModuleRelease>;
const entry: ModuleEntry = { path, repository: "moriyoshi/gpui-go", versions };
const worker = createWorker([entry]);

function request(worker: MetadataWorker, suffix: string, method = "GET"): Response {
  return worker.fetch(new Request(`https://native.example.org${suffix}`, { method }));
}

test("module and subpackage discovery point Go at the proxy", async () => {
  for (const suffix of ["/gpui/darwin-arm64?go-get=1", "/gpui/darwin-arm64/bindings?go-get=1"]) {
    const response = request(worker, suffix);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("Content-Type") ?? "", /^text\/html/);
    assert.ok((await response.text()).includes(
      `name="go-import" content="${path} mod https://native.example.org/proxy"`));
  }
  assert.equal(request(worker, "/gpui/darwin-arm64-other?go-get=1").status, 404);
  assert.equal(worker.fetch(new Request("https://wrong.example.org/gpui/darwin-arm64?go-get=1")).status, 404);
});

test("a nested module has its own discovery and proxy metadata", async () => {
  const nested = `${path}/nested`;
  const multiple = createWorker([entry, {
    ...entry, path: nested,
    versions: { "v0.1.0": { ...versions["v0.2.0"], goMod: `module ${nested}\n` } },
  }]);
  assert.ok((await request(multiple, "/gpui/darwin-arm64/nested?go-get=1").text())
    .includes(`content="${nested} mod `));
  assert.equal(await request(multiple, `/proxy/${nested}/@v/v0.1.0.mod`).text(), `module ${nested}\n`);
  assert.equal(request(multiple, `/proxy/${nested}/@v/v0.2.0.zip`).status, 404);
});

test("list is semantically sorted and latest prefers a stable release", async () => {
  assert.equal(await request(worker, `/proxy/${path}/@v/list`).text(), "v0.2.0\nv0.10.0\nv1.0.0-rc.1\n");
  const latest = request(worker, `/proxy/${path}/@latest`);
  assert.deepEqual(await latest.json(), { Version: "v0.10.0", Time: versions["v0.10.0"].time });
  assert.equal(latest.headers.get("Cache-Control"), "public, max-age=300");
});

test("latest compares prerelease numeric identifiers numerically", async () => {
  const prerelease = createWorker([{ ...entry, versions: Object.fromEntries(
    ["v0.1.0-beta.2", "v0.1.0-beta.10", "v0.1.0-beta.10.a"].map((version) =>
      [version, { ...versions["v0.2.0"], tag: version }]),
  ) }]);
  assert.equal((await request(prerelease, `/proxy/${path}/@latest`).json()).Version, "v0.1.0-beta.10.a");
});

test("pinned metadata preserves original go.mod bytes and publication time", async () => {
  const mod = request(worker, `/proxy/${path}/@v/v0.2.0.mod`);
  assert.equal(await mod.text(), goMod);
  assert.equal(mod.headers.get("Cache-Control"), "public, max-age=31536000, immutable");
  const info = request(worker, `/proxy/${path}/@v/v0.2.0.info`);
  assert.deepEqual(await info.json(), { Version: "v0.2.0", Time: versions["v0.2.0"].time });
});

test("zip requests redirect directly to a pinned public GitHub release", async () => {
  const response = request(worker, `/proxy/${path}/@v/v0.2.0.zip`);
  assert.equal(response.status, 302);
  assert.equal(response.headers.get("Location"),
    "https://github.com/moriyoshi/gpui-go/releases/download/v0.2.0/gpui-darwin-arm64.zip");
  assert.equal(await response.text(), "");
  // A query parameter cannot change the target or destination.
  const other = request(worker, `/proxy/${path}/@v/v0.2.0.zip?arch=amd64&url=https://evil.example`);
  assert.equal(other.headers.get("Location"), response.headers.get("Location"));
});

test("Go case encoding applies to both module paths and prerelease versions", async () => {
  const mixedPath = "native.example.org/GPUI/v2";
  const mixed = createWorker([{ ...entry, path: mixedPath, versions: {
    "v2.0.0-RC.1": { ...versions["v0.2.0"], goMod: `module ${mixedPath}\n`, tag: "v2.0.0-RC.1" },
  } }]);
  const encoded = "/proxy/native.example.org/!g!p!u!i/v2/@v/v2.0.0-!r!c.1.info";
  assert.equal((await request(mixed, encoded).json()).Version, "v2.0.0-RC.1");
  assert.equal(request(mixed, "/proxy/native.example.org/GPUI/v2/@v/v2.0.0-RC.1.zip").status, 404);
});

test("GET and HEAD agree on metadata and redirect headers", async () => {
  for (const suffix of ["/gpui/darwin-arm64?go-get=1", `/proxy/${path}/@v/v0.2.0.mod`,
    `/proxy/${path}/@v/v0.2.0.zip`, `/proxy/${path}/@v/list`, "/missing"]) {
    const get = request(worker, suffix);
    const head = request(worker, suffix, "HEAD");
    assert.equal(head.status, get.status);
    assert.deepEqual([...head.headers], [...get.headers]);
    assert.equal(await head.text(), "");
  }
});

test("unknown modules and versions fail closed without cached misses", async () => {
  for (const suffix of ["/", "/missing", `/proxy/${path}/@v/v9.0.0.zip`,
    `/proxy/${path}/@v/v0.2.0.zip/extra`, "/proxy/unlisted.example.org/gpui/@v/v0.2.0.zip",
    `/proxy/${path}/@v/latest.zip`, `/proxy/${path}/@v/v0.2.0.sig`]) {
    const response = request(worker, suffix);
    assert.equal(response.status, 404, suffix);
    assert.equal(response.headers.get("Cache-Control"), "no-store");
    assert.match(response.headers.get("Content-Type") ?? "", /^text\/plain/);
    assert.equal(response.headers.has("Location"), false);
  }
  assert.equal(request(worker, `/proxy/${path}/@v/list`, "POST").status, 405);
  assert.equal(request(worker, `/proxy/${path}/@v/list`, "POST").headers.get("Allow"), "GET, HEAD");
  assert.equal(request(createWorker(catalog), `/proxy/${path}/@v/v0.2.0.zip`).status, 404);
});

test("modules at the domain root share it with the documentation site", async () => {
  const rooted = "gpui-go.dev/darwin-arm64";
  const metadata = createWorker([{ ...entry, path: rooted, versions: {
    "v0.2.0": { ...versions["v0.2.0"], goMod: `module ${rooted}\n` },
  } }]);
  const assets: string[] = [];
  const env: SiteEnv = { ASSETS: { fetch: async (request) => {
    assets.push(new URL(request.url).pathname);
    return new Response("docs", { status: 200 });
  } } };
  const get = (suffix: string, method = "GET") =>
    createSite(metadata).fetch(new Request(`https://gpui-go.dev${suffix}`, { method }), env);

  for (const suffix of ["/", "/404.html", "/darwin-arm64-other?go-get=1", "/proxy"]) {
    assert.equal(await (await get(suffix)).text(), "docs");
  }
  assert.deepEqual(assets, ["/", "/404.html", "/darwin-arm64-other", "/proxy"]);

  assets.length = 0;
  assert.ok((await (await get("/darwin-arm64?go-get=1")).text())
    .includes(`content="${rooted} mod https://gpui-go.dev/proxy"`));
  assert.ok((await (await get("/darwin-arm64/bindings?go-get=1")).text()).includes(`content="${rooted} mod `));
  assert.equal(await (await get(`/proxy/${rooted}/@v/v0.2.0.mod`)).text(), `module ${rooted}\n`);
  assert.equal((await get("/proxy/gpui-go.dev/unknown/@v/list")).status, 404);
  assert.equal((await get("/", "POST")).status, 405);
  assert.deepEqual(assets, []);
});

test("the deployed entry point sends non-module paths to the assets", async () => {
  const env: SiteEnv = { ASSETS: { fetch: async () => new Response("docs") } };
  const response = await defaultWorker.fetch(new Request("https://gpui-go.dev/"), env);
  assert.equal(await response.text(), "docs");
});

test("empty module has a version list but no latest release", async () => {
  const empty = createWorker([{ ...entry, versions: {} }]);
  assert.equal(await request(empty, `/proxy/${path}/@v/list`).text(), "");
  assert.equal(request(empty, `/proxy/${path}/@latest`).status, 404);
});

test("invalid publication metadata is rejected when the catalog is loaded", () => {
  const patches: Partial<ModuleEntry>[] = [
    { path: "native.example.org/../gpui" },
    { path: "native.example.org/gpui/v1" },
    { repository: "https://evil.example/repo" },
    { versions: { "v2.0.0": versions["v0.2.0"] } },
    { versions: { "v0.2.0+build": versions["v0.2.0"] } },
    { versions: { "v0.2.0-rc.01": versions["v0.2.0"] } },
    { versions: { "v0.0.0-20261108000000-123456789abc": versions["v0.2.0"] } },
    { versions: { "v0.2.0": { ...versions["v0.2.0"], tag: ".." } } },
    { versions: { "v0.2.0": { ...versions["v0.2.0"], asset: "../other.zip" } } },
    { versions: { "v0.2.0": { ...versions["v0.2.0"], goMod: "module wrong.example.org/gpui\n" } } },
    { versions: { "v0.2.0": { ...versions["v0.2.0"], time: "now" } } },
    { versions: { "v0.2.0": { ...versions["v0.2.0"], time: "2026-02-30T00:00:00Z" } } },
  ];
  for (const patch of patches) {
    assert.throws(() => createWorker([{ ...entry, ...patch }]));
  }
  assert.throws(() => createWorker([entry, entry]), /duplicate/);
});
