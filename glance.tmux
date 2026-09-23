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

# Bind All Panes (Go-To Teleport) view (prefix + g)
tmux bind-key "$glance_key" display-popup -E -w "$popup_width" -h "$popup_height" "$GLANCE_BIN list-all #{pane_id}"

# Bind Attention Hub (prefix + b)
tmux bind-key "$hub_key" display-popup -E -w "$popup_width" -h "$popup_height" "$GLANCE_BIN list auto #{pane_id}"

# Bind Vigil toggle (prefix + v)
tmux bind-key "$vigil_key" run-shell "$GLANCE_BIN toggle-vigil"

# Hook pane focus changes to auto-acknowledge completed tasks and alerts
tmux set-hook -g pane-focus-in "run-shell '$GLANCE_BIN on-focus #{pane_id}'"

# Tier 2 Opt-in: Harpoon Session Slots (prefix + C-h, C-j, C-k, C-l)
enable_harpoon="$(tmux show-option -gqv @glance_enable_harpoon)"
if [[ "$enable_harpoon" =~ ^(on|yes|true|1)$ ]]; then
    tmux bind-key -r C-h run-shell "$GLANCE_BIN jump-slot h #{pane_id}"
    tmux bind-key -r C-j run-shell "$GLANCE_BIN jump-slot j #{pane_id}"
    tmux bind-key -r C-k run-shell "$GLANCE_BIN jump-slot k #{pane_id}"
    tmux bind-key -r C-l run-shell "$GLANCE_BIN jump-slot l #{pane_id}"
fi

# Tier 2 Opt-in: Jumplist Backtracking & History (prefix + Tab, C-z, C-y, -r u, -r U)
enable_jumplist="$(tmux show-option -gqv @glance_enable_jumplist)"
if [[ "$enable_jumplist" =~ ^(on|yes|true|1)$ ]]; then
    history_key="$(tmux show-option -gqv @glance_history_key)"
    history_key="${history_key:-Tab}"
    if [[ -n "$history_key" && "$history_key" != "off" && "$history_key" != "none" ]]; then
        tmux bind-key "$history_key" display-popup -E -w "$popup_width" -h "$popup_height" "$GLANCE_BIN list-history #{pane_id}"
    fi

    tmux bind-key C-z run-shell "$GLANCE_BIN jump-back #{pane_id}"
    tmux bind-key C-y run-shell "$GLANCE_BIN jump-forward #{pane_id}"
    tmux bind-key -r u run-shell "$GLANCE_BIN jump-back #{pane_id}"
    tmux bind-key -r U run-shell "$GLANCE_BIN jump-forward #{pane_id}"
fi
