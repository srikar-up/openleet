#!/usr/bin/env bash
# Capture the top-bar focus state (keyboard focus on the account label with its
# tooltip). VHS cannot send F-keys, so tmux holds the session in the focused
# state first, then VHS attaches and screenshots it.
#
# Requires: tmux, vhs, and bin/openleet-linux (cross-compiled from the host).
set -uo pipefail

export PATH="$HOME/bin:$PATH"
export CHROME_PATH="${CHROME_PATH:-/usr/bin/google-chrome-stable}"
export COLORTERM=truecolor
export TERM="${TERM:-xterm-256color}"

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="/tmp/openleet-focus-shot-$$"
SESSION="openleet-focus-shot"
OUT="home-focus.png"
TAPE=".tui-focus-$$.tape"
WIDTH="${VHS_WIDTH:-1670}"
HEIGHT="${VHS_HEIGHT:-900}"

if [ ! -x "$ROOT/bin/openleet-linux" ]; then
  echo "missing bin/openleet-linux; cross-compile it first" >&2
  exit 1
fi

cp "$ROOT/bin/openleet-linux" "$BIN"
chmod +x "$BIN"

tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION" -x 160 -y 45 \
  "COLORTERM=truecolor TERM=xterm-256color $BIN --view home --theme openleet"
tmux set-option -g status off 2>/dev/null || true
sleep 3

# F6 focuses the chrome (breadcrumb), then Tab twice reaches the account label.
tmux send-keys -t "$SESSION" F6
sleep 0.5
tmux send-keys -t "$SESSION" Tab
sleep 0.5
tmux send-keys -t "$SESSION" Tab
sleep 0.8

if ! tmux capture-pane -t "$SESSION" -p | sed -n '2p' | grep -q "Account"; then
  echo "focus tooltip missing; is F6 wired for Home?" >&2
  tmux kill-session -t "$SESSION" 2>/dev/null || true
  rm -f "$BIN"
  exit 1
fi

cat > "$TAPE" <<EOF
Set Shell "bash"
Set FontSize 16
Set Width $WIDTH
Set Height $HEIGHT
Set Padding 0
Set Margin 0
Set Theme "Catppuccin Mocha"

Hide
Type "tmux attach -t $SESSION"
Enter
Sleep 3s
Show

Sleep 1s
Screenshot $OUT
EOF

rm -f "$OUT"
vhs "$TAPE"
tmux kill-session -t "$SESSION" 2>/dev/null || true
rm -f "$TAPE" "$BIN"
ls -l "$OUT"
