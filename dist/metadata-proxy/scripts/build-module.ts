import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { copyFile, cp, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { assertSourceRevision, flags, packageRelease } from "./release.ts";
import { nativeGoEnvironment, nativeGoVersion } from "./profile.ts";
import { libraryAssetName, moduleName, nativeTarget } from "./targets.ts";

const run = promisify(execFile);

// Stage 4: wrap the stage 2 package as a Go module ZIP.
const options = flags(process.argv.slice(2), ["bao-root", "target", "version", "repository", "time", "source-sha", "out"]);
const target = nativeTarget(options.target!);
const repo = fileURLToPath(new URL("../../../", import.meta.url));
const out = resolve(options.out!);
const version = options.version!;
assertSourceRevision("source-sha", options["source-sha"]!, repo);
const module = moduleName(target.go);
const moduleRoot = join(out, "module");
await cp(join(out, "package"), moduleRoot, { recursive: true, errorOnExist: true, force: false });
await writeFile(join(moduleRoot, "go.mod"), `module ${module}\n\ngo ${nativeGoVersion}\n`, { flag: "wx" });
for (const [from, to] of [
  [join(repo, "LICENSE"), "LICENSE"],
  [join(repo, "NOTICE"), "NOTICE"],
  [join(repo, "rust/native/LICENSE-APACHE"), "LICENSE-GPUI"],
  [join(repo, "rust/native/NOTICE"), "NOTICE-GPUI"],
  [join(repo, "rust/native/Cargo.lock"), "Cargo-native.lock"],
  [join(resolve(options["bao-root"]!), "LICENSE"), "LICENSE-BAO"],
  [join(out, "cargo-metadata.json"), "cargo-metadata.json"],
] as const) {
  await copyFile(from, join(moduleRoot, to));
}
const library = await readFile(join(out, "assets", libraryAssetName(version, target)));
const bao = JSON.parse(await readFile(join(out, "bao.json"), "utf8")) as { commit: string; version: string; executable_sha256: string };
const goVersion = (await run("go", ["env", "GOVERSION"], { env: nativeGoEnvironment() })).stdout.trim();
const rustVersion = (await run("rustc", ["--version"])).stdout.trim();
await writeFile(join(moduleRoot, "publication.json"), JSON.stringify({
  schema: 1, module, version, target: target.go, rust_target: target.rust, source_commit: options["source-sha"],
  bao_commit: bao.commit, bao_version: bao.version, bao_executable_sha256: bao.executable_sha256,
  native_archive_sha256: createHash("sha256").update(library).digest("hex"),
  go: goVersion, consumer_go: nativeGoVersion, rust: rustVersion,
  ...(process.platform === "darwin"
    ? { macos_sdk: (await run("xcrun", ["--sdk", "macosx", "--show-sdk-version"])).stdout.trim() } : {}),
}, null, 2) + "\n", { flag: "wx" });
console.log(JSON.stringify(await packageRelease({ root: moduleRoot, module, version, target: target.go,
  repository: options.repository!, time: options.time!, out: join(out, "assets") }), null, 2));
