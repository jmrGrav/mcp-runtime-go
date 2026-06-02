# mcp-runtime-go

Production-grade Go runtime for OAuth proxying, MCP integration, shadow deployment, auditability, and future Hugo MCP migration.

## Project Goals

This runtime replaces the Python `mcp-oauth-proxy` with a strongly-typed, observable, and operationally hardened Go service. The migration follows a zero-downtime shadow deployment strategy: Go runs alongside Python, receives mirrored traffic, and its decisions are compared before it becomes authoritative.

Goals:
- Full behavioral parity with the Python OAuth proxy (RFC 6749, RFC 7591, RFC 8414, RFC 9728, PKCE S256)
- Structured JSON audit logging with per-request correlation IDs
- Shadow deployment tooling (`shadow-compare`) for log-level parity verification
- Hardened systemd deployment with seccomp-style restrictions
- Foundation for future Hugo MCP domain integration

## Architecture

```
cmd/
  mcp-runtime/       Entry point
  shadow-compare/    Audit log comparison tool

internal/
  config/            Environment-driven configuration
  context/           Per-request ID propagation
  httpserver/        HTTP server lifecycle
  oauthproxy/        OAuth 2.0 domain (handlers, service, proxy, models)
  observability/     Structured audit logger (JSON lines)
  runtime/           App wiring, request-ID middleware
  security/          PKCE S256, redirect URI validation, random generation
  storage/           Token store (JSON, compatible with Python format)

deploy/
  env/               Environment file template (no real values)
  nginx/             OpenResty mirror config example
  systemd/           Hardened service unit
```

## Shadow Deployment Strategy

The runtime is designed for zero-risk migration via shadow mode:

1. **Python stays authoritative** — all real traffic is served by the existing Python proxy.
2. **Go runs in shadow** — OpenResty mirrors requests to the Go service on a separate port.
3. **Audit comparison** — `shadow-compare` matches Python and Go audit logs by request ID, flagging any decision divergence on critical events (`token_issued`, `authorize_approved`, `client_registered`).
4. **Gate criteria** — cutover is blocked until the 24h+ shadow comparison passes with 0 critical mismatches.

```
[Client]
    │
    ▼
[OpenResty]─────────────────────────────────────────────────────┐
    │                                                            │ mirror
    ▼                                                            ▼
[Python :8084] ── authoritative ── audit-hugo.log    [Go :8085] ── shadow ── audit-shadow.jsonl
                                                            │
                                                     [shadow-compare]
                                                            │
                                                     PASS / FAIL
```

The `shadow-compare` tool matches events using:
- **Primary**: shared `request_id` (when both sides use a correlation header injected by OpenResty)
- **Fallback**: time + event type + source IP within a 2-second window (`--allow-unsafe-fallback`)

## Current Status

| Component | Status |
|---|---|
| OAuth proxy parity (RFC 6749/7591/8414/9728) | Complete |
| PKCE S256 | Complete |
| TLS backend verification (`MCP_CA_CERT`) | Complete |
| Shadow mode deployment | Active (24h observation gate) |
| Audit log comparison tooling | Complete |
| Go authoritative cutover | Pending shadow gate |
| Hugo MCP domain | Planned |

## Configuration

All configuration is via environment variables. See `deploy/env/mcp-runtime-shadow.env.example` for the full list.

Key variables:

| Variable | Description |
|---|---|
| `SHADOW_MODE` | `true` = shadow only, `false` = authoritative |
| `LISTEN_HOST` | Bind address (default `127.0.0.1`) |
| `LISTEN_PORT` | Bind port |
| `MCP_BACKEND_URL` | Upstream MCP server URL |
| `MCP_PROXY_BASE_URL` | Public base URL for OAuth metadata |
| `MCP_CLIENT_ID` | OAuth client ID |
| `MCP_CA_CERT` | Custom CA certificate for backend TLS |
| `MCP_TOKEN_STORE` | Path to token persistence file |
| `MCP_AUDIT_LOG` | Path to structured audit log |
| `MANDATORY_PKCE` | Require PKCE on all authorization flows |

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

## Deployment

```bash
# 1. Copy and configure environment file
cp deploy/env/mcp-runtime-shadow.env.example /etc/mcp-runtime-go/mcp-runtime.env
# edit with real values

# 2. Install systemd unit
cp deploy/systemd/mcp-runtime-shadow.service /etc/systemd/system/mcp-runtime.service
systemctl daemon-reload
systemctl enable --now mcp-runtime

# 3. Run shadow healthcheck
./scripts/healthcheck-shadow.sh http://127.0.0.1:<port>

# 4. After 24h+ observation, run comparison
./scripts/shadow-compare-48h.sh <python_log> <go_log> <report_dir>
```

## Roadmap

- [ ] Pass 24h shadow gate → Go becomes authoritative for MCP OAuth
- [ ] Decommission Python shadow setup
- [ ] Add Hugo MCP domain (`internal/hugomcp`)
- [ ] Implement SQLite token store for multi-domain scale
- [ ] Add Prometheus metrics endpoint

## Relationship with mcp-oauth-proxy

This project is the strategic successor to [mcp-oauth-proxy](https://github.com/jmrGrav/mcp-oauth-proxy), the Python implementation. The Python service remains fully functional and is the current authoritative proxy. The Go runtime is currently running in shadow mode alongside it.

See [docs/MIGRATION_PLAN.md](docs/MIGRATION_PLAN.md) for the full migration plan and [docs/SHADOW_MODE.md](docs/SHADOW_MODE.md) for the shadow deployment strategy.

## License

MIT
