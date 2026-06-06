# Operations Reference

## Current Go Live State

Go (`mcp-runtime.service`) is authoritative for all MCP and OAuth traffic on `mcp-hugo.arleo.eu`
as of 2026-06-03. The Python implementation (`hugo-mcp-proxy.service`) is preserved on disk as a
rollback reference but is stopped and disabled.

### Service topology

| Service | Port | State | Role |
|---|---|---|---|
| `mcp-runtime.service` | 8086 | **active/enabled** | Authoritative Go runtime |
| `mcp-runtime-shadow.service` | 8085 | disabled | Shadow mode — retired 2026-06-03 |
| `hugo-mcp-proxy.service` | 8084 | disabled | Preserved as rollback reference |

### Routing

```
Public internet
    → Cloudflare
    → OpenResty (mcp-hugo.arleo.eu)
    → Go mcp-runtime 127.0.0.1:8086
    → Hugo MCP backend (internal)
```

### Endpoint access matrix

| Endpoint | Who can reach it | Notes |
|---|---|---|
| `POST /mcp` | Anthropic IP ranges (see CrowdSec section) | Requires valid Bearer token; unauthenticated returns 401 + WWW-Authenticate |
| `GET /.well-known/oauth-authorization-server` | Anthropic + operator | OAuth metadata discovery |
| `GET /.well-known/oauth-protected-resource` | Anthropic + operator | Protected resource metadata |
| `POST /register` | Anthropic + operator | RFC 7591 dynamic client registration — open |
| `GET /authorize` | **Operator IP only** | Single-tenant: only the operator's browser completes login |
| `POST /token` | Anthropic IP ranges | Token exchange after successful authorize |
| `GET /metrics` | localhost only | Prometheus-text internal metrics (not exposed publicly) |

### Single-tenant authorization

`/authorize` is restricted to the operator IP via two layers:
1. A dedicated nginx `location = /authorize` block with `allow <operator-ip>; deny all;`
   (configured in `/usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu`).
2. A Go-level CIDR check against `TRUSTED_AUTHORIZE_CIDRS` in the runtime env file.

The operator IP is not documented here. Retrieve it from:
```
sudo grep TRUSTED_AUTHORIZE_CIDRS /etc/mcp-runtime/mcp-runtime.env
```

This means:
- Any Claude.ai account can discover the MCP and register a client.
- Only requests from the operator's IP can complete the OAuth login step.
- Anyone else attempting to connect via Claude.ai receives 403 at the authorize step.

### Token storage

v1.1+ uses SQLite WAL mode at `/opt/mcp-oauth-proxy/tokens.db` (configured via `TOKENS_DB`).
The legacy JSON store (`TOKENS_FILE`) is retained for emergency rollback only. The runtime logs
the active backend at INFO level on startup.

---

## Rollback

See **[ROLLBACK_PRODUCTION.md](ROLLBACK_PRODUCTION.md)** for the current, executable rollback procedure.

The `ROLLBACK.md` file in this directory describes the **historical shadow mode rollback**
(Go shadow → Python authoritative). It is no longer applicable to the current production setup.

---

## Known Operational Pitfall — CrowdSec / OpenResty Local Heuristic Scorer

### Symptom

Anthropic or Claude.ai backend IPs receive `403` responses with very short response time
(`request_time ≈ 0.000`) and a fixed small response body (~150 bytes). The full OAuth flow
(metadata discovery, `/register`, `/authorize`) appears blocked. Running `sudo cscli decisions list`
shows **no active bans**.

### Root cause

CrowdSec's OpenResty bouncer runs a **local heuristic scorer** in the nginx Lua shared dict.
This scorer accumulates a per-IP anomaly score across requests. Anthropic's backend makes rapid
sequential requests during the OAuth flow (unauthenticated `/mcp` → discovery → register →
authorize → token), which can cross the heuristic block threshold and trigger an inline ban.

This ban is **local to the Lua shared dict** — it never creates a LAPI decision, so
`cscli decisions list` appears empty. The LAPI allowlist (`cscli allowlists`) prevents LAPI
decisions but does **not** bypass the local heuristic scorer.

### Fix

Add the affected IP ranges to the `$lan` geo map in the OpenResty config:

```
/usr/local/openresty/nginx/conf/conf.d/lan.conf
```

IPs with `$lan = 1` skip `crowdsec.access.check()` entirely before the scorer runs.
After editing:

```bash
sudo /usr/local/openresty/nginx/sbin/nginx -t
sudo systemctl reload openresty
```

**The actual Anthropic IP ranges are NOT documented here** — they are maintained in the local
nginx configuration only (never committed to this repository). If the ranges are unknown, check
the existing `lan.conf` file or contact Anthropic support for the current ranges.

### What NOT to do

- Do not add to `cscli allowlists` — this prevents new LAPI decisions but has no effect on
  the local heuristic scorer that is causing the 403.
- Do not restart CrowdSec — the heuristic scores survive service reload and only clear on
  full OpenResty worker rotation (which reload does trigger).
- Do not conclude "CrowdSec is clean" from `cscli decisions list` — the block is below the
  LAPI layer.

---

## Audit Log Paths

| Log | Path | Retention |
|---|---|---|
| Go authoritative audit | `/var/log/mcp-runtime-go/audit.jsonl` | Permanent (set up logrotate) |
| Go shadow audit (historical) | `/var/log/mcp-runtime-go/audit-shadow.jsonl` | Preserved, no longer written |
| OpenResty access log | `/var/log/nginx/mcp-hugo.access.log` | System rotation |
| OpenResty error log | `/var/log/nginx/mcp-hugo.error.log` | System rotation |

**Note:** The audit log is append-only with no built-in rotation. Configure logrotate for
`/var/log/mcp-runtime-go/audit.jsonl` to prevent unbounded growth.

---

## Quick Diagnostics

```bash
# Service health
systemctl status mcp-runtime --no-pager

# Readiness probe (503 = dependency unavailable)
curl -s http://127.0.0.1:8086/readyz

# Internal metrics (Prometheus text format)
curl -s http://127.0.0.1:8086/metrics

# Recent audit events
sudo tail -20 /var/log/mcp-runtime-go/audit.jsonl

# Recent access (last 10 requests)
sudo tail -10 /var/log/nginx/mcp-hugo.access.log | python3 -c "
import sys,json
for l in sys.stdin:
    try:
        d=json.loads(l)
        print(d['time'],d['method'],d['uri'],'status='+str(d['status']),'src='+d['real_ip'])
    except: pass
"

# Check for CrowdSec blocks (LAPI layer — does NOT show heuristic blocks)
sudo cscli decisions list

# Run healthcheck
curl -s http://127.0.0.1:8086/healthz
```
