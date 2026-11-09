import { createHash } from "node:crypto";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import catalog from "../catalog.ts";
import { mergeCatalog } from "./catalog.ts";
import { flags, type ReleaseMetadata } from "./release.ts";
import { assertPublishablePrefix, modulePrefix } from "./targets.ts";

// Stage 3: bind every per-target asset under one checksum list.
const options = flags(process.argv.slice(2), ["assets", "version", "source-sha", "bao-sha", "out"], ["allow-placeholder-prefix"]);
if (options["allow-placeholder-prefix"] !== "yes") assertPublishablePrefix(modulePrefix);
const assets = resolve(options.assets!);
const out = resolve(options.out!);
const names = (await readdir(assets)).filter((name) => name !== "SHA256SUMS").sort();
const sums = new Map<string, string>();
for (const name of names.filter((name) => !name.endsWith(".sha256"))) {
  sums.set(name, createHash("sha256").update(await readFile(join(assets, name))).digest("hex"));
}
for (const name of names.filter((name) => name.endsWith(".sha256"))) {
  const [digest, subject] = (await readFile(join(assets, name), "utf8")).trim().split(/\s+/);
  if (!subject || sums.get(subject) !== digest) throw new Error(`Checksum sidecar disagrees with ${subject ?? name}`);
}
await writeFile(join(assets, "SHA256SUMS"), [...sums].map(([name, digest]) => `${digest}  ${name}\n`).join(""), { flag: "wx" });
const metadata = await Promise.all(names.filter((name) => /^native-release-.*\.json$/.test(name))
  .map(async (name) => JSON.parse(await readFile(join(assets, name), "utf8")) as ReleaseMetadata));
if (!metadata.length) throw new Error("No module release metadata in assets");
await writeFile(join(out, "catalog-candidate.ts"),
  'import type { ModuleEntry } from "./types.ts";\n\n'
    + `const catalog: readonly ModuleEntry[] = ${JSON.stringify(mergeCatalog(catalog, metadata), null, 2)};\n\nexport default catalog;\n`,
  { flag: "wx" });
const targets = metadata.map((item) => item.target).sort();
await writeFile(join(out, "release-notes.md"),
  `Native GPUI release ${options.version} for ${targets.join(", ")}.\n\n`
    + `GPUI source: ${options["source-sha"]}\nBao source: ${options["bao-sha"]}\n\n`
    + "Each target has a static library (.a, or .lib on Windows), a -syso.zip of Bao-generated objects and bindings, "
    + "and a Go module ZIP. SHA256SUMS covers every asset.\n\n"
    + "Windows libraries use the GNU ABI (MinGW/LLVM-MinGW), not the MSVC ABI.\n\n"
    + "Before publishing: review dependency/asset notices, add independently signed Bao release envelopes, "
    + "and approve catalog activation. The workflow supplies no production signing authority.\n",
  { flag: "wx" });
console.log(`${sums.size} assets covered by SHA256SUMS`);
