import table from "../native-targets.json" with { type: "json" };
import type { AssetLoader, MetadataWorker, NativeTable, SiteEnv, SiteWorker, WorkerEnv } from "./types.ts";

interface ParsedVersion {
  numbers: readonly [bigint, bigint, bigint];
  pre: readonly string[];
}

export interface PublishedRelease {
  version: string;
  time: string;
  goMod: string;
  location: string;
}

const MUTABLE_CACHE = "public, max-age=300";
const VERSION_CACHE = "public, max-age=31536000, immutable";
const VERSION = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;
const MAX_METADATA = 64 * 1024;

// Go's proxy protocol escapes uppercase ASCII in module paths and versions.
function escapeGo(value: string): string {
  return value.replace(/[A-Z]/g, (letter) => `!${letter.toLowerCase()}`);
}

function unescapeGo(value: string): string | undefined {
  if (/[A-Z]|![^a-z]|!$/.test(value)) return undefined;
  return value.replace(/!([a-z])/g, (_, letter: string) => letter.toUpperCase());
}

function parseVersion(version: string): ParsedVersion {
  const match = VERSION.exec(version);
  if (!match) throw new Error(`Noncanonical release version: ${version}`);
  const pre = match[4]?.split(".") ?? [];
  if (pre.some((part) => /^0\d+$/.test(part))) {
    throw new Error(`Noncanonical prerelease version: ${version}`);
  }
  // This endpoint publishes tagged releases, not VCS revision queries.
  if (/^v\d+\.\d+\.\d+-(?:0\.)?\d{14}-[0-9a-f]{12}$/.test(version)) {
    throw new Error(`Pseudo-versions are not supported: ${version}`);
  }
  return { numbers: [BigInt(match[1]!), BigInt(match[2]!), BigInt(match[3]!)], pre };
}

export function isReleaseVersion(version: string): boolean {
  try {
    parseVersion(version);
    return true;
  } catch {
    return false;
  }
}

function compare<T extends string | number | bigint>(a: T, b: T): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

export function compareVersions(a: string, b: string): number {
  const x = parseVersion(a);
  const y = parseVersion(b);
  for (const i of [0, 1, 2] as const) {
    const order = compare(x.numbers[i], y.numbers[i]);
    if (order) return order;
  }
  const left = x.pre;
  const right = y.pre;
  if (!left.length || !right.length) return compare(Number(!left.length), Number(!right.length));
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    const l = left[i]!;
    const r = right[i]!;
    const ln = /^\d+$/.test(l);
    const rn = /^\d+$/.test(r);
    const order = ln && rn ? compare(BigInt(l), BigInt(r))
      : ln !== rn ? (ln ? -1 : 1) : compare(l, r);
    if (order) return order;
  }
  return compare(left.length, right.length);
}

// Go's @latest rule: the highest stable version, else the highest prerelease.
export function latestVersion(versions: readonly string[]): string | undefined {
  const sorted = versions.filter(isReleaseVersion).sort(compareVersions);
  return sorted.filter((version) => !parseVersion(version).pre.length).at(-1) ?? sorted.at(-1);
}

// Versions v2 and later need the matching /vN module suffix.
function fitsModule(path: string, version: string): boolean {
  const parsed = parseVersion(version);
  const major = /\/v([2-9]\d*|1\d+)$/.exec(path)?.[1];
  return major ? parsed.numbers[0] === BigInt(major) : parsed.numbers[0] <= 1n;
}

export const targetSlug = (go: string): string => go.replace("/", "-");

export const releaseAssetName = (version: string, go: string): string =>
  `gpui-native-${version}-${targetSlug(go)}.zip`;

export const metadataAssetName = (go: string): string => `native-release-${targetSlug(go)}.json`;

export const assetURL = (repository: string, version: string, name: string): string =>
  `https://github.com/${repository}/releases/download/${encodeURIComponent(version)}/${encodeURIComponent(name)}`;

function checkTable(native: NativeTable): void {
  if (!/^[a-z0-9.-]+\/[a-z0-9._/-]+$/.test(native.modulePrefix)
      || !native.modulePrefix.split("/")[0]!.includes(".")
      || native.modulePrefix.split("/").some((part) => !part || part === "." || part === "..")) {
    throw new Error(`Invalid module prefix: ${native.modulePrefix}`);
  }
  if (!/^[A-Za-z0-9_-][A-Za-z0-9_.-]*\/[A-Za-z0-9_-][A-Za-z0-9_.-]*$/.test(native.repository)) {
    throw new Error(`Invalid GitHub repository: ${native.repository}`);
  }
  const seen = new Set<string>();
  for (const { go } of native.targets) {
    if (!/^[a-z0-9]+\/[a-z0-9]+$/.test(go) || seen.has(go)) throw new Error(`Invalid or duplicate target: ${go}`);
    seen.add(go);
  }
}

// Check one target's release metadata against the module and version it must
// describe, and return what the proxy serves for it. Throws on any mismatch.
export function validateRelease(native: NativeTable, go: string, version: string, metadata: unknown): PublishedRelease {
  const path = `${native.modulePrefix}/${go}`;
  if (!fitsModule(path, version)) {
    throw new Error(`Version ${version} does not match module path ${path}`);
  }
  const record = metadata as { schema?: unknown; target?: unknown; entry?: { path?: unknown; repository?: unknown; versions?: unknown } } | null;
  if (typeof record !== "object" || record === null || record.schema !== 1 || record.target !== go) {
    throw new Error(`Release metadata for ${path}@${version} is not schema-1 metadata for ${go}`);
  }
  const entry = record.entry;
  if (entry?.path !== path || entry.repository !== native.repository) {
    throw new Error(`Release metadata for ${path}@${version} names another module or repository`);
  }
  const versions = entry.versions as Record<string, Partial<Record<"time" | "tag" | "asset" | "goMod", unknown>>> | undefined;
  if (typeof versions !== "object" || versions === null || Object.keys(versions).join() !== version) {
    throw new Error(`Release metadata for ${path}@${version} must describe exactly that version`);
  }
  const release = versions[version]!;
  const time = release.time;
  if (typeof time !== "string" || !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(time)
      || !Number.isFinite(Date.parse(time))
      || new Date(time).toISOString().slice(0, 19) !== time.slice(0, 19)) {
    throw new Error(`Invalid release time for ${path}@${version}`);
  }
  if (release.tag !== version || release.asset !== releaseAssetName(version, go)) {
    throw new Error(`Unexpected release tag or asset for ${path}@${version}`);
  }
  // Preserve the publisher's exact go.mod, including requirements/retractions.
  const goMod = release.goMod;
  if (typeof goMod !== "string"
      || !goMod.split(/\r?\n/).some((line) => line.trim() === `module ${path}` || line.trim() === `module "${path}"`)) {
    throw new Error(`Missing or mismatched go.mod for ${path}@${version}`);
  }
  return { version, time, goMod, location: assetURL(native.repository, version, release.asset) };
}

// Release assets redirect to signed storage URLs, which fetch follows.
export const fetchAsset: AssetLoader = async (url) => {
  const response = await fetch(url, { redirect: "follow" });
  if (response.status === 404) return undefined;
  if (!response.ok) throw new Error(`${url} returned ${response.status}`);
  const text = await response.text();
  if (text.length > MAX_METADATA) throw new Error(`${url} is too large`);
  return text;
};

function html(value: string): string {
  const entities: Readonly<Record<string, string>> = {
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;",
  };
  return value.replace(/[&<>"']/g, (char) => entities[char]!);
}

function response(
  request: Request,
  body: string | null,
  type: string,
  cache = MUTABLE_CACHE,
  status = 200,
  extra: Readonly<Record<string, string>> = {},
): Response {
  return new Response(request.method === "HEAD" ? null : body, {
    status,
    headers: {
      "Content-Type": type,
      "Cache-Control": cache,
      "X-Content-Type-Options": "nosniff",
      ...extra,
    },
  });
}

function notFound(request: Request): Response {
  return response(request, "Module or version not found\n", "text/plain; charset=utf-8", "no-store", 404);
}

function failure(request: Request, status: number, message: string): Response {
  return response(request, message + "\n", "text/plain; charset=utf-8", "no-store", status);
}

const info = (release: PublishedRelease): string =>
  JSON.stringify({ Version: release.version, Time: release.time }) + "\n";

// The module list comes from the target table; versions come from each
// release's own metadata asset, so publishing a release needs no catalog edit.
export function createWorker(native: NativeTable, load: AssetLoader = fetchAsset): MetadataWorker {
  checkTable(native);
  const modules = native.targets.map(({ go }) => {
    const path = `${native.modulePrefix}/${go}`;
    return {
      go, path,
      host: path.split("/")[0],
      discoveryPath: "/" + path.slice(path.indexOf("/") + 1),
      proxyPath: "/proxy/" + escapeGo(path),
    };
  }).sort((a, b) => b.path.length - a.path.length);
  const discovered = (url: URL) => modules.find((entry) =>
    entry.host === url.hostname
      && (url.pathname === entry.discoveryPath || url.pathname.startsWith(entry.discoveryPath + "/")));

  // Published releases are immutable, so a validated answer is kept for the
  // isolate's lifetime. Misses and failures are retried on the next request.
  const known = new Map<string, Promise<PublishedRelease | undefined>>();
  function published(go: string, version: string): Promise<PublishedRelease | undefined> {
    const key = `${go}@${version}`;
    let pending = known.get(key);
    if (!pending) {
      pending = load(assetURL(native.repository, version, metadataAssetName(go))).then((text) =>
        text === undefined ? undefined : validateRelease(native, go, version, JSON.parse(text)));
      known.set(key, pending);
      pending.then((release) => { if (!release) known.delete(key); }, () => known.delete(key));
    }
    return pending;
  }

  return {
    serves(request: Request): boolean {
      const url = new URL(request.url);
      return (request.method !== "GET" && request.method !== "HEAD")
        || url.pathname.startsWith("/proxy/") || discovered(url) !== undefined;
    },
    async fetch(request: Request, env: WorkerEnv): Promise<Response> {
      if (request.method !== "GET" && request.method !== "HEAD") {
        return response(request, "Method not allowed\n", "text/plain; charset=utf-8", "no-store", 405,
          { Allow: "GET, HEAD" });
      }
      const url = new URL(request.url);
      const path = url.pathname;
      if (!path.startsWith("/proxy/")) {
        const module = discovered(url);
        if (!module) return notFound(request);
        const content = html(`${module.path} mod ${url.origin}/proxy`);
        return response(request,
          `<!doctype html>\n<html><head><meta name="go-import" content="${content}"></head>`
            + `<body>Go module ${html(module.path)}</body></html>\n`,
          "text/html; charset=utf-8");
      }
      const module = modules.find((entry) => path.startsWith(entry.proxyPath + "/"));
      if (!module) return notFound(request);
      const query = path.slice(module.proxyPath.length);
      const current = env.NATIVE_VERSION || undefined;
      if (current !== undefined && !isReleaseVersion(current)) {
        return failure(request, 500, "NATIVE_VERSION is not a release version");
      }
      if (query === "/@v/list") {
        return response(request, current ? current + "\n" : "", "text/plain; charset=utf-8");
      }
      const match = query === "/@latest" ? [query, current, "latest"] as const
        : /^\/@v\/([^/]+)\.(info|mod|zip)$/.exec(query);
      const version = match?.[1] === undefined ? undefined : unescapeGo(match[1]);
      if (!match || version === undefined || !isReleaseVersion(version)
          || !fitsModule(module.path, version)) {
        return notFound(request);
      }
      let release: PublishedRelease | undefined;
      try {
        release = await published(module.go, version);
      } catch (error) {
        console.error(`${module.path}@${version}:`, error);
        return failure(request, 502, "Release metadata is unavailable or invalid");
      }
      if (!release) return notFound(request);
      switch (match[2]) {
        case "latest":
          return response(request, info(release), "application/json; charset=utf-8");
        case "info":
          return response(request, info(release), "application/json; charset=utf-8", VERSION_CACHE);
        case "mod":
          return response(request, release.goMod, "text/plain; charset=utf-8", VERSION_CACHE);
        default:
          return response(request, null, "application/zip", VERSION_CACHE, 302, { Location: release.location });
      }
    },
  };
}

// Modules sit at the domain root beside the documentation, so module routes win.
export function createSite(metadata: MetadataWorker): SiteWorker {
  return {
    fetch(request: Request, env: SiteEnv): Response | Promise<Response> {
      return metadata.serves(request) ? metadata.fetch(request, env) : env.ASSETS.fetch(request);
    },
  };
}

export const nativeTable: NativeTable = table;

export default createSite(createWorker(nativeTable));
