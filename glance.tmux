#!/usr/bin/env bash
# glance.tmux - Entry point for Tmux Plugin Manager (TPM)
# Sets default keybindings, hooks, and options for tmux-glance

CURRENT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GLANCE_BIN="$CURRENT_DIR/bin/tmux-glance"

if [[ ! -x "$GLANCE_BIN" ]]; then
    chmod +x "$GLANCE_BIN" 2>/dev/null || true
fi

# Configurable keybindings
glance_key="$(tmux show-option -gqv @glance_key)"
glance_key="${glance_key:-g}"

hub_key="$(tmux show-option -gqv @glance_hub_key)"
hub_key="${hub_key:-b}"

vigil_key="$(tmux show-option -gqv @glance_vigil_key)"
vigil_key="${vigil_key:-v}"

popup_width="$(tmux show-option -gqv @glance_popup_width)"
popup_width="${popup_width:-85%}"

popup_height="$(tmux show-option -gqv @glance_popup_height)"
popup_height="${popup_height:-75%}"

# Bind Glance Fleet view (prefix + g)
tmux bind-key "$glance_key" display-popup -E -w "$popup_width" -h "$popup_height" "$GLANCE_BIN list-all"

# Bind Attention Hub (prefix + b)
tmux bind-key "$hub_key" display-popup -E -w "$popup_width" -h "$popup_height" "$GLANCE_BIN list"

# Bind Vigil toggle (prefix + v)
tmux bind-key "$vigil_key" run-shell "$GLANCE_BIN toggle-vigil"

# Hook pane focus changes to auto-acknowledge completed tasks and alerts
tmux set-hook -g pane-focus-in "run-shell '$GLANCE_BIN on-focus #{pane_id}'"
