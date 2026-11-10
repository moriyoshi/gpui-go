import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { lstat, mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { zipSync } from "fflate";
import { metadataAssetName, releaseAssetName as assetName, validateRelease } from "../worker.ts";
import type { ModuleEntry, ReleaseMetadata } from "../types.ts";
import { nativeTarget } from "./targets.ts";

export type { ReleaseMetadata };

const MAX_ZIP = 500 * 1024 * 1024;
const MAX_METADATA = 16 * 1024 * 1024;

export function releaseAssetName(version: string, target: string): string {
  nativeTarget(target);
  return assetName(version, target);
}

export interface PackageOptions {
  root: string;
  module: string;
  version: string;
  target: string;
  repository: string;
  time: string;
  out: string;
}

export async function packageRelease(options: PackageOptions): Promise<ReleaseMetadata> {
  const { root, version, target, repository, time, out } = options;
  if (options.module !== options.module.toLowerCase()) {
    throw new Error("Publication module paths must be lowercase");
  }
  const name = releaseAssetName(version, target);
  const goMod = await readFile(join(root, "go.mod"), "utf8");
  const entry: ModuleEntry = {
    path: options.module,
    repository,
    versions: { [version]: { time, tag: version, asset: name, goMod } },
  };
  // Validate exactly as the Worker will when it reads the metadata back.
  if (!options.module.endsWith(`/${target}`)) throw new Error(`Module path must end in /${target}`);
  validateRelease({ modulePrefix: options.module.slice(0, -target.length - 1), repository, targets: [{ go: target }] },
    target, version, { schema: 1, target, entry });
  if (!(await lstat(root)).isDirectory() || resolve(out).startsWith(resolve(root) + "/")) {
    throw new Error("Module root must be a directory and output must be outside it");
  }
  const files: Record<string, Uint8Array> = Object.create(null) as Record<string, Uint8Array>;
  const names = new Set<string>();
  let total = 0;
  let nativeObjects = 0;
  async function walk(relative = ""): Promise<void> {
    for (const child of (await readdir(join(root, relative))).sort()) {
      if (!/^[A-Za-z0-9_.-]+$/.test(child) || child === ".git"
          || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(child)) {
        throw new Error(`Nonportable module filename: ${child}`);
      }
      const path = relative ? `${relative}/${child}` : child;
      if (names.has(path.toLowerCase())) throw new Error(`Case-colliding module path: ${path}`);
      names.add(path.toLowerCase());
      const stat = await lstat(join(root, path));
      if (stat.isSymbolicLink()) throw new Error(`Symlink in module: ${path}`);
      if (stat.isDirectory()) {
        if (child === "vendor") throw new Error("Vendored trees are not published in native modules");
        await walk(path);
        continue;
      }
      if (!stat.isFile()) throw new Error(`Nonregular module file: ${path}`);
      if (relative && child === "go.mod") throw new Error(`Nested module: ${path}`);
      if ((child === "go.mod" || child === "LICENSE") && stat.size > MAX_METADATA) {
        throw new Error(`Oversized metadata: ${path}`);
      }
      total += stat.size;
      if (total > MAX_ZIP) throw new Error("Uncompressed module exceeds Go's 500 MiB limit");
      const bytes = await readFile(join(root, path));
      if (bytes.length !== stat.size) throw new Error(`Module changed while packaging: ${path}`);
      files[`${options.module}@${version}/${path}`] = bytes;
      if (path.endsWith(".syso")) {
        if (!path.endsWith(`_${target.replace("/", "_")}.syso`)) {
          throw new Error(`Native object does not match ${target}: ${path}`);
        }
        nativeObjects++;
      }
    }
  }
  await walk();
  if (!nativeObjects) throw new Error("No native objects in module");
  if (!names.has("license") || !names.has("notice")) throw new Error("Module needs LICENSE and NOTICE");
  const bytes = zipSync(files, { level: 9, mtime: new Date(1980, 0, 1) });
  if (bytes.length > MAX_ZIP) throw new Error("Compressed module exceeds Go's 500 MiB limit");
  const metadata: ReleaseMetadata = {
    schema: 1, target, entry,
    zip: { name, sha256: createHash("sha256").update(bytes).digest("hex"), size: bytes.length },
  };
  await mkdir(out, { recursive: true }); // Stages share assets; "wx" below refuses to overwrite.
  await writeFile(join(out, name), bytes, { flag: "wx" });
  await writeFile(join(out, metadataAssetName(target)), JSON.stringify(metadata, null, 2) + "\n", { flag: "wx" });
  await writeChecksum(out, name);
  return metadata;
}

export async function writeChecksum(directory: string, name: string): Promise<void> {
  const digest = createHash("sha256").update(await readFile(join(directory, name))).digest("hex");
  await writeFile(join(directory, name + ".sha256"), `${digest}  ${name}\n`, { flag: "wx" });
}

// A local rehearsal of an uncommitted tree can never reach the Release job,
// which passes real commits; "unverified-local" only skips this comparison.
export function assertSourceRevision(label: string, expected: string, directory?: string): void {
  if (expected === "unverified-local") return;
  if (!/^[0-9a-f]{40}$/.test(expected)) throw new Error(`--${label} must be an exact Git commit`);
  if (directory && execFileSync("git", ["-C", directory, "rev-parse", "HEAD"], { encoding: "utf8" }).trim() !== expected) {
    throw new Error(`Source revision mismatch: ${directory}`);
  }
}

export function flags(args: string[], required: readonly string[], optional: readonly string[] = []): Record<string, string> {
  const result: Record<string, string> = {};
  for (let i = 0; i < args.length; i += 2) {
    const key = args[i]?.replace(/^--/, "");
    const value = args[i + 1];
    if (!key || !args[i]!.startsWith("--") || !(required.includes(key) || optional.includes(key)) || !value || result[key]) {
      throw new Error(`Expected unique --${required.join(", --")} arguments`);
    }
    result[key] = value;
  }
  for (const key of required) if (!result[key]) throw new Error(`Missing --${key}`);
  return result;
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const options = flags(process.argv.slice(2), ["root", "module", "version", "target", "repository", "time", "out"]);
  console.log(JSON.stringify(await packageRelease({ root: options.root!, module: options.module!,
    version: options.version!, target: options.target!, repository: options.repository!, time: options.time!, out: options.out! }), null, 2));
}
