#!/usr/bin/env bash
# Antigravity Sentinel for tmux-glance
# Monitors Google DeepMind Antigravity sessions, classifies interactive prompts,
# and normalizes in-place thinking spinners to prevent false unread alarms.

# Default command triggers registered in the routing table
# shellcheck disable=SC2034
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

    local non_empty last_line tail_text
    non_empty=$(tmux capture-pane -p -t "$pane_id" 2>/dev/null | grep -v '^[[:space:]]*$')
    last_line=$(echo "$non_empty" | tail -n 1)
    # Sample bottom lines to evaluate active prompt frame
    tail_text=$(echo "$non_empty" | tail -n 15)

    # 1. Blocked / Waiting cues:
    # A) Subagent explicitly reported Blocked in lifecycle widget: e.g. "● Agent(...)  Blocked · ..."
    # B) Approval card header: e.g. "┃ self needs approval for Bash"
    # C) Interactive permission prompt: "Requesting permission for" or "Run this command?" (not followed by Command finished)
    if echo "$tail_text" | grep -qE '^[[:space:]]*●[[:space:]]+Agent\(.*Blocked' || \
       echo "$tail_text" | grep -qE '^[[:space:]]*[┃|][[:space:]]*.*needs[[:space:]]+approval[[:space:]]+for' || \
       (echo "$tail_text" | grep -qE '(Requesting[[:space:]]permission[[:space:]]for|Run[[:space:]]this[[:space:]]command\?)' && ! echo "$tail_text" | grep -qE 'Command[[:space:]]+finished'); then
        printf "waiting\twaiting for confirmation in %s\n" "$(basename "$path")"
    elif [[ "$last_line" =~ esc[[:space:]]to[[:space:]]cancel ]] || echo "$tail_text" | grep -qE '^[[:space:]]*●[[:space:]]+Agent\(.*Running'; then
        printf "running\trunning in %s\n" "$(basename "$path")"
    elif [[ "$last_line" =~ \?[[:space:]]for[[:space:]]shortcuts ]] || [[ "$last_line" =~ ^\>[[:space:]]*$ ]]; then
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
