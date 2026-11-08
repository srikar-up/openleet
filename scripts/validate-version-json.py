#!/usr/bin/env python3
"""Validate version.json before it is committed or shipped."""

from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
FEED = ROOT / "version.json"
REPO = "srikar-up/openleet"
OS_ORDER = ("windows", "linux", "macos")
ARCH_ORDER = ("x86_64", "arm64")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")
VERSION_RE = re.compile(r"^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$")


def fail(message: str) -> None:
    raise SystemExit(f"version.json invalid: {message}")


def require(condition: bool, message: str) -> None:
    if not condition:
        fail(message)


def main() -> int:
    if not FEED.exists():
        fail("file is missing")

    try:
        data = json.loads(FEED.read_text(encoding="utf-8"))
    except json.JSONDecodeError as error:
        fail(f"not valid JSON: {error}")

    require(data.get("schemaVersion") == 1, "schemaVersion must be 1")
    require(data.get("repo") == REPO, f"repo must be {REPO}")

    latest = data.get("latest") or {}
    tag = latest.get("tag", "")
    require(tag.startswith("v"), "latest.tag must start with v")
    version = tag[1:]
    require(latest.get("version") == version, "latest.version must match latest.tag")
    require(bool(VERSION_RE.fullmatch(version)), f"latest.version is not semver: {version}")
    require(latest.get("channel") in {"stable", "alpha", "beta", "rc", "prerelease"}, "invalid channel")
    require(isinstance(latest.get("prerelease"), bool), "latest.prerelease must be a boolean")
    require(latest.get("releaseUrl", "").endswith(f"/releases/tag/{tag}"), "latest.releaseUrl mismatch")
    require(
        latest.get("checksumsUrl", "").endswith(f"/releases/download/{tag}/checksums.txt"),
        "latest.checksumsUrl mismatch",
    )

    downloads = data.get("downloads") or {}
    for os_name in OS_ORDER:
        require(os_name in downloads, f"downloads.{os_name} is missing")
        for arch in ARCH_ORDER:
            entry = downloads[os_name].get(arch)
            require(isinstance(entry, dict), f"downloads.{os_name}.{arch} is missing")
            name = entry.get("name", "")
            expected_archive = "zip" if os_name == "windows" else "tar.gz"
            expected_binary = "openleet.exe" if os_name == "windows" else "openleet"
            require(name.startswith("openleet_"), f"{os_name}/{arch} filename is not an OpenLeet asset")
            require(
                entry.get("url", "").endswith(f"/releases/download/{tag}/{name}"),
                f"{os_name}/{arch} URL does not match the tag and filename",
            )
            require(
                re.fullmatch(rf"https://api\.github\.com/repos/{re.escape(REPO)}/releases/assets/\d+", entry.get("apiUrl", ""))
                is not None,
                f"{os_name}/{arch} apiUrl is invalid",
            )
            require(entry.get("archive") == expected_archive, f"{os_name}/{arch} archive type mismatch")
            require(entry.get("binary") == expected_binary, f"{os_name}/{arch} binary name mismatch")
            require(isinstance(entry.get("size"), int) and entry["size"] > 0, f"{os_name}/{arch} size is invalid")
            require(
                SHA256_RE.fullmatch(entry.get("sha256", "")) is not None,
                f"{os_name}/{arch} SHA-256 is invalid",
            )

    install = data.get("install") or {}
    windows_install = install.get("windows") or {}
    require(
        str(windows_install.get("powershell", "")).startswith("irm "),
        "install.windows.powershell is missing",
    )
    require(
        str(windows_install.get("url", "")).endswith("/install.ps1"),
        "install.windows.url is missing",
    )
    for os_name in ("linux", "macos"):
        installer = (install.get(os_name) or {}).get("installer") or {}
        require(
            str(installer.get("url", "")).endswith("/install.sh"),
            f"install.{os_name}.installer.url is missing",
        )
        require(
            "install.sh" in str(installer.get("curl", "")) and "| bash" in str(installer.get("curl", "")),
            f"install.{os_name}.installer.curl is missing",
        )
        require(
            "install.sh" in str(installer.get("wget", "")) and "| bash" in str(installer.get("wget", "")),
            f"install.{os_name}.installer.wget is missing",
        )
    for os_name in OS_ORDER:
        for arch in ARCH_ORDER:
            commands = install.get(os_name, {}).get(arch) or {}
            require(bool(commands.get("curl")), f"install.{os_name}.{arch}.curl is missing")
            if os_name != "windows":
                require(bool(commands.get("wget")), f"install.{os_name}.{arch}.wget is missing")

    links = data.get("links") or {}
    require(links.get("versionFeed", "").endswith("/version.json"), "links.versionFeed is missing")
    require(links.get("installer", "").endswith("/install.ps1"), "links.installer is missing")
    require(links.get("unixInstaller", "").endswith("/install.sh"), "links.unixInstaller is missing")

    print(
        f"version.json OK: {tag} · "
        f"{len(downloads['windows']) + len(downloads['linux']) + len(downloads['macos'])} platform assets"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
