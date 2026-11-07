#!/usr/bin/env bash
# Headless TUI capture.
#
# Requires (inside WSL): ttyd, ffmpeg, vhs and a Chromium build.
#   ttyd    -> https://github.com/tsl0922/ttyd/releases (static Linux binary)
#   ffmpeg  -> https://github.com/eugeneware/ffmpeg-static/releases
#   vhs     -> go install github.com/charmbracelet/vhs@v0.12.1 (GOOS=linux)
#   chrome  -> google-chrome-stable
#
# Usage (from a WSL checkout at the repository root):
#   bash scripts/capture-tui.sh                 # roadmap
#   bash scripts/capture-tui.sh stats           # dashboard
#   bash scripts/capture-tui.sh settings        # schema-driven settings
#   bash scripts/capture-tui.sh settings "" appearance  # Appearance (ex gallery)
#   bash scripts/capture-tui.sh settings dracula  # settings, themed
#   bash scripts/capture-tui.sh home tron       # landing, themed
#
# Outputs `<view>.png` for the default theme and `<view>-<theme>.png` when a
# theme is supplied.
#
# The Linux binary is built automatically when a Go toolchain is available on
# the host; otherwise build it manually with:
#   GOOS=linux GOARCH=amd64 go build -o bin/openleet-linux ./cmd/openleet
set -uo pipefail

export PATH="$HOME/bin:$PATH"
export CHROME_PATH="${CHROME_PATH:-/usr/bin/google-chrome-stable}"
# Without truecolor, Lip Gloss maps every near-black shade to ANSI-256 #232
# and the panels become indistinguishable.
export COLORTERM=truecolor
export TERM="${TERM:-xterm-256color}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VIEW="${1:-roadmap}"
THEME="${2:-}"
SECTION="${3:-}"

case "$VIEW" in
  home|roadmap|solve|stats|settings) ;;
  themes)
    # The theme gallery moved into Settings > Appearance; capture that instead.
    VIEW="settings"
    if [ -z "$SECTION" ]; then
      SECTION="appearance"
    fi
    ;;
  *)
    echo "unknown view '$VIEW' (expected: home, roadmap, solve, stats, settings)" >&2
    exit 2
    ;;
esac

if [ ! -x bin/openleet-linux ]; then
  echo "missing bin/openleet-linux; cross-compile it from the host first" >&2
  exit 1
fi

# WSL2's 9p mount can SIGSEGV when executing a large Go binary straight from
# a Windows drive mount. Stage it on the Linux filesystem so VHS and tmux run a
# binary the kernel can map normally. exec from /mnt still works for small
# files, which is why this only shows up after the app grows.
RUN_BIN="/tmp/openleet-capture-$$"
cp bin/openleet-linux "$RUN_BIN"
chmod +x "$RUN_BIN"

COMMAND="$RUN_BIN --view $VIEW"
case "$VIEW" in
  roadmap) BASE="screenshot" ;;
  *)       BASE="$VIEW" ;;
esac
if [ -n "$THEME" ]; then
  COMMAND="$COMMAND --theme $THEME"
  BASE="$BASE-$THEME"
fi
if [ "$VIEW" = "settings" ] && [ -n "$SECTION" ]; then
  COMMAND="$COMMAND --settings-section $SECTION"
  BASE="$BASE-$SECTION"
fi
OUTPUT="$BASE.png"

WIDTH="${VHS_WIDTH:-1920}"
HEIGHT="${VHS_HEIGHT:-1080}"
if [ -n "${VHS_WIDTH:-}" ] || [ -n "${VHS_HEIGHT:-}" ]; then
  OUTPUT="${OUTPUT%.png}-${WIDTH}x${HEIGHT}.png"
fi

TAPE=".tui-capture-$$.tape"
trap 'rm -f "$TAPE" "$RUN_BIN"' EXIT

cat > "$TAPE" <<EOF
Set Shell "bash"
Set FontSize 16
Set Width $WIDTH
Set Height $HEIGHT
Set Padding 0
Set Margin 0
Set Theme "Catppuccin Mocha"

Hide
Type "$COMMAND"
Enter
Sleep 10s
Show

Sleep 1500ms
Screenshot $OUTPUT
EOF

rm -f "$OUTPUT"
vhs "$TAPE"
ls -l "$OUTPUT"
