import catalog from "./catalog.ts";
import type { MetadataWorker, ModuleEntry, SiteEnv, SiteWorker } from "./types.ts";

interface ParsedVersion {
  numbers: readonly [bigint, bigint, bigint];
  pre: readonly string[];
}

interface CompiledRelease {
  version: string;
  parsed: ParsedVersion;
  goMod: string;
  info: string;
  location: string;
}

const MUTABLE_CACHE = "public, max-age=300";
const VERSION_CACHE = "public, max-age=31536000, immutable";
const VERSION = /^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$/;

// Go's proxy protocol escapes uppercase ASCII in module paths and versions.
function escapeGo(value: string): string {
  return value.replace(/[A-Z]/g, (letter) => `!${letter.toLowerCase()}`);
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

function compare<T extends string | number | bigint>(a: T, b: T): number {
  return a < b ? -1 : a > b ? 1 : 0;
}

function compareVersions(a: CompiledRelease, b: CompiledRelease): number {
  for (const i of [0, 1, 2] as const) {
    const order = compare(a.parsed.numbers[i], b.parsed.numbers[i]);
    if (order) return order;
  }
  const left = a.parsed.pre;
  const right = b.parsed.pre;
  if (!left.length || !right.length) return compare(Number(!left.length), Number(!right.length));
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    const x = left[i]!;
    const y = right[i]!;
    const xn = /^\d+$/.test(x);
    const yn = /^\d+$/.test(y);
    const order = xn && yn ? compare(BigInt(x), BigInt(y))
      : xn !== yn ? (xn ? -1 : 1) : compare(x, y);
    if (order) return order;
  }
  return compare(left.length, right.length);
}

function component(value: string, label: string): string {
  if (typeof value !== "string" || !value || /[\x00-\x20\x7f\\/?#]/.test(value)
      || value === "." || value === "..") {
    throw new Error(`Invalid ${label}`);
  }
  return encodeURIComponent(value);
}

function compileCatalog(entries: readonly ModuleEntry[]) {
  const paths = new Set<string>();
  return entries.map((entry) => {
    const path = entry.path;
    if (typeof path !== "string" || !/^[a-z0-9.-]+\/[A-Za-z0-9._/-]+$/.test(path)
        || !path.split("/")[0]!.includes(".")
        || path.split("/").some((part) => !part || part === "." || part === "..")
        || paths.has(path)) {
      throw new Error(`Invalid or duplicate module path: ${path}`);
    }
    paths.add(path);
    if (!/^[A-Za-z0-9_-][A-Za-z0-9_.-]*\/[A-Za-z0-9_-][A-Za-z0-9_.-]*$/.test(entry.repository)) {
      throw new Error(`Invalid GitHub repository for ${path}`);
    }
    const major = /\/v([2-9]\d*|1\d+)$/.exec(path)?.[1];
    if (/\/v\d+$/.test(path) && !major) {
      throw new Error(`Invalid major version suffix: ${path}`);
    }
    const releases = Object.entries(entry.versions).map(([version, release]) => {
      const parsed = parseVersion(version);
      if (major ? parsed.numbers[0] !== BigInt(major) : parsed.numbers[0] > 1n) {
        throw new Error(`Version ${version} does not match module path ${path}`);
      }
      if (!/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z$/.test(release.time)
          || !Number.isFinite(Date.parse(release.time))
          || new Date(release.time).toISOString().slice(0, 19) !== release.time.slice(0, 19)) {
        throw new Error(`Invalid release time for ${path}@${version}`);
      }
      // Preserve the publisher's exact go.mod, including requirements/retractions.
      if (typeof release.goMod !== "string"
          || !release.goMod.split(/\r?\n/).some((line) =>
            line.trim() === `module ${path}` || line.trim() === `module "${path}"`)) {
        throw new Error(`Missing or mismatched go.mod for ${path}@${version}`);
      }
      const tag = component(release.tag, "GitHub release tag");
      const asset = component(release.asset, "GitHub release asset");
      return {
        version, parsed, goMod: release.goMod,
        info: JSON.stringify({ Version: version, Time: release.time }) + "\n",
        location: `https://github.com/${entry.repository}/releases/download/${tag}/${asset}`,
      };
    }).sort(compareVersions);
    const stable = releases.filter((release) => !release.parsed.pre.length);
    return {
      path,
      host: path.split("/")[0],
      discoveryPath: "/" + path.slice(path.indexOf("/") + 1),
      proxyPath: "/proxy/" + escapeGo(path),
      releases: new Map(releases.map((release) => [escapeGo(release.version), release])),
      list: releases.map((release) => release.version).join("\n") + (releases.length ? "\n" : ""),
      latest: stable.at(-1) ?? releases.at(-1),
    };
  }).sort((a, b) => b.path.length - a.path.length);
}

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

export function createWorker(entries: readonly ModuleEntry[]): MetadataWorker {
  const modules = compileCatalog(entries);
  const discovered = (url: URL) => modules.find((entry) =>
    entry.host === url.hostname
      && (url.pathname === entry.discoveryPath || url.pathname.startsWith(entry.discoveryPath + "/")));
  return {
    serves(request: Request): boolean {
      const url = new URL(request.url);
      return (request.method !== "GET" && request.method !== "HEAD")
        || url.pathname.startsWith("/proxy/") || discovered(url) !== undefined;
    },
    fetch(request: Request): Response {
      if (request.method !== "GET" && request.method !== "HEAD") {
        return response(request, "Method not allowed\n", "text/plain; charset=utf-8", "no-store", 405,
          { Allow: "GET, HEAD" });
      }
      const url = new URL(request.url);
      const path = url.pathname;
      if (path.startsWith("/proxy/")) {
        for (const module of modules) {
          if (!path.startsWith(module.proxyPath + "/")) continue;
          const query = path.slice(module.proxyPath.length);
          if (query === "/@v/list") {
            return response(request, module.list, "text/plain; charset=utf-8");
          }
          if (query === "/@latest") {
            return module.latest
              ? response(request, module.latest.info, "application/json; charset=utf-8")
              : notFound(request);
          }
          const match = /^\/@v\/([^/]+)\.(info|mod|zip)$/.exec(query);
          if (!match) continue;
          const release = module.releases.get(match[1]!);
          if (!release) continue;
          switch (match[2]) {
            case "info":
              return response(request, release.info, "application/json; charset=utf-8", VERSION_CACHE);
            case "mod":
              return response(request, release.goMod, "text/plain; charset=utf-8", VERSION_CACHE);
            case "zip":
              return response(request, null, "application/zip", VERSION_CACHE, 302,
                { Location: release.location });
          }
        }
        return notFound(request);
      }
      const module = discovered(url);
      if (!module) return notFound(request);
      const content = html(`${module.path} mod ${url.origin}/proxy`);
      return response(request,
        `<!doctype html>\n<html><head><meta name="go-import" content="${content}"></head>`
          + `<body>Go module ${html(module.path)}</body></html>\n`,
        "text/html; charset=utf-8");
    },
  };
}

// Modules sit at the domain root beside the documentation, so module routes win.
export function createSite(metadata: MetadataWorker): SiteWorker {
  return {
    fetch(request: Request, env: SiteEnv): Response | Promise<Response> {
      return metadata.serves(request) ? metadata.fetch(request) : env.ASSETS.fetch(request);
    },
  };
}

export default createSite(createWorker(catalog));
