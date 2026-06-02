#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 3 ]; then
  printf 'Usage: %s <python_audit_log> <go_audit_log> <report_dir> [window_hours]\n' "$0" >&2
  printf '  window_hours: optional note for the report filename (default: 48)\n' >&2
  exit 1
fi

python_log="$1"
go_log="$2"
report_dir="$3"
window_hours="${4:-48}"
timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
report_file="${report_dir%/}/shadow-compare-${window_hours}h-${timestamp}.txt"

mkdir -p "$report_dir"

# Mirror traffic always has independent request IDs (Python and Go each generate their own).
# --allow-unsafe-fallback enables time+event+source-IP matching (2s window) which is the
# only viable correlation strategy for mirror-mode shadow comparison.
if [ -x "./bin/shadow-compare" ]; then
  compare_bin="./bin/shadow-compare"
elif [ -x "/home/jm/mcp-runtime-go/bin/shadow-compare" ]; then
  compare_bin="/home/jm/mcp-runtime-go/bin/shadow-compare"
else
  compare_bin="go run ./cmd/shadow-compare"
fi

{
  printf 'Shadow comparison run: %s\n' "$timestamp"
  printf 'Window: %sh\n' "$window_hours"
  printf 'Python log: %s\n' "$python_log"
  printf 'Go log: %s\n' "$go_log"
  printf 'Mode: --allow-unsafe-fallback (time+event+ip, 2s window)\n'
  printf '\n'
  $compare_bin \
    --allow-unsafe-fallback \
    -python "$python_log" \
    -go "$go_log"
} | tee "$report_file"

printf 'Report written to %s\n' "$report_file"
