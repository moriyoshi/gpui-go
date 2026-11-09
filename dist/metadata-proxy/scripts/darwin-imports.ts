import { readdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

// Stock Go's Mach-O binder writes dylib ordinals 1-127 as a 4-bit immediate, so
// ordinal 16 becomes 0 ("this image") and dyld aborts at launch. Bao's package
// already names 15 dylibs, and a consumer reaching package net adds libresolv.
// Bind through umbrellas that re-export these frameworks to leave room.
const framework = (name: string, version = "A"): string => `/System/Library/Frameworks/${name}.framework/Versions/${version}/${name}`;
const umbrellas: ReadonlyMap<string, string> = new Map([
  [framework("AppKit", "C"), framework("Cocoa")],
  [framework("Foundation", "C"), framework("Cocoa")],
  [framework("CoreGraphics"), framework("ApplicationServices")],
  [framework("CoreText"), framework("ApplicationServices")],
  [framework("ColorSync"), framework("ApplicationServices")],
]);

// One ordinal is reserved for libresolv, and the executable itself never uses one.
const MAX_PACKAGE_DYLIBS = 14;

const importLine = /^(\/\/go:cgo_import_dynamic\s+\S+(?:\s+\S+)?\s+")([^"]+)(".*)$/gm;

export function rewriteDarwinImports(source: string, seen: Set<string>): string {
  return source.replace(importLine, (_line, head: string, path: string, tail: string) => {
    const library = umbrellas.get(path) ?? path;
    seen.add(library);
    return head + library + tail;
  });
}

function rewriteManifestLibraries(manifest: string): string {
  let rewritten = manifest;
  for (const [from, to] of umbrellas) rewritten = rewritten.replaceAll(`"${from}"`, `"${to}"`);
  return rewritten;
}

export async function consolidateDarwinImports(packageRoot: string): Promise<void> {
  const libraries = new Set<string>();
  for (const name of (await readdir(packageRoot)).filter((file) => file.endsWith(".go") || file === "bao-manifest.json").sort()) {
    const path = join(packageRoot, name);
    const source = await readFile(path, "utf8");
    const rewritten = name.endsWith(".go") ? rewriteDarwinImports(source, libraries) : rewriteManifestLibraries(source);
    if (rewritten !== source) await writeFile(path, rewritten);
  }
  if (libraries.size > MAX_PACKAGE_DYLIBS) {
    throw new Error(`Generated package imports ${libraries.size} dylibs; Go's Mach-O binder only keeps ${MAX_PACKAGE_DYLIBS} safely`);
  }
}
