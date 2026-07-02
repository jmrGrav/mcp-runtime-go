# Operations

This document is the current operator reference for installing, running, validating,
and recovering `mcp-runtime-go` in production.

## Installation

Production runs as `mcp-runtime.service`.

Typical install flow:

1. Deploy the binary.
2. Install the env file under `/etc/mcp-runtime-go/mcp-runtime.env`.
3. Install the systemd unit.
4. Start the service.
5. Validate `/readyz`, `/healthz`, and `/metrics` on `127.0.0.1:8086`.

## Configuration

Use environment variables only.

Required values usually include:

- `PROXY_BASE_URL`
- `HUGO_MCP_URL`
- `HUGO_HOST`
- `HUGO_TOKEN`
- `CLIENT_ID`
- `CLIENT_SECRET`

Common production settings:

- `LISTEN_HOST=127.0.0.1`
- `LISTEN_PORT=8086`
- `USE_SQLITE=true`
- `TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db`
- `AUDIT_LOG_FILE=/var/log/mcp-runtime-go/audit.jsonl`
- `TRUSTED_PROXIES=127.0.0.1,::1`
- `MANDATORY_PKCE=true`
- `ALLOW_TOKEN_STORE_RECOVERY=false`
- `ANONYMOUS_ENABLED=false`
- `ANONYMOUS_PUBLIC_TOOLS=`

Legacy `GRAV_*` variables are supported only as compatibility fallback.

## Optional Anonymous Read-Only Mode

Anonymous MCP access is disabled by default.

Enable it only for public read-only MCP backends:

```bash
ANONYMOUS_ENABLED=true
ANONYMOUS_PUBLIC_TOOLS=search_pages,get_page,list_pages
```

Behavior:

- no `Authorization` header: allowed only for protocol setup, `tools/list`, and
  `tools/call` names present in `ANONYMOUS_PUBLIC_TOOLS`;
- anonymous `tools/list`: response is filtered to advertise only
  `ANONYMOUS_PUBLIC_TOOLS`;
- invalid `Authorization: Bearer ...`: always rejected with `401`;
- valid bearer token: authenticated proxy behavior is unchanged.

Do not enable this mode in front of an administrative MCP backend unless every
write-capable tool is excluded from the public allowlist and separately tested.

## Systemd

The service is expected to run as a hardened unit with:

- `NoNewPrivileges=true`
- `ProtectSystem=strict`
- `ProtectHome=true`
- `MemoryDenyWriteExecute=true`
- a restricted syscall profile
- explicit read/write paths for the state and log directories

Key checks:

```bash
systemctl status mcp-runtime --no-pager
systemctl cat mcp-runtime
```

## Edge / Proxy Notes

- Public traffic flows through Cloudflare and OpenResty.
- CrowdSec / OpenResty controls may block at the edge before Go sees the request.
- `/authorize` is intentionally restricted to trusted operator IPs.
- If a request is blocked at the edge, check the OpenResty access/error logs and CrowdSec decisions.

## Health and Metrics

- `GET /healthz` is the basic liveness check.
- `GET /readyz` must return `200` before traffic is considered safe.
- `GET /metrics` is loopback-only and should not be publicly reachable.

Useful commands:

```bash
curl -fsS http://127.0.0.1:8086/healthz
curl -fsS http://127.0.0.1:8086/readyz
curl -fsS http://127.0.0.1:8086/metrics
```

## Logs

- Application audit log: `/var/log/mcp-runtime-go/audit.jsonl`
- OpenResty access log: `/var/log/nginx/mcp-hugo.access.log`
- OpenResty error log: `/var/log/nginx/mcp-hugo.error.log`

Audit logs are append-only and should be rotated with `logrotate`.

## Logrotate

Recommended behavior:

- rotate the audit log
- compress old rotations
- keep enough history for incident review
- signal the service if log reopening is required

## Backups

Before major config changes, keep copies of:

- the systemd unit
- the env file
- the OpenResty site config
- the token store

## SQLite Migration

SQLite WAL is the production store.

Migration procedure:

1. Stop `mcp-runtime.service`.
2. Back up `tokens.json` and the env file.
3. Set `TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db`.
4. Run `mcp-runtime migrate-storage`.
5. Set `USE_SQLITE=true`.
6. Start the service.
7. Verify `readyz`, `metrics`, and token continuity.

Rollback from migration:

- restore the env file backup
- restore the JSON token file backup
- restart the service with `USE_SQLITE=false`

## Rollback

Current production rollback is back to the authoritative state that was last validated.
Use the archived rollback notes if you need additional historical context.

Practical rollback checklist:

1. Confirm the replacement binary or config is the likely failure source.
2. Restore the last known-good env file.
3. Validate the systemd unit and OpenResty config.
4. Restart the service.
5. Recheck `/readyz`, `/healthz`, audit logs, and metrics.

## Common Pitfalls

- Missing `TOKENS_DB` or mismatched `USE_SQLITE` can point the runtime at the wrong store.
- `/authorize` failures are often edge policy or trusted proxy issues, not application bugs.
- Metrics may be healthy locally even when public routing is broken.
- An audit write failure should be treated as a production issue, not a warning to ignore.
