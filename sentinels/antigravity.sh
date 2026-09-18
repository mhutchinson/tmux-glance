#!/usr/bin/env bash
# Antigravity Sentinel for tmux-glance
# Monitors Google DeepMind Antigravity sessions, classifies interactive prompts,
# and normalizes in-place thinking spinners to prevent false unread alarms.

# Default command triggers registered in the routing table
sentinel_antigravity_default_commands=("agy" "antigravity")

sentinel_antigravity_matches() {
    local cmd="$1"
    local pattern="${TMUX_GLANCE_ANTIGRAVITY_PATTERN:-^(agy|antigravity)$}"
    [[ "$cmd" =~ $pattern ]]
}

sentinel_antigravity_classify() {
    local pane_id="$1"
    local path="$2"
    local cmd="$3"

    local tail_text
    tail_text=$(tmux capture-pane -p -t "$pane_id" 2>/dev/null | tail -n 4)

    # Classify state based on strict footer cues
    if [[ "$tail_text" =~ (Requesting[[:space:]]permission[[:space:]]for|Run[[:space:]]this[[:space:]]command\?|Navigate[[:space:]]·[[:space:]]tab[[:space:]]Amend|1\.[[:space:]]Yes,[[:space:]]run[[:space:]]command|\(y/n\)) ]]; then
        printf "waiting\twaiting for confirmation in %s\n" "$(basename "$path")"
    elif [[ "$tail_text" =~ "esc to cancel" ]]; then
        printf "running\trunning in %s\n" "$(basename "$path")"
    elif [[ "$tail_text" =~ "\? for shortcuts" ]]; then
        printf "idle\tidle in %s\n" "$(basename "$path")"
    else
        printf "unknown\t%s\n" "$(basename "$path")"
    fi
}

# Intelligent Screen Normalizer:
# Filters braille thinking spinners ([⠋⠙⠹...]) and in-place 'Thinking...' text
# before computing hash, ensuring thought stream updates don't trigger unread alarms.
sentinel_antigravity_fingerprint() {
    local pane_id="$1"
    tmux capture-pane -p -t "$pane_id" 2>/dev/null \
        | grep -vE '^[[:space:]]*[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷][[:space:]]' \
        | sed -E 's/Thinking\.\.\..*//' \
        | (md5 -q 2>/dev/null || md5sum 2>/dev/null | cut -d' ' -f1 || cksum | cut -d' ' -f1)
}
