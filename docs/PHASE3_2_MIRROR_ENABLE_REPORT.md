# Phase 3.2 Safe Shadow Mirror Enablement

## Final Verdict

MIRROR ACTIVE

## Summary

The OpenResty mirror for `mcp-hugo.arleo.eu` was enabled safely and reloaded successfully while keeping Python authoritative and leaving public routing unchanged.

## Files Modified

- `/usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu`
- `/usr/local/openresty/nginx/conf/conf.d/mcp-allowlist.conf`

## Backup Locations

- `/usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.bak-20260601_022134`
- `/usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-mirror-20260601_022156`
- `/usr/local/openresty/nginx/conf/backups/mcp-allowlist.conf.bak-20260601_022642`

## Mirror Configuration Applied

- Python remains the primary `proxy_pass` target for `/mcp`.
- A mirrored internal location was added:
  - `location = /__mcp_shadow { internal; ... }`
- The mirror preserves:
  - method
  - query string
  - request body via `mirror_request_body on`
- The mirror stays internal and is not publicly exposed.
- `X-Client-Request-ID` is propagated as informational correlation only.

## Nginx Validation

`sudo /usr/local/openresty/nginx/sbin/nginx -t`

Result:

- syntax OK
- configuration test successful

## Reload Result

`sudo systemctl reload openresty`

Result:

- reload succeeded
- `openresty.service` remained active

## Verification Performed

### Shadow health

`./scripts/healthcheck-shadow.sh http://127.0.0.1:8085`

Result:

- `OK: healthz`
- `OK: readyz`
- `OK: oauth metadata`
- `OK: protected resource metadata`

### Shadow status

`./scripts/shadow-status.sh /var/log/nginx/mcp-hugo.access.log /var/log/mcp-runtime-go/audit-shadow.jsonl /var/log/mcp-runtime-go/reports`

Result:

- OpenResty status visible
- local status command could not read the audit file without elevated privileges
- this is a tooling permission issue, not a shadow runtime failure

## Public Endpoint Check

- The public MCP route continued to return `401` with the expected `WWW-Authenticate` metadata.
- No public route was switched to Go.
- Python remained authoritative.

## Mirrored Event Observation

- A local origin request was sent through the mirrored `/mcp` path.
- The mirror config was exercised successfully at the HTTP layer.
- The Go audit log did not gain a new line from that specific path because the mirrored handler `/mcp/` is a proxy path that does not emit an audit entry on unauthenticated requests.

## Go Shadow Confirmation

- `mcp-runtime-shadow.service` remained active on `127.0.0.1:8085`.
- The shadow healthcheck passed after the mirror reload.

## Notes / Limitations

- The mirror is active and safe.
- The specific `/mcp` traffic mirrored here does not itself produce a Go audit line unless it exercises an audited OAuth path such as `/authorize` or `/token`.
- That means the mirror is live, but a first mirrored audit event was not observed from this exact path.

## Rollback

- No rollback was performed.
- The original backup files were created before any changes.

