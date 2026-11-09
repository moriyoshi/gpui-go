import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import catalog from "../catalog.ts";
import { createWorker } from "../worker.ts";
import type { ModuleEntry, ModuleRelease } from "../types.ts";
import type { ReleaseMetadata } from "./release.ts";
import { releaseAssetName } from "./release.ts";

export function mergeCatalog(existing: readonly ModuleEntry[], additions: readonly ReleaseMetadata[]): ModuleEntry[] {
  createWorker(existing);
  const merged = new Map<string, { repository: string; versions: Record<string, ModuleRelease> }>();
  for (const entry of existing) merged.set(entry.path, { repository: entry.repository, versions: { ...entry.versions } });
  for (const metadata of additions) {
    if (metadata.schema !== 1 || Object.keys(metadata.entry.versions).length !== 1) {
      throw new Error("Expected one version in a schema-1 release metadata file");
    }
    const entry = metadata.entry;
    createWorker([entry]);
    const [version, release] = Object.entries(entry.versions)[0]!;
    if (metadata.zip.name !== release.asset || release.asset !== releaseAssetName(version, metadata.target)
        || !/^[0-9a-f]{64}$/.test(metadata.zip.sha256)
        || !Number.isSafeInteger(metadata.zip.size) || metadata.zip.size <= 0) {
      throw new Error("Invalid ZIP metadata");
    }
    const module = merged.get(entry.path) ?? { repository: entry.repository, versions: {} };
    if (module.repository !== entry.repository) throw new Error(`Repository changed for ${entry.path}`);
    const previous = module.versions[version];
    if (previous && (previous.time !== release.time || previous.tag !== release.tag
        || previous.asset !== release.asset || previous.goMod !== release.goMod)) {
      throw new Error(`Refusing to change published ${entry.path}@${version}`);
    }
    module.versions[version] = release;
    merged.set(entry.path, module);
  }
  const result = [...merged].sort(([a], [b]) => a.localeCompare(b)).map(([path, entry]) => ({ path, ...entry }));
  createWorker(result);
  return result;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const [output, ...inputs] = process.argv.slice(2);
  if (!output || !inputs.length) throw new Error("Usage: catalog.ts NEW_OUTPUT.ts RELEASE_METADATA.json [...]");
  const additions = await Promise.all(inputs.map(async (path) => JSON.parse(await readFile(path, "utf8")) as ReleaseMetadata));
  const merged = mergeCatalog(catalog, additions);
  await writeFile(output,
    'import type { ModuleEntry } from "./types.ts";\n\n'
      + `const catalog: readonly ModuleEntry[] = ${JSON.stringify(merged, null, 2)};\n\nexport default catalog;\n`,
    { flag: "wx" });
}
