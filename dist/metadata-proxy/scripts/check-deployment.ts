import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { assetURL, metadataAssetName, nativeTable, releaseAssetName, validateRelease, type PublishedRelease } from "../worker.ts";
import type { NativeTable } from "../types.ts";

type Fetch = (request: Request) => Promise<Response>;

// Every target's metadata must be published and valid, and its ZIP downloadable,
// before a deploy advertises the version.
export async function checkRelease(native: NativeTable, version: string, fetcher: Fetch): Promise<Map<string, PublishedRelease>> {
  const releases = new Map<string, PublishedRelease>();
  for (const { go } of native.targets) {
    const url = assetURL(native.repository, version, metadataAssetName(go));
    const metadata = await fetcher(new Request(url, { redirect: "follow" }));
    if (!metadata.ok) throw new Error(`Release metadata is unavailable (${metadata.status}): ${url}`);
    const release = validateRelease(native, go, version, await metadata.json());
    const zip = assetURL(native.repository, version, releaseAssetName(version, go));
    const hosted = await fetcher(new Request(zip, { method: "HEAD", redirect: "follow" }));
    if (!hosted.ok) throw new Error(`Release asset is unavailable (${hosted.status}): ${zip}`);
    releases.set(go, release);
  }
  return releases;
}

// Exercise the routes stock Go uses against the release's own metadata.
export async function checkDeployment(origin: string, native: NativeTable, version: string, fetcher: Fetch): Promise<number> {
  const home = await fetcher(new Request(`${origin}/`));
  if (home.status !== 200 || !home.headers.get("content-type")?.startsWith("text/html")) {
    throw new Error(`Documentation home page is unavailable (${home.status})`);
  }
  const releases = await checkRelease(native, version, fetcher);
  for (const [go, release] of releases) {
    const path = `${native.modulePrefix}/${go}`;
    const discovery = await fetcher(new Request(`${origin}/${path.split("/").slice(1).join("/")}?go-get=1`));
    const expected = `${path} mod ${origin}/proxy`;
    if (discovery.status !== 200 || !(await discovery.text()).includes(expected)) {
      throw new Error(`Discovery for ${path} did not advertise ${expected}`);
    }
    const proxy = `${origin}/proxy/${path}`;
    const list = await (await fetcher(new Request(`${proxy}/@v/list`))).text();
    if (!list.split("\n").includes(version)) throw new Error(`${path} does not list ${version}`);
    const latest = await fetcher(new Request(`${proxy}/@latest`));
    const want = JSON.stringify({ Version: version, Time: release.time });
    if (!latest.ok || JSON.stringify(await latest.json()) !== want) throw new Error(`${path}@latest is not ${want}`);
  }
  await checkVersionRoutes(origin, native, version, releases, fetcher);
  return releases.size;
}

// The routes Go fetches for an explicit version, which serve any published
// release whether or not it is the one the deployment advertises.
export async function checkVersionRoutes(origin: string, native: NativeTable, version: string,
  releases: ReadonlyMap<string, PublishedRelease>, fetcher: Fetch): Promise<void> {
  for (const [go, release] of releases) {
    const path = `${native.modulePrefix}/${go}`;
    const proxy = `${origin}/proxy/${path}`;
    const info = await fetcher(new Request(`${proxy}/@v/${version}.info`));
    const want = JSON.stringify({ Version: version, Time: release.time });
    if (!info.ok || JSON.stringify(await info.json()) !== want) throw new Error(`${path}@${version}.info is not ${want} (${info.status})`);
    const goMod = await fetcher(new Request(`${proxy}/@v/${version}.mod`));
    if (!goMod.ok || await goMod.text() !== release.goMod) throw new Error(`${path}@${version} serves different go.mod bytes (${goMod.status})`);
    const zip = await fetcher(new Request(`${proxy}/@v/${version}.zip`, { redirect: "manual" }));
    const location = zip.headers.get("location");
    if (zip.status !== 302 || location !== release.location) throw new Error(`${path}@${version} redirects to ${location} (${zip.status})`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const [origin, version] = process.argv.slice(2);
  if (!origin || !version) throw new Error("Usage: check-deployment.ts ORIGIN VERSION");
  const count = await checkDeployment(origin.replace(/\/$/, ""), nativeTable, version, (request) => fetch(request));
  console.log(`${version} checks out for ${count} modules at ${origin}`);
}
