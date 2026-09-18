#!/usr/bin/env bash
set -euo pipefail

export LC_ALL="${LC_ALL:-C.UTF-8}"
export LANG="${LANG:-C.UTF-8}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=========================================="
echo " Running tmux-glance Automated Test Suite"
echo "=========================================="
echo ""

bash "$SCRIPT_DIR/test_routing.sh"
echo ""
bash "$SCRIPT_DIR/test_normalizer.sh"
echo ""
bash "$SCRIPT_DIR/test_tmux_lifecycle.sh"
echo ""

echo "=========================================="
echo " ✅ ALL UNIT & INTEGRATION TESTS PASSED!"
echo "=========================================="
