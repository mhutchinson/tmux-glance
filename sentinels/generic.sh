#!/usr/bin/env bash
# Generic Sentinel for tmux-glance
# Fallback sentinel for standard shells, compilation jobs, tests, and watches.

sentinel_generic_matches() {
    # Fallback matches all commands
    return 0
}

sentinel_generic_classify() {
    local pane_id="$1"
    local path="$2"
    local cmd="$3"
    printf "watching\t%s in %s\n" "$cmd" "$(basename "$path")"
}

sentinel_generic_fingerprint() {
    local pane_id="$1"
    tmux capture-pane -p -t "$pane_id" 2>/dev/null \
        | (md5 -q 2>/dev/null || md5sum 2>/dev/null | cut -d' ' -f1 || cksum | cut -d' ' -f1)
}
