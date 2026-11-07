#!/usr/bin/env bash
# Resize receipt: prove every view repaints to the new grid without a keypress.
#
# The resize is applied with `tmux resize-window`, then the pane is captured
# after it changes. No input is sent between the resize and the capture, so a
# frame that still has the old width is a real regression (the "snap on
# keypress" bug).
#
# Usage (from a checkout):
#   bash scripts/resize-receipt.sh
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

export TERM=xterm-256color
export COLORTERM=truecolor

if [ ! -x bin/openleet-linux ]; then
  echo "missing bin/openleet-linux; cross-compile it first" >&2
  exit 1
fi

# Execute from the Linux filesystem: WSL2's 9p mount can SIGSEGV a large Go
# binary started straight from a Windows drive mount.
RUN_BIN="/tmp/openleet-resize-$$"
cp bin/openleet-linux "$RUN_BIN"
chmod +x "$RUN_BIN"
trap 'rm -f "$RUN_BIN"' EXIT

failures=0

frame_width() {
  tmux capture-pane -p -t "$1" | awk '{ print length($0) }' | sort -n | tail -n 1
}

# wait_for_frame waits until the pane has painted anything at all.
wait_for_frame() {
  local session="$1" _
  for _ in $(seq 1 60); do
    if [ -n "$(tmux capture-pane -p -t "$session" | tr -d '[:space:]')" ]; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

# wait_for_width waits until the first frame has actually laid out at the
# launch width. The app paints its startup fallback (100 cells) before the
# first WindowSizeMsg arrives; capturing that as the baseline makes the shrink
# assertion compare against the placeholder instead of the real frame.
wait_for_width() {
  local session="$1" want="$2" _
  for _ in $(seq 1 60); do
    if [ "$(frame_width "$session")" -ge $((want - 8)) ]; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

check() {
  local view="$1" from="$2" to="$3"
  local session="tui-${view}-$$"
  local fromw="${from%x*}" tow="${to%x*}"
  local fromh="${from#*x}" toh="${to#*x}"

  tmux kill-session -t "$session" 2>/dev/null || true
  tmux new-session -d -s "$session" -x "$fromw" -y "$fromh" \
    "$RUN_BIN --view $view --theme openleet"

  if ! wait_for_frame "$session"; then
    printf '%-8s %s -> %s | FAIL: no frame painted within 6s\n' "$view" "$from" "$to" >&2
    failures=$((failures + 1))
    tmux kill-session -t "$session" 2>/dev/null || true
    return
  fi

  # Let the first WindowSizeMsg settle before measuring the baseline.
  if ! wait_for_width "$session" "$fromw"; then
    printf '%-8s %s -> %s | FAIL: startup never laid out at %s columns\n' "$view" "$from" "$to" "$fromw" >&2
    failures=$((failures + 1))
    tmux kill-session -t "$session" 2>/dev/null || true
    return
  fi

  local before
  before=$(frame_width "$session")

  # No keys are sent after this point: the redraw must be driven by the resize
  # notification alone.
  tmux resize-window -t "$session" -x "$tow" -y "$toh"

  local after=""
  local _
  for _ in $(seq 1 40); do
    after=$(frame_width "$session")
    if [ "$after" -ne "$before" ]; then
      break
    fi
    sleep 0.1
  done
  sleep 0.3

  after=$(frame_width "$session")
  local lines
  lines=$(tmux capture-pane -p -t "$session" | wc -l)

  printf '%-8s %s -> %s | before=%s width=%s lines=%s\n' \
    "$view" "$from" "$to" "$before" "$after" "$lines"

  # tmux omits trailing default-background cells, so the captured width can be a
  # few cells short; the in-process resize test asserts the exact grid. What the
  # PTY must prove is reflow with no keypress.
  local problem=""
  if [ "$after" -gt "$tow" ] || [ "$after" -lt $((tow - 8)) ]; then
    problem="captured width $after is not within 8 cells of $tow"
  elif [ "$lines" -ne "$toh" ]; then
    problem="captured $lines rows, want $toh"
  elif [ "$tow" -lt "$fromw" ] && [ "$after" -ge "$before" ]; then
    problem="frame did not shrink"
  elif [ "$tow" -gt "$fromw" ] && [ "$after" -le "$before" ]; then
    problem="frame did not grow"
  fi

  if [ -n "$problem" ]; then
    echo "  FAIL: $problem" >&2
    failures=$((failures + 1))
  fi

  tmux kill-session -t "$session" 2>/dev/null || true
}

for view in home roadmap solve stats themes; do
  check "$view" "189x56" "110x32"
  check "$view" "110x32" "160x48"
done

if [ "$failures" -ne 0 ]; then
  echo "$failures view(s) failed the resize receipt" >&2
  exit 1
fi
echo "all views tracked both resize directions with no keypress"
