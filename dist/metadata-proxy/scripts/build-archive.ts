import { execFileSync, spawnSync } from "node:child_process";
import { copyFile, mkdir, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { flags, writeChecksum } from "./release.ts";
import { libraryAssetName, nativeTarget } from "./targets.ts";

// Stage 1: one static library holding the GPUI backend and Sable for a target.
const options = flags(process.argv.slice(2), ["target", "version", "out"]);
const target = nativeTarget(options.target!);
const repo = fileURLToPath(new URL("../../../", import.meta.url));
const out = resolve(options.out!);
const manifest = join(repo, "rust/native/Cargo.toml");
// Matches the Makefile; Cargo would otherwise default to a directory per crate.
const targetDir = process.env.CARGO_TARGET_DIR ? resolve(process.env.CARGO_TARGET_DIR) : join(repo, "target");
// Published modules include automation so consumers can run uitest; Go code
// still has to opt in with the gpui_e2e tag.
const build = spawnSync("cargo", ["build", "--locked", "--release", "--manifest-path", manifest, "--features", "automation", "--target", target.rust],
  { cwd: repo, stdio: "inherit", env: { ...process.env, CARGO_TARGET_DIR: targetDir } });
if (build.status !== 0) throw new Error(`cargo build failed for ${target.go}`);
const assets = join(out, "assets");
await mkdir(assets, { recursive: true });
const name = libraryAssetName(options.version!, target);
await copyFile(join(targetDir, target.rust, "release/libgpui_go_native.a"), join(assets, name));
await writeChecksum(assets, name);
await writeFile(join(out, "cargo-metadata.json"), execFileSync("cargo",
  ["metadata", "--locked", "--format-version", "1", "--filter-platform", target.rust, "--manifest-path", manifest],
  { cwd: repo, maxBuffer: 256 * 1024 * 1024 }), { flag: "wx" });
console.log(join(assets, name));
