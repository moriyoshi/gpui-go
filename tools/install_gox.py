#!/usr/bin/env python3
"""Install the pinned GoX generator into the repository's ignored bin directory."""

import argparse
import hashlib
import io
import os
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile
from urllib.request import Request, urlopen


VERSION = "v0.3.3"
# Upstream release asset digests for the same version as the Go runtime module.
SHA256 = {
    "darwin_amd64": "89d5f208a7d2e70e91d24c72a5eb05e60e35b7b1d2103698e3f06c2fe427555e",
    "darwin_arm64": "92da05afd21336076299e2add53ec95ec3a0678e196223d853f28f6e4266e895",
    "linux_amd64": "c9485d7e40dd33bccf1ecac6cdcfa39d8bc76466a18e7f6e61af21f8d339fa6b",
    "linux_arm64": "759635078d589b5e9944164e4ed4f0146d783c07018159f5b389896586f70363",
}


def unpack(data, expected):
    if hashlib.sha256(data).hexdigest() != expected:
        raise ValueError("GoX archive checksum mismatch")
    # Read only these regular files; never extract paths or links from a tar.
    files = {}
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        for member in archive.getmembers():
            name = member.name.removeprefix("./")
            if name in ("gox", "LICENSE") and member.isfile():
                files[name] = archive.extractfile(member).read()
    if set(files) != {"gox", "LICENSE"}:
        raise ValueError("GoX archive lacks its executable or license")
    return files


def write_atomic(path, data, mode):
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists() and path.read_bytes() == data:
        return
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as stream:
        temp = Path(stream.name)
        try:
            stream.write(data)
            stream.flush()
            os.chmod(temp, mode)
            os.replace(temp, path)
        finally:
            temp.unlink(missing_ok=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    if args.version != VERSION:
        raise ValueError("update the pinned release checksums before changing GoX")
    if f"github.com/doors-dev/gox {VERSION}" not in (root / "go.mod").read_text():
        raise ValueError("GoX generator and runtime versions disagree")
    arch = {"x86_64": "amd64", "amd64": "amd64", "arm64": "arm64", "aarch64": "arm64"}.get(platform.machine().lower())
    target = f"{platform.system().lower()}_{arch}"
    if target not in SHA256:
        raise ValueError(f"no pinned GoX generator for {target}")
    asset = f"gox_{target}.tar.gz"
    cache = root / ".cache" / "gox" / VERSION / asset
    if not cache.exists():
        request = Request(
            f"https://github.com/doors-dev/gox/releases/download/{VERSION}/{asset}",
            headers={"User-Agent": "gpui-go-build"},
        )
        with urlopen(request, timeout=60) as response:
            data = response.read()
        unpack(data, SHA256[target])
        write_atomic(cache, data, 0o644)
    files = unpack(cache.read_bytes(), SHA256[target])
    binary = root / "bin" / "gox"
    write_atomic(binary, files["gox"], 0o755)
    write_atomic(root / "bin" / "gox.LICENSE", files["LICENSE"], 0o644)
    version = subprocess.check_output([str(binary), "ver"], text=True).strip()
    if version != VERSION:
        raise ValueError(f"GoX executable reports {version!r}, expected {VERSION}")
    print(f"GoX {version}: {binary}")


if __name__ == "__main__":
    main()
