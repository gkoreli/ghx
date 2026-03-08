#!/usr/bin/env bash
# bench/run.sh — Run benchmark cases and append results to RESULTS.md
set -euo pipefail
cd "$(dirname "$0")"

TOKEN_COUNT="./token-count"
RESULTS="RESULTS.md"

measure() {
  local label="$1"; shift
  local tmpout; tmpout=$(mktemp)

  local start; start=$(python3 -c "import time; print(time.time())")
  "$@" > "$tmpout" 2>/dev/null || true
  local end; end=$(python3 -c "import time; print(time.time())")

  local ms; ms=$(python3 -c "print(int(($end - $start) * 1000))")
  local bytes; bytes=$(wc -c < "$tmpout")
  local tokens; tokens=$(python3 "$TOKEN_COUNT" < "$tmpout")
  local lines; lines=$(wc -l < "$tmpout" | tr -d ' ')

  echo "$label|$ms|$bytes|$tokens|$lines"
  rm -f "$tmpout"
}

# Init results file if empty or missing
if [ ! -f "$RESULTS" ] || [ ! -s "$RESULTS" ]; then
  cat > "$RESULTS" << 'EOF'
# Benchmark Results

| Case | Method | Time (ms) | Bytes | Tokens | Lines | API Calls |
|------|--------|-----------|-------|--------|-------|-----------|
EOF
fi

for case_file in cases/*.sh; do
  # Reset variables
  unset NAME DESCRIPTION cmd_a cmd_b cmd_a_label cmd_b_label cmd_a_calls cmd_b_calls
  source "$case_file"

  echo "Running: $NAME"
  echo "  $DESCRIPTION"

  # Run method A
  echo "  → $cmd_a_label..."
  result_a=$(measure "$cmd_a_label" cmd_a)
  IFS='|' read -r a_label a_ms a_bytes a_tokens a_lines <<< "$result_a"

  # Run method B
  echo "  → $cmd_b_label..."
  result_b=$(measure "$cmd_b_label" cmd_b)
  IFS='|' read -r b_label b_ms b_bytes b_tokens b_lines <<< "$result_b"

  # Append to results
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "| $NAME | $cmd_a_label | ${a_ms}ms | $a_bytes | $a_tokens | $a_lines | $cmd_a_calls |" >> "$RESULTS"
  echo "| | $cmd_b_label | ${b_ms}ms | $b_bytes | $b_tokens | $b_lines | $cmd_b_calls |" >> "$RESULTS"

  # Print summary
  echo "  Results:"
  echo "    $cmd_a_label: ${a_ms}ms, $a_tokens tokens, $a_lines lines"
  echo "    $cmd_b_label: ${b_ms}ms, $b_tokens tokens, $b_lines lines"
  echo ""
done

echo "Results written to $RESULTS"
