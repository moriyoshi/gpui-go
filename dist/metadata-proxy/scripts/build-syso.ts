import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { zipSync } from "fflate";
import { assertSourceRevision, flags, writeChecksum } from "./release.ts";
import { nativeGoEnvironment, nativeGoVersion } from "./profile.ts";
import { libraryAssetName, nativeTarget, sysoAssetName, targetSlug } from "./targets.ts";
import { consolidateDarwinImports } from "./darwin-imports.ts";
import { prelinkLinuxArchive } from "./linux-prelink.ts";

const run = promisify(execFile);

// Stage 2: translate the stage 1 library into .syso objects and typed bindings.
const options = flags(process.argv.slice(2), ["bao", "bao-sha", "target", "version", "out"], ["windows-runtime"]);
const target = nativeTarget(options.target!);
const out = resolve(options.out!);
const bao = resolve(options.bao!);
const goVersion = (await run("go", ["env", "GOVERSION"], { env: nativeGoEnvironment() })).stdout.trim();
if (goVersion !== `go${nativeGoVersion}`) throw new Error(`Native publication requires Go ${nativeGoVersion}; got ${goVersion}`);
const baoVersion = JSON.parse((await run(bao, ["version", "-json"])).stdout) as {
  version: string; commit: string; verified_go_linker: string;
};
if (baoVersion.verified_go_linker !== `go${nativeGoVersion}` || baoVersion.commit !== options["bao-sha"]) {
  throw new Error("Bao binary must match the pinned source commit and verified Go linker profile");
}
assertSourceRevision("bao-sha", options["bao-sha"]!);

const library = join(out, "assets", libraryAssetName(options.version!, target));
const bindings = fileURLToPath(new URL("../../native-bindings.json", import.meta.url));
const config = fileURLToPath(new URL("../../bao.config.json", import.meta.url));
const packageRoot = join(out, "package");
const args = ["build", "-config", config, "-target", target.go, "-o", packageRoot];
const env: NodeJS.ProcessEnv = { ...process.env };
if (target.go.startsWith("windows/")) {
  // Rust's GNU std leaves its CRT and import libraries to this toolchain; the config names them.
  if (!options["windows-runtime"]) throw new Error("Windows targets need --windows-runtime LLVM_MINGW_TARGET_PREFIX");
  const runtime = resolve(options["windows-runtime"]);
  env.LLVM_MINGW_TARGET = runtime;
  args.push("-link-notice", join(runtime, "share/mingw32/COPYING.MinGW-w64-runtime.txt"));
  const rustDocs = join((await run("rustc", ["--print", "sysroot"])).stdout.trim(), "share/doc/rust");
  args.push("-link-notice", join(rustDocs, "COPYRIGHT-library.html"));
  for (const license of (await readdir(join(rustDocs, "licenses"))).sort()) args.push("-link-notice", join(rustDocs, "licenses", license));
}
let input = library;
if (target.go.startsWith("linux/")) {
  const prelink = join(out, "prelink");
  await mkdir(prelink);
  const { functions } = JSON.parse(await readFile(bindings, "utf8")) as { functions: { symbol: string }[] };
  input = prelinkLinuxArchive(library, prelink, functions.map((fn) => fn.symbol));
}
args.push(input);
const result = await run(bao, args, { env, maxBuffer: 32 * 1024 * 1024 });
if (target.go.startsWith("darwin/")) await consolidateDarwinImports(packageRoot);
await writeFile(join(out, "bao-build.log"), result.stdout + result.stderr, { flag: "wx" });
await writeFile(join(out, "bao.json"), JSON.stringify({
  commit: options["bao-sha"], version: baoVersion.version,
  executable_sha256: createHash("sha256").update(await readFile(bao)).digest("hex"),
}, null, 2) + "\n", { flag: "wx" });

const files: Record<string, Uint8Array> = Object.create(null) as Record<string, Uint8Array>;
for (const name of (await readdir(packageRoot)).sort()) files[`${targetSlug(target.go)}/${name}`] = await readFile(join(packageRoot, name));
if (!Object.keys(files).some((name) => name.endsWith(".syso"))) throw new Error("Bao generated no native objects");
const assets = join(out, "assets");
await mkdir(assets, { recursive: true });
const name = sysoAssetName(options.version!, target.go);
await writeFile(join(assets, name), zipSync(files, { level: 9, mtime: new Date(1980, 0, 1) }), { flag: "wx" });
await writeChecksum(assets, name);
console.log(join(assets, name));
