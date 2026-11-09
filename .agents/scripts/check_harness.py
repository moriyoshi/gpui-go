#!/usr/bin/env python3
"""Check the local agent harness without Git, network, or third-party packages."""

import argparse
from pathlib import Path
import re
from urllib.parse import unquote, urlsplit


REQUIRED = (
    "AGENTS.md",
    "./ARCHITECTURE.md",
    ".agents/.gitignore",
    ".agents-workspace/.gitignore",
    ".agents/docs/OVERVIEW.md",
    ".agents/docs/QUALITY_GATE.md",
    ".agents/docs/TESTING.md",
    ".agents/docs/TODO.md",
    ".agents/docs/JOURNAL.md",
    ".agents/docs/HARNESS.md",
    ".agents/docs/LTM/INDEX.md",
)


def prose_lines(text):
    """Exclude fenced/inline code so literal Unicode examples stay untouched."""
    fence_char, fence_length = None, 0
    for number, line in enumerate(text.splitlines(), 1):
        fence = re.match(r"^\s*(`{3,}|~{3,})(.*)$", line)
        if fence:
            marker, suffix = fence.groups()
            if fence_char is None:
                fence_char, fence_length = marker[0], len(marker)
            elif marker[0] == fence_char and len(marker) >= fence_length and not suffix.strip():
                fence_char = None
            continue
        if fence_char is None:
            yield number, re.sub(r"(`+).*?\1", "", line)


def check_document(path, root):
    errors = []
    for number, line in prose_lines(path.read_text()):
        for match in re.finditer(r"!?\[[^\]\n]*\]\(([^)\n]+)\)", line):
            raw = match.group(1).strip()
            if not raw:
                continue
            target = raw[1:raw.find(">")] if raw.startswith("<") else raw.split()[0]
            try:
                url = urlsplit(target)
            except ValueError:
                errors.append(f"{path}:{number}: malformed link target: {target}")
                continue
            if url.scheme or url.netloc or not url.path:
                continue
            destination = (
                root / unquote(url.path).lstrip("/")
                if url.path.startswith("/")
                else path.parent / unquote(url.path)
            )
            if not destination.exists():
                errors.append(f"{path}:{number}: missing link target: {target}")
        # URL contents and inline code are literal interfaces, not prose.
        prose = re.sub(r"https?://\S+", "", line)
        if any(character in prose for character in map(chr, (0xFF08, 0xFF09, 0xFF1A))):
            errors.append(f"{path}:{number}: full-width punctuation in prose")
    return errors


def check_skill(path):
    errors = []
    text = path.read_text()
    header = re.match(r"\A---\n(.*?)\n---(?:\n|$)", text, re.DOTALL)
    if header is None:
        return [f"{path}: missing or unclosed skill frontmatter"]
    # The maintained skills use single-line scalar name/description fields.
    # This checks that contract, not arbitrary YAML syntax or block scalars.
    fields = dict(re.findall(r"^([a-z-]+):\s*(.*?)\s*$", header.group(1), re.MULTILINE))
    name = fields.get("name", "").strip("\"'")
    description = fields.get("description", "").strip("\"'")
    if name != path.parent.name or len(name) > 64 or not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", name):
        errors.append(f"{path}: skill name must match its directory in kebab-case")
    if not description or len(description) > 1024 or description in ("|", ">"):
        errors.append(f"{path}: missing or invalid single-line description")
    if not text[header.end():].strip():
        errors.append(f"{path}: missing skill instructions")
    return errors


def check(root):
    errors = [f"missing harness file: {name}" for name in REQUIRED if not (root / name).is_file()]
    skills = sorted((root / ".agents/skills").glob("*/SKILL.md"))
    if not skills:
        errors.append("no repository skills found")
    documents = {root / "AGENTS.md", root / "ARCHITECTURE.md", root / "README.md", root / "rust/zed-editor/README.md"}
    documents.update((root / ".agents/docs").rglob("*.md"))
    documents.update(skills)
    for path in sorted(documents):
        if path.is_file():
            errors.extend(check_document(path, root))
    for path in skills:
        errors.extend(check_skill(path))
    return errors, len(documents), len(skills)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    errors, documents, skills = check(args.root.resolve())
    if errors:
        print("\n".join(errors))
        return 1
    print(f"Harness OK: {documents} documents, {skills} skills; local link targets and metadata checked")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
