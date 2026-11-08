#!/usr/bin/env python3
"""Generate version.json from the latest OpenLeet GitHub release.

The feed deliberately includes prereleases (``/releases/latest`` hides them),
and pulls the SHA-256 values from the release's ``checksums.txt`` asset.

Usage:
    python scripts/generate-version-json.py
    GITHUB_TOKEN=ghp_... python scripts/generate-version-json.py --tag v0.5.0-beta
"""

from __future__ import annotations

import argparse
import json
import os
import re
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

API_ROOT = "https://api.github.com"
DEFAULT_REPO = "srikar-up/openleet"
DEFAULT_REF = "main"
OS_ORDER = ("windows", "linux", "macos")
ARCH_ORDER = ("x86_64", "arm64")
OS_NAMES = {"Windows": "windows", "Linux": "linux", "Darwin": "macos"}
ASSET_RE = re.compile(r"^openleet_(Windows|Linux|Darwin)_(x86_64|arm64)\.(zip|tar\.gz)$")
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


def request(url: str, token: str | None, accept: str, attempts: int = 4) -> urllib.request.addinfourl:
    headers = {
        "User-Agent": "openleet-version-feed",
        "Accept": accept,
    }
    if token:
        headers["Authorization"] = f"Bearer {token}"

    for attempt in range(1, attempts + 1):
        try:
            return urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=60)
        except urllib.error.HTTPError as error:
            if error.code < 500 or attempt == attempts:
                raise
        except (urllib.error.URLError, ConnectionError, TimeoutError):
            if attempt == attempts:
                raise
        time.sleep(1.5 * attempt)

    raise SystemExit(f"request failed: {url}")


def get_json(url: str, token: str | None) -> dict:
    with request(url, token, "application/vnd.github+json") as response:
        return json.load(response)


def download_asset(asset: dict, token: str | None) -> bytes:
    with request(asset["url"], token, "application/octet-stream") as response:
        return response.read()


def resolve_release(repo: str, tag: str | None, token: str | None) -> dict:
    if tag:
        encoded = urllib.parse.quote(tag, safe="")
        return get_json(f"{API_ROOT}/repos/{repo}/releases/tags/{encoded}", token)

    releases = get_json(f"{API_ROOT}/repos/{repo}/releases?per_page=30", token)
    published = [release for release in releases if not release.get("draft")]
    if not published:
        raise SystemExit("no published release found")

    # GitHub returns newest first, but do not depend on that ordering.
    return max(
        published,
        key=lambda release: release.get("published_at") or release.get("created_at") or "",
    )


def read_checksums(assets: dict[str, dict], token: str | None) -> dict[str, str]:
    checksum_asset = assets.get("checksums.txt")
    if not checksum_asset:
        raise SystemExit("release has no checksums.txt asset")

    text = download_asset(checksum_asset, token).decode("utf-8")
    checksums: dict[str, str] = {}
    for line in text.splitlines():
        parts = line.split()
        if len(parts) < 2:
            continue
        digest = parts[0].lower()
        name = parts[-1].lstrip("*")
        if SHA256_RE.fullmatch(digest):
            checksums[name] = digest
    return checksums


def channel_for(tag: str, prerelease: bool) -> str:
    if not prerelease:
        return "stable"
    lowered = tag.lower()
    for channel in ("alpha", "beta", "rc"):
        if f"-{channel}" in lowered or f".{channel}" in lowered:
            return channel
    return "prerelease"


def human_size(size: int) -> str:
    return f"{size / (1024 * 1024):.1f} MB"


def build_downloads(release: dict, checksums: dict[str, str]) -> dict:
    assets = {asset["name"]: asset for asset in release.get("assets", [])}
    found: dict[str, dict[str, dict]] = {os_name: {} for os_name in OS_ORDER}

    for name, asset in assets.items():
        match = ASSET_RE.match(name)
        if not match:
            continue

        os_name = OS_NAMES[match.group(1)]
        arch = match.group(2)
        digest = checksums.get(name)
        if not digest:
            raise SystemExit(f"checksums.txt has no SHA-256 for {name}")

        found[os_name][arch] = {
            "url": asset["browser_download_url"],
            "apiUrl": asset["url"],
            "name": name,
            "size": asset["size"],
            "sizeHuman": human_size(asset["size"]),
            "sha256": digest,
            "archive": "zip" if name.endswith(".zip") else "tar.gz",
            "binary": "openleet.exe" if os_name == "windows" else "openleet",
        }

    # Keep the published order stable and put the common x86_64 target first.
    downloads = {
        os_name: {arch: found[os_name][arch] for arch in ARCH_ORDER if arch in found[os_name]}
        for os_name in OS_ORDER
    }

    missing = [
        f"{os_name}/{arch}"
        for os_name in OS_ORDER
        for arch in ARCH_ORDER
        if arch not in downloads[os_name]
    ]
    if missing:
        raise SystemExit(f"release is missing assets: {', '.join(missing)}")

    return downloads


def build_install(downloads: dict, repo: str, ref: str) -> dict:
    raw_base = f"https://raw.githubusercontent.com/{repo}/{ref}"
    install = {
        "windows": {
            "url": f"{raw_base}/install.ps1",
            "powershell": f"irm {raw_base}/install.ps1 | iex",
        },
        "linux": {
            "installer": {
                "url": f"{raw_base}/install.sh",
                "curl": f"curl -fsSL {raw_base}/install.sh | bash",
                "wget": f"wget -qO- {raw_base}/install.sh | bash",
            },
        },
        "macos": {
            "installer": {
                "url": f"{raw_base}/install.sh",
                "curl": f"curl -fsSL {raw_base}/install.sh | bash",
                "wget": f"wget -qO- {raw_base}/install.sh | bash",
            },
        },
    }

    for os_name in OS_ORDER:
        for arch in ARCH_ORDER:
            entry = downloads[os_name][arch]
            if os_name == "windows":
                install[os_name][arch] = {
                    "curl": f"curl.exe -fsSL {entry['url']} -o openleet.zip && tar -xf openleet.zip",
                }
            else:
                install[os_name][arch] = {
                    "curl": f"curl -fsSL {entry['url']} -o openleet.tar.gz && tar -xzf openleet.tar.gz",
                    "wget": f"wget -q {entry['url']} -O openleet.tar.gz && tar -xzf openleet.tar.gz",
                }

    return install


def build_feed(repo: str, ref: str, release: dict, checksums: dict[str, str]) -> dict:
    tag = release["tag_name"]
    downloads = build_downloads(release, checksums)
    raw_base = f"https://raw.githubusercontent.com/{repo}/{ref}"

    return {
        "schemaVersion": 1,
        "name": "OpenLeet",
        "repo": repo,
        "homepage": f"https://github.com/{repo}",
        "latest": {
            "name": release.get("name") or tag,
            "tag": tag,
            "version": tag[1:] if tag.startswith("v") else tag,
            "channel": channel_for(tag, bool(release.get("prerelease"))),
            "prerelease": bool(release.get("prerelease")),
            "draft": bool(release.get("draft")),
            "publishedAt": release.get("published_at"),
            "releaseUrl": release.get("html_url"),
            "checksumsUrl": next(
                asset["browser_download_url"]
                for asset in release["assets"]
                if asset["name"] == "checksums.txt"
            ),
        },
        "downloads": downloads,
        "install": build_install(downloads, repo, ref),
        "links": {
            "repository": f"https://github.com/{repo}",
            "releases": f"https://github.com/{repo}/releases",
            "readme": f"https://github.com/{repo}#readme",
            "documentation": f"https://github.com/{repo}/tree/{ref}/readme",
            "issues": f"https://github.com/{repo}/issues",
            "license": f"https://github.com/{repo}/blob/{ref}/LICENSE",
            "installer": f"{raw_base}/install.ps1",
            "unixInstaller": f"{raw_base}/install.sh",
            "versionFeed": f"{raw_base}/version.json",
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=DEFAULT_REPO, help="GitHub repository (owner/name)")
    parser.add_argument("--ref", default=DEFAULT_REF, help="branch used in raw/documentation links")
    parser.add_argument("--tag", help="release tag to encode (default: latest published release)")
    parser.add_argument("--output", default="version.json", help="output path")
    args = parser.parse_args()

    token = os.environ.get("GITHUB_TOKEN") or os.environ.get("GH_TOKEN")
    release = resolve_release(args.repo, args.tag, token)
    assets = {asset["name"]: asset for asset in release.get("assets", [])}
    checksums = read_checksums(assets, token)
    feed = build_feed(args.repo, args.ref, release, checksums)

    output = Path(args.output)
    output.write_text(
        json.dumps(feed, indent=2, ensure_ascii=False) + "\n",
        encoding="utf-8",
        newline="\n",
    )
    print(f"wrote {output} for {feed['latest']['tag']} ({len(checksums)} checksums)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
