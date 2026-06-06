# mcp-runtime-go

[![Go](https://img.shields.io/badge/go-1.25-blue)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/jmrGrav/mcp-runtime-go)](https://github.com/jmrGrav/mcp-runtime-go/releases/latest)
[![CI](https://github.com/jmrGrav/mcp-runtime-go/actions/workflows/ci.yml/badge.svg)](https://github.com/jmrGrav/mcp-runtime-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/jmrGrav/mcp-runtime-go)](LICENSE)

Production-grade Go runtime for OAuth 2.0 + PKCE and MCP proxying, currently authoritative in production.

Stable since **v1.3.0**. SQLite WAL token store active. Claude.ai validated end-to-end.

## Overview

`mcp-runtime-go` implements the full OAuth 2.0 Authorization Code + PKCE flow
(RFC 6749, RFC 7591, RFC 8414, RFC 9728) and proxies authenticated requests to a
Hugo MCP backend. It is the Go successor to
[mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy) and has been
authoritative in production since the shadow-mode cutover.

Key capabilities:

- OAuth 2.0 Authorization Code + PKCE (RFC 6749 / RFC 7636 S256)
- Dynamic Client Registration (RFC 7591)
- Authenticated MCP reverse proxy
- SQLite WAL token persistence (CGo-free, `modernc.org/sqlite`)
- Structured JSON audit logging with request-ID correlation
- `/readyz` and `/healthz` probes
- Prometheus-text metrics endpoint (loopback only)
- Hardened systemd service unit

## Architecture

```
Claude.ai
  ↓
Cloudflare → OpenResty (CrowdSec)
  ↓
mcp-runtime-go  127.0.0.1:8086
  ├── OAuth 2.0 + PKCE
  ├── Dynamic Client Registration
  ├── SQLite WAL token store
  ├── JSON audit log
  ├── Prometheus metrics  (/metrics — loopback only)
  └── readiness / health probes
  ↓
Hugo MCP backend (internal TLS)
```

### Repository layout

```
cmd/
  mcp-runtime/       Main entry point — HTTP server + migrate-storage subcommand
  shadow-compare/    Historical audit-log parity tool (retired from CI)

internal/
  config/            Environment-driven configuration (HUGO_* canonical, GRAV_* legacy)
  context/           Per-request ID propagation
  httpserver/        HTTP server lifecycle (graceful shutdown, WaitReady)
  oauthproxy/        OAuth 2.0 domain: handlers, service, proxy, models
  observability/     Structured JSON audit logger + Prometheus counters
  runtime/           App wiring, request-ID middleware
  security/          PKCE S256, redirect URI validation, secure random
  storage/           SQLite WAL (production) + JSON (legacy / migration compatibility)

deploy/
  env/               Environment file template
  logrotate/         Audit log rotation example
  nginx/             OpenResty config example (historical mirror config archived)
  systemd/           Production-hardened systemd service unit
```

See [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) for full detail.

## Security Model

- Mandatory PKCE S256 (`MANDATORY_PKCE=true` default)
- Redirect URI allowlist — only registered URIs accepted
- Tokens stored as SHA-256 hashes — plain tokens never written to disk
- Constant-time credential comparison throughout
- Fail-closed token persistence — token rejected if store write fails
- SQLite WAL persistence with WAL checkpoint on migration
- TLS backend verification via `MCP_CA_CERT`
- Hardened systemd unit (`NoNewPrivileges`, `MemoryDenyWriteExecute`, syscall filter, `ProtectSystem=strict`)
- CrowdSec / OpenResty edge protection (LAPI + heuristic scorer)
- Structured audit log with UUID request-ID correlation
- `/readyz` verifies config, token store, and audit log before accepting traffic
- CIDR-gated `/authorize` endpoint

## Building

```bash
go build -o bin/mcp-runtime ./cmd/mcp-runtime
```

## Testing

```bash
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
./scripts/test-all.sh
```

Coverage gate: ≥ 60% (enforced in CI).

## Configuration

All configuration is via environment variables.
Copy `deploy/env/mcp-runtime-shadow.env.example` and fill in real values.

| Variable | Default | Description |
|---|---|---|
| `LISTEN_HOST` | `127.0.0.1` | Bind address |
| `LISTEN_PORT` | `8086` | Bind port |
| `HUGO_MCP_URL` | — | Hugo MCP backend URL (required) |
| `HUGO_HOST` | — | Backend hostname for `Host` header |
| `HUGO_TOKEN` | — | Bearer token for backend auth (required) |
| `PROXY_BASE_URL` | — | Public base URL for OAuth metadata |
| `CLIENT_ID` | — | OAuth client ID |
| `CLIENT_SECRET` | — | OAuth client secret |
| `MCP_CA_CERT` | — | Path to CA cert for backend TLS |
| `USE_SQLITE` | `true` | Use SQLite WAL token store (recommended) |
| `TOKENS_DB` | `…/tokens.db` | SQLite database path |
| `TOKENS_FILE` | `…/tokens.json` | JSON store path (legacy / rollback only) |
| `AUDIT_LOG_FILE` | — | Path to audit JSONL log |
| `TRUSTED_PROXIES` | — | Comma-separated trusted proxy CIDRs |
| `MANDATORY_PKCE` | `true` | Require PKCE on all token exchanges |
| `ALLOW_TOKEN_STORE_RECOVERY` | `false` | Allow startup with corrupt store |
| `LOG_LEVEL` | `info` | Log level |

**Naming:** `HUGO_*` is the canonical env prefix. `GRAV_*` names (`GRAV_MCP_URL`,
`GRAV_TOKEN`, `GRAV_HOST`) are accepted as legacy fallback with a `[WARN]` log and
will be removed in a future cleanup release.

**Storage:** `USE_SQLITE=true` (default) activates SQLite WAL. JSON store is retained
for emergency rollback only and logs a `[WARN]` at startup if active.

## Token Store Migration

To migrate an existing JSON token store to SQLite:

```bash
# Source env file and run migration (service must be stopped first)
sudo systemctl stop mcp-runtime.service
sudo -u mcp-runtime bash -c \
  'set -a; source /etc/mcp-runtime-go/mcp-runtime.env; \
   TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db; \
   /usr/local/bin/mcp-runtime migrate-storage'
sudo systemctl start mcp-runtime.service
```

The migration renames `tokens.json` → `tokens.json.migrated` on success.
See [docs/operations/SQLITE_MIGRATION_RUNBOOK.md](docs/operations/SQLITE_MIGRATION_RUNBOOK.md)
for the full procedure and rollback steps.

## Current Status

| Component | Status |
|---|---|
| OAuth Authorization Code + PKCE | Production stable |
| Dynamic Client Registration | Production stable |
| Claude.ai integration | Production validated |
| Go authoritative runtime | Complete |
| SQLite WAL token store | Production active |
| Prometheus metrics | Active (loopback) |
| Readiness / health probes | Active |
| Python proxy replacement | Complete |
| Shadow deployment | Historical / retired |

## Production Validation — v1.3.0

Validated on 2026-06-06 after SQLite migration:

| Check | Result |
|---|---|
| `systemctl status mcp-runtime` | active (running) |
| `/readyz` | 200 OK |
| `mcp_token_persistence_failures_total` | 0 |
| `mcp_audit_write_failures_total` | 0 |
| `mcp_proxy_errors_total` | 0 |
| `mcp_tokens_rejected_total` | 0 |
| Real Claude.ai `proxy_hit` post-migration | 200 ✅ |
| Active tokens preserved through migration | 1 / 1 |

Full report: [docs/POST_DEPLOYMENT_VALIDATION.md](docs/POST_DEPLOYMENT_VALIDATION.md)
and [docs/V1_3_SQLITE_MIGRATION_EXECUTION_REPORT.md](docs/V1_3_SQLITE_MIGRATION_EXECUTION_REPORT.md).

## Roadmap

- **v1.3.x** — production observation, operational fixes, logrotate packaging
- **v1.4** — remove `GRAV_*` legacy fallback, clean up shadow-compare dead code
- **v1.4** — logrotate deployment packaging (`deploy/logrotate/`)
- **future** — Debian package / install script
- **future** — multi-backend MCP support

## Migration History

`mcp-runtime-go` started as a shadow deployment running alongside the Python proxy
([mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy)). OpenResty mirrored
production traffic to both services. After a 24h+ observation window, `shadow-compare`
verified audit log parity before cutover was allowed.

Since **v1.3.0**, Go is fully authoritative. Python is preserved on disk as a rollback
reference but is stopped and disabled. `shadow-compare` is a historical diagnostic tool,
no longer part of the main operational flow.

See [docs/deployment/SHADOW_MODE.md](docs/deployment/SHADOW_MODE.md) for the original
shadow strategy and [docs/PHASE4_CUTOVER_REPORT.md](docs/PHASE4_CUTOVER_REPORT.md) for
the cutover execution record.

## Relationship with mcp-oauth-proxy

`mcp-runtime-go` is the production Go successor to
[mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy).
The Python implementation is no longer authoritative and remains only as a rollback
reference. See [docs/migration/MIGRATION_PLAN.md](docs/migration/MIGRATION_PLAN.md).

## Documentation

Full documentation is in [`docs/`](docs/) — start with [`docs/INDEX.md`](docs/INDEX.md).

## License

MIT
