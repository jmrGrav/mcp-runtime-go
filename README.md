# mcp-runtime-go

[![Go](https://img.shields.io/badge/go-1.25-blue)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/jmrGrav/mcp-runtime-go)](https://github.com/jmrGrav/mcp-runtime-go/releases/latest)
[![CI](https://github.com/jmrGrav/mcp-runtime-go/actions/workflows/ci.yml/badge.svg)](https://github.com/jmrGrav/mcp-runtime-go/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/jmrGrav/mcp-runtime-go)](LICENSE)

Production Go runtime for OAuth 2.0 + PKCE and MCP proxying. The Go service is authoritative in production.

## Quick Start

```bash
go build -o bin/mcp-runtime ./cmd/mcp-runtime
go test ./...
systemctl status mcp-runtime --no-pager
```

## What It Does

- OAuth 2.0 Authorization Code + PKCE
- Dynamic Client Registration
- Authenticated MCP reverse proxy
- SQLite WAL token storage
- Structured audit logging
- `/healthz`, `/readyz`, and loopback metrics

## Architecture

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the package map, OAuth flow, storage model,
security model, and historical context.

## Security

Highlights:

- mandatory PKCE
- redirect URI validation
- hashed token storage
- trusted-proxy enforcement
- fail-closed persistence and audit paths
- backend TLS validation

## Configuration

The canonical configuration prefix is `HUGO_*`.

Common settings:

- `LISTEN_HOST=127.0.0.1`
- `LISTEN_PORT=8086`
- `HUGO_MCP_URL`
- `HUGO_HOST`
- `HUGO_TOKEN`
- `PROXY_BASE_URL`
- `CLIENT_ID`
- `CLIENT_SECRET`
- `USE_SQLITE=true`
- `TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db`
- `AUDIT_LOG_FILE=/var/log/mcp-runtime-go/audit.jsonl`

Legacy `GRAV_*` variables remain compatibility fallbacks only.

## Operations

See [docs/OPERATIONS.md](docs/OPERATIONS.md) for installation, systemd, OpenResty / CrowdSec
notes, health checks, metrics, logs, logrotate, backup, rollback, and SQLite migration.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md) for current status, remaining debt, and next steps.

## Status

- Production stable since `v1.3.0`
- SQLite WAL active in production
- Claude.ai validated end-to-end
- Shadow mode retired

## Documentation History

Historical reports, audits, migration notes, and shadow-era material live under `docs/archive/`.

## License

MIT
