# hugo-public-mcp OAuth Staging Hardening Report

Date: 2026-07-02

Scope: `staging-mcp.arleo.eu` only. Production `mcp.arleo.eu` was not moved
behind `mcp-runtime-go`.

## Auth Model

The production-candidate model is:

- anonymous MCP access remains enabled for public read-only tools;
- OAuth is optional;
- OAuth does not unlock private tools in the current design;
- no write tools, admin tools, Hugo rebuild, shell, filesystem, or private Hugo
  MCP access are exposed;
- bearer tokens are constrained by a scope-to-tool ACL before traffic reaches the
  backend.

Current staging allowlists:

```text
ANONYMOUS_PUBLIC_TOOLS=list_pages,get_page,search_pages,get_recent_posts,list_tags,list_categories,get_sitemap,get_feed,get_site_information
AUTHENTICATED_SCOPE_TOOLS=mcp:list_pages|get_page|search_pages|get_recent_posts|list_tags|list_categories|get_sitemap|get_feed|get_site_information
```

`TRUSTED_AUTHORIZE_CIDRS` no longer uses `0.0.0.0/0,::/0`. Staging now uses:

```text
82.65.145.189/32,192.168.1.0/24,127.0.0.1/32,::1/128
```

OpenResty staging also has an explicit `/authorize` location with `allow` for
the operator public IP and LAN, then `deny all`.

## Comparison

| Area | `mcp.arleo.eu` production | `staging-mcp.arleo.eu` OAuth staging |
| --- | --- | --- |
| Runtime | `hugo-public-mcp` directly | `mcp-runtime-go` proxying to `hugo-public-mcp` |
| Auth requirement | none | anonymous allowed, OAuth optional |
| OAuth discovery | absent on production | present and valid for staging |
| Public tools | read-only Hugo tools | same read-only tools |
| Private tools | none | none |
| Bearer invalid | not applicable | `401` with `WWW-Authenticate` |
| Bearer valid | not applicable | constrained by `AUTHENTICATED_SCOPE_TOOLS` |
| `/authorize` exposure | not present | restricted to operator/LAN at OpenResty and Go CIDR gate |
| Backend port exposure | production app port remains unchanged | staging app binds `127.0.0.1:8092` only |
| IsItAgentReady role | canonical public MCP | OAuth staging endpoint only |

## Benefits

- Adds real OAuth Authorization Code + PKCE discovery without making OAuth
  mandatory for public read-only content.
- Keeps public anonymous tools available for agents that do not need tokens.
- Prevents bearer tokens from becoming implicit broad backend access.
- Provides a staging path to test client interoperability before any production
  cutover.

## Risks

- Dynamic Client Registration is still single-tenant: it returns the configured
  client identity rather than creating independent durable clients.
- There is no token revocation endpoint.
- There are no refresh tokens.
- The current public read-only use case does not require private scopes; adding
  private tools later needs a separate scope design and tests.
- `/authorize` is intentionally operator/LAN restricted. This is safer for
  staging, but a fully public OAuth consent model would need a real user-auth or
  consent ceremony before production.

## Revocation and Refresh Token Decision

Do not add refresh tokens or revocation for the current public read-only staging
candidate.

Reasoning:

- anonymous read-only access remains available without OAuth;
- OAuth tokens currently unlock only the same public read-only tools;
- short-lived access tokens plus SQLite WAL persistence are sufficient for
  staging validation;
- adding refresh/revocation before private scopes would increase surface and
  operational burden without clear benefit.

Revisit this only if a future design introduces private scopes or longer-lived
authenticated sessions.

## Validation Evidence

Local/runtime:

```text
systemctl is-active mcp-runtime-staging.service -> active
listener -> 127.0.0.1:8092
GET http://127.0.0.1:8092/healthz -> OK
GET http://127.0.0.1:8092/readyz -> OK
```

Public staging endpoints:

```text
/.well-known/oauth-authorization-server -> 200 application/json
/.well-known/oauth-protected-resource   -> 200 application/json
/auth.md                                -> 200 text/markdown
/healthz                                -> 200 text/plain
/readyz                                 -> 200 text/plain
```

Security behavior:

```text
Go direct /authorize with X-Forwarded-For: 203.0.113.10 -> 403
OAuth DCR + Authorization Code PKCE + token exchange -> OK
Authenticated tools/list -> filtered to read-only ACL
Authenticated get_site_information -> OK
Authenticated publish_post -> 403 before backend
Anonymous tools/list -> filtered to read-only tools
```

Leak scan over public staging discovery and health endpoints:

```text
No /home/jm
No 192.168.
No .git
No token or secret patterns
```

Project validation:

```text
go test ./...        -> PASS
go test -race ./...  -> PASS
go vet ./...         -> PASS
gitleaks detect      -> PASS
```

`golangci-lint run ./...` is not clean because of existing errcheck/staticcheck
debt in unrelated tests and handlers. The new unused symbol found during this
pass was removed.

IsItAgentReady staging:

```text
level: 0 Not Ready
OAuth Discovery: PASS
OAuth Protected Resource: PASS
auth.md: PASS
MCP Server Card: PASS
robots.txt/content-signal: PASS
```

The remaining staging failures are expected because `staging-mcp.arleo.eu` is
not the full Hugo content site:

- sitemap
- DNS-AID
- API Catalog
- Agent Skills
- Markdown negotiation
- A2A
- WebMCP

## Rollback

Disable staging OAuth runtime:

```bash
sudo systemctl disable --now mcp-runtime-staging.service
```

Remove staging vhost:

```bash
sudo rm -f /usr/local/openresty/nginx/conf/sites-enabled/staging-mcp.arleo.eu
sudo openresty -t
sudo systemctl reload openresty
```

Restore saved staging backups if needed:

```bash
sudo cp -a /etc/mcp-runtime-go/mcp-runtime-staging.env.bak-authorize-hardening-YYYYMMDD-HHMMSS /etc/mcp-runtime-go/mcp-runtime-staging.env
sudo cp -a /usr/local/bin/mcp-runtime-staging.bak-YYYYMMDD-HHMMSS /usr/local/bin/mcp-runtime-staging
sudo systemctl restart mcp-runtime-staging.service
```

Production rollback is not required for this pass because production was not
changed.

## Verdict

GO for a future production-candidate PR that documents and reviews this model.

NO-GO for direct production cutover today.

Before production, decide explicitly whether `/authorize` should remain
operator/LAN-only or whether a real public consent/user-auth model is required.
Do not expose OAuth broadly without that decision.
