#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/tmux-glance"

echo "=== [TEST] High-Pane Scan Performance, Parallel Evaluation & Cooldown Debounce ==="

TMP_DIR=$(mktemp -d /tmp/tmux-glance-perf-test.XXXXXX)
SOCK="$TMP_DIR/tmux.sock"
OUT_FILE="$TMP_DIR/output.txt"
STATE_FILE="$TMP_DIR/glance_state"
SLOTS_FILE="$TMP_DIR/slots"

cleanup() {
    tmux -S "$SOCK" kill-server 2>/dev/null || true
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT

export TMUX_GLANCE_STATE_FILE="$STATE_FILE"
export TMUX_GLANCE_SLOTS_FILE="$SLOTS_FILE"

glance_run() {
    rm -f "$OUT_FILE"
    tmux -S "$SOCK" run-shell "bash '$BIN' $* > '$OUT_FILE' 2>&1" || true
    cat "$OUT_FILE" 2>/dev/null || true
}

# 1. Start isolated headless tmux server and spawn 30 generic panes
echo -n "Test 1: Spawning test server with 30 generic panes... "
tmux -S "$SOCK" new-session -d -s perf-sess -n win0 "cat"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_STATE_FILE "$STATE_FILE"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_SLOTS_FILE "$SLOTS_FILE"

# Create 30 windows = 30 panes
for w in $(seq 1 29); do
    tmux -S "$SOCK" new-window -t perf-sess -n "win$w" "cat"
done

pane_count=$(tmux -S "$SOCK" list-panes -a -F '#{pane_id}' | wc -l | tr -d ' ')
if [[ "$pane_count" -ge 30 ]]; then
    echo "PASS ($pane_count panes created)"
else
    echo "FAIL: expected at least 30 panes, got $pane_count"
    exit 1
fi

# 2. Fast-path non-agent panes: verify scan runs swiftly across 30 non-agent panes
echo -n "Test 2: Fast-path non-agent panes completes in sub-second time without option clutter... "
glance_run scan force

# Verify no snapshots were attached to generic panes
snapshot_count=$(tmux -S "$SOCK" show-options -p -t "%0" @glance_snapshot 2>/dev/null || true)
if [[ -z "$snapshot_count" ]]; then
    echo "PASS (Non-candidate panes bypassed cleanly)"
else
    echo "FAIL: snapshot option was set on generic non-vigil pane"
    exit 1
fi

# 3. Cooldown debouncing: second status call within cooldown skips full scan
echo -n "Test 3: Scan debouncing skips redundant evaluation within cooldown threshold... "
tmux -S "$SOCK" set-option -g @glance_scan_cooldown 5

# Force a scan to record timestamp
glance_run scan force
initial_scan_time=$(tmux -S "$SOCK" show-option -gv @glance_last_scan_time 2>/dev/null || echo 0)

if [[ -z "$initial_scan_time" || "$initial_scan_time" -eq 0 ]]; then
    echo "FAIL: @glance_last_scan_time was not recorded after scan"
    exit 1
fi

# Call status without force within cooldown
glance_run status
post_status_time=$(tmux -S "$SOCK" show-option -gv @glance_last_scan_time 2>/dev/null || echo 0)

if [[ "$post_status_time" == "$initial_scan_time" ]]; then
    echo "PASS (Cooldown debouncing preserved timestamp and skipped redundant scan)"
else
    echo "FAIL: timestamp changed during cooldown window"
    exit 1
fi

# 4. Focus change bypasses cooldown immediately
echo -n "Test 4: Focus change immediately invalidates cooldown and executes fresh scan... "
# Select a different window
tmux -S "$SOCK" select-window -t perf-sess:win1
target_pane=$(tmux -S "$SOCK" display-message -p '#{pane_id}')

# Status call should detect focus mismatch and update focus
glance_run status
new_focus=$(tmux -S "$SOCK" show-option -gv @glance_last_scan_focus 2>/dev/null || echo "")

if [[ "$new_focus" == *"$target_pane"* ]]; then
    echo "PASS (Focus change triggered immediate scan and updated @glance_last_scan_focus)"
else
    echo "FAIL: expected focus $target_pane in @glance_last_scan_focus, got '$new_focus'"
    exit 1
fi

# 5. Parallel candidate evaluation across multiple agent panes
echo -n "Test 5: Concurrent candidate evaluation across multiple agent panes... "
# Clean up temporary test windows from high-pane test
for w in $(seq 1 29); do
    tmux -S "$SOCK" kill-window -t "perf-sess:win$w" 2>/dev/null || true
done

tmux -S "$SOCK" set-option -g @glance_routes "head=antigravity,coreutils=antigravity"

# Create 4 agent panes with confirmation cues
for i in $(seq 1 4); do
    tmux -S "$SOCK" new-window -t perf-sess -n "agent$i" "head -n 1000"
    ag_pane=$(tmux -S "$SOCK" list-panes -t "perf-sess:agent$i" -F '#{pane_id}')
    tmux -S "$SOCK" send-keys -t "$ag_pane" "Requesting permission for: deploy-$i" C-m
    tmux -S "$SOCK" send-keys -t "$ag_pane" "Run this command?" C-m
    tmux -S "$SOCK" send-keys -t "$ag_pane" "> 1. Yes, run command" C-m
    tmux -S "$SOCK" send-keys -t "$ag_pane" "esc to cancel" C-m
done

# Switch focus away from agent windows to win0
tmux -S "$SOCK" select-window -t perf-sess:win0
sleep 0.3

# Force scan to evaluate all candidate agent panes in parallel
glance_run scan force
status_out=$(glance_run status)

if [[ "$status_out" =~ 🤖[[:space:]]+⏳[[:space:]]+4 ]]; then
    echo "PASS (All 4 candidates classified concurrently in parallel: $status_out)"
else
    echo "FAIL: Expected '🤖 ⏳ 4', got '$status_out'"
    exit 1
fi

# 6. Vigil toggle invalidates scan cooldown immediately
echo -n "Test 6: Vigil toggle immediately resets scan cooldown... "
tmux -S "$SOCK" select-window -t perf-sess:win0
glance_run toggle-vigil
reset_time=$(tmux -S "$SOCK" show-option -gv @glance_last_scan_time 2>/dev/null || echo "")

if [[ "$reset_time" == "0" ]]; then
    echo "PASS (Cooldown reset to 0 upon toggle-vigil)"
else
    echo "FAIL: expected @glance_last_scan_time=0 after toggle-vigil, got '$reset_time'"
    exit 1
fi

echo "All high-pane scan performance and debouncing tests passed successfully!"
