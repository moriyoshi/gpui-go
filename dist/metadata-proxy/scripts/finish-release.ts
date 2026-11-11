// Pairs the root module with a drafted native release, then publishes it.
//
// The draft vX was built from commit C and has no tag yet. This script requires
// the native modules at vX from the draft's own ZIPs, commits the go.mod/go.sum
// change as C' on top of C, signs tag vX on C', pushes both to origin and
// publishes the draft on that tag. Without --publish yes it stops after leaving
// the change in the working tree.
import { execFile } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";
import { createHash } from "node:crypto";
import { strFromU8, unzipSync } from "fflate";
import type { ReleaseMetadata } from "../types.ts";
import { metadataAssetName, nativeTable, releaseAssetName, validateRelease } from "../worker.ts";
import { checkReleaseVersion } from "./check-version.ts";
import { checkRelease, checkVersionRoutes } from "./check-deployment.ts";
import { checkNativeRequirements } from "./check-tag.ts";
import { nativeGoEnvironment } from "./profile.ts";
import { flags } from "./release.ts";
import { hostTarget, moduleName, nativeTargets } from "./targets.ts";

const exec = promisify(execFile);
const repo = fileURLToPath(new URL("../../../", import.meta.url));
const options = flags(process.argv.slice(2), ["version"], ["publish"]);
const version = options.version!;
const publish = options.publish === "yes";
if (options.publish && !["yes", "no"].includes(options.publish)) throw new Error("--publish takes yes or no");
checkReleaseVersion(version);

async function output(file: string, args: string[], env: NodeJS.ProcessEnv = process.env): Promise<string> {
  const result = await exec(file, args, { cwd: repo, env, maxBuffer: 64 * 1024 * 1024 });
  return result.stdout.trim();
}
const git = (...args: string[]): Promise<string> => output("git", args);
const gh = (...args: string[]): Promise<string> => output("gh", args);

const release = JSON.parse(await gh("release", "view", version, "--json", "isDraft,targetCommitish,tagName")) as {
  isDraft: boolean; targetCommitish: string;
};
if (publish && !release.isDraft) throw new Error(`Release ${version} is already published`);
if (await git("status", "--porcelain")) throw new Error("The working tree must be clean");
const head = await git("rev-parse", "HEAD");

// A rerun after a failure past the commit finds the pairing commit on HEAD.
let source = head;
let resumed = false;
if (release.targetCommitish !== head) {
  const parent = await git("rev-parse", "HEAD^");
  const changed = (await git("diff", "--name-only", "HEAD^", "HEAD")).split("\n").sort().join(",");
  if (!publish || parent !== release.targetCommitish || changed !== "go.mod,go.sum") {
    throw new Error(`Draft ${version} was built from ${release.targetCommitish}; check out that commit (HEAD is ${head})`);
  }
  checkNativeRequirements(await output("go", ["mod", "edit", "-json"], nativeGoEnvironment()), version);
  source = parent;
  resumed = true;
}
if (publish) {
  // The push fast-forwards main to C', which a resumed run may have pushed already.
  await git("fetch", "--quiet", "origin", "main");
  const main = await git("rev-parse", "origin/main");
  if (!(resumed && main === head) && !await isAncestor(main, source)) {
    throw new Error("origin/main has moved past the release commit; build a new release from main");
  }
  const remoteTag = await peeledRemoteTag();
  if (remoteTag && !(resumed && remoteTag === head)) {
    throw new Error(`Tag ${version} already exists on origin at ${remoteTag}`);
  }
}

if (!resumed) {
  const work = await mkdtemp(join(tmpdir(), "gpui-finish-release-"));
  try {
    await pairModules(work);
  } finally {
    await rm(work, { recursive: true, force: true });
  }
  if (!publish) {
    console.log(`Required the native modules at ${version} in go.mod and go.sum. Review with git diff; nothing was committed.`);
    process.exit(0);
  }
  await git("commit", "--quiet", "-m", `Require native modules ${version}`, "--", "go.mod", "go.sum");
}
const pairing = await git("rev-parse", "HEAD");
// Sign the tag with the user's git signing configuration (user.signingkey,
// gpg.format), before anything is pushed.
const local = await git("rev-parse", "--verify", "--quiet", `refs/tags/${version}^{commit}`).catch(() => "");
if (!local) await git("tag", "--sign", "--message", version, version, pairing);
else if (local !== pairing) throw new Error(`Local tag ${version} is at ${local}, not ${pairing}; delete it and rerun`);
if (await git("cat-file", "-t", `refs/tags/${version}`) !== "tag"
    || !/^-----BEGIN [A-Z ]*SIGNATURE-----$/m.test(await git("cat-file", "tag", `refs/tags/${version}`))) {
  throw new Error(`Local tag ${version} is not a signed annotated tag; delete it and rerun`);
}
// The tag exists before the release is published; Check release tag waits for that.
await git("push", "--quiet", "--atomic", "origin", "HEAD:refs/heads/main", `refs/tags/${version}`);
await gh("release", "edit", version, "--verify-tag", "--draft=false");
if (await peeledRemoteTag() !== pairing) throw new Error(`Published ${version}, but origin's tag is not at ${pairing}`);
console.log(`Published ${version}: signed tag ${version} is ${pairing}, which requires the native modules built from ${source}`);
await waitForWorker();
await warmMirror();

// The public Go mirror caches a failed fetch, so it is asked only once the
// Worker serves every module of the version.
async function waitForWorker(): Promise<void> {
  const origin = `https://${nativeTable.modulePrefix.split("/")[0]}`;
  const deadline = Date.now() + 10 * 60_000;
  for (;;) {
    try {
      const releases = await checkRelease(nativeTable, version, (request) => fetch(request));
      await checkVersionRoutes(origin, nativeTable, version, releases, (request) => fetch(request));
      console.log(`${origin} serves ${version} for every target`);
      return;
    } catch (error) {
      if (Date.now() > deadline) throw new Error(`${origin} does not serve ${version} yet; check the Worker before announcing it`, { cause: error });
      await new Promise((resolve) => setTimeout(resolve, 10_000));
    }
  }
}

// Reports, rather than fails on, a mirror answer: the release is already out.
async function warmMirror(): Promise<void> {
  const root = (JSON.parse(await output("go", ["mod", "edit", "-json"], nativeGoEnvironment())) as { Module: { Path: string } }).Module.Path;
  for (const module of [root, ...nativeTargets.map((target) => moduleName(target.go))]) {
    const response = await fetch(`https://proxy.golang.org/${module}/@v/${version}.info`);
    if (response.ok) continue;
    console.warn(`proxy.golang.org answered ${response.status} for ${module}@${version}: ${(await response.text()).trim()}`);
    console.warn("The mirror and sum.golang.org cache failures for a while; until then, go get needs GONOSUMDB=gpui-go.dev/__binary for these.");
  }
}

function isAncestor(ancestor: string, commit: string): Promise<boolean> {
  return git("merge-base", "--is-ancestor", ancestor, commit).then(() => true, () => false);
}

// The commit origin's tag names, or "" when origin has no such tag.
async function peeledRemoteTag(): Promise<string> {
  const lines = (await git("ls-remote", "--tags", "origin", `refs/tags/${version}`, `refs/tags/${version}^{}`)).split("\n");
  const peeled = lines.find((line) => line.endsWith("^{}")) ?? lines.find(Boolean) ?? "";
  return peeled.split("\t")[0] ?? "";
}

// Requires every native module at version, resolved from the draft's own ZIPs
// so that go.sum holds the hashes the public checksum database will compute.
async function pairModules(work: string): Promise<void> {
  const assets = join(work, "assets");
  const names = nativeTargets.flatMap((target) => [metadataAssetName(target.go), releaseAssetName(version, target.go)]);
  await gh("release", "download", version, "--dir", assets, ...names.flatMap((name) => ["--pattern", name]));
  const proxy = join(work, "proxy");
  for (const target of nativeTargets) {
    const metadata = JSON.parse(await readFile(join(assets, metadataAssetName(target.go)), "utf8")) as ReleaseMetadata;
    const published = validateRelease(nativeTable, target.go, version, metadata);
    const zip = await readFile(join(assets, metadata.zip.name));
    if (metadata.zip.name !== releaseAssetName(version, target.go) || zip.length !== metadata.zip.size
        || createHash("sha256").update(zip).digest("hex") !== metadata.zip.sha256) {
      throw new Error(`The ${target.go} module ZIP does not match its release metadata`);
    }
    const files = unzipSync(zip, { filter: (file) => file.name === `${metadata.entry.path}@${version}/publication.json` });
    const record = Object.values(files)[0];
    const commit = record && (JSON.parse(strFromU8(record)) as { source_commit?: string }).source_commit;
    if (commit !== source) throw new Error(`The ${target.go} module was built from ${commit ?? "an unknown commit"}, not ${source}`);
    const dir = join(proxy, metadata.entry.path, "@v");
    await mkdir(dir, { recursive: true });
    await writeFile(join(dir, version + ".zip"), zip);
    await writeFile(join(dir, version + ".mod"), published.goMod);
    await writeFile(join(dir, version + ".info"), JSON.stringify({ Version: version, Time: published.time }));
    await writeFile(join(dir, "list"), version + "\n");
  }
  const base = nativeGoEnvironment();
  const upstream = await output("go", ["env", "GOPROXY"], base);
  const env: NodeJS.ProcessEnv = {
    ...base, GOFLAGS: "-modcacherw", GOWORK: "off", GOMODCACHE: join(work, "module-cache"),
    GOPROXY: `${pathToFileURL(proxy).href},${upstream}`,
    // The draft's modules are not in the checksum database until publication.
    GONOSUMDB: nativeTable.modulePrefix,
  };
  await output("go", ["get", ...nativeTargets.map((target) => `${moduleName(target.go)}@${version}`)], env);
  await output("go", ["mod", "tidy"], env);
  const changed = [await git("diff", "--name-only", "HEAD"), await git("ls-files", "--others", "--exclude-standard")]
    .join("\n").split("\n").filter(Boolean).sort().join(",");
  if (changed !== "go.mod,go.sum") throw new Error(`Pairing changed ${changed}; expected only go.mod and go.sum`);
  checkNativeRequirements(await output("go", ["mod", "edit", "-json"], env), version);
  // The paired go.sum must be enough to build the host's own target from it.
  if (nativeTargets.some((target) => target.go === hostTarget())) {
    const binary = join(work, "counter");
    const strict = { ...env, GOFLAGS: "-modcacherw -mod=readonly", CGO_ENABLED: "0" };
    await output("go", ["build", "-tags", "gpui_prebuilt,gpui_headless", "-o", binary, "./examples/counter"], strict);
    await output(binary, ["-smoke"], strict);
  }
}
