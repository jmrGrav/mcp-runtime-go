# Architecture

`mcp-runtime-go` is the authoritative Go runtime for OAuth 2.0 + PKCE and MCP proxying.
It owns the live production flow.

## System View

```text
Claude.ai
  -> Cloudflare
  -> OpenResty + CrowdSec
  -> mcp-runtime-go (127.0.0.1:8086)
      -> OAuth 2.0 / PKCE
      -> Dynamic Client Registration
      -> SQLite WAL token store
      -> Audit log + metrics
      -> /healthz and /readyz
  -> Hugo MCP backend (internal TLS)
```

## Packages

- `cmd/mcp-runtime`
  - Composition root, HTTP server startup, and `migrate-storage`.
- `internal/runtime`
  - App wiring, lifecycle, graceful shutdown, request ID middleware.
- `internal/httpserver`
  - HTTP server wrapper, readiness coordination, and shutdown handling.
- `internal/oauthproxy`
  - OAuth handlers, client registration, token exchange, and MCP proxying.
- `internal/security`
  - PKCE S256, redirect URI validation, request info, random generation.
- `internal/storage`
  - Token persistence with SQLite WAL in production and JSON for migration / rollback.
- `internal/observability`
  - Structured audit logging and process metrics.
- `internal/config`
  - Environment-driven configuration with fail-closed validation.
- `internal/context`
  - Request-scoped client request ID propagation.

## OAuth Flow

1. `/.well-known/oauth-authorization-server` and `/.well-known/oauth-protected-resource` expose metadata.
2. `/register` performs Dynamic Client Registration.
3. `/authorize` is gated by operator IP / trusted proxy validation and PKCE.
4. `/token` exchanges the authorization code for an access token.
5. `/mcp` proxies authenticated requests to the Hugo backend.

Important guarantees:

- redirect URIs must match the registered allowlist
- PKCE S256 is mandatory by default
- authorization codes are single-use
- bearer tokens are stored only as hashes
- failures in persistence or audit logging are fail-closed

## Storage

- Production store: SQLite WAL.
- Legacy store: JSON token file retained only for migration / rollback paths.
- Migration path: `mcp-runtime migrate-storage`.
- The runtime checkpoints SQLite during migration and validates the resulting store before serving traffic.

## Observability

- JSON audit events are written for OAuth, proxy, and readiness events.
- Metrics are exposed only on loopback.
- `/readyz` checks configuration, token store availability, and audit writeability.
- `/healthz` is a basic process liveness endpoint.

## Configuration

The canonical configuration prefix is `HUGO_*`.

Legacy `GRAV_*` variables are accepted only for compatibility and are intentionally treated as historical.

Common runtime controls:

- `LISTEN_HOST`, `LISTEN_PORT`
- `HUGO_MCP_URL`, `HUGO_HOST`, `HUGO_TOKEN`
- `PROXY_BASE_URL`
- `CLIENT_ID`, `CLIENT_SECRET`
- `MCP_CA_CERT`
- `USE_SQLITE`, `TOKENS_DB`, `TOKENS_FILE`
- `AUDIT_LOG_FILE`
- `TRUSTED_PROXIES`
- `MANDATORY_PKCE`

## Security Model

- PKCE is mandatory.
- Redirect URIs are validated against the registered client record.
- Requests are bound to trusted proxy IPs before extracting client identity.
- Tokens are hashed before persistence.
- Audit logging is structured and request-scoped.
- The backend TLS connection validates the configured CA certificate.
- Systemd hardening constrains the runtime process at the OS boundary.

## Historical / Legacy

- Python rollback references are retained only as historical recovery context.
- Additional history, audits, and migration notes live under `docs/archive/`.
