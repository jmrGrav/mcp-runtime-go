#!/usr/bin/env bash
set -euo pipefail

service_name="${1:-mcp-runtime-shadow}"
python_log="${2:-}"
go_log="${3:-}"

if command -v systemctl >/dev/null 2>&1; then
  printf '\n== systemd status ==\n'
  systemctl status "$service_name" --no-pager || true
else
  printf 'systemctl not available\n'
fi

if command -v journalctl >/dev/null 2>&1; then
  printf '\n== recent logs ==\n'
  journalctl -u "$service_name" --no-pager -n 50 || true
fi

count_events() {
  local path="$1"
  if [ -n "$path" ] && [ -f "$path" ]; then
    printf '%s' "$(wc -l < "$path" | tr -d ' ')"
  else
    printf '0'
  fi
}

size_bytes() {
  local path="$1"
  if [ -n "$path" ] && [ -f "$path" ]; then
    stat -c '%s' "$path"
  else
    printf '0'
  fi
}

printf '\n== audit log summary ==\n'
printf 'python log: %s bytes, ~%s events\n' "$(size_bytes "$python_log")" "$(count_events "$python_log")"
printf 'go log:     %s bytes, ~%s events\n' "$(size_bytes "$go_log")" "$(count_events "$go_log")"
