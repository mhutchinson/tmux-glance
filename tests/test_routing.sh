#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$SCRIPT_DIR/../bin/tmux-glance"
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

# 4. Custom sentinel directory precedence
echo -n "Test 6: Custom sentinel directory overrides built-in sentinel... "
tmp_sentinel_dir=$(mktemp -d)
cat << 'EOF' > "$tmp_sentinel_dir/antigravity.sh"
sentinel_antigravity_default_commands=("custom-agent-override")
sentinel_antigravity_matches() {
    [[ "$1" == "custom-agent-override" ]]
}
sentinel_antigravity_classify() {
    printf "waiting\tcustom sentinel override\n"
}
sentinel_antigravity_fingerprint() {
    echo "custom-hash-val"
}
EOF

res_cmd=$(TMUX_GLANCE_SENTINEL_DIR="$tmp_sentinel_dir" "$ENGINE" get-sentinel "custom-agent-override" 2>/dev/null | tr -d '\n')
rm -rf "$tmp_sentinel_dir"

if [[ "$res_cmd" == "antigravity" ]]; then
    echo "PASS"
else
    echo "FAIL: expected 'antigravity', got cmd='$res_cmd'"
    exit 1
fi

echo "All routing tests passed successfully!"
