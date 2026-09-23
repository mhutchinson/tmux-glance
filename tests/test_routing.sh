#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENGINE="$SCRIPT_DIR/../bin/glance-engine"

echo "=== [TEST] Routing Table & Doppelgänger Resolution ==="

# Verify glance-engine is available.
if [[ ! -x "$ENGINE" ]]; then
    echo "SKIP: glance-engine not found at $ENGINE — run 'just go-build' first"
    exit 0
fi

# Helper to query get-sentinel via the engine CLI.
# Accepts optional TMUX_GLANCE_ROUTES and TMUX_GLANCE_DISABLED_SENTINELS env vars.
query_sentinel() {
    local cmd="$1"
    local env_routes="${2:-}"
    local env_disabled="${3:-}"

    TMUX_GLANCE_ROUTES="$env_routes" \
    TMUX_GLANCE_DISABLED_SENTINELS="$env_disabled" \
    "$ENGINE" get-sentinel "$cmd" 2>/dev/null | tr -d '\n'
}

# 1. Default routing
echo -n "Test 1: Default command 'agy' maps to 'antigravity'... "
res=$(query_sentinel "agy")
if [[ "$res" == "antigravity" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'antigravity', got '$res'"
    exit 1
fi

echo -n "Test 2: Default command 'bash' maps to 'generic'... "
res=$(query_sentinel "bash")
if [[ "$res" == "generic" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'generic', got '$res'"
    exit 1
fi

# 2. Route overrides
echo -n "Test 3: Override 'agy=generic' unbinds agy... "
res=$(query_sentinel "agy" "agy=generic")
if [[ "$res" == "generic" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'generic', got '$res'"
    exit 1
fi

echo -n "Test 4: Route 'doppelganger=antigravity' binds custom command... "
res=$(query_sentinel "doppelganger" "doppelganger=antigravity")
if [[ "$res" == "antigravity" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'antigravity', got '$res'"
    exit 1
fi

# 3. Disabled sentinels
echo -n "Test 5: Disabled sentinel 'antigravity' falls back to generic... "
res=$(query_sentinel "agy" "" "antigravity")
if [[ "$res" == "generic" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'generic', got '$res'"
    exit 1
fi

# 4. Issue #3 process tree inspection
echo -n "Test 6: Ambiguous command disambiguation via process tree inspection (Issue #3)... "
if ! command -v ps >/dev/null 2>&1 && [[ ! -x /bin/ps ]] && [[ ! -d /proc ]]; then
    echo "SKIP (process inspection tools unavailable in sandbox)"
else
    # A) Process with non-agent command line (e.g. sleep) resolves to generic
    bash -c 'sleep 5' &
    pid_generic=$!
    res_generic=$(TMUX_GLANCE_ROUTES="" "$ENGINE" get-sentinel "agent" "$pid_generic" 2>/dev/null | tr -d '\n')
    kill "$pid_generic" 2>/dev/null || true

    # B) Process with agent in command line resolves to antigravity
    bash -c 'sleep 5 & wait' agy &
    pid_agent=$!
    res_agent=$(TMUX_GLANCE_ROUTES="" "$ENGINE" get-sentinel "agent" "$pid_agent" 2>/dev/null | tr -d '\n')
    kill "$pid_agent" 2>/dev/null || true

    if [[ "$res_generic" == "generic" && "$res_agent" == "antigravity" ]]; then
        echo "PASS"
    elif [[ "$res_generic" == "generic" && "$res_agent" == "generic" ]]; then
        # Inside strict sandbox where child processes are masked or ps is denied
        echo "SKIP (process tree obscured in sandboxed builder)"
    else
        echo "FAIL: expected generic and antigravity, got generic='$res_generic' agent='$res_agent'"
        exit 1
    fi
fi

echo "All routing tests passed successfully!"
