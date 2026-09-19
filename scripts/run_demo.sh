#!/usr/bin/env bash
set -euo pipefail

export TERM=xterm-256color
export COLORTERM=truecolor

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEMO_DIR="${TMPDIR:-/tmp}/tmux-glance-demo-$$"
GLANCE_BIN="$REPO_DIR/bin/tmux-glance"
STATE_FILE="$DEMO_DIR/glance_state"

mkdir -p "$DEMO_DIR"

# Reset any previous session
tmux -L glance-demo kill-server 2>/dev/null || true
rm -rf "$DEMO_DIR/glance_state"* "$DEMO_DIR"/*.txt 2>/dev/null || true
rm -f ~/.local/state/nvim/swap/*demo* 2>/dev/null || true

# 1. Write Go code
cat << 'GOEOF' > "$DEMO_DIR/main.go"
package main

import "fmt"

// Fib returns the n-th Fibonacci number.
func Fib(n int) int {
	if n <= 1 {
		return n
	}
	return Fib(n-1) + Fib(n-2)
}

func main() {
	for i := 0; i < 10; i++ {
		fmt.Printf("Fib(%d) = %d\n", i, Fib(i))
	}
}
GOEOF

# 2. Write test output
cat << 'TESTEOF' > "$DEMO_DIR/test_out.txt"
=== RUN   TestFib
    fib_test.go:15: Fib(7) = 12; want 13
--- FAIL: TestFib (0.00s)
FAIL
FAIL	fibonacci	0.018s
FAIL
TESTEOF

# 3. Write agent output
cat << 'AGYEOF' > "$DEMO_DIR/agent_out.txt"
I've analyzed fib_test.go and resolved the base case edge condition.
Applying patch to fix base cases...

Requesting permission for: git apply fix_fib.patch
Run this command? (y/n)
> 1. Yes, run command
  2. No, let me review
esc to cancel
AGYEOF

# 4. Start tmux server hermetically (-f /dev/null) with nvim -n (no swap prompt)
tmux -L glance-demo -f /dev/null new-session -d -s glance -n "editor" -x 120 -y 30 "nvim -n -u NONE -c 'syntax on' -c 'set number' -c 'set cursorline' -c 'colorscheme desert' $DEMO_DIR/main.go"

# Set base-index and prefix
tmux -L glance-demo set -g base-index 1
tmux -L glance-demo set -g pane-base-index 1
tmux -L glance-demo move-window -r
tmux -L glance-demo set -g prefix C-b
tmux -L glance-demo bind-key C-b send-prefix

# Configure environment
tmux -L glance-demo set-environment -g TMUX_GLANCE_STATE_FILE "$STATE_FILE"
tmux -L glance-demo set-environment -g PATH "$REPO_DIR/bin:$PATH"
tmux -L glance-demo set-option -g @glance_routes "cat=antigravity,head=antigravity"

# Load glance keybindings and hooks
tmux -L glance-demo run-shell "$REPO_DIR/glance.tmux"

# Set Catppuccin aesthetic
tmux -L glance-demo set -g status 2
tmux -L glance-demo set -g status-style "bg=#1e1e2e,fg=#cdd6f4"
tmux -L glance-demo set -g status-left "#[bg=#89b4fa,fg=#11111b,bold] 👁️ tmux-glance #[default] "
tmux -L glance-demo set -g status-left-length 30
tmux -L glance-demo set -g status-right-length 80
tmux -L glance-demo set -g status-interval 1
tmux -L glance-demo set -g status-right "#($GLANCE_BIN status) #[fg=#6c7086]10:42 "
tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#6c7086] 💡 DEMO: Editing main.go in flow • Background tasks run silently "

# Window 2: Test runner
tmux -L glance-demo new-window -t glance:2 -n "tests" -c "$DEMO_DIR" "bash -c 'cat $DEMO_DIR/test_out.txt; sleep 600'"
test_pane=$(tmux -L glance-demo list-panes -t glance:2 -F '#{pane_id}')

# Window 3: Autonomous Agent
tmux -L glance-demo new-window -t glance:3 -n "agy" -c "$DEMO_DIR" "bash -c 'cat $DEMO_DIR/agent_out.txt; read -r -p \"\" choice; echo \"Approved: applying patch...\"; sleep 600'"
agent_pane=$(tmux -L glance-demo list-panes -t glance:3 -F '#{pane_id}')

# Setup initial state: quiet watch on test pane + running agent
cat << STATEEOF > "$STATE_FILE"
$test_pane	glance	2	1	$DEMO_DIR	go test	watching go test in fibonacci	manual	watching
$agent_pane	glance	3	1	$DEMO_DIR	agy	running in fibonacci	auto	running
STATEEOF

# Return to window 1 (editor)
editor_win=$(tmux -L glance-demo list-windows -F '#{window_id}' | head -n 1)
tmux -L glance-demo select-window -t "$editor_win"

# Timeline daemon
(
    # 0s - 3s: Editing code quietly
    sleep 3

    # 3s: Background tasks trigger alerts!
    cat << ALERT_EOF > "$STATE_FILE"
$test_pane	glance	2	1	$DEMO_DIR	go test	test failed in fibonacci	manual	alert
$agent_pane	glance	3	1	$DEMO_DIR	agy	waiting for confirmation in fibonacci	auto	waiting
ALERT_EOF
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#f38ba8,bold] 💡 DEMO: Alert! Status bar shows test failure 🚨 1 and agent waiting 🤖 ⏳ 1 "
    tmux -L glance-demo refresh-client -S

    # 6s: User presses prefix + b
    sleep 3
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#fab387,bold] 💡 DEMO • ⌨️  KEY: prefix + b  •  Opening Attention Hub (Viewfinder) "
    tmux -L glance-demo refresh-client -S

    # 8.5s: Inside viewfinder browsing test error
    sleep 2.5
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#89b4fa] 🔮 Crystal Ball Peering • Live preview into test error without leaving nvim "
    tmux -L glance-demo refresh-client -S

    # 11.5s: Browsing agent prompt
    sleep 3
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#89b4fa] 🔮 Crystal Ball Peering • Peeking into agent confirmation prompt "
    tmux -L glance-demo refresh-client -S

    # 14.5s: User hits Enter to teleport
    sleep 3
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#a6e3a1,bold] 💡 DEMO • ⌨️  KEY: Enter  •  Instant Teleportation into agent pane "
    tmux -L glance-demo refresh-client -S

    # 17.5s: Agent approved & staying in flow
    sleep 3
    tmux -L glance-demo set -g status-format[1] "#[align=centre,bg=#181825,fg=#a6e3a1] ✨ Focus entered pane • Alert auto-cleared • Stay in flow "
    tmux -L glance-demo refresh-client -S
) &

# Attach to tmux
exec tmux -L glance-demo attach-session -t glance
