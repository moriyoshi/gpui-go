#!/usr/bin/env python3
"""Compare checked-in GoX examples with isolated execution of go:generate."""

import argparse
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("sources", nargs="+", help="repository-relative .gox source paths")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    scratch = root / ".agents-workspace" / "tmp"
    scratch.mkdir(parents=True, exist_ok=True)
    versions = "\n".join(line for line in (root / "go.mod").read_text().splitlines() if line.startswith(("go ", "toolchain ")))
    for name in args.sources:
        source = root / name
        generated = source.with_suffix(".x.go")
        directives = source.parent / "generate.go"
        with tempfile.TemporaryDirectory(prefix="gox-check-", dir=scratch) as directory:
            temp = Path(directory)
            destination = temp / source.relative_to(root)
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source, destination)
            shutil.copyfile(directives, destination.parent / directives.name)
            (temp / "bin").mkdir()
            (temp / "bin" / "gox").symlink_to(root / "bin" / "gox")
            (temp / "go.mod").write_text("module example.invalid/gox-generation\n\n" + versions + "\n")
            # Execute the maintained directives, including their gofmt step.
            subprocess.run(["go", "generate", directives.name], cwd=destination.parent, check=True)
            expected = destination.with_suffix(".x.go")
            if not generated.exists() or expected.read_bytes() != generated.read_bytes():
                raise SystemExit(f"{generated.relative_to(root)} is stale; run make generate-gox")
        print(f"{generated.relative_to(root)} matches go generate")


if __name__ == "__main__":
    main()
