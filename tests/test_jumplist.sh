#!/usr/bin/env bash
set -euo pipefail

# Ensure test execution never inherits or interacts with an active outer tmux session
unset TMUX TMUX_PANE

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

# 2. Empty jump list initially shows current active pane
echo -n "Test 2: Empty jump list outputs current active pane as CUR... "
res=$(glance_run list-raw history)
if [[ "$res" == *"📍 CUR"* && "$res" == *"$pane3"* ]]; then
    echo "PASS"
else
    echo "FAIL: Expected current pane as CUR, got: $res"
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
echo -n "Test 7: list-raw history formats unified timeline (CUR, BACK #1, BACK #2)... "
hist_out=$(glance_run list-raw history)
if [[ "$hist_out" == *"📍 CUR"* && "$hist_out" == *"⏮️  BACK #1"* && "$hist_out" == *"⏮️  BACK #2"* && "$hist_out" == *"$pane2"* && "$hist_out" == *"$pane1"* ]]; then
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
echo -n "Test 15: clear-history empties both stacks and retains CUR... "
printf "%s\n" "$pane1" >> "$TMUX_GLANCE_JUMP_BACK_FILE"
printf "%s\n" "$pane4" >> "$TMUX_GLANCE_JUMP_FORWARD_FILE"
glance_run clear-history
if [[ -s "$TMUX_GLANCE_JUMP_BACK_FILE" || -s "$TMUX_GLANCE_JUMP_FORWARD_FILE" ]]; then
    echo "FAIL: Expected empty jump stack files"
    exit 1
fi
hist_raw=$(glance_run list-raw history)
if [[ "$hist_raw" != *"📍 CUR"* ]]; then
    echo "FAIL: Expected CUR entry after clear, got: $hist_raw"
    exit 1
fi
echo "PASS"

# 16. Unified Timeline with Forward, Current, and Back entries
echo -n "Test 16: Unified timeline correctly orders FWD above CUR above BACK... "
tmux -S "$SOCK" select-window -t test-sess:win1
printf "%s\n" "$pane4" > "$TMUX_GLANCE_JUMP_FORWARD_FILE"
printf "%s\n" "$pane3" > "$TMUX_GLANCE_JUMP_BACK_FILE"
timeline_raw=$(glance_run list-raw history)
fwd_line=$(echo "$timeline_raw" | grep "FWD #1" || true)
cur_line=$(echo "$timeline_raw" | grep "CUR" || true)
back_line=$(echo "$timeline_raw" | grep "BACK #1" || true)
if [[ -z "$fwd_line" || -z "$cur_line" || -z "$back_line" ]]; then
    echo "FAIL: Missing timeline components: $timeline_raw"
    exit 1
fi
# Check vertical ordering (FWD before CUR before BACK)
fwd_pos=$(echo "$timeline_raw" | grep -n "FWD #1" | cut -d: -f1)
cur_pos=$(echo "$timeline_raw" | grep -n "CUR" | cut -d: -f1)
back_pos=$(echo "$timeline_raw" | grep -n "BACK #1" | cut -d: -f1)
if [[ "$fwd_pos" -lt "$cur_pos" && "$cur_pos" -lt "$back_pos" ]]; then
    echo "PASS (FWD at $fwd_pos, CUR at $cur_pos, BACK at $back_pos)"
else
    echo "FAIL: Incorrect vertical timeline ordering (FWD=$fwd_pos, CUR=$cur_pos, BACK=$back_pos)"
    exit 1
fi

# 17. Cursor Position Arithmetic (get_history_cursor_pos)
echo -n "Test 17: Cursor position targets BACK #1 when available, else FWD #1, else CUR... "
# With 1 FWD and 1 BACK: target should be line 3 (BACK #1)
cpos=$(glance_run history-cursor-pos)
if [[ "$cpos" -ne 3 ]]; then
    echo "FAIL: Expected cursor pos 3 for BACK #1, got $cpos"
    exit 1
fi
# With only FWD (no BACK): target should be line 1 (FWD #1)
: > "$TMUX_GLANCE_JUMP_BACK_FILE"
cpos_fwd=$(glance_run history-cursor-pos)
if [[ "$cpos_fwd" -ne 1 ]]; then
    echo "FAIL: Expected cursor pos 1 for FWD #1, got $cpos_fwd"
    exit 1
fi
# With clean slate (no FWD and no BACK): target should be line 1 (CUR)
: > "$TMUX_GLANCE_JUMP_FORWARD_FILE"
cpos_cur=$(glance_run history-cursor-pos)
if [[ "$cpos_cur" -ne 1 ]]; then
    echo "FAIL: Expected cursor pos 1 for CUR, got $cpos_cur"
    exit 1
fi
echo "PASS"

# 18. Interactive FZF history navigation and CUR no-op
echo -n "Test 18: Interactive history starts on BACK #1 and pressing Enter on CUR is a safe no-op... "
printf "%s\n" "$pane4" > "$TMUX_GLANCE_JUMP_FORWARD_FILE"
printf "%s\n" "$pane3" > "$TMUX_GLANCE_JUMP_BACK_FILE"
tmux -S "$SOCK" select-window -t test-sess:win1
tmux -S "$SOCK" new-window -t test-sess -n test-fzf "env TMUX_GLANCE_DIR='$TMP_DIR' TMUX_GLANCE_JUMP_BACK_FILE='$TMP_DIR/jump_back' TMUX_GLANCE_JUMP_FORWARD_FILE='$TMP_DIR/jump_forward' $BIN list-history; sleep 1"
sleep 0.5
# Send Up to move from BACK #1 to CUR, then Enter to select CUR
tmux -S "$SOCK" send-keys -t test-sess:test-fzf "Up"
sleep 0.2
tmux -S "$SOCK" send-keys -t test-sess:test-fzf "Enter"
sleep 0.4
active_p=$(tmux -S "$SOCK" display-message -p -t test-sess:win1 '#{pane_id}')
if [[ "$active_p" == "$pane1" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected active pane to remain $pane1, got $active_p"
    exit 1
fi
tmux -S "$SOCK" kill-window -t test-sess:test-fzf 2>/dev/null || true

# 19. Interactive FZF history selection shifts frame without wiping forward stack (Issue #2)
echo -n "Test 19: History modal selection shifts CUR frame and preserves forward stack (Issue #2)... "
printf "%s\n" "$pane4" > "$TMUX_GLANCE_JUMP_FORWARD_FILE"
printf "%s\n" "$pane3" > "$TMUX_GLANCE_JUMP_BACK_FILE"
tmux -S "$SOCK" select-window -t test-sess:win1
tmux -S "$SOCK" new-window -t test-sess -n test-fzf "env TMUX_GLANCE_DIR='$TMP_DIR' TMUX_GLANCE_JUMP_BACK_FILE='$TMP_DIR/jump_back' TMUX_GLANCE_JUMP_FORWARD_FILE='$TMP_DIR/jump_forward' $BIN list-history; sleep 1"
sleep 0.5
# Cursor starts on BACK #1 ($pane3). Press Enter to jump to $pane3.
tmux -S "$SOCK" send-keys -t test-sess:test-fzf "Enter"
sleep 0.5
fwd_after=$(cat "$TMUX_GLANCE_JUMP_FORWARD_FILE" 2>/dev/null || true)
back_after=$(cat "$TMUX_GLANCE_JUMP_BACK_FILE" 2>/dev/null || true)
active_p=$(tmux -S "$SOCK" display-message -p -t test-sess:win3 '#{pane_id}')
# Verify active pane is now pane3, back stack is empty (pane3 was consumed), and pane4 remains in forward stack
if [[ "$active_p" == "$pane3" ]] && [[ "$fwd_after" == *"$pane4"* ]] && [[ -z "$back_after" ]]; then
    echo "PASS (Forward preserved: $fwd_after, Back: empty, Active: $active_p)"
else
    echo "FAIL: Expected active=$pane3 and forward stack to retain $pane4, got: active='$active_p', fwd='$fwd_after', back='$back_after'"
    exit 1
fi
tmux -S "$SOCK" kill-window -t test-sess:test-fzf 2>/dev/null || true

echo "All jumplist tests passed successfully!"

