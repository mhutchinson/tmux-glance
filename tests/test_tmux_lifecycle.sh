#!/usr/bin/env bash
set -euo pipefail

export LC_ALL="${LC_ALL:-C.UTF-8}"
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$(cd "$SCRIPT_DIR/.." && pwd)/bin/tmux-glance"

echo "=== [TEST] Headless Tmux Server & Watch Lifecycle Integration ==="

TEST_ID="$$"
TMP_BASE="${TMPDIR:-/tmp}"
SOCK="${TMP_BASE}/glance-test-${TEST_ID}.sock"
STATE_FILE="${TMP_BASE}/glance-state-${TEST_ID}.txt"
STATUS_FILE="${TMP_BASE}/glance-status-${TEST_ID}.txt"

cleanup() {
    tmux -S "$SOCK" kill-server 2>/dev/null || true
    rm -f "$SOCK" "${STATE_FILE}"* "${STATUS_FILE}"* 2>/dev/null || true
}
trap cleanup EXIT

# 1. Start isolated headless tmux server
echo -n "Test 1: Spawning isolated headless tmux server... "
tmux -S "$SOCK" new-session -d -s test-sess -n win1 "cat"
tmux -S "$SOCK" set-environment -g PATH "$PATH"
tmux -S "$SOCK" set-environment -g LC_ALL "${LC_ALL:-C.UTF-8}"
tmux -S "$SOCK" set-environment -g LANG "${LANG:-C.UTF-8}"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_STATE_FILE "$STATE_FILE"
tmux -S "$SOCK" set-option -g default-shell "$(type -p bash || echo "$SHELL")"
echo "PASS"

# Helper to read status output
get_status() {
    rm -f "$STATUS_FILE"
    tmux -S "$SOCK" run-shell "$BIN status > '$STATUS_FILE'"
    cat "$STATUS_FILE" 2>/dev/null || true
}

# 2. Test initial empty state
echo -n "Test 2: Initial status summary is quiet... "
status_out=$(get_status)
if [[ -z "$status_out" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected empty status, got '$status_out'"
    exit 1
fi

# 3. Add Vigil to win1
echo -n "Test 3: Toggle Vigil on win1... "
pane1_id=$(tmux -S "$SOCK" list-panes -t test-sess:win1 -F '#{pane_id}')
tmux -S "$SOCK" select-window -t test-sess:win1
tmux -S "$SOCK" run-shell "$BIN toggle-vigil"
status_out=$(get_status)
if [[ "$status_out" =~ 👁️[[:space:]]+1 ]]; then
    echo "PASS (Status: $status_out)"
else
    echo "FAIL: Expected '👁️ 1', got '$status_out'"
    exit 1
fi

# 4. Create win2 and switch to it so win1 is in the background
tmux -S "$SOCK" new-window -t test-sess -n win2 "cat"
tmux -S "$SOCK" select-window -t test-sess:win2

# 5. Append output to background pane1
echo -n "Test 4: Output change in background triggers dynamic Alert... "
tmux -S "$SOCK" send-keys -t "$pane1_id" "Build completed successfully!" C-m
# Run scan to detect change
tmux -S "$SOCK" run-shell "$BIN scan"
status_out=$(get_status)
if [[ "$status_out" =~ 🚨[[:space:]]+1 ]]; then
    echo "PASS (Upgraded to: $status_out)"
else
    echo "FAIL: Expected '🚨 1', got '$status_out'"
    exit 1
fi

# 6. Focus win1 and acknowledge alert
echo -n "Test 5: Focusing pane auto-acknowledges alert back to quiet watch... "
tmux -S "$SOCK" select-window -t test-sess:win1
tmux -S "$SOCK" run-shell "$BIN on-focus $pane1_id"
status_out=$(get_status)
if [[ "$status_out" =~ 👁️[[:space:]]+1 ]] && ! [[ "$status_out" =~ 🚨 ]]; then
    echo "PASS (Acknowledged back to: $status_out)"
else
    echo "FAIL: Expected '👁️ 1' without alert, got '$status_out'"
    exit 1
fi

# 7. Live Preview actively streams pane contents
echo -n "Test 6: Live preview actively streams pane contents... "
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "$BIN preview $pane1_id > '$STATUS_FILE' 2>&1 & sleep 0.3; kill -TERM \$! 2>/dev/null || true"
preview_out=$(cat "$STATUS_FILE" 2>/dev/null || true)
if [[ "$preview_out" =~ "Build completed successfully!" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected 'Build completed successfully!', got '$preview_out'"
    exit 1
fi

# 8. Untoggle Vigil
echo -n "Test 7: Toggle Vigil off releases watch... "
tmux -S "$SOCK" run-shell "$BIN toggle-vigil"
status_out=$(get_status)
if [[ -z "$status_out" ]]; then
    echo "PASS (Status quiet)"
else
    echo "FAIL: Expected empty status, got '$status_out'"
    exit 1
fi

echo "All headless tmux lifecycle tests passed successfully!"
