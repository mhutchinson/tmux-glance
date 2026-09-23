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
tail_text=$(echo "$buf4" | grep -v '^[[:space:]]*$' | tail -n 10)
if [[ "$tail_text" =~ (Requesting[[:space:]]permission[[:space:]]for|Run[[:space:]]this[[:space:]]command\?|Navigate[[:space:]]·|1\.[[:space:]]Yes,[[:space:]]run[[:space:]]command|\(y/n\)) ]]; then
    echo "PASS (Classified as waiting)"
else
    echo "FAIL: Failed to classify permission prompt"
    exit 1
fi

echo -n "Test 5: Classification of idle prompt (? for shortcuts)... "
buf_idle=$(cat << 'EOF'
Welcome to Antigravity CLI
>
─────────────────────────────────────────────
? for shortcuts                                accept-edits · Gemini 3.8 Flash · medium
EOF
)
tail_idle=$(echo "$buf_idle" | grep -v '^[[:space:]]*$' | tail -n 10)
if [[ "$tail_idle" =~ \?[[:space:]]for[[:space:]]shortcuts ]]; then
    echo "PASS (Classified as idle)"
else
    echo "FAIL: Failed to classify idle prompt"
    exit 1
fi

echo -n "Test 6: Classification with trailing blank lines... "
buf_trailing=$(cat << 'EOF'
Welcome to Antigravity CLI
? for shortcuts                                accept-edits · Gemini 3.8 Flash · medium




EOF
)
tail_trailing=$(echo "$buf_trailing" | grep -v '^[[:space:]]*$' | tail -n 10)
if [[ "$tail_trailing" =~ \?[[:space:]]for[[:space:]]shortcuts ]]; then
    echo "PASS (Classified as idle despite trailing blank lines)"
else
    echo "FAIL: Failed to classify with trailing blank lines"
    exit 1
fi

echo -n "Test 7: Classification of running execution (esc to cancel)... "
buf_running=$(cat << 'EOF'
Executing step 3/5...
esc to cancel                                  Gemini 3.8 Flash · medium
EOF
)
tail_running=$(echo "$buf_running" | grep -v '^[[:space:]]*$' | tail -n 10)
if [[ "$tail_running" =~ esc[[:space:]]to[[:space:]]cancel ]]; then
    echo "PASS (Classified as running)"
else
    echo "FAIL: Failed to classify running state"
    exit 1
fi

echo -n "Test 8: Classification of subagent approval / blocked agent prompt... "
buf_blocked=$(cat << 'EOF'
 ┃ self needs approval for Bash
 ┃ ─────────────────────────────────────────────────────────────────────────────────────
 ┃
 ┃ ● Bash(top -l 1 -n 5 -o cpu)
 ┃
 ┃ ctrl+y approve · alt+j manage
───────────────────────────────────────────────────────────────────────────────────────────
>
───────────────────────────────────────────────────────────────────────────────────────────
  ● Agent(self)  Blocked · Running command · 6s
───────────────────────────────────────────────────────────────────────────────────────────
? for shortcuts                       accept-edits · Gemini 3.8 Flash · low · 1 subagent(s)
EOF
)
tail_blocked=$(echo "$buf_blocked" | grep -v '^[[:space:]]*$' | tail -n 15)
if echo "$tail_blocked" | grep -qE '^[[:space:]]*●[[:space:]]+Agent\(.*Blocked' && \
   echo "$tail_blocked" | grep -qE 'needs[[:space:]]+approval[[:space:]]+for'; then
    echo "PASS (Classified as waiting/blocked via semantic cues)"
else
    echo "FAIL: Failed to classify subagent approval / blocked agent prompt"
    exit 1
fi

echo "All normalization tests passed successfully!"
