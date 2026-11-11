import assert from "node:assert/strict";
import test from "node:test";
import defaultWorker, { createAssetLoader, createSite, createWorker, latestVersion, nativeTable, validateRelease } from "./worker.ts";
import type { MetadataWorker, NativeTable, ReleaseMetadata, SiteEnv, WorkerEnv } from "./types.ts";

const table: NativeTable = {
  modulePrefix: "native.example.org/gpui",
  repository: "moriyoshi/gpui-go",
  targets: [{ go: "darwin/arm64" }, { go: "linux/amd64" }],
};
const path = "native.example.org/gpui/darwin/arm64";
const goMod = `module ${path}\n\ngo 1.26.5\n`;
const download = "https://github.com/moriyoshi/gpui-go/releases/download";

function metadata(version: string, go = "darwin/arm64", time = "2026-11-01T00:00:00Z"): ReleaseMetadata {
  const slug = go.replace("/", "-");
  const asset = `gpui-native-${version}-${slug}.zip`;
  return {
    schema: 1, target: go,
    entry: { path: `${table.modulePrefix}/${go}`, repository: table.repository,
      versions: { [version]: { time, tag: version, asset, goMod: `module ${table.modulePrefix}/${go}\n\ngo 1.26.5\n` } } },
    zip: { name: asset, sha256: "0".repeat(64), size: 1 },
  };
}

// A release store keyed by asset URL; loads counts every request it receives.
function store(releases: readonly ReleaseMetadata[]) {
  const assets = new Map(releases.map((item) => {
    const version = Object.keys(item.entry.versions)[0]!;
    return [`${download}/${version}/native-release-${item.target.replace("/", "-")}.json`, JSON.stringify(item)];
  }));
  const loads: string[] = [];
  return { assets, loads, load: async (url: string) => { loads.push(url); return assets.get(url); } };
}

const releases = store([metadata("v0.1.0"), metadata("v0.2.0", "darwin/arm64", "2026-11-02T00:00:00Z"),
  metadata("v0.3.0-RC.1"), metadata("v0.2.0", "linux/amd64")]);
const worker = createWorker(table, releases.load);
const current: WorkerEnv = { NATIVE_VERSION: "v0.2.0" };

function request(worker: MetadataWorker, suffix: string, env: WorkerEnv = current, method = "GET"): Promise<Response> {
  return worker.fetch(new Request(`https://native.example.org${suffix}`, { method }), env);
}

test("module and subpackage discovery point Go at the proxy", async () => {
  for (const suffix of ["/gpui/darwin/arm64?go-get=1", "/gpui/darwin/arm64/bindings?go-get=1"]) {
    const response = await request(worker, suffix);
    assert.equal(response.status, 200);
    assert.match(response.headers.get("Content-Type") ?? "", /^text\/html/);
    assert.ok((await response.text()).includes(
      `name="go-import" content="${path} mod https://native.example.org/proxy"`));
  }
  assert.equal((await request(worker, "/gpui/darwin/arm64-other?go-get=1")).status, 404);
  assert.equal((await worker.fetch(new Request("https://wrong.example.org/gpui/darwin/arm64?go-get=1"), current)).status, 404);
});

test("list and latest advertise NATIVE_VERSION from that release's metadata", async () => {
  assert.equal(await (await request(worker, `/proxy/${path}/@v/list`)).text(), "v0.2.0\n");
  const latest = await request(worker, `/proxy/${path}/@latest`);
  assert.deepEqual(await latest.json(), { Version: "v0.2.0", Time: "2026-11-02T00:00:00Z" });
  assert.equal(latest.headers.get("Cache-Control"), "public, max-age=300");
  const linux = await request(worker, `/proxy/${table.modulePrefix}/linux/amd64/@latest`);
  assert.equal((await linux.json()).Version, "v0.2.0");
});

test("without NATIVE_VERSION the list is empty and latest is missing", async () => {
  for (const env of [{}, { NATIVE_VERSION: "" }]) {
    assert.equal(await (await request(worker, `/proxy/${path}/@v/list`, env)).text(), "");
    assert.equal((await request(worker, `/proxy/${path}/@latest`, env)).status, 404);
    assert.equal((await request(worker, `/proxy/${path}/@v/v0.1.0.info`, env)).status, 200);
  }
});

test("a malformed NATIVE_VERSION fails loudly instead of listing it", async () => {
  const response = await request(worker, `/proxy/${path}/@v/list`, { NATIVE_VERSION: "main" });
  assert.equal(response.status, 500);
  assert.equal(response.headers.get("Cache-Control"), "no-store");
});

test("pinned metadata preserves go.mod bytes and publication time, listed or not", async () => {
  const mod = await request(worker, `/proxy/${path}/@v/v0.1.0.mod`);
  assert.equal(await mod.text(), goMod);
  assert.equal(mod.headers.get("Cache-Control"), "public, max-age=31536000, immutable");
  const info = await request(worker, `/proxy/${path}/@v/v0.1.0.info`);
  assert.deepEqual(await info.json(), { Version: "v0.1.0", Time: "2026-11-01T00:00:00Z" });
});

test("zip requests redirect directly to the release asset", async () => {
  const response = await request(worker, `/proxy/${path}/@v/v0.2.0.zip`);
  assert.equal(response.status, 302);
  assert.equal(response.headers.get("Location"), `${download}/v0.2.0/gpui-native-v0.2.0-darwin-arm64.zip`);
  assert.equal(await response.text(), "");
  // A query parameter cannot change the target or destination.
  const other = await request(worker, `/proxy/${path}/@v/v0.2.0.zip?arch=amd64&url=https://evil.example`);
  assert.equal(other.headers.get("Location"), response.headers.get("Location"));
});

test("Go case encoding applies to prerelease versions", async () => {
  assert.equal((await (await request(worker, `/proxy/${path}/@v/v0.3.0-!r!c.1.info`)).json()).Version, "v0.3.0-RC.1");
  assert.equal((await request(worker, `/proxy/${path}/@v/v0.3.0-RC.1.info`)).status, 404);
});

test("a validated release is loaded once; misses are retried", async () => {
  const fresh = store([metadata("v0.1.0")]);
  const cached = createWorker(table, fresh.load);
  await request(cached, `/proxy/${path}/@v/v0.1.0.mod`);
  await request(cached, `/proxy/${path}/@v/v0.1.0.zip`);
  assert.equal(fresh.loads.length, 1);
  await request(cached, `/proxy/${path}/@v/v0.9.0.mod`);
  await request(cached, `/proxy/${path}/@v/v0.9.0.mod`);
  assert.equal(fresh.loads.length, 3);
});

test("GET and HEAD agree on metadata and redirect headers", async () => {
  for (const suffix of ["/gpui/darwin/arm64?go-get=1", `/proxy/${path}/@v/v0.2.0.mod`,
    `/proxy/${path}/@v/v0.2.0.zip`, `/proxy/${path}/@v/list`, `/proxy/${path}/@latest`, "/missing"]) {
    const get = await request(worker, suffix);
    const head = await request(worker, suffix, current, "HEAD");
    assert.equal(head.status, get.status);
    assert.deepEqual([...head.headers], [...get.headers]);
    assert.equal(await head.text(), "");
  }
});

test("unknown modules, versions and routes fail closed without cached misses", async () => {
  const before = releases.loads.length;
  for (const suffix of ["/", "/missing", `/proxy/${path}/@v/v0.2.0.zip/extra`,
    "/proxy/unlisted.example.org/gpui/@v/v0.2.0.zip", `/proxy/${path}/@v/latest.zip`,
    `/proxy/${path}/@v/v0.2.0.sig`, `/proxy/${path}/@v/v0.2.zip`, `/proxy/${path}/@v/v0.2.0+build.zip`,
    `/proxy/${path}/@v/v0.2.0-rc.01.zip`, `/proxy/${path}/@v/v0.0.0-20261108000000-123456789abc.zip`,
    `/proxy/${path}/@v/v2.0.0.zip`, `/proxy/${path}/@v/v0.2.0-!.zip`]) {
    const response = await request(worker, suffix);
    assert.equal(response.status, 404, suffix);
    assert.equal(response.headers.get("Cache-Control"), "no-store");
    assert.match(response.headers.get("Content-Type") ?? "", /^text\/plain/);
    assert.equal(response.headers.has("Location"), false);
  }
  // Malformed versions are rejected before anything is fetched.
  assert.equal(releases.loads.length, before);
  const missing = await request(worker, `/proxy/${path}/@v/v9.0.0.zip`);
  assert.equal(missing.status, 404);
  assert.equal(missing.headers.has("Location"), false);
  assert.equal((await request(worker, `/proxy/${path}/@v/list`, current, "POST")).status, 405);
  assert.equal((await request(worker, `/proxy/${path}/@v/list`, current, "POST")).headers.get("Allow"), "GET, HEAD");
});

test("metadata that does not describe the requested module and version is refused", async () => {
  const good = metadata("v0.5.0");
  const release = good.entry.versions["v0.5.0"]!;
  const patches: unknown[] = [
    { ...good, schema: 2 },
    { ...good, target: "linux/amd64" },
    { ...good, entry: { ...good.entry, path: "native.example.org/gpui/linux/amd64" } },
    { ...good, entry: { ...good.entry, repository: "someone/else" } },
    { ...good, entry: { ...good.entry, versions: { "v0.4.0": release } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": release, "v0.4.0": release } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": { ...release, tag: "v0.4.0" } } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": { ...release, asset: "../other.zip" } } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": { ...release, goMod: "module wrong.example.org/gpui\n" } } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": { ...release, time: "now" } } } },
    { ...good, entry: { ...good.entry, versions: { "v0.5.0": { ...release, time: "2026-02-30T00:00:00Z" } } } },
    null,
  ];
  assert.doesNotThrow(() => validateRelease(table, "darwin/arm64", "v0.5.0", good));
  for (const patch of patches) {
    assert.throws(() => validateRelease(table, "darwin/arm64", "v0.5.0", patch), Error, JSON.stringify(patch));
    const bad = createWorker(table, async () => JSON.stringify(patch));
    const response = await request(bad, `/proxy/${path}/@v/v0.5.0.zip`);
    assert.equal(response.status, 502);
    assert.equal(response.headers.get("Cache-Control"), "no-store");
    assert.equal(response.headers.has("Location"), false);
  }
  const garbled = createWorker(table, async () => "{");
  assert.equal((await request(garbled, `/proxy/${path}/@v/v0.5.0.mod`)).status, 502);
});

test("a failed metadata fetch is reported and retried", async () => {
  let attempts = 0;
  const flaky = createWorker(table, async () => {
    if (attempts++ === 0) throw new Error("GitHub is down");
    return JSON.stringify(metadata("v0.1.0"));
  });
  const failed = await request(flaky, `/proxy/${path}/@v/v0.1.0.mod`);
  assert.equal(failed.status, 502);
  assert.match(await failed.text(), /unavailable or invalid: GitHub is down/);
  assert.equal((await request(flaky, `/proxy/${path}/@v/v0.1.0.mod`)).status, 200);
});

test("asset downloads retry transient GitHub failures but not refusals", async () => {
  const url = `${download}/v0.1.0/native-release-darwin-arm64.json`;
  const sequence = (...steps: (number | Error)[]) => {
    const calls: string[] = [];
    const get = async (target: string) => {
      calls.push(target);
      const step = steps.shift() ?? 200;
      if (step instanceof Error) throw step;
      return new Response(step === 200 ? "{}" : "error", { status: step });
    };
    return { calls, load: createAssetLoader(get, [0, 0, 0]) };
  };
  // A rate limit, a server error and a network failure each get another try.
  const transient = sequence(429, 503, new TypeError("connection reset"));
  assert.equal(await transient.load(url), "{}");
  assert.equal(transient.calls.length, 4);
  const missing = sequence(404);
  assert.equal(await missing.load(url), undefined);
  assert.equal(missing.calls.length, 1);
  const refused = sequence(403);
  await assert.rejects(refused.load(url), /GitHub returned 403/);
  assert.equal(refused.calls.length, 1);
  const down = sequence(502, 502, 502, 502, 200);
  await assert.rejects(down.load(url), /GitHub returned 502/);
  assert.equal(down.calls.length, 4);
  // Anything else thrown, such as a bug, is not retried.
  const broken = sequence(new RangeError("bug"));
  await assert.rejects(broken.load(url), /bug/);
  assert.equal(broken.calls.length, 1);
});

test("the published v0.0.1 metadata validates against the target table", () => {
  // Verbatim from https://github.com/moriyoshi/gpui-go/releases/download/v0.0.1/native-release-darwin-arm64.json
  const published = JSON.parse(`{
    "schema": 1, "target": "darwin/arm64",
    "entry": { "path": "gpui-go.dev/__binary/darwin/arm64", "repository": "moriyoshi/gpui-go",
      "versions": { "v0.0.1": { "time": "2026-11-10T08:53:52Z", "tag": "v0.0.1",
        "asset": "gpui-native-v0.0.1-darwin-arm64.zip",
        "goMod": "module gpui-go.dev/__binary/darwin/arm64\\n\\ngo 1.26.10\\n" } } },
    "zip": { "name": "gpui-native-v0.0.1-darwin-arm64.zip",
      "sha256": "40d1cc4233fd119a5b4284af66ba723aad1b68da066c02da4c226c3490ba102f", "size": 17693760 } }`);
  assert.deepEqual(validateRelease(nativeTable, "darwin/arm64", "v0.0.1", published), {
    version: "v0.0.1", time: "2026-11-10T08:53:52Z",
    goMod: "module gpui-go.dev/__binary/darwin/arm64\n\ngo 1.26.10\n",
    location: `${download}/v0.0.1/gpui-native-v0.0.1-darwin-arm64.zip`,
  });
});

test("modules at the domain root share it with the documentation site", async () => {
  const rooted = "gpui-go.dev/__binary/darwin/arm64";
  const live = store([{ ...metadata("v0.2.0"), entry: { ...metadata("v0.2.0").entry, path: rooted,
    versions: { "v0.2.0": { ...metadata("v0.2.0").entry.versions["v0.2.0"]!, goMod: `module ${rooted}\n` } } } }]);
  const metadataWorker = createWorker(nativeTable, live.load);
  const assets: string[] = [];
  const env: SiteEnv = { ...current, ASSETS: { fetch: async (request) => {
    assets.push(new URL(request.url).pathname);
    return new Response("docs", { status: 200 });
  } } };
  const get = (suffix: string, method = "GET") =>
    createSite(metadataWorker).fetch(new Request(`https://gpui-go.dev${suffix}`, { method }), env);

  for (const suffix of ["/", "/404.html", "/__binary/darwin/arm64-other?go-get=1", "/__binary/darwin?go-get=1", "/proxy"]) {
    assert.equal(await (await get(suffix)).text(), "docs");
  }
  assert.deepEqual(assets, ["/", "/404.html", "/__binary/darwin/arm64-other", "/__binary/darwin", "/proxy"]);

  assets.length = 0;
  assert.ok((await (await get("/__binary/darwin/arm64?go-get=1")).text())
    .includes(`content="${rooted} mod https://gpui-go.dev/proxy"`));
  assert.ok((await (await get("/__binary/darwin/arm64/bindings?go-get=1")).text()).includes(`content="${rooted} mod `));
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

test("latest prefers a stable release and compares prerelease identifiers numerically", () => {
  assert.equal(latestVersion(["v0.2.0", "v0.10.0", "v1.0.0-rc.1", "nightly"]), "v0.10.0");
  assert.equal(latestVersion(["v0.1.0-beta.2", "v0.1.0-beta.10", "v0.1.0-beta.10.a"]), "v0.1.0-beta.10.a");
  assert.equal(latestVersion(["v0.0.1"]), "v0.0.1");
  assert.equal(latestVersion(["main"]), undefined);
});

test("invalid target tables are rejected when the Worker is created", () => {
  const patches: Partial<NativeTable>[] = [
    { modulePrefix: "native.example.org/../gpui" },
    { modulePrefix: "localhost/gpui" },
    { modulePrefix: "native.example.org/GPUI" },
    { repository: "https://evil.example/repo" },
    { targets: [{ go: "darwin/arm64" }, { go: "darwin/arm64" }] },
    { targets: [{ go: "darwin-arm64" }] },
  ];
  for (const patch of patches) {
    assert.throws(() => createWorker({ ...table, ...patch }), Error, JSON.stringify(patch));
  }
});
