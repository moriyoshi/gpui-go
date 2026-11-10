import { resolve } from "node:path";
import { pathToFileURL } from "node:url";
import { latestVersion, nativeTable } from "../worker.ts";
import { checkRelease } from "./check-deployment.ts";

// Choose the version the Worker advertises: an explicit one, or Go's @latest
// among the published release tags on stdin. Either must carry every target.
if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  let version = process.argv[2];
  if (!version) {
    let input = "";
    for await (const chunk of process.stdin) input += chunk;
    const tags = input.split(/\s+/).filter(Boolean);
    version = latestVersion(tags);
    if (!version) throw new Error("No published release has a canonical version tag");
  }
  await checkRelease(nativeTable, version, (request) => fetch(request));
  console.log(version);
}
