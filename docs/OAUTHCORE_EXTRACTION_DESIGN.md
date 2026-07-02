# OAuth Core Extraction Design

Date: 2026-07-02

Related issue: https://github.com/jmrGrav/mcp-runtime-go/issues/6

Status: design only. No runtime behavior change, no production deployment, no tag,
and no release are authorized by this document.

## Goal

Prepare a clean reusable OAuth component boundary inside `mcp-runtime-go` so
`hugo-public-mcp` can later integrate real optional OAuth without running
`mcp-runtime-go` as a second production proxy.

The near-term target is an internal package boundary, not a new public module:

```text
mcp-runtime-go
  internal/oauthcore/
    metadata
    client registration
    authorization code + PKCE
    token exchange
    token validation
    scope-to-tool ACL
    request source policy
    audit redaction helpers

  internal/oauthproxy/
    HTTP handlers
    reverse proxy
    mcp-runtime-go-specific backend token injection
```

## Constraints

- Do not change current `mcp-runtime-go` production behavior.
- Do not modify `hugo-public-mcp` in this extraction step.
- Do not publish a shared module yet.
- Do not add fake OAuth, OIDC, JWKS, revocation, or refresh-token endpoints.
- Do not weaken `/authorize` source restrictions.
- Keep `/authorize` operator-only by default.
- Keep proxy-specific behavior out of the reusable core.

## Current Coupling

`internal/oauthproxy` currently owns both reusable OAuth policy and
`mcp-runtime-go` proxy behavior.

Reusable concepts currently inside `oauthproxy`:

- Authorization Server Metadata
- Protected Resource Metadata
- Dynamic Client Registration request/response models
- Authorization Code + PKCE validation
- token exchange and token hashing
- authorization-code lifetime and access-token lifetime
- trusted proxy/source IP checks through `internal/security`
- RFC 6749 error mapping

Proxy-specific concepts currently inside `oauthproxy`:

- reverse proxy construction
- backend URL parsing
- backend host rewriting
- backend bearer token injection
- hop-by-hop header stripping
- `/mcp` path rewriting
- anonymous response filtering on proxied backend responses

The extraction should separate those two sets without changing either behavior.

## Proposed Package Boundary

Create `internal/oauthcore` in small steps.

Suggested files:

```text
internal/oauthcore/models.go
internal/oauthcore/config.go
internal/oauthcore/metadata.go
internal/oauthcore/service.go
internal/oauthcore/pkce.go
internal/oauthcore/redirect.go
internal/oauthcore/tokens.go
internal/oauthcore/source_policy.go
internal/oauthcore/scope_tools.go
internal/oauthcore/errors.go
```

### `models.go`

Own wire/domain models that are not proxy-specific:

- `AuthCode`
- `RegistrationRequest`
- `RegistrationResponse`
- `AuthorizeRequest`
- `TokenExchangeRequest`
- `TokenResponse`

### `config.go`

Own only generic OAuth config:

- issuer
- resource URL
- client ID / client secret
- redirect URI allowlist
- auth-code TTL
- access-token TTL
- mandatory PKCE
- trusted proxies
- trusted authorize CIDRs
- supported scopes
- scope-to-tool ACL

Do not include:

- Hugo backend URL
- Hugo backend host
- Hugo backend token
- reverse-proxy URL/path settings

### `metadata.go`

Build metadata payloads without `http.ResponseWriter`:

- `AuthorizationServerMetadata(Config) map[string]any`
- `ProtectedResourceMetadata(Config) map[string]any`

The HTTP layer decides methods, headers, status codes, and JSON encoding.

### `service.go`

Own OAuth state transitions:

- client registration validation
- authorization-code issuance
- authorization-code consumption
- token exchange
- access-token validation
- token purge

The service should depend on a generic token store and a generic audit/log
callback, not on `oauthproxy` or reverse-proxy internals.

### `tokens.go`

Define the token store interface in core terms:

```go
type TokenStore interface {
    Load(context.Context) (map[string]float64, error)
    Save(context.Context, map[string]float64) error
    Close() error
}
```

Keep the existing `internal/storage.Store` adapter in `mcp-runtime-go` until a
future shared module decision is made.

### `source_policy.go`

Own trusted proxy/source IP resolution and `/authorize` source allowlist checks.

Inputs should be explicit:

- remote address
- `X-Forwarded-For`
- trusted proxy CIDRs/IPs
- allowed authorize CIDRs/IPs

This avoids embedding `*http.Request` into pure policy code when possible.

### `scope_tools.go`

Own parsing and evaluation of scope-to-tool ACL:

- parse `mcp:list_pages|get_page`
- decide whether a tool name is allowed for a scope
- filter `tools/list` data structures only through explicit inputs

Do not put reverse-proxy response mutation in this package.

### `errors.go`

Own RFC error mapping:

- authorization endpoint error code mapping
- token endpoint error code/status mapping

HTTP handlers remain responsible for redirect or JSON response formatting.

## Adapter Strategy

`internal/oauthproxy` should become an adapter around `oauthcore`:

- translate environment config into `oauthcore.Config`;
- expose HTTP handlers;
- perform method checks and HTTP response formatting;
- call `oauthcore.Service` for registration, authorize, token, validation, and
  ACL decisions;
- keep reverse proxy behavior local to `oauthproxy`.

This keeps current `mcp-runtime-go` behavior intact while making the OAuth
policy reusable by `hugo-public-mcp`.

## Extraction Order

1. Move pure models into `internal/oauthcore`.
2. Move PKCE, random, redirect URI, and request source helpers behind
   `oauthcore` wrappers while keeping existing tests green.
3. Move metadata builders into `oauthcore` and keep handler output byte-for-byte
   equivalent where practical.
4. Move registration, auth-code, token exchange, token validation, and purge into
   `oauthcore.Service`.
5. Add `oauthproxy` adapter tests that prove the public HTTP behavior did not
   change.
6. Move scope-to-tool ACL into `oauthcore` after PR #5 is merged or rebased, so
   the ACL is not duplicated.
7. Reassess whether a separate module is justified only after
   `hugo-public-mcp` consumes the internal boundary in staging.

## Test Requirements

Every extraction step must preserve or add tests for:

- metadata payload shape;
- protected resource metadata shape;
- invalid redirect URI fail-closed behavior;
- DCR validation;
- PKCE success/failure;
- missing state rejection;
- single-use authorization code;
- expired authorization code;
- token hash validation;
- token persistence failure fail-closed behavior;
- trusted proxy/source IP behavior;
- operator-only `/authorize` policy;
- scope-to-tool allow/deny behavior once ACL is in scope;
- no token/secret leakage in audit fields.

## Risks

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Extracting too much at once | OAuth regression in runtime proxy | Move one behavior at a time and keep HTTP adapter tests green |
| Core still depends on proxy concepts | `hugo-public-mcp` cannot reuse it cleanly | Explicitly ban Hugo backend URL/token/path fields from `oauthcore.Config` |
| Shared module too early | Public API freezes before requirements are clear | Keep `internal/oauthcore` first |
| Divergence with PR #5 ACL work | Duplicate scope-to-tool logic | Land/rebase PR #5 before extracting ACL logic |
| Public `/authorize` pressure | Security regression | Keep operator-only default and require separate issue for public consent model |

## GO / NO-GO

GO:

- create `internal/oauthcore` as an internal boundary;
- keep `oauthproxy` as the only HTTP/proxy adapter;
- keep existing runtime behavior unchanged;
- use tests to prove handler compatibility.

NO-GO:

- publish `mcp-oauth-go` now;
- change production runtime behavior;
- add public `/authorize`;
- add fake OIDC/JWKS/revocation/refresh endpoints;
- modify `hugo-public-mcp` before the extraction boundary is reviewed.
