#!/usr/bin/env bash
# OpenLeet installer for Linux and macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash
#   wget -qO- https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash
#
# The script reads version.json, detects the OS and architecture, verifies the
# release checksum, and installs the binary to ~/.local/bin (or /usr/local/bin
# when run as root).

set -euo pipefail

REPO="srikar-up/openleet"
FEED_URL="${OPENLEET_FEED_URL:-https://raw.githubusercontent.com/${REPO}/main/version.json}"
INSTALL_DIR="${OPENLEET_INSTALL_DIR:-}"
DOWNLOADER="${OPENLEET_DOWNLOADER:-}"

say() {
  printf '%s\n' "$*"
}

warn() {
  printf 'warning: %s\n' "$*" >&2
}

die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
OpenLeet installer for Linux and macOS.

Usage:
  curl -fsSL https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash
  wget -qO- https://raw.githubusercontent.com/srikar-up/openleet/main/install.sh | bash

Environment:
  OPENLEET_INSTALL_DIR  install directory (default: ~/.local/bin, or /usr/local/bin as root)
  OPENLEET_DOWNLOADER   force "curl" or "wget"
  OPENLEET_FEED_URL     override version.json (for mirrors and testing)

The script detects Linux/macOS and x86_64/arm64, downloads the latest release,
verifies its SHA-256 from checksums.txt, and installs the binary.
EOF
}

case "${1:-}" in
  -h | --help)
    usage
    exit 0
    ;;
  "")
    ;;
  *)
    die "unknown argument: $1 (try --help)"
    ;;
esac

case "$(uname -s)" in
  Linux)
    os_name="linux"
    os_title="Linux"
    ;;
  Darwin)
    os_name="macos"
    os_title="Darwin"
    ;;
  *)
    die "unsupported operating system: $(uname -s). Use install.ps1 on Windows."
    ;;
esac

case "$(uname -m)" in
  x86_64 | amd64)
    arch="x86_64"
    ;;
  arm64 | aarch64)
    arch="arm64"
    ;;
  *)
    die "unsupported architecture: $(uname -m)"
    ;;
esac

asset_name="openleet_${os_title}_${arch}.tar.gz"

if [ -z "$DOWNLOADER" ]; then
  if command -v curl >/dev/null 2>&1; then
    DOWNLOADER="curl"
  elif command -v wget >/dev/null 2>&1; then
    DOWNLOADER="wget"
  else
    die "curl or wget is required"
  fi
fi

case "$DOWNLOADER" in
  curl | wget) ;;
  *) die "OPENLEET_DOWNLOADER must be curl or wget" ;;
esac

download() {
  local url="$1"
  local dest="$2"
  local accept="${3:-}"
  local args=()

  if [ "$DOWNLOADER" = "curl" ]; then
    args=(-fsSL --retry 2 --retry-delay 1 --connect-timeout 15)
    if [ -n "$accept" ]; then
      args+=(-H "Accept: $accept")
    fi
    args+=(-o "$dest" "$url")
    curl "${args[@]}"
  else
    args=(-q --tries=3 --timeout=30)
    if [ -n "$accept" ]; then
      args+=(--header="Accept: $accept")
    fi
    args+=(-O "$dest" "$url")
    wget "${args[@]}"
  fi
}

field() {
  printf '%s\n' "$1" |
    sed -n "s/^[[:space:]]*\"$2\": \"\\(.*\\)\",$/\\1/p" |
    head -n 1
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  elif command -v openssl >/dev/null 2>&1; then
    openssl dgst -sha256 "$1" | awk '{print $NF}'
  else
    die "no SHA-256 tool found (need sha256sum, shasum, or openssl)"
  fi
}

tmp="$(mktemp -d "${TMPDIR:-/tmp}/openleet.XXXXXX")" || die "could not create a temporary directory"
trap 'rm -rf "$tmp"' EXIT INT TERM

feed="$tmp/version.json"
say "Reading $FEED_URL"
download "$FEED_URL" "$feed" || die "could not download $FEED_URL"
# Accept mirrors that serve CRLF without breaking the line-oriented parser.
tr -d '\r' < "$feed" > "$feed.lf"
mv "$feed.lf" "$feed"

tag="$(sed -n 's/^[[:space:]]*"tag": "\(.*\)",$/\1/p' "$feed" | head -n 1)"
os_block="$(sed -n "/^    \"${os_name}\": {/,/^    },/p" "$feed")"
block="$(printf '%s\n' "$os_block" | sed -n "/^      \"${arch}\": {/,/^      },/p")"
[ -n "$block" ] || die "version.json has no entry for $os_name/$arch"

url="$(field "$block" "url")"
api_url="$(field "$block" "apiUrl")"
feed_sha="$(field "$block" "sha256")"
checksums_url="$(sed -n 's/^[[:space:]]*"checksumsUrl": "\(.*\)",$/\1/p' "$feed" | head -n 1)"

[ -n "$url" ] || die "version.json has no download URL for $asset_name"

archive="$tmp/$asset_name"
say "Downloading $asset_name"
if ! download "$url" "$archive"; then
  if [ -z "$api_url" ]; then
    die "download failed: $url"
  fi
  warn "direct download failed; retrying through the GitHub API"
  download "$api_url" "$archive" "application/octet-stream" || die "download failed: $api_url"
fi

expected=""
if [ -n "$checksums_url" ] && download "$checksums_url" "$tmp/checksums.txt"; then
  expected="$(grep -F "  $asset_name" "$tmp/checksums.txt" | awk '{print $1}' | head -n 1 || true)"
fi
if [ -z "$expected" ]; then
  expected="$feed_sha"
fi

if [ -n "$expected" ]; then
  actual="$(sha256_file "$archive")"
  if [ "$actual" != "$expected" ]; then
    die "SHA-256 mismatch for $asset_name (expected $expected, got $actual)"
  fi
  say "SHA-256 verified"
else
  warn "no SHA-256 available for $asset_name; skipping verification"
fi

tar -xzf "$archive" -C "$tmp" openleet || die "could not extract $asset_name"
chmod +x "$tmp/openleet" 2>/dev/null || true
if ! "$tmp/openleet" --version >/dev/null 2>&1; then
  die "the downloaded binary did not run on this machine"
fi

if [ -z "$INSTALL_DIR" ]; then
  if [ "$(id -u)" = "0" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi

mkdir -p "$INSTALL_DIR" || die "could not create $INSTALL_DIR"
if [ ! -w "$INSTALL_DIR" ]; then
  die "$INSTALL_DIR is not writable; set OPENLEET_INSTALL_DIR"
fi

if ! install -m 0755 "$tmp/openleet" "$INSTALL_DIR/openleet" 2>/dev/null; then
  cp "$tmp/openleet" "$INSTALL_DIR/openleet" || die "could not write $INSTALL_DIR/openleet"
  chmod 0755 "$INSTALL_DIR/openleet" || die "could not make $INSTALL_DIR/openleet executable"
fi

say ""
say "OpenLeet ${tag:-latest} installed to $INSTALL_DIR/openleet"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    say ""
    say "Add OpenLeet to PATH:"
    say "  export PATH=\"$INSTALL_DIR:\$PATH\""
    say "Then add that line to ~/.profile or ~/.zshrc."
    ;;
esac
say ""
say "Run: openleet"
