import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import catalog from "../catalog.ts";
import type { ModuleEntry } from "../types.ts";

type Fetch = (request: Request) => Promise<Response>;

// Exercise the routes stock Go uses, then the release asset each redirect names.
export async function checkDeployment(origin: string, entries: readonly ModuleEntry[], fetcher: Fetch): Promise<number> {
  let checked = 0;
  const home = await fetcher(new Request(`${origin}/`));
  if (home.status !== 200 || !home.headers.get("content-type")?.startsWith("text/html")) {
    throw new Error(`Documentation home page is unavailable (${home.status})`);
  }
  for (const entry of entries) {
    const discovery = await fetcher(new Request(`${origin}/${entry.path.split("/").slice(1).join("/")}?go-get=1`));
    const expected = `${entry.path} mod ${origin}/proxy`;
    if (discovery.status !== 200 || !(await discovery.text()).includes(expected)) {
      throw new Error(`Discovery for ${entry.path} did not advertise ${expected}`);
    }
    const list = await (await fetcher(new Request(`${origin}/proxy/${entry.path}/@v/list`))).text();
    for (const [version, release] of Object.entries(entry.versions)) {
      if (!list.split("\n").includes(version)) throw new Error(`${entry.path} does not list ${version}`);
      const goMod = await (await fetcher(new Request(`${origin}/proxy/${entry.path}/@v/${version}.mod`))).text();
      if (goMod !== release.goMod) throw new Error(`${entry.path}@${version} serves different go.mod bytes`);
      const zip = await fetcher(new Request(`${origin}/proxy/${entry.path}/@v/${version}.zip`, { redirect: "manual" }));
      const location = zip.headers.get("location");
      const asset = `https://github.com/${entry.repository}/releases/download/${release.tag}/${release.asset}`;
      if (zip.status !== 302 || location !== asset) throw new Error(`${entry.path}@${version} redirects to ${location}`);
      const hosted = await fetcher(new Request(asset, { method: "HEAD", redirect: "follow" }));
      if (!hosted.ok) throw new Error(`Release asset is unavailable (${hosted.status}): ${asset}`);
      checked++;
    }
  }
  return checked;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const origin = process.argv[2];
  if (!origin) throw new Error("Usage: check-deployment.ts ORIGIN");
  console.log(`${await checkDeployment(origin.replace(/\/$/, ""), catalog, (request) => fetch(request))} published versions check out at ${origin}`);
}
