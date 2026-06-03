# mcp-runtime-go

Production-grade Go runtime for OAuth 2.0 proxying and MCP integration. Designed as a zero-downtime replacement for [mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy) using a shadow deployment strategy.

## Overview

`mcp-runtime-go` implements the full OAuth 2.0 Authorization Code + PKCE flow (RFC 6749, RFC 7591, RFC 8414, RFC 9728) and proxies authenticated requests to a backend MCP server. It is built to run alongside the existing Python proxy in shadow mode — receiving mirrored traffic and producing comparable audit logs — before becoming authoritative.

## Architecture

```
cmd/
  mcp-runtime/       HTTP server entry point
  shadow-compare/    Audit log parity comparison tool

internal/
  config/            Environment-driven configuration
  context/           Per-request ID propagation
  httpserver/        HTTP server lifecycle (graceful shutdown)
  oauthproxy/        OAuth 2.0 domain: handlers, service, proxy, models
  observability/     Structured JSON audit logger (JSON lines)
  runtime/           App wiring, request-ID middleware
  security/          PKCE S256, redirect URI validation, secure random
  storage/           Token store (JSON, compatible with Python format)

deploy/
  env/               Environment file template
  nginx/             OpenResty mirror config example
  systemd/           Hardened systemd service unit
```

See [docs/architecture/ARCHITECTURE.md](docs/architecture/ARCHITECTURE.md) for full detail.

## Shadow Deployment

Go runs in parallel with Python. OpenResty mirrors production traffic to both. After a 24h+ observation window, `shadow-compare` verifies parity before cutover is allowed.

```
[OpenResty] ──→ Python :8084  (authoritative)  ──→ audit-hugo.log
            └─→ Go     :8085  (shadow)          ──→ audit-shadow.jsonl
                                                          │
                                                   shadow-compare
                                                          │
                                                   PASS / BLOCKED
```

See [docs/deployment/SHADOW_MODE.md](docs/deployment/SHADOW_MODE.md) for the full strategy and cutover criteria.

## Security Model

- OAuth 2.1 with mandatory PKCE S256
- Redirect URI whitelist (`*.claude.ai`, `*.anthropic.com` only)
- Tokens stored as SHA-256 hashes — plain tokens never written to disk
- Constant-time credential comparison throughout
- TLS backend verification via `MCP_CA_CERT`
- Hardened systemd unit (`NoNewPrivileges`, `MemoryDenyWriteExecute`, syscall filter)
- Per-request audit log with UUID correlation IDs

## Building

```bash
go build -o bin/mcp-runtime ./cmd/mcp-runtime
go build -o bin/shadow-compare ./cmd/shadow-compare
```

## Testing

```bash
go test ./...
go test -race ./...
./scripts/test-all.sh
```

## Configuration

All configuration is via environment variables. Copy `deploy/env/mcp-runtime-shadow.env.example` and fill in real values.

Key variables: `SHADOW_MODE`, `LISTEN_HOST`, `LISTEN_PORT`, `MCP_BACKEND_URL`, `MCP_PROXY_BASE_URL`, `MCP_CLIENT_ID`, `MCP_CA_CERT`, `MCP_TOKEN_STORE`, `MCP_AUDIT_LOG`, `MANDATORY_PKCE`.

## Current Status

| Component | Status |
|---|---|
| OAuth proxy parity (RFC 6749/7591/8414/9728) | Complete |
| PKCE S256 | Complete |
| TLS backend verification | Complete |
| Shadow deployment | Active — 24h gate passed |
| Go authoritative cutover | Pending |
| Hugo MCP domain | Planned |

## Roadmap

- [ ] Cutover: OpenResty → Go authoritative for MCP OAuth
- [ ] Hugo MCP second domain (`internal/hugomcp`)
- [ ] SQLite token store for multi-domain scale
- [ ] Prometheus metrics endpoint
- [ ] GitHub Actions CI

## Documentation

Full documentation is in [`docs/`](docs/) — start with [`docs/INDEX.md`](docs/INDEX.md).

## Relationship with mcp-oauth-proxy

This project is the Go successor to [mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy). Python remains authoritative while Go passes the shadow gate. See [docs/migration/MIGRATION_PLAN.md](docs/migration/MIGRATION_PLAN.md).

## License

MIT
