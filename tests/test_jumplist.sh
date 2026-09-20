#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/tmux-glance"

echo "=== [TEST] Jump History & Jumplist Backtracking ==="

TMP_DIR=$(mktemp -d /tmp/tmux-glance-jumplist-test.XXXXXX)
SOCK="$TMP_DIR/tmux.sock"
OUT_FILE="$TMP_DIR/output.txt"
trap 'tmux -S "$SOCK" kill-server 2>/dev/null || true; rm -rf "$TMP_DIR"' EXIT

export TMUX_GLANCE_JUMP_DIR="$TMP_DIR"
export TMUX_GLANCE_JUMP_BACK_FILE="$TMP_DIR/jump_back"
export TMUX_GLANCE_JUMP_FORWARD_FILE="$TMP_DIR/jump_forward"
export TMUX_GLANCE_STATE_FILE="$TMP_DIR/glance_state"
export TMUX_GLANCE_SLOTS_FILE="$TMP_DIR/slots"

# Helper to run glance commands inside the test tmux server
glance_run() {
    rm -f "$OUT_FILE"
    tmux -S "$SOCK" run-shell "bash '$BIN' $* > '$OUT_FILE' 2>&1" || true
    cat "$OUT_FILE" 2>/dev/null || true
}

# 1. Start isolated headless tmux server
echo -n "Test 1: Spawning test tmux server with multiple windows... "
tmux -S "$SOCK" new-session -d -s test-sess -n win1 "cat"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_JUMP_DIR "$TMP_DIR"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_JUMP_BACK_FILE "$TMP_DIR/jump_back"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_JUMP_FORWARD_FILE "$TMP_DIR/jump_forward"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_STATE_FILE "$TMP_DIR/glance_state"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_SLOTS_FILE "$TMP_DIR/slots"

pane1=$(tmux -S "$SOCK" list-panes -t test-sess:win1 -F '#{pane_id}')
tmux -S "$SOCK" new-window -t test-sess -n win2 "cat"
pane2=$(tmux -S "$SOCK" list-panes -t test-sess:win2 -F '#{pane_id}')
tmux -S "$SOCK" new-window -t test-sess -n win3 "cat"
pane3=$(tmux -S "$SOCK" list-panes -t test-sess:win3 -F '#{pane_id}')
echo "PASS (Panes: $pane1, $pane2, $pane3)"

# 2. Empty jump list initially
echo -n "Test 2: Empty jump list outputs quiet empty placeholder... "
res=$(glance_run list-raw history)
if [[ "$res" =~ "🕒 Empty" && "$res" =~ "No jump history recorded yet" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected empty placeholder, got: $res"
    exit 1
fi

# 3. Record jump pane1 -> pane2
echo -n "Test 3: Recording jump pushes source pane to back stack... "
glance_run record-jump "$pane1" "$pane2"
back_content=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")
if [[ "$back_content" == "$pane1" ]]; then
    echo "PASS (Back stack: $back_content)"
else
    echo "FAIL: Expected '$pane1', got '$back_content'"
    exit 1
fi

# 4. Record jump pane2 -> pane3
echo -n "Test 4: Recording subsequent jump prepends to back stack... "
glance_run record-jump "$pane2" "$pane3"
line1=$(sed -n '1p' "$TMUX_GLANCE_JUMP_BACK_FILE")
line2=$(sed -n '2p' "$TMUX_GLANCE_JUMP_BACK_FILE")
if [[ "$line1" == "$pane2" && "$line2" == "$pane1" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected [$pane2, $pane1], got [$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")]"
    exit 1
fi

# 5. Deduplication of consecutive jumps
echo -n "Test 5: Consecutive duplicate jumps are deduplicated... "
glance_run record-jump "$pane2" "$pane3"
count=$(wc -l < "$TMUX_GLANCE_JUMP_BACK_FILE" | tr -d ' ')
if [[ "$count" -eq 2 ]]; then
    echo "PASS (Stack count remained 2)"
else
    echo "FAIL: Expected count 2, got $count"
    exit 1
fi

# 6. Same source and dest is a no-op
echo -n "Test 6: Same source and destination jump is a no-op... "
glance_run record-jump "$pane3" "$pane3"
count=$(wc -l < "$TMUX_GLANCE_JUMP_BACK_FILE" | tr -d ' ')
if [[ "$count" -eq 2 ]]; then
    echo "PASS"
else
    echo "FAIL: Expected count 2, got $count"
    exit 1
fi

# 7. Formatted history list output
echo -n "Test 7: list-raw history formats live panes with metadata and preview target... "
hist_out=$(glance_run list-raw history)
if [[ "$hist_out" == *"🕒 Jump #1"* && "$hist_out" == *"🕒 Jump #2"* && "$hist_out" == *"$pane2"* && "$hist_out" == *"$pane1"* ]]; then
    echo "PASS"
else
    echo "FAIL: Formatted history output missing expected entries: $hist_out"
    exit 1
fi

# 8. Jump back (Undo)
echo -n "Test 8: jump-back switches to previous pane and updates forward stack... "
tmux -S "$SOCK" select-window -t test-sess:win3
glance_run jump-back "$pane3"
back_after=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")
fwd_after=$(cat "$TMUX_GLANCE_JUMP_FORWARD_FILE")
if [[ "$back_after" == "$pane1" && "$fwd_after" == "$pane3" ]]; then
    echo "PASS (Back: $back_after, Forward: $fwd_after)"
else
    echo "FAIL: Expected Back=[$pane1], Forward=[$pane3], got Back=[$back_after], Forward=[$fwd_after]"
    exit 1
fi

# 9. Jump back to oldest location
echo -n "Test 9: jump-back to oldest location empties back stack... "
glance_run jump-back "$pane2"
back_oldest=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")
if [[ -z "$back_oldest" ]]; then
    echo "PASS (Back stack empty)"
else
    echo "FAIL: Expected empty back stack, got: $back_oldest"
    exit 1
fi

# 10. Attempt jump back past oldest
echo -n "Test 10: jump-back past oldest location is a safe no-op... "
glance_run jump-back "$pane1"
if [[ -z "$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected empty back stack"
    exit 1
fi

# 11. Jump forward (Redo)
echo -n "Test 11: jump-forward traverses back up the stack... "
glance_run jump-forward "$pane1"
back_fwd=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")
fwd_fwd=$(cat "$TMUX_GLANCE_JUMP_FORWARD_FILE")
if [[ "$back_fwd" == "$pane1" && "$fwd_fwd" == "$pane3" ]]; then
    echo "PASS (Back: $back_fwd, Forward: $fwd_fwd)"
else
    echo "FAIL: Expected Back=[$pane1], Forward=[$pane3], got Back=[$back_fwd], Forward=[$fwd_fwd]"
    exit 1
fi

# 12. Forward stack invalidation on new jump
echo -n "Test 12: New jump invalidates and clears forward stack... "
tmux -S "$SOCK" new-window -t test-sess -n win4 "cat"
pane4=$(tmux -S "$SOCK" list-panes -t test-sess:win4 -F '#{pane_id}')
glance_run record-jump "$pane2" "$pane4"
fwd_cleared=$(cat "$TMUX_GLANCE_JUMP_FORWARD_FILE")
if [[ -z "$fwd_cleared" ]]; then
    echo "PASS (Forward stack cleared)"
else
    echo "FAIL: Expected forward stack to be cleared, got: $fwd_cleared"
    exit 1
fi

# 13. Dead pane pruning on jump-back
echo -n "Test 13: Dead panes are silently pruned during jump-back... "
# Currently back stack has [pane2, pane1]
tmux -S "$SOCK" kill-window -t test-sess:win2
# Now pane2 is dead. When we jump back from pane4, it should skip dead pane2 and pop pane1!
glance_run jump-back "$pane4"
back_pruned=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE")
if [[ -z "$back_pruned" ]]; then
    echo "PASS (Dead pane skipped, landed on pane1)"
else
    echo "FAIL: Expected empty back stack after skipping dead pane, got: $back_pruned"
    exit 1
fi

# 14. Stack size bounding (max 50)
echo -n "Test 14: Jump stack size is bounded to max 50 entries... "
for i in $(seq 1 60); do
    printf "%%dummy%d\n" "$i" >> "$TMUX_GLANCE_JUMP_BACK_FILE"
done
glance_run record-jump "$pane1" "$pane4"
bound_count=$(wc -l < "$TMUX_GLANCE_JUMP_BACK_FILE" | tr -d ' ')
if [[ "$bound_count" -le 50 ]]; then
    echo "PASS (Bounded to $bound_count entries)"
else
    echo "FAIL: Expected <= 50 entries, got $bound_count"
    exit 1
fi

# 15. Stack clearing (clear-history)
echo -n "Test 15: clear-history empties both stacks and updates raw listing... "
printf "%s\n" "$pane1" >> "$TMUX_GLANCE_JUMP_BACK_FILE"
printf "%s\n" "$pane4" >> "$TMUX_GLANCE_JUMP_FORWARD_FILE"
glance_run clear-history
if [[ -s "$TMUX_GLANCE_JUMP_BACK_FILE" || -s "$TMUX_GLANCE_JUMP_FORWARD_FILE" ]]; then
    echo "FAIL: Expected empty jump stack files"
    exit 1
fi
hist_raw=$(glance_run list-raw history)
if [[ "$hist_raw" != *"No jump history recorded yet"* ]]; then
    echo "FAIL: Expected empty history message, got: $hist_raw"
    exit 1
fi
echo "PASS"

echo "All jumplist tests passed successfully!"
