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

## Optional Anonymous MCP Mode

By default, `/mcp` remains authenticated and requires a valid bearer token.

When `ANONYMOUS_ENABLED=true`, the proxy accepts unauthenticated MCP JSON-RPC
requests only for a narrow public surface:

- protocol setup methods: `initialize`, `notifications/initialized`, `ping`
- `tools/list`
- `tools/call` only when `params.name` appears in `ANONYMOUS_PUBLIC_TOOLS`

Invalid bearer tokens are still rejected with `401`; anonymous fallback is only
available when no `Authorization` header is present.

Anonymous `tools/list` responses are filtered so only tools named in
`ANONYMOUS_PUBLIC_TOOLS` are advertised to anonymous clients. The filter supports
plain JSON-RPC responses and server-sent event `data:` JSON payloads.

This mode is intended for public read-only MCP servers. It must not be used in
front of a backend that exposes write or administrative tools unless every
publicly callable tool is intentionally allowlisted and tested.

## Optional OAuth Scope-to-Tool ACL

OAuth is optional for public read-only deployments. Anonymous requests can keep
using the public read-only surface, while agents that present a valid bearer
token may be constrained by a scope-to-tool ACL.

Configure the ACL with `AUTHENTICATED_SCOPE_TOOLS`:

```text
AUTHENTICATED_SCOPE_TOOLS=mcp:list_pages|get_page|search_pages|get_recent_posts|list_tags|list_categories|get_sitemap|get_feed|get_site_information
```

Semantics:

- if `AUTHENTICATED_SCOPE_TOOLS` is empty, valid bearer tokens retain the legacy
  proxy behavior;
- if a `mcp` mapping is present, valid bearer tokens may call only the listed
  `tools/call` names;
- authenticated `tools/list` responses are filtered to advertise only the tools
  allowed by the scope;
- protocol setup methods (`initialize`, `notifications/initialized`, `ping`)
  remain allowed;
- methods outside that narrow MCP surface are rejected before the backend is
  reached.

For `hugo-public-mcp`, the production candidate model is:

- anonymous read-only remains available;
- OAuth is optional and does not unlock private tools yet;
- bearer tokens are limited to the same public read-only tools as anonymous
  clients until a separate design introduces private scopes.

Refresh tokens and token revocation are intentionally not part of this model yet.
Short-lived access tokens plus SQLite WAL persistence are sufficient for the
current public read-only staging validation. Add revocation or refresh tokens
only if a future private-tool design requires them.

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
- `ANONYMOUS_ENABLED`
- `ANONYMOUS_PUBLIC_TOOLS`
- `AUTHENTICATED_SCOPE_TOOLS`

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
