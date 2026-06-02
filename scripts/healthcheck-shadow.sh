#!/usr/bin/env bash
set -euo pipefail

base_url="${1:-http://127.0.0.1:8084}"

fail() {
  printf '%s\n' "$1" >&2
  exit 1
}

check() {
  local path="$1"
  local label="$2"
  if ! curl -fsS --max-time 5 "${base_url}${path}" >/dev/null; then
    fail "FAIL: ${label} (${base_url}${path})"
  fi
  printf 'OK: %s\n' "${label}"
}

check "/healthz" "healthz"
check "/readyz" "readyz"
check "/.well-known/oauth-authorization-server" "oauth metadata"
check "/.well-known/oauth-protected-resource" "protected resource metadata"

printf 'Shadow healthcheck passed for %s\n' "$base_url"
