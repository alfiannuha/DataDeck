#!/usr/bin/env python3
"""Package DataDeck release binaries into deterministic archives.

Creates .tar.gz (Unix targets) and .zip (Windows) containing the binary and a
VERSION file, and writes SHA256SUMS covering the archives plus any SBOM/notice
files present. Deterministic (fixed member mtimes; gzip mtime 0) so archives
are reproducible for the same inputs.
"""
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import os
import tarfile
import zipfile


def sha256(path: str) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1 << 20), b""):
            digest.update(chunk)
    return digest.hexdigest()


def tar_gz(path: str, entries: list[tuple[str, bytes]]) -> None:
    raw = io.BytesIO()
    with tarfile.open(fileobj=raw, mode="w") as tar:
        for name, data in entries:
            info = tarfile.TarInfo(name=name)
            info.size = len(data)
            info.mtime = 0
            info.uid = 0
            info.gid = 0
            info.uname = ""
            info.gname = ""
            info.mode = 0o755
            tar.addfile(info, io.BytesIO(data))
    with open(path, "wb") as out:
        with gzip.GzipFile(fileobj=out, mode="wb", mtime=0) as gz:
            gz.write(raw.getvalue())


def zip_file(path: str, entries: list[tuple[str, bytes]]) -> None:
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for name, data in entries:
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.external_attr = 0o755 << 16
            archive.writestr(info, data)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--version", required=True)
    parser.add_argument("--release-dir", required=True)
    parser.add_argument("--prefix", default="datadeck")
    args = parser.parse_args()

    release_dir = args.release_dir
    prefix = args.prefix
    version = args.version
    version_bytes = (version + "\n").encode()

    produced: list[str] = []
    for name in sorted(os.listdir(release_dir)):
        if not name.startswith(f"{prefix}_{version}_"):
            continue
        if name.endswith((".tar.gz", ".zip", "SHA256SUMS")):
            continue
        _, _, os_name, arch = name.rsplit("_", 3)
        ext = ".exe" if arch.endswith(".exe") else ""
        if ext:
            arch = arch[: -len(ext)]
        binary = open(os.path.join(release_dir, name), "rb").read()
        inner = f"datadeck{ext}"
        entries = [(inner, binary), ("VERSION", version_bytes)]

        if os_name == "windows":
            archive_name = f"{prefix}_{version}_{os_name}_{arch}.zip"
            zip_file(os.path.join(release_dir, archive_name), entries)
        else:
            archive_name = f"{prefix}_{version}_{os_name}_{arch}.tar.gz"
            tar_gz(os.path.join(release_dir, archive_name), entries)
        produced.append(archive_name)
        print(f"packaged {archive_name}")

    extra: list[str] = []
    for root, _dirs, files in os.walk(release_dir):
        for filename in files:
            rel = os.path.relpath(os.path.join(root, filename), release_dir)
            if rel.endswith((".json", ".txt")) and rel not in ("SHA256SUMS",):
                extra.append(rel)

    checksum_targets = sorted(produced + extra)
    with open(os.path.join(release_dir, "SHA256SUMS"), "w") as sums:
        for rel in checksum_targets:
            sums.write(f"{sha256(os.path.join(release_dir, rel))}  {rel}\n")
    print(f"wrote SHA256SUMS ({len(checksum_targets)} entries)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
