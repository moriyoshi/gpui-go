import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

export interface NativeTarget {
  go: string;
  rust: string;
  runner: string;
  library: "a" | "lib";
}

const table = JSON.parse(readFileSync(new URL("../../native-targets.json", import.meta.url), "utf8")) as {
  modulePrefix: string;
  targets: NativeTarget[];
};

export const modulePrefix = table.modulePrefix;
export const nativeTargets: readonly NativeTarget[] = table.targets;

export function nativeTarget(go: string): NativeTarget {
  const target = nativeTargets.find((candidate) => candidate.go === go);
  if (!target) throw new Error(`Unknown native target: ${go}`);
  return target;
}

export const targetSlug = (go: string): string => go.replace("/", "-");

// Module paths nest GOOS and GOARCH under the prefix, as in gpui-go.dev/__binary/darwin/arm64.
export const moduleName = (go: string): string => `${modulePrefix}/${go}`;

// Stage 1 assets keep one name per target; Windows archives use the .lib suffix.
export const libraryAssetName = (version: string, target: NativeTarget): string =>
  `gpui-native-${version}-${targetSlug(target.go)}.${target.library}`;

export const sysoAssetName = (version: string, go: string): string =>
  `gpui-native-${version}-${targetSlug(go)}-syso.zip`;

export const hostTarget = (): string =>
  `${process.platform === "win32" ? "windows" : process.platform}/${process.arch === "x64" ? "amd64" : process.arch}`;

export function selectTargets(selection: string): NativeTarget[] {
  if (selection === "all") return [...nativeTargets];
  return selection.split(",").map((go) => nativeTarget(go.trim()));
}

export const isPlaceholderPrefix = (prefix: string): boolean => /(^|\.)example\.(org|com|net)(\/|$)/.test(prefix);

export function assertPublishablePrefix(prefix: string): void {
  if (isPlaceholderPrefix(prefix)) {
    throw new Error(`Set modulePrefix in dist/native-targets.json and regenerate the shim before publishing: ${prefix}`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  if (process.argv[2] === "--domain") {
    console.log(isPlaceholderPrefix(modulePrefix) ? "" : modulePrefix.split("/")[0]);
    process.exit(0);
  }
  if (process.env.REQUIRE_PUBLISHABLE === "true") assertPublishablePrefix(modulePrefix);
  const include = selectTargets(process.argv[2] ?? "all").map((target) => ({ ...target, slug: targetSlug(target.go) }));
  console.log(JSON.stringify({ include }));
}
