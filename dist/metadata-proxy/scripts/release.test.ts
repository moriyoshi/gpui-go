import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test, { after } from "node:test";
import { unzipSync, strFromU8 } from "fflate";
import { packageRelease, type PackageOptions } from "./release.ts";
import { mergeCatalog } from "./catalog.ts";
import { rewriteDarwinImports } from "./darwin-imports.ts";
import { checkDeployment } from "./check-deployment.ts";
import { foldFunctionAliases } from "./linux-prelink.ts";
import { createSite, createWorker } from "../worker.ts";
import { assertPublishablePrefix, libraryAssetName, moduleName, modulePrefix, nativeTarget, nativeTargets, selectTargets, sysoAssetName } from "./targets.ts";

const fixtures: string[] = [];
after(async () => { await Promise.all(fixtures.map((path) => rm(path, { recursive: true, force: true }))); });

async function fixture(): Promise<PackageOptions> {
  const dir = await mkdtemp(join(tmpdir(), "gpui-module-publication-"));
  fixtures.push(dir);
  const root = join(dir, "module");
  await mkdir(root);
  const module = "native.example.org/gpui/darwin-arm64";
  await writeFile(join(root, "go.mod"), `module ${module}\n\ngo 1.26.5\n`);
  await writeFile(join(root, "bindings.go"), "package native\n");
  // Opaque ZIP fixture only. These bytes are never linked or executed.
  await writeFile(join(root, "native_0_darwin_arm64.syso"), "object fixture");
  await writeFile(join(root, "LICENSE"), "fixture license\n");
  await writeFile(join(root, "NOTICE"), "fixture notice\n");
  return { root, module, version: "v0.1.0", target: "darwin/arm64", repository: "moriyoshi/gpui-go",
    time: "2026-11-08T00:00:00Z", out: join(dir, "assets") };
}

test("module ZIP contains original bytes under the versioned module prefix", async () => {
  const options = await fixture();
  const metadata = await packageRelease(options);
  const zip = await readFile(join(options.out, metadata.zip.name));
  const files = unzipSync(zip);
  const prefix = `${options.module}@${options.version}/`;
  assert.equal(Object.keys(files).length, 5);
  assert.ok(Object.keys(files).every((name) => name.startsWith(prefix)));
  assert.equal(strFromU8(files[prefix + "go.mod"]!), await readFile(join(options.root, "go.mod"), "utf8"));
  assert.equal(strFromU8(files[prefix + "native_0_darwin_arm64.syso"]!), "object fixture");
  assert.equal(metadata.entry.versions[options.version]!.asset, metadata.zip.name);
  const repeated = await packageRelease({ ...options, out: options.out + "-repeat" });
  assert.equal(repeated.zip.sha256, metadata.zip.sha256);
  await assert.rejects(packageRelease(options), /EEXIST/);
});

test("packager rejects mismatched object targets and symlinks", async () => {
  const wrong = await fixture();
  await writeFile(join(wrong.root, "native_linux_arm64.syso"), "wrong target");
  await assert.rejects(packageRelease(wrong), /does not match/);
  const linked = await fixture();
  await symlink("LICENSE", join(linked.root, "license-link"));
  await assert.rejects(packageRelease(linked), /Symlink/);
});

test("packager rejects nested modules and missing notices", async () => {
  const nested = await fixture();
  await mkdir(join(nested.root, "nested"));
  await writeFile(join(nested.root, "nested/go.mod"), "module example.org/other\n");
  await assert.rejects(packageRelease(nested), /Nested module/);
  const missing = await fixture();
  // The mutation is scoped to a fixture this test created.
  const { unlink } = await import("node:fs/promises");
  await unlink(join(missing.root, "NOTICE"));
  await assert.rejects(packageRelease(missing), /LICENSE and NOTICE/);
});

test("packager rejects case collisions", async (context) => {
  const collision = await fixture();
  await writeFile(join(collision.root, "Bindings.go"), "package native\n");
  const { readdir } = await import("node:fs/promises");
  const names = await readdir(collision.root);
  if (!names.includes("bindings.go") || !names.includes("Bindings.go")) {
    context.skip("Needs a case-sensitive filesystem");
    return;
  }
  await assert.rejects(packageRelease(collision), /Case-colliding/);
});

test("catalog merge preserves old releases and refuses changed existing metadata", async () => {
  const first = await packageRelease(await fixture());
  const existing = mergeCatalog([], [first]);
  const nextOptions = await fixture();
  const next = await packageRelease({ ...nextOptions, version: "v0.2.0" });
  const merged = mergeCatalog(existing, [next]);
  assert.deepEqual(Object.keys(merged[0]!.versions), ["v0.1.0", "v0.2.0"]);
  assert.deepEqual(mergeCatalog(existing, [first]), existing);
  const changed = { ...first, entry: { ...first.entry,
    versions: { "v0.1.0": { ...first.entry.versions["v0.1.0"]!, time: "2026-11-09T00:00:00Z" } } } };
  assert.throws(() => mergeCatalog(existing, [changed]), /Refusing to change published/);
  assert.throws(() => mergeCatalog(existing, [{ ...first, target: "linux/arm64" }]), /Invalid ZIP/);
});

test("library and syso asset names follow the target table", () => {
  assert.equal(libraryAssetName("v0.1.0", nativeTarget("darwin/arm64")), "gpui-native-v0.1.0-darwin-arm64.a");
  assert.equal(libraryAssetName("v0.1.0", nativeTarget("windows/amd64")), "gpui-native-v0.1.0-windows-amd64.lib");
  assert.equal(sysoAssetName("v0.1.0", "linux/arm64"), "gpui-native-v0.1.0-linux-arm64-syso.zip");
});

test("module paths nest GOOS and GOARCH under the prefix", () => {
  assert.equal(moduleName("darwin/arm64"), `${modulePrefix}/darwin/arm64`);
  assert.equal(moduleName("windows/amd64"), `${modulePrefix}/windows/amd64`);
});

test("target selection accepts all or an explicit list and rejects unknown targets", () => {
  assert.equal(selectTargets("all").length, nativeTargets.length);
  assert.deepEqual(selectTargets("linux/amd64, darwin/arm64").map((target) => target.go), ["linux/amd64", "darwin/arm64"]);
  assert.throws(() => selectTargets("plan9/amd64"), /Unknown native target/);
});

test("placeholder module prefixes cannot be published", () => {
  assert.throws(() => assertPublishablePrefix("native.example.org/gpui"), /modulePrefix/);
  assert.doesNotThrow(() => assertPublishablePrefix("go.example-corp.dev/gpui"));
});

test("Darwin imports bind through umbrellas and leave other libraries alone", () => {
  const seen = new Set<string>();
  const source = [
    '//go:cgo_import_dynamic bao_import_1 CTLineDraw "/System/Library/Frameworks/CoreText.framework/Versions/A/CoreText"',
    '//go:cgo_import_dynamic bao_import_2 NSLog "/System/Library/Frameworks/Foundation.framework/Versions/C/Foundation"',
    '//go:cgo_import_dynamic _ _ "/usr/lib/libobjc.A.dylib"',
    "func untouched() {}",
  ].join("\n");
  const lines = rewriteDarwinImports(source, seen).split("\n");
  assert.match(lines[0]!, /CTLineDraw "\/System\/Library\/Frameworks\/ApplicationServices\.framework/);
  assert.match(lines[1]!, /NSLog "\/System\/Library\/Frameworks\/Cocoa\.framework/);
  assert.equal(lines[2], '//go:cgo_import_dynamic _ _ "/usr/lib/libobjc.A.dylib"');
  assert.equal(lines[3], "func untouched() {}");
  assert.equal(seen.size, 3);
});

test("deployment check follows the routes Go uses and detects a missing asset", async () => {
  const entry = { path: "native.example.org/gpui/darwin-arm64", repository: "moriyoshi/gpui-go", versions: { "v0.1.0": {
    time: "2026-11-09T00:00:00Z", tag: "v0.1.0", asset: "gpui-native-v0.1.0-darwin-arm64.zip",
    goMod: "module native.example.org/gpui/darwin-arm64\n\ngo 1.26.10\n" } } };
  const worker = createSite(createWorker([entry]));
  const page = (status: number) => ({ ASSETS: { fetch: async () => new Response("docs", { status, headers: { "Content-Type": "text/html" } }) } });
  const fetcher = (assetStatus: number, docsStatus = 200) => async (request: Request): Promise<Response> =>
    request.url.startsWith("https://github.com/") ? new Response(null, { status: assetStatus }) : worker.fetch(request, page(docsStatus));
  assert.equal(await checkDeployment("https://native.example.org", [entry], fetcher(200)), 1);
  await assert.rejects(checkDeployment("https://native.example.org", [entry], fetcher(404)), /unavailable \(404\)/);
  await assert.rejects(checkDeployment("https://other.example.org", [entry], fetcher(200)), /did not advertise/);
  await assert.rejects(checkDeployment("https://native.example.org", [entry], fetcher(200, 404)), /home page is unavailable \(404\)/);
});

test("function aliases fold onto one symbol, preferring exports", () => {
  // ELF64: header, section table, symtab (null + 4 FUNCs), strtab, rela.
  const names = "\0alias_a\0exported\0other\0data\0";
  const offset = (name: string): number => names.indexOf(`\0${name}\0`) + 1;
  const elf = Buffer.alloc(64 + 40 * 4 + 24 * 5 + names.length + 24 * 3);
  const section = (index: number, type: number, at: number, size: number, link = 0): void => {
    const base = 64 + index * 40;
    elf.writeUInt32LE(type, base + 4);
    elf.writeBigUInt64LE(BigInt(at), base + 0x18);
    elf.writeBigUInt64LE(BigInt(size), base + 0x20);
    elf.writeUInt32LE(link, base + 0x28);
  };
  elf.writeBigUInt64LE(64n, 0x28);
  elf.writeUInt16LE(40, 0x3a);
  elf.writeUInt16LE(4, 0x3c);
  const symtab = 64 + 160, strtab = symtab + 120, rela = strtab + names.length;
  section(1, 2, symtab, 120, 2);
  section(2, 3, strtab, names.length);
  section(3, 4, rela, 72, 1);
  elf.write(names, strtab, "latin1");
  const symbol = (index: number, name: string, info: number, value: bigint): void => {
    const at = symtab + index * 24;
    elf.writeUInt32LE(offset(name), at);
    elf[at + 4] = info;
    elf.writeUInt16LE(5, at + 6);
    elf.writeBigUInt64LE(value, at + 8);
    elf.writeBigUInt64LE(16n, at + 16);
  };
  symbol(1, "alias_a", 0x12, 0x100n);
  symbol(2, "exported", 0x12, 0x100n);
  symbol(3, "other", 0x12, 0x200n);
  symbol(4, "data", 0x11, 0x100n);
  for (const [index, target] of [1, 3, 4].entries()) elf.writeUInt32LE(target, rela + index * 24 + 12);
  assert.equal(foldFunctionAliases(elf, new Set(["exported"])), 1);
  assert.deepEqual([0, 1, 2].map((index) => elf.readUInt32LE(rela + index * 24 + 12)), [2, 3, 4]);
});
