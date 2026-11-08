# Version feed

[`version.json`](../version.json) is the machine-readable release feed for
websites, launch pages, and install scripts. It is generated from the newest
published GitHub release, **including prereleases**, so it never has to scrape
the GitHub HTML and never hits the unauthenticated API rate limit on a page
view.

## URLs

| Use | URL |
| :--- | :--- |
| Raw (recommended) | `https://raw.githubusercontent.com/srikar-up/openleet/main/version.json` |
| jsDelivr mirror | `https://cdn.jsdelivr.net/gh/srikar-up/openleet@main/version.json` |
| Release asset | `https://github.com/srikar-up/openleet/releases/download/<tag>/version.json` |

The raw endpoint is CDN-cached for a few minutes. Append a cache-busting query
such as `?ts=...` when a page must see a release immediately. jsDelivr caches
longer; use it when a slightly stale tag is acceptable.

## Shape

```json
{
  "schemaVersion": 1,
  "name": "OpenLeet",
  "repo": "srikar-up/openleet",
  "homepage": "https://github.com/srikar-up/openleet",
  "latest": {
    "name": "OpenLeet v0.5.0-beta",
    "tag": "v0.5.0-beta",
    "version": "0.5.0-beta",
    "channel": "beta",
    "prerelease": true,
    "draft": false,
    "publishedAt": "2026-11-07T18:23:18Z",
    "releaseUrl": "https://github.com/srikar-up/openleet/releases/tag/v0.5.0-beta",
    "checksumsUrl": "https://github.com/srikar-up/openleet/releases/download/v0.5.0-beta/checksums.txt"
  },
  "downloads": {
    "linux": {
      "x86_64": {
        "url": "https://github.com/srikar-up/openleet/releases/download/v0.5.0-beta/openleet_Linux_x86_64.tar.gz",
        "apiUrl": "https://api.github.com/repos/srikar-up/openleet/releases/assets/721579173",
        "name": "openleet_Linux_x86_64.tar.gz",
        "size": 6967150,
        "sizeHuman": "6.6 MB",
        "sha256": "12ad37b477040db8d1f2c77ed5d960ce08adabb6afb1beaf027ff33d6aafdd5a",
        "archive": "tar.gz",
        "binary": "openleet"
      }
    }
  },
  "install": {
    "linux": {
      "installer": {
        "url": "https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh",
        "curl": "curl -fsSL https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash",
        "wget": "wget -qO- https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash"
      }
    }
  },
  "links": {}
}
```

The real file contains all six platform assets and the complete `install` and
`links` objects.

### `latest`

| Field | Meaning |
| :--- | :--- |
| `name` | Release title, e.g. `OpenLeet v0.5.0-beta`. |
| `tag` | Git tag, including the `v`, e.g. `v0.5.0-beta`. |
| `version` | Tag without the `v`, e.g. `0.5.0-beta`. |
| `channel` | `stable`, `alpha`, `beta`, `rc`, or `prerelease`. |
| `prerelease` | `true` when the latest release is a GitHub pre-release. |
| `draft` | Always `false`; drafts are filtered out. |
| `publishedAt` | ISO-8601 publication timestamp. |
| `releaseUrl` | Human-readable release page. |
| `checksumsUrl` | Direct link to the release's `checksums.txt`. |

> `/releases/latest` **excludes prereleases**. Do not use it for this project
> while the newest tag is `-beta`. Use this feed, or list releases and filter
> `draft == false`.

### `downloads`

`downloads` is keyed by `windows`, `linux`, and `macos`, then by `x86_64` and
`arm64`. Each entry has:

| Field | Meaning |
| :--- | :--- |
| `url` | Direct `github.com/.../releases/download/...` link. |
| `apiUrl` | GitHub API asset endpoint; scripts can use it as a fallback with `Accept: application/octet-stream`. |
| `name` | Archive filename. |
| `size` | Exact size in bytes. |
| `sizeHuman` | Display size, e.g. `6.6 MB`. |
| `sha256` | Lowercase SHA-256 digest from `checksums.txt`. |
| `archive` | `zip` or `tar.gz`. |
| `binary` | `openleet.exe` or `openleet`. |

Use `x86_64` for Intel/AMD 64-bit machines and `arm64` for Apple Silicon,
Windows on Arm, and 64-bit ARM Linux.

### `install`

| Key | Contents |
| :--- | :--- |
| `windows.url` | Link to `install.ps1`. |
| `windows.powershell` | The one-line Windows installer command. |
| `windows.<arch>.curl` | Download and extract the Windows ZIP with `curl.exe` and `tar`. |
| `linux.installer.url` | Link to `install.sh`. |
| `linux.installer.curl` | The Linux one-line installer through `curl`. |
| `linux.installer.wget` | The same through `wget`. |
| `linux.<arch>.curl` | Download and extract the Linux archive directly with `curl`. |
| `linux.<arch>.wget` | The same with `wget`. |
| `macos.installer.url` | Link to `install.sh`. |
| `macos.installer.curl` | The macOS one-line installer through `curl`. |
| `macos.installer.wget` | The same through `wget`. |
| `macos.<arch>.curl` | Download and extract the macOS archive directly with `curl`. |
| `macos.<arch>.wget` | The same with `wget`. |

There is no Scoop, Homebrew, or WinGet package yet. Do not advertise those
commands until a manifest is published.

### Install with curl or wget

```bash
# curl
curl -fsSL https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash

# wget
wget -qO- https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash
```

[`install.sh`](../install.sh) detects Linux/macOS and `x86_64`/`arm64`, reads
this feed, downloads the release archive, verifies its SHA-256, and installs
`openleet` to `~/.local/bin` (`/usr/local/bin` when run as root).

| Environment variable | Effect |
| :--- | :--- |
| `OPENLEET_INSTALL_DIR` | Install somewhere other than the default directory. |
| `OPENLEET_DOWNLOADER` | Force `curl` or `wget`. |
| `OPENLEET_FEED_URL` | Read a mirrored feed instead of the raw GitHub URL. |

If the direct GitHub download endpoint fails, the installer retries through the
entry's `apiUrl` with `Accept: application/octet-stream`. If `checksums.txt`
cannot be fetched, it falls back to the `sha256` in this feed.

### `links`

`repository`, `releases`, `readme`, `documentation`, `issues`, `license`,
`installer`, `unixInstaller`, and `versionFeed`.

## Pick the right download in a browser

```js
const feed = await fetch(
  "https://raw.githubusercontent.com/srikar-up/openleet/main/version.json",
  { cache: "no-store" }
).then((response) => response.json());

const platform = (
  navigator.userAgentData?.platform ||
  navigator.platform ||
  navigator.userAgent
).toLowerCase();

const os = platform.includes("win")
  ? "windows"
  : platform.includes("mac")
    ? "macos"
    : "linux";

// userAgentData.architecture is not available in every browser.
const arch = navigator.userAgentData?.architecture === "arm" ? "arm64" : "x86_64";
const asset = feed.downloads[os][arch] ?? feed.downloads[os].x86_64;

window.location.href = asset.url;
```

For a server-rendered site, map the request's `User-Agent` to the same keys and
redirect. Always keep a hardcoded fallback tag and URL so a failed fetch does
not leave the download button blank.

## Verify a download

```bash
# Linux
sha256sum openleet_Linux_x86_64.tar.gz

# macOS
shasum -a 256 openleet_Darwin_arm64.tar.gz
```

```powershell
# Windows
Get-FileHash .\openleet_Windows_x86_64.zip -Algorithm SHA256
```

Compare the result with `downloads.<os>.<arch>.sha256`.

## Update flow

[`.github/workflows/version.yml`](../.github/workflows/version.yml) runs when a
release is published, prereleased, released, edited, unpublished, or deleted.
It:

1. runs [`scripts/generate-version-json.py`](../scripts/generate-version-json.py),
2. validates the result with [`scripts/validate-version-json.py`](../scripts/validate-version-json.py),
3. commits `version.json` to `main` when it changed,
4. attaches the same file to the current GitHub release.

Regenerate it locally with:

```bash
GITHUB_TOKEN=... python scripts/generate-version-json.py
python scripts/validate-version-json.py
```

Generation fails if any of the six platform assets or its SHA-256 is missing, so
a partial release never replaces the feed.
