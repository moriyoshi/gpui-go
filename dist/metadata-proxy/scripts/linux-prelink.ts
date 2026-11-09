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
const STT_FUNC = 2;
const SHN_LORESERVE = 0xff00;

// Go's findfunc table holds about 256 functions per 4 KiB. LLVM's identical
// code folding leaves hundreds of aliases on one body, so retarget relocations
// to one symbol per body and let the unreferenced aliases be stripped.
export function foldFunctionAliases(object: Buffer, exports: ReadonlySet<string>): number {
  const sectionTable = Number(object.readBigUInt64LE(0x28));
  const sectionSize = object.readUInt16LE(0x3a);
  const count = object.readUInt16LE(0x3c);
  const section = (index: number) => {
    const at = sectionTable + index * sectionSize;
    return { type: object.readUInt32LE(at + 4), offset: Number(object.readBigUInt64LE(at + 0x18)),
      size: Number(object.readBigUInt64LE(at + 0x20)), link: object.readUInt32LE(at + 0x28) };
  };
  const sections = Array.from({ length: count }, (_, index) => section(index));
  const symtab = sections.find((candidate) => candidate.type === SHT_SYMTAB);
  if (!symtab) throw new Error("Combined object has no symbol table");
  const strings = sections[symtab.link]!;
  const nameOf = (at: number): string => {
    const start = strings.offset + object.readUInt32LE(at);
    return object.toString("latin1", start, object.indexOf(0, start));
  };
  const canonical = new Map<string, number>();
  const redirect = new Map<number, number>();
  const symbols = symtab.size / 24;
  // Exports go first so they are always the symbol that survives.
  const order = [...Array(symbols).keys()].sort((a, b) =>
    Number(exports.has(nameOf(symtab.offset + b * 24))) - Number(exports.has(nameOf(symtab.offset + a * 24))));
  for (const index of order) {
    const at = symtab.offset + index * 24;
    const shndx = object.readUInt16LE(at + 6);
    if ((object[at + 4]! & 0xf) !== STT_FUNC || shndx === 0 || shndx >= SHN_LORESERVE) continue;
    const key = `${shndx}:${object.readBigUInt64LE(at + 8)}:${object.readBigUInt64LE(at + 16)}`;
    const first = canonical.get(key);
    if (first === undefined) canonical.set(key, index);
    else redirect.set(index, first);
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
  writeFileSync(merged, object);
  // Two passes: objcopy decides what is unneeded before applying localization,
  // and it always keeps global symbols of a relocatable object.
  execFileSync("objcopy", ["--keep-global-symbols=" + keep, merged], { stdio: "inherit" });
  execFileSync("objcopy", ["--strip-unneeded", "--remove-section=.llvmbc", "--remove-section=.llvmcmd",
    "--remove-section=.comment", merged, combined], { stdio: "inherit" });
  console.log(`Prelinked ${library}: folded ${folded} function aliases`);
  return combined;
}
