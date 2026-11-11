import { execFileSync } from "node:child_process";
import { resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { checkReleaseVersion } from "./check-version.ts";
import { moduleName, nativeTargets } from "./targets.ts";

// Checks `go mod edit -json` output: a version tag must require every native
// module at its own version, as the finish-release script makes it.
export function checkNativeRequirements(goModJSON: string, version: string): void {
  const required = new Map((JSON.parse(goModJSON) as { Require?: { Path: string; Version: string }[] }).Require
    ?.map((requirement) => [requirement.Path, requirement.Version]) ?? []);
  const wrong = nativeTargets.map((target) => moduleName(target.go))
    .filter((module) => required.get(module) !== version)
    .map((module) => `${module} ${required.get(module) ?? "(missing)"}`);
  if (wrong.length) throw new Error(`go.mod must require the native modules at ${version}: ${wrong.join(", ")}`);
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const version = process.argv[2];
  if (!version) throw new Error("Usage: check-tag.ts VERSION");
  checkReleaseVersion(version);
  const repo = fileURLToPath(new URL("../../../", import.meta.url));
  checkNativeRequirements(execFileSync("go", ["mod", "edit", "-json"], { cwd: repo, encoding: "utf8" }), version);
}
