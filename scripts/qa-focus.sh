#!/usr/bin/env bash
# QA helper: launch OpenLeet in tmux, focus the account label with the keyboard
# (F6, Tab, Tab), verify the tooltip, and print the captured pane.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="/tmp/openleet-focus-$$"
SESSION="openleet-focus-qa"

cp "$ROOT/bin/openleet-linux" "$BIN"
chmod +x "$BIN"

tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION" -x 160 -y 45 \
  "COLORTERM=truecolor TERM=xterm-256color $BIN --view home --theme openleet"

sleep 3
tmux send-keys -t "$SESSION" F6
sleep 0.5
tmux send-keys -t "$SESSION" Tab
sleep 0.5
tmux send-keys -t "$SESSION" Tab
sleep 0.8

echo "--- header and tooltip row ---"
tmux capture-pane -t "$SESSION" -p | sed -n '1,2p'

if tmux capture-pane -t "$SESSION" -p | sed -n '2p' | grep -q "Account"; then
  echo "FOCUS_OK"
else
  echo "FOCUS_MISSING"
fi

tmux kill-session -t "$SESSION" 2>/dev/null || true
rm -f "$BIN"
