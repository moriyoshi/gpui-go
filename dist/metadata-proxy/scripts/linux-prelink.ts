import { execFileSync } from "node:child_process";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

// GPUI's Linux archive has 334,676 sections, past both Bao's ELF reader and
// Go's. Keep only code reachable from the exports, then merge per-function
// sections. The padding and alias folding below fit Go's function table.
const SECTION_SCRIPT = `SECTIONS {
  .text 0 : SUBALIGN(32) { *(.text .text.*) }
  .rodata 0 : { *(.rodata .rodata.*) }
  .data.rel.ro 0 : { *(.data.rel.ro .data.rel.ro.*) }
  .data 0 : { *(.data .data.*) }
  .bss 0 : { *(.bss .bss.*) }
  .tdata 0 : { *(.tdata .tdata.*) }
  .tbss 0 : { *(.tbss .tbss.*) }
  .eh_frame 0 : { KEEP(*(.eh_frame .eh_frame.*)) }
  .gcc_except_table 0 : { *(.gcc_except_table .gcc_except_table.*) }
  .init_array 0 : { KEEP(*(SORT_BY_INIT_PRIORITY(.init_array.*))) KEEP(*(.init_array)) }
  .fini_array 0 : { KEEP(*(SORT_BY_INIT_PRIORITY(.fini_array.*))) KEEP(*(.fini_array)) }
}
`;

const SHT_SYMTAB = 2;
const SHT_RELA = 4;
const SHF_EXECINSTR = 0x4;
const STT_FUNC = 2;
const SHN_LORESERVE = 0xff00;

interface ElfSection {
  name: string;
  type: number;
  flags: number;
  offset: number;
  size: number;
  link: number;
}

interface ElfSymbol {
  index: number;
  at: number;
  name: string;
  func: boolean;
  section: number;
  value: bigint;
  size: bigint;
}

// Reads the section headers and symbol table of a 64-bit little-endian ELF
// relocatable object, which is all the prelink produces.
function readElf(object: Buffer): { sections: ElfSection[]; symbols: ElfSymbol[] } {
  const sectionTable = Number(object.readBigUInt64LE(0x28));
  const sectionSize = object.readUInt16LE(0x3a);
  const count = object.readUInt16LE(0x3c);
  const names = object.readUInt16LE(0x3e);
  const raw = Array.from({ length: count }, (_, index) => {
    const at = sectionTable + index * sectionSize;
    return { nameOffset: object.readUInt32LE(at), type: object.readUInt32LE(at + 4),
      flags: Number(object.readBigUInt64LE(at + 8)), offset: Number(object.readBigUInt64LE(at + 0x18)),
      size: Number(object.readBigUInt64LE(at + 0x20)), link: object.readUInt32LE(at + 0x28) };
  });
  const text = (table: number, offset: number) => {
    const start = raw[table]!.offset + offset;
    return object.toString("latin1", start, object.indexOf(0, start));
  };
  const sections = raw.map((section) => ({ ...section, name: names < count ? text(names, section.nameOffset) : "" }));
  const symtabIndex = sections.findIndex((candidate) => candidate.type === SHT_SYMTAB);
  if (symtabIndex < 0) throw new Error("Combined object has no symbol table");
  const symtab = sections[symtabIndex]!;
  const symbols = Array.from({ length: symtab.size / 24 }, (_, index) => {
    const at = symtab.offset + index * 24;
    return { index, at, name: text(symtab.link, object.readUInt32LE(at)), func: (object[at + 4]! & 0xf) === STT_FUNC,
      section: object.readUInt16LE(at + 6), value: object.readBigUInt64LE(at + 8), size: object.readBigUInt64LE(at + 16) };
  });
  return { sections, symbols };
}

// Go's findfunc table starts at the first function of the text, and a code
// section is a container that the table skips. Code before a section's first
// function symbol therefore leaves a hole Go's linker rejects. ld -r turns
// calls within the section into section-relative relocations, so
// --strip-unneeded may drop the symbol of whichever function lands first.
// sectionStartFunctions names one function at offset 0 of each code section;
// missingSectionStarts lists code sections without one.
export function sectionStartFunctions(object: Buffer): string[] {
  const { sections, symbols } = readElf(object);
  const names = new Map<number, string>();
  for (const symbol of symbols) {
    const section = sections[symbol.section];
    if (symbol.func && symbol.value === 0n && symbol.name && section && section.flags & SHF_EXECINSTR && !names.has(symbol.section)) {
      names.set(symbol.section, symbol.name);
    }
  }
  return [...names.values()];
}

export function missingSectionStarts(object: Buffer): string[] {
  const { sections, symbols } = readElf(object);
  const starts = new Set(symbols.filter((symbol) => symbol.func && symbol.value === 0n).map((symbol) => symbol.section));
  return sections.flatMap((section, index) =>
    section.flags & SHF_EXECINSTR && section.size > 0 && !starts.has(index) ? [section.name || `#${index}`] : []);
}

// Go's findfunc table holds about 256 functions per 4 KiB. LLVM's identical
// code folding leaves hundreds of aliases on one body, so retarget relocations
// to one symbol per body and let the unreferenced aliases be stripped.
export function foldFunctionAliases(object: Buffer, exports: ReadonlySet<string>): number {
  const { sections, symbols } = readElf(object);
  const canonical = new Map<string, number>();
  const redirect = new Map<number, number>();
  // Exports go first so they are always the symbol that survives.
  const order = [...symbols].sort((a, b) => Number(exports.has(b.name)) - Number(exports.has(a.name)));
  for (const symbol of order) {
    if (!symbol.func || symbol.section === 0 || symbol.section >= SHN_LORESERVE) continue;
    const key = `${symbol.section}:${symbol.value}:${symbol.size}`;
    const first = canonical.get(key);
    if (first === undefined) canonical.set(key, symbol.index);
    else redirect.set(symbol.index, first);
  }
  for (const rela of sections.filter((candidate) => candidate.type === SHT_RELA)) {
    for (let at = rela.offset + 12; at < rela.offset + rela.size; at += 24) {
      const target = redirect.get(object.readUInt32LE(at));
      if (target !== undefined) object.writeUInt32LE(target, at);
    }
  }
  return redirect.size;
}

export function prelinkLinuxArchive(library: string, workDirectory: string, exports: readonly string[]): string {
  const script = join(workDirectory, "sections.ld");
  const merged = join(workDirectory, "merged.o");
  const keep = join(workDirectory, "exports.txt");
  const combined = join(workDirectory, "combined.o");
  writeFileSync(script, SECTION_SCRIPT);
  writeFileSync(keep, exports.join("\n") + "\n");
  execFileSync("ld", ["-r", "--gc-sections", "-e", exports[0]!, ...exports.flatMap((symbol) => ["-u", symbol]),
    "-T", script, "-o", merged, library], { stdio: "inherit" });
  const object = readFileSync(merged);
  const folded = foldFunctionAliases(object, new Set(exports));
  const starts = sectionStartFunctions(object);
  writeFileSync(merged, object);
  // Two passes: objcopy decides what is unneeded before applying localization,
  // and it always keeps global symbols of a relocatable object.
  execFileSync("objcopy", ["--keep-global-symbols=" + keep, merged], { stdio: "inherit" });
  execFileSync("objcopy", ["--strip-unneeded", ...starts.map((name) => "--keep-symbol=" + name),
    "--remove-section=.llvmbc", "--remove-section=.llvmcmd", "--remove-section=.comment", merged, combined], { stdio: "inherit" });
  const missing = missingSectionStarts(readFileSync(combined));
  if (missing.length) {
    throw new Error(`Code sections ${missing.join(", ")} start without a function symbol; Go's linker would report a hole in findfunctab`);
  }
  console.log(`Prelinked ${library}: folded ${folded} function aliases`);
  return combined;
}
