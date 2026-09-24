#!/usr/bin/env bash
set -euo pipefail

export LC_ALL="${LC_ALL:-C.UTF-8}"
export LANG="${LANG:-C.UTF-8}"

# Ensure test runners are strictly hermetic and never inherit or interact with
# an active outer tmux session or pane if run from inside tmux.
unset TMUX TMUX_PANE

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=========================================="
echo " Running tmux-glance Automated Test Suite"
echo "=========================================="
echo ""

bash "$SCRIPT_DIR/test_routing.sh"
echo ""
bash "$SCRIPT_DIR/test_normalizer.sh"
echo ""
bash "$SCRIPT_DIR/test_jumplist.sh"
echo ""
bash "$SCRIPT_DIR/test_tmux_lifecycle.sh"
echo ""
bash "$SCRIPT_DIR/test_perf_scan.sh"
echo ""

echo "=========================================="
echo " ✅ ALL UNIT & INTEGRATION TESTS PASSED!"
echo "=========================================="
