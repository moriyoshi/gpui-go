import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { isReleaseVersion } from "../worker.ts";

// A release tag must be a version the Worker will serve for the module paths
// in native-targets.json, which have no /vN suffix and so stop at v1.
export function checkReleaseVersion(version: string): void {
  if (!isReleaseVersion(version)) {
    throw new Error(`${version} is not a canonical release version such as v0.1.0 or v1.0.0-rc.1`);
  }
  if (!/^v[01]\./.test(version)) {
    throw new Error(`${version} would need a /v${version.slice(1).split(".")[0]} suffix on the native module paths`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const version = process.argv[2];
  if (!version) throw new Error("Usage: check-version.ts VERSION");
  checkReleaseVersion(version);
}
