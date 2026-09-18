#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$SCRIPT_DIR/../sentinels/antigravity.sh"

echo "=== [TEST] Antigravity Thinking Spinner & Buffer Normalization ==="

# Helper function that runs the exact normalizer pipeline on stdin text
normalize_buffer() {
    grep -vE '^[[:space:]]*[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏⣾⣽⣻⢿⡿⣟⣯⣷][[:space:]]' \
        | sed -E 's/Thinking\.\.\..*//' \
        | (md5 -q 2>/dev/null || md5sum 2>/dev/null | cut -d' ' -f1 || cksum | cut -d' ' -f1)
}

# Buffer Frame 1: Spinner is ⠋, thinking about architecture
buf1=$(cat << 'EOF'
Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⠋ Thinking... inspecting repository dependencies
? for shortcuts
EOF
)

# Buffer Frame 2: Spinner rotated to ⣾, thinking updated in-place
buf2=$(cat << 'EOF'
Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⣾ Thinking... synthesizing response from neural model
? for shortcuts
EOF
)

# Buffer Frame 3: Spinner rotated to ⣽, different thought stream
buf3=$(cat << 'EOF'
Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
⣽ Thinking... checking git status
? for shortcuts
EOF
)

# Buffer Frame 4: Actual output! The agent finished thinking and printed real tool output
buf4=$(cat << 'EOF'
Welcome to Antigravity CLI
Running tasks in /Users/mhutchinson/project
Requesting permission for run_command: git status
? for shortcuts
EOF
)

echo -n "Test 1: Frame 1 and Frame 2 produce identical fingerprints despite spinner rotation... "
hash1=$(echo "$buf1" | normalize_buffer)
hash2=$(echo "$buf2" | normalize_buffer)
if [[ "$hash1" == "$hash2" ]]; then
    echo "PASS (Hash: $hash1)"
else
    echo "FAIL: hash1 ($hash1) != hash2 ($hash2)"
    exit 1
fi

echo -n "Test 2: Frame 2 and Frame 3 produce identical fingerprints despite thought change... "
hash3=$(echo "$buf3" | normalize_buffer)
if [[ "$hash2" == "$hash3" ]]; then
    echo "PASS (Hash: $hash2)"
else
    echo "FAIL: hash2 ($hash2) != hash3 ($hash3)"
    exit 1
fi

echo -n "Test 3: Frame 4 (actual output change) produces a different fingerprint... "
hash4=$(echo "$buf4" | normalize_buffer)
if [[ "$hash1" != "$hash4" ]]; then
    echo "PASS (New Hash: $hash4)"
else
    echo "FAIL: hash1 ($hash1) == hash4 ($hash4)"
    exit 1
fi

echo -n "Test 4: Classification of waiting permission prompt... "
mock_pane_capture() {
    echo "$buf4"
}
# Test classifier logic directly
tail_text=$(echo "$buf4" | tail -n 4)
if [[ "$tail_text" =~ (Requesting[[:space:]]permission[[:space:]]for|Run[[:space:]]this[[:space:]]command\?|Navigate[[:space:]]·[[:space:]]tab[[:space:]]Amend|1\.[[:space:]]Yes,[[:space:]]run[[:space:]]command|\(y/n\)) ]]; then
    echo "PASS (Classified as waiting)"
else
    echo "FAIL: Failed to classify permission prompt"
    exit 1
fi

echo "All normalization tests passed successfully!"
