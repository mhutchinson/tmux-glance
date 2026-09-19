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
    rm -rf "$SOCK" "${STATE_FILE}"* "${STATUS_FILE}"* "${TMP_BASE}/glance-bin-${TEST_ID}"* 2>/dev/null || true
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
    tmux -S "$SOCK" run-shell "bash '$BIN' status > '$STATUS_FILE'"
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
tmux -S "$SOCK" run-shell "bash '$BIN' toggle-vigil"
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
tmux -S "$SOCK" run-shell "bash '$BIN' scan"
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
tmux -S "$SOCK" run-shell "bash '$BIN' on-focus $pane1_id"
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
tmux -S "$SOCK" run-shell "bash '$BIN' preview $pane1_id > '$STATUS_FILE' 2>&1 & sleep 0.3; kill -TERM \$! 2>/dev/null || true"
preview_out=$(cat "$STATUS_FILE" 2>/dev/null || true)
if [[ "$preview_out" =~ "Build completed successfully!" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected 'Build completed successfully!', got '$preview_out'"
    exit 1
fi

# 8. Untoggle Vigil
echo -n "Test 7: Toggle Vigil off releases watch... "
tmux -S "$SOCK" run-shell "bash '$BIN' toggle-vigil"
status_out=$(get_status)
if [[ -z "$status_out" ]]; then
    echo "PASS (Status quiet)"
else
    echo "FAIL: Expected empty status, got '$status_out'"
    exit 1
fi

# 9. Autonomous agent lifecycle testing
echo -n "Test 8: Autonomous agent starts idle and quiet... "
tmux -S "$SOCK" set-option -g @glance_routes "head=antigravity,coreutils=antigravity"
tmux -S "$SOCK" new-window -t test-sess -n win3 "head -n 1000"
pane3_id=$(tmux -S "$SOCK" list-panes -t test-sess:win3 -F '#{pane_id}')
tmux -S "$SOCK" select-window -t test-sess:win2
# Send initial idle prompt
tmux -S "$SOCK" send-keys -t "$pane3_id" "Welcome to Antigravity" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "? for shortcuts" C-m
sleep 0.2
tmux -S "$SOCK" run-shell "bash '$BIN' scan"
status_out=$(get_status)
if [[ -z "$status_out" ]]; then
    echo "PASS (Agent is idle and quiet)"
else
    echo "FAIL: Expected empty status for idle agent, got '$status_out'"
    exit 1
fi

# 10. Agent asks for confirmation
echo -n "Test 9: Agent asking for confirmation triggers Waiting status... "
tmux -S "$SOCK" send-keys -t "$pane3_id" "Requesting permission for: git status" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "Run this command?" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "> 1. Yes, run command" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "esc to cancel" C-m
sleep 0.2
tmux -S "$SOCK" run-shell "bash '$BIN' scan"
status_out=$(get_status)
if [[ "$status_out" =~ 🤖[[:space:]]+⏳[[:space:]]+1 ]]; then
    echo "PASS (Upgraded to waiting: $status_out)"
else
    echo "FAIL: Expected '🤖 ⏳ 1', got '$status_out'"
    exit 1
fi

# 11. Agent resolves and returns to idle
echo -n "Test 10: Idle resolution clears waiting status... "
tmux -S "$SOCK" send-keys -t "$pane3_id" "Command finished." C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "? for shortcuts" C-m
sleep 0.2
tmux -S "$SOCK" select-window -t test-sess:win3
tmux -S "$SOCK" run-shell "bash '$BIN' on-focus $pane3_id"
tmux -S "$SOCK" run-shell "bash '$BIN' scan"
status_out=$(get_status)
if [[ -z "$status_out" ]] || ! [[ "$status_out" =~ ⏳ ]]; then
    echo "PASS (Waiting status cleared)"
else
    echo "FAIL: Expected waiting status cleared, got '$status_out'"
    exit 1
fi

# 12. Dead pane pruning
echo -n "Test 11: Dead pane pruning removes terminated panes from state and status... "
tmux -S "$SOCK" new-window -t test-sess -n win_dead "cat"
pane_dead_id=$(tmux -S "$SOCK" list-panes -t test-sess:win_dead -F '#{pane_id}')
tmux -S "$SOCK" select-window -t test-sess:win_dead
tmux -S "$SOCK" run-shell "bash '$BIN' toggle-vigil"
tmux -S "$SOCK" select-window -t test-sess:win2
# Verify it was added
status_out=$(get_status)
if ! [[ "$status_out" =~ 👁️ ]]; then
    echo "FAIL: Expected vigil watch before pane kill, got '$status_out'"
    exit 1
fi
# Kill the window
tmux -S "$SOCK" kill-window -t test-sess:win_dead
# Run scan / status
status_out=$(get_status)
if [[ -z "$status_out" ]] && ! grep -q "$pane_dead_id" "$STATE_FILE" 2>/dev/null; then
    echo "PASS (Dead pane pruned from state)"
else
    echo "FAIL: Dead pane was not pruned, status='$status_out'"
    exit 1
fi

# 13. Composite multi-badge status line formatting (User-First domain ordering)
echo -n "Test 12: Composite multi-badge status line preserves User-First ordering... "
# Create panes for each category:
# 1. Alert pane:
tmux -S "$SOCK" new-window -t test-sess -n win_c_alert "cat"
pane_c_alert=$(tmux -S "$SOCK" list-panes -t test-sess:win_c_alert -F '#{pane_id}')
tmux -S "$SOCK" select-window -t test-sess:win_c_alert
tmux -S "$SOCK" run-shell "bash '$BIN' toggle-vigil"

# 2. Quiet Vigil pane:
tmux -S "$SOCK" new-window -t test-sess -n win_c_vigil "cat"
tmux -S "$SOCK" select-window -t test-sess:win_c_vigil
tmux -S "$SOCK" run-shell "bash '$BIN' toggle-vigil"

# 3. Agent Waiting pane:
tmux -S "$SOCK" new-window -t test-sess -n win_c_wait "head -n 1000"
pane_c_wait=$(tmux -S "$SOCK" list-panes -t test-sess:win_c_wait -F '#{pane_id}')
tmux -S "$SOCK" send-keys -t "$pane_c_wait" "Requesting permission for command" C-m

# 4. Agent Running pane:
tmux -S "$SOCK" new-window -t test-sess -n win_c_run "head -n 1000"
pane_c_run=$(tmux -S "$SOCK" list-panes -t test-sess:win_c_run -F '#{pane_id}')
tmux -S "$SOCK" send-keys -t "$pane_c_run" "Processing batch items" C-m
tmux -S "$SOCK" send-keys -t "$pane_c_run" "esc to cancel" C-m

# Trigger alert on win_c_alert
tmux -S "$SOCK" send-keys -t "$pane_c_alert" "ALERT TRIGGER OUTPUT" C-m

# Switch to win2 so all test panes are background
tmux -S "$SOCK" select-window -t test-sess:win2
sleep 0.2
tmux -S "$SOCK" run-shell "bash '$BIN' scan"

status_out=$(get_status)
# Strip ANSI escapes to check order: 🚨 1  👁️ 1  🤖 ⏳ 1  🤖 ⚡ 1
clean_status=$(echo "$status_out" | sed -E 's/#\[[^]]*\]//g')
if [[ "$clean_status" =~ 🚨[[:space:]]*1.*👁️[[:space:]]*1.*🤖[[:space:]]*⏳[[:space:]]*1.*🤖[[:space:]]*⚡[[:space:]]*1 ]]; then
    echo "PASS (Composite status: $clean_status)"
else
    echo "FAIL: Expected user-first ordering (🚨 -> 👁️ -> 🤖 ⏳ -> 🤖 ⚡), got '$clean_status'"
    exit 1
fi

# Clean up composite test windows
tmux -S "$SOCK" kill-window -t test-sess:win_c_alert
tmux -S "$SOCK" kill-window -t test-sess:win_c_vigil
tmux -S "$SOCK" kill-window -t test-sess:win_c_wait
tmux -S "$SOCK" kill-window -t test-sess:win_c_run
tmux -S "$SOCK" run-shell "bash '$BIN' scan"

# 14. Agent process exit detection
echo -n "Test 13: Agent process exit transitions state to 'done' (Finished)... "
tmux -S "$SOCK" new-window -t test-sess -n win_c_exit "bash"
pane_c_exit=$(tmux -S "$SOCK" list-panes -t test-sess:win_c_exit -F '#{pane_id}')
# Seed the state file with an agent running in this pane
printf "%s\ttest-sess\t1\t0\t/tmp\thead\trunning in tmp\tauto\trunning\n" "$pane_c_exit" >> "$STATE_FILE"
tmux -S "$SOCK" select-window -t test-sess:win2
# When scan runs, cmd is 'cat' (generic) and ps has no agy, so it detects exit -> done
tmux -S "$SOCK" run-shell "bash '$BIN' scan"
status_out=$(get_status)
if [[ "$status_out" =~ 🤖[[:space:]]+✓[[:space:]]+1 ]]; then
    echo "PASS (Detected process exit, status: $status_out)"
else
    echo "FAIL: Expected '🤖 ✓ 1', got '$status_out'"
    exit 1
fi
# Focus to acknowledge and clear done
tmux -S "$SOCK" select-window -t test-sess:win_c_exit
tmux -S "$SOCK" run-shell "bash '$BIN' on-focus $pane_c_exit"
tmux -S "$SOCK" kill-window -t test-sess:win_c_exit
status_out=$(get_status)
if [[ "$status_out" =~ 🤖 ]]; then
    echo "FAIL: Done status not cleared on focus"
    exit 1
fi

# 15. Stale lock recovery
echo -n "Test 14: Stale directory lock is automatically recovered... "
# Manually create a stale lock
mkdir -p "${STATE_FILE}.lock"
# Running status should encounter the lock, break it after threshold, and succeed
status_out=$(get_status)
if [[ ! -d "${STATE_FILE}.lock" ]]; then
    echo "PASS (Stale lock broken and released)"
else
    echo "FAIL: Lock directory still exists after operation"
    exit 1
fi

echo "All headless tmux lifecycle tests passed successfully!"
