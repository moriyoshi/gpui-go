import { execFile } from "node:child_process";
import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import { promisify } from "node:util";
import type { ReleaseMetadata } from "./release.ts";
import { flags } from "./release.ts";
import { nativeGoEnvironment, nativeGoVersion } from "./profile.ts";
import { hostTarget, nativeTarget, targetSlug } from "./targets.ts";

const run = promisify(execFile);
const options = flags(process.argv.slice(2), ["release", "target", "out", "binary"]);
const target = nativeTarget(options.target!);
const release = resolve(options.release!);
const out = resolve(options.out!);
const metadata = JSON.parse(await readFile(join(release, `assets/native-release-${targetSlug(target.go)}.json`), "utf8")) as ReleaseMetadata;
const module = metadata.entry.path;
const [version, entry] = Object.entries(metadata.entry.versions)[0]!;
if (metadata.target !== target.go || hostTarget() !== target.go) {
  throw new Error(`Native smoke of ${target.go} must run on that platform; host is ${hostTarget()}`);
}
const [goos, goarch] = target.go.split("/") as [string, string];
const isDarwin = goos === "darwin";
const executable = (path: string): string => goos === "windows" ? path + ".exe" : path;
const goVersion = (await run("go", ["env", "GOVERSION"], {
  env: nativeGoEnvironment(),
})).stdout.trim();
if (goVersion !== `go${nativeGoVersion}`) throw new Error(`Native smoke requires Go ${nativeGoVersion}; got ${goVersion}`);
await mkdir(out);
const proxy = join(out, "proxy");
const proxyModule = join(proxy, module, "@v");
await mkdir(proxyModule, { recursive: true });
await copyFile(join(release, "assets", metadata.zip.name), join(proxyModule, version + ".zip"));
await writeFile(join(proxyModule, version + ".mod"), entry.goMod);
await writeFile(join(proxyModule, version + ".info"), JSON.stringify({ Version: version, Time: entry.time }));
await writeFile(join(proxyModule, "list"), version + "\n");
await writeFile(join(out, "go.mod"), `module example.org/gpui-native-release-smoke\n\ngo ${nativeGoVersion}\n\nrequire ${module} ${version}\n`);
const template = await readFile(new URL("../../native-smoke/main.go.tmpl", import.meta.url), "utf8");
await writeFile(join(out, "main.go"), template.replaceAll("{{MODULE_PATH}}", module));
const env: NodeJS.ProcessEnv = {
  ...nativeGoEnvironment(),
  GOENV: "off", GOFLAGS: "", GOWORK: "off", GOTOOLCHAIN: "local", GOTELEMETRY: "off",
  CGO_ENABLED: "0", GOOS: goos, GOARCH: goarch, GOPROXY: pathToFileURL(proxy).href,
  // This unpublished local fixture cannot be in the public checksum database.
  GOSUMDB: "off", GOMODCACHE: join(out, "module-cache"),
  GOCACHE: fileURLToPath(new URL("../../../.cache/go-build", import.meta.url)),
  // Go requires absolute compiler paths; a rooted path lacks a drive on Windows.
  CC: resolve("/gpui-release-no-cc"), CXX: resolve("/gpui-release-no-cxx"), RUSTC: resolve("/gpui-release-no-rustc"),
};
const commands: string[] = [];
async function command(file: string, args: string[], environment = env, cwd = out): Promise<void> {
  commands.push([file, ...args].join(" "));
  const log = join(out, `check-${commands.length}.log`);
  try {
    const result = await run(file, args, { cwd, env: environment, timeout: 600_000, maxBuffer: 16 * 1024 * 1024 });
    await writeFile(log, result.stdout + result.stderr);
  } catch (error) {
    const failure = error as Error & { stdout?: string; stderr?: string };
    await writeFile(log, (failure.stdout ?? "") + (failure.stderr ?? "") + String(failure));
    throw error;
  }
}
await command("go", ["mod", "download", "-json", `${module}@${version}`]);
await command("go", ["mod", "verify"]);
const binary = executable(resolve(options.binary!));
await writeFile(binary, "", { flag: "wx" });
await command("go", ["build", "-mod=readonly", "-o", binary, "."]);
for (const count of ["1", "4"]) await command(binary, [], { ...env, GOMAXPROCS: count });
if (isDarwin) {
  await command("codesign", ["--force", "--sign", "-", "--options", "runtime", "--timestamp=none", binary]);
  await command("codesign", ["--verify", "--strict", "--verbose=2", binary]);
  await command(binary, []);
}

// Build the real host through the umbrella shim against the same module. The
// shim's other targets are never loaded: their files are excluded by GOOS/GOARCH.
const repo = fileURLToPath(new URL("../../../", import.meta.url));
const modfile = join(out, "umbrella.mod");
await copyFile(join(repo, "go.mod"), modfile);
await copyFile(join(repo, "go.sum"), join(out, "umbrella.sum"));
const umbrellaEnv: NodeJS.ProcessEnv = {
  ...env, GOPROXY: `${pathToFileURL(proxy).href},https://proxy.golang.org`,
  GOSUMDB: "sum.golang.org", GONOSUMDB: module, GONOSUMCHECK: module,
};
await command("go", ["mod", "edit", "-go=" + nativeGoVersion, "-toolchain=none", "-require=" + module + "@" + version, modfile], umbrellaEnv, repo);
const consumer = executable(resolve(options.binary!) + "-counter");
await command("go", ["build", "-mod=mod", "-modfile=" + modfile, "-tags", "gpui,gpui_prebuilt,gpui_headless", "-o", consumer, "./examples/counter"], umbrellaEnv, repo);
await command(consumer, ["-smoke"], { ...umbrellaEnv, GOMAXPROCS: "4" });

// The driver fails unless the archive advertises automation. Only Linux can
// run it on every runner, because GPUI there starts without a display server.
const driver = executable(resolve(options.binary!) + "-uitest");
await command("go", ["build", "-mod=mod", "-modfile=" + modfile, "-tags", "gpui,gpui_prebuilt,gpui_e2e", "-o", driver, "./examples/uitest"], umbrellaEnv, repo);
if (goos === "linux") await command(driver, [], umbrellaEnv, repo);
await writeFile(join(out, "commands.json"), JSON.stringify(commands, null, 2) + "\n");
console.log("PASS: native module ZIP downloaded, built without native compilers, initialized/torn down, the host ran headless through the umbrella shim, and the uitest driver linked");
