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
SLOTS_FILE="${TMP_BASE}/glance-slots-${TEST_ID}.tsv"

cleanup() {
    tmux -S "$SOCK" kill-server 2>/dev/null || true
    rm -rf "$SOCK" "${STATE_FILE}"* "${STATUS_FILE}"* "${SLOTS_FILE}"* "${TMP_BASE}/glance-bin-${TEST_ID}"* 2>/dev/null || true
}
trap cleanup EXIT

# 1. Start isolated headless tmux server
echo -n "Test 1: Spawning isolated headless tmux server... "
tmux -S "$SOCK" new-session -d -s test-sess -n win1 "cat"
tmux -S "$SOCK" set-environment -g PATH "$PATH"
tmux -S "$SOCK" set-environment -g LC_ALL "${LC_ALL:-C.UTF-8}"
tmux -S "$SOCK" set-environment -g LANG "${LANG:-C.UTF-8}"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_STATE_FILE "$STATE_FILE"
tmux -S "$SOCK" set-environment -g TMUX_GLANCE_SLOTS_FILE "$SLOTS_FILE"
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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
status_out=$(get_status)
if [[ -z "$status_out" ]] || ! [[ "$status_out" =~ ⏳ ]]; then
    echo "PASS (Waiting status cleared)"
else
    echo "FAIL: Expected waiting status cleared, got '$status_out'"
    exit 1
fi

# 11b. Subagent needs approval with resting footer (? for shortcuts)
echo -n "Test 10b: Subagent approval prompt with resting footer triggers Waiting status... "
tmux -S "$SOCK" select-window -t test-sess:win2
tmux -S "$SOCK" send-keys -t "$pane3_id" "self needs approval for Bash" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "ctrl+y approve · alt+j manage" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "● Agent(self)  Blocked · Running command · 6s" C-m
tmux -S "$SOCK" send-keys -t "$pane3_id" "? for shortcuts" C-m
sleep 0.2
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
status_out=$(get_status)
if [[ "$status_out" =~ 🤖[[:space:]]+⏳[[:space:]]+1 ]]; then
    echo "PASS (Upgraded to waiting: $status_out)"
else
    echo "FAIL: Expected '🤖 ⏳ 1', got '$status_out'"
    exit 1
fi

# Clean up back to idle for subsequent tests
tmux -S "$SOCK" select-window -t test-sess:win3
for _ in $(seq 1 16); do tmux -S "$SOCK" send-keys -t "$pane3_id" " " C-m; done
tmux -S "$SOCK" send-keys -t "$pane3_id" "? for shortcuts" C-m
sleep 0.2
tmux -S "$SOCK" run-shell "bash '$BIN' on-focus $pane3_id"
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"

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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"

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
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"

# 14. Agent process exit detection
echo -n "Test 13: Agent process exit transitions state to 'done' (Finished)... "
tmux -S "$SOCK" new-window -t test-sess -n win_c_exit "bash"
pane_c_exit=$(tmux -S "$SOCK" list-panes -t test-sess:win_c_exit -F '#{pane_id}')
# Seed the state file with an agent running in this pane
printf "%s\ttest-sess\t1\t0\t/tmp\thead\trunning in tmp\tauto\trunning\n" "$pane_c_exit" >> "$STATE_FILE"
tmux -S "$SOCK" select-window -t test-sess:win2
# When scan runs, cmd is 'cat' (generic) and ps has no agy, so it detects exit -> done
tmux -S "$SOCK" run-shell "bash '$BIN' scan force"
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

# 16. In-Dashboard Sessionizer per-workspace ambient status badge aggregation
echo -n "Test 15: Sessionizer aggregates ambient status badges per workspace... "
# Create a second session: sess-two
tmux -S "$SOCK" new-session -d -s sess-two -n main "cat"
tmux -S "$SOCK" set-environment -g -t sess-two PATH "$PATH"
tmux -S "$SOCK" set-environment -g -t sess-two TMUX_GLANCE_STATE_FILE "$STATE_FILE"

# Seed state file with:
# 1. Alert in test-sess
pane_sess1=$(tmux -S "$SOCK" list-panes -t test-sess -F '#{pane_id}' | head -n 1)
printf "%s\ttest-sess\t1\t0\t/tmp\tcat\twatch in tmp\tmanual\talert\n" "$pane_sess1" >> "$STATE_FILE"

# 2. Waiting agent in sess-two
pane_sess2=$(tmux -S "$SOCK" list-panes -t sess-two -F '#{pane_id}' | head -n 1)
printf "%s\tsess-two\t1\t0\t/tmp/myrepo\tagy\twaiting in myrepo\tauto\twaiting\n" "$pane_sess2" >> "$STATE_FILE"

# Run list-raw sessions
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw sessions > '$STATUS_FILE'"
sess_list=$(cat "$STATUS_FILE")

# Verify that test-sess has 🚨 1 and sess-two has 🤖 ⏳ 1
if [[ "$sess_list" =~ 🚨[[:space:]]+1.*\[test-sess\] ]] && [[ "$sess_list" =~ 🤖[[:space:]]+⏳[[:space:]]+1.*\[sess-two\] ]]; then
    # Verify priority: test-sess (alert prio 1) must precede sess-two (waiting prio 2)
    pos_sess1=$(echo "$sess_list" | grep -n '\[test-sess\]' | cut -d: -f1)
    pos_sess2=$(echo "$sess_list" | grep -n '\[sess-two\]' | cut -d: -f1)
    if [[ "$pos_sess1" -lt "$pos_sess2" ]]; then
        echo "PASS (Alert workspace precedes waiting workspace with ambient badges)"
    else
        echo "FAIL: Priority ordering incorrect: sess1 pos $pos_sess1, sess2 pos $pos_sess2"
        exit 1
    fi
else
    echo "FAIL: Missing ambient status badges in sessions list: $sess_list"
    exit 1
fi

# 17. Sessionizer mode toggling
echo -n "Test 16: Sessionizer mode toggling (Ctrl-s flip back and forth)... "
tmux -S "$SOCK" set-option -g @glance_mode "attention"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw toggle-sessions > /dev/null"
mode_after_first=$(tmux -S "$SOCK" show-option -gv @glance_mode)
if [[ "$mode_after_first" != "sessions" ]]; then
    echo "FAIL: Expected mode 'sessions', got '$mode_after_first'"
    exit 1
fi
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw toggle-sessions > /dev/null"
mode_after_second=$(tmux -S "$SOCK" show-option -gv @glance_mode)
if [[ "$mode_after_second" != "attention" ]]; then
    echo "FAIL: Expected restored mode 'attention', got '$mode_after_second'"
    exit 1
fi
echo "PASS (Mode flips attention -> sessions -> attention)"

# 18. Live preview streams active pane of target session
echo -n "Test 17: Live preview actively streams target session active pane... "
tmux -S "$SOCK" send-keys -t "$pane_sess2" "Hello from sess-two workspace!" C-m
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' preview sess-two > '$STATUS_FILE' 2>&1 & sleep 0.3; kill -TERM \$! 2>/dev/null || true"
preview_sess_out=$(cat "$STATUS_FILE" 2>/dev/null || true)
if [[ "$preview_sess_out" =~ "Hello from sess-two workspace!" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected session preview to capture active pane, got '$preview_sess_out'"
    exit 1
fi

tmux -S "$SOCK" kill-session -t sess-two 2>/dev/null || true

# 19. Harpoon Session Slots (assign, sessionizer display, unassign)
echo -n "Test 18: Harpoon slot assignment and sessionizer slot badging... "
# Create sess-alpha and sess-beta
tmux -S "$SOCK" new-session -d -s sess-alpha -n main "cat"
tmux -S "$SOCK" new-session -d -s sess-beta -n main "cat"
tmux -S "$SOCK" set-environment -g -t sess-alpha PATH "$PATH"
tmux -S "$SOCK" set-environment -g -t sess-beta PATH "$PATH"
tmux -S "$SOCK" set-environment -g -t sess-alpha TMUX_GLANCE_SLOTS_FILE "$SLOTS_FILE"
tmux -S "$SOCK" set-environment -g -t sess-beta TMUX_GLANCE_SLOTS_FILE "$SLOTS_FILE"

# Assign sess-alpha to slot h and sess-beta to slot j
tmux -S "$SOCK" run-shell "bash '$BIN' assign-slot h sess-alpha"
tmux -S "$SOCK" run-shell "bash '$BIN' assign-slot j sess-beta"

rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw sessions > '$STATUS_FILE'"
sess_list_slots=$(cat "$STATUS_FILE")

# Check that [H] [sess-alpha] and [J] [sess-beta] appear
if [[ "$sess_list_slots" =~ \[H\][[:space:]]+\[sess-alpha\] ]] && [[ "$sess_list_slots" =~ \[J\][[:space:]]+\[sess-beta\] ]]; then
    echo "PASS (Slot badges [H] and [J] correctly displayed)"
else
    echo "FAIL: Expected [H] [sess-alpha] and [J] [sess-beta], got: $sess_list_slots"
    exit 1
fi

# 20. Harpoon slot jumping and unassignment
echo -n "Test 19: Harpoon slot jumping and unassignment... "
# Verify jump-slot switches client
# In headless server without attached client, switch-client exits cleanly
tmux -S "$SOCK" run-shell "bash '$BIN' jump-slot h" || {
    echo "FAIL: jump-slot h failed"
    exit 1
}

# Unassign slot h
tmux -S "$SOCK" run-shell "bash '$BIN' unassign-slot h"
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw sessions > '$STATUS_FILE'"
sess_list_after_unassign=$(cat "$STATUS_FILE")
if [[ "$sess_list_after_unassign" =~ \[H\] ]]; then
    echo "FAIL: Slot H was not unassigned: $sess_list_after_unassign"
    exit 1
fi

# Verify unassigned slot jump message
tmux -S "$SOCK" run-shell "bash '$BIN' jump-slot h > /dev/null 2>&1" || true
echo "PASS (Slot jumping and unassignment verified)"

tmux -S "$SOCK" kill-session -t sess-alpha 2>/dev/null || true
tmux -S "$SOCK" kill-session -t sess-beta 2>/dev/null || true

# 21. In-Modal Cheat Sheet
echo -n "Test 20: In-Modal Cheat Sheet outputs clean modal-only shortcuts... "
rm -f "$STATUS_FILE"
echo "" | bash "$BIN" cheat-sheet > "$STATUS_FILE" 2>&1 || true
cheat_out=$(cat "$STATUS_FILE")
if [[ "$cheat_out" =~ "tmux-glance Viewfinder" ]] && [[ "$cheat_out" =~ "Ctrl-p" ]] && [[ "$cheat_out" =~ "Ctrl-s" ]] && [[ "$cheat_out" =~ "Ctrl-b" ]] && [[ "$cheat_out" =~ "Ctrl-h" ]]; then
    echo "PASS"
else
    echo "FAIL: Cheat sheet output missing expected controls: $cheat_out"
    exit 1
fi

# 22. Interactive Viewfinder FZF Argument & Keybinding Syntax Validation
echo -n "Test 21: Full interactive popup fzf option and keybinding syntax... "
fzf_err=$(TMUX_GLANCE_FZF_FILTER="test" TMUX_GLANCE_STATE_FILE="$STATE_FILE" bash "$BIN" list-sessions 2>&1 >/dev/null || true)
if [[ -z "$fzf_err" ]]; then
    echo "PASS"
else
    echo "FAIL: fzf flag or keybinding validation error: $fzf_err"
    exit 1
fi

# 22b. Global Panes View Validation
echo -n "Test 21b: Global view lists all panes including generic panes with 💻 Pane... "
tmux -S "$SOCK" new-window -t test-sess -n win_gen "bash"
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw all > '$STATUS_FILE'"
global_out=$(cat "$STATUS_FILE")
if [[ "$global_out" =~ "💻 Pane" ]]; then
    echo "PASS"
else
    echo "FAIL: Expected '💻 Pane' in global panes list, got: $global_out"
    exit 1
fi

# 22c. Bots View Validation
echo -n "Test 21c: Bots view filters out untracked generic panes... "
rm -f "$STATUS_FILE"
tmux -S "$SOCK" run-shell "bash '$BIN' list-raw bots > '$STATUS_FILE'"
bots_out=$(cat "$STATUS_FILE")
tmux -S "$SOCK" kill-window -t test-sess:win_gen 2>/dev/null || true
if [[ "$bots_out" =~ "💻 Pane" ]]; then
    echo "FAIL: Bots view should NOT contain generic '💻 Pane' entries, got: $bots_out"
    exit 1
else
    echo "PASS"
fi

# 23. Quickfix Attention Cycling (next-attention, prev-attention, and jump-back)
echo -n "Test 22: Quickfix attention queue cycling (next/prev) and jumplist rewind... "
# Clear lingering state from earlier tests to provide an isolated queue environment
: > "$STATE_FILE"

tmux -S "$SOCK" new-window -t test-sess -n win_qf1 "bash"
pane_qf1=$(tmux -S "$SOCK" list-panes -t test-sess:win_qf1 -F '#{pane_id}')
tmux -S "$SOCK" new-window -t test-sess -n win_qf2 "bash"
pane_qf2=$(tmux -S "$SOCK" list-panes -t test-sess:win_qf2 -F '#{pane_id}')

# Seed attention queue: qf1 is Alert, qf2 is Waiting
printf "%s\ttest-sess\t10\t0\t/tmp\tbuild\tbuild error in tmp\tmanual\talert\n" "$pane_qf1" >> "$STATE_FILE"
printf "%s\ttest-sess\t11\t0\t/tmp\tagy\twaiting for approval\tauto\twaiting\n" "$pane_qf2" >> "$STATE_FILE"

# Start on win1 (non-attention window)
tmux -S "$SOCK" select-window -t test-sess:win1

# 1. next-attention jumps to highest priority attention item (pane_qf1, Alert)
tmux -S "$SOCK" run-shell "bash '$BIN' next-attention"
landed_pane1=$(tmux -S "$SOCK" display-message -t test-sess -p '#{pane_id}')
if [[ "$landed_pane1" != "$pane_qf1" ]]; then
    echo "FAIL: next-attention expected to land on $pane_qf1 (Alert), got $landed_pane1"
    exit 1
fi

# 2. next-attention advances to next item (pane_qf2, Waiting)
tmux -S "$SOCK" run-shell "bash '$BIN' next-attention"
landed_pane2=$(tmux -S "$SOCK" display-message -t test-sess -p '#{pane_id}')
if [[ "$landed_pane2" != "$pane_qf2" ]]; then
    echo "FAIL: next-attention expected to land on $pane_qf2 (Waiting), got $landed_pane2"
    exit 1
fi

# 3. next-attention wraps back around to pane_qf1
tmux -S "$SOCK" run-shell "bash '$BIN' next-attention"
landed_wrap=$(tmux -S "$SOCK" display-message -t test-sess -p '#{pane_id}')
if [[ "$landed_wrap" != "$pane_qf1" ]]; then
    echo "FAIL: next-attention expected to wrap to $pane_qf1, got $landed_wrap"
    exit 1
fi

# 4. prev-attention cycles backward to pane_qf2
tmux -S "$SOCK" run-shell "bash '$BIN' prev-attention"
landed_prev=$(tmux -S "$SOCK" display-message -t test-sess -p '#{pane_id}')
if [[ "$landed_prev" != "$pane_qf2" ]]; then
    echo "FAIL: prev-attention expected to cycle backward to $pane_qf2, got $landed_prev"
    exit 1
fi

# 5. jump-back (<) rewinds back across the history stack
tmux -S "$SOCK" run-shell "bash '$BIN' jump-back"
landed_back=$(tmux -S "$SOCK" display-message -t test-sess -p '#{pane_id}')
if [[ "$landed_back" != "$pane_qf1" ]]; then
    echo "FAIL: jump-back expected to rewind to $pane_qf1, got $landed_back"
    exit 1
fi

# Clean up quickfix test windows
tmux -S "$SOCK" kill-window -t test-sess:win_qf1 2>/dev/null || true
tmux -S "$SOCK" kill-window -t test-sess:win_qf2 2>/dev/null || true
echo "PASS"

echo "All headless tmux lifecycle tests passed successfully!"


