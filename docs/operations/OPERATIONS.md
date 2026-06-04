# Operations Reference

## Current Go Live State

Go (`mcp-runtime.service`) is authoritative for all MCP and OAuth traffic on `mcp-hugo.arleo.eu`
as of 2026-06-03. The Python implementation (`<PYTHON_SERVICE>`) is preserved on disk as a
rollback reference but is stopped and disabled.

### Service topology

| Service | Port | State | Role |
|---|---|---|---|
| `mcp-runtime.service` | `<GO_PORT>` | **active/enabled** | Authoritative Go runtime |
| `mcp-runtime-shadow.service` | 8085 | disabled | Shadow mode — no longer used |
| `<PYTHON_SERVICE>` | 8084 | disabled | Preserved as rollback reference |

### Routing

```
Public internet
    → Cloudflare
    → OpenResty (mcp-hugo.arleo.eu)
    → Go mcp-runtime 127.0.0.1:<GO_PORT>
    → Grav MCP backend (internal)
```

### Endpoint access matrix

| Endpoint | Who can reach it | Notes |
|---|---|---|
| `POST /mcp` | `<ANTHROPIC_IP_RANGE>` | Requires valid Bearer token; unauthenticated returns 401 + WWW-Authenticate |
| `GET /.well-known/oauth-authorization-server` | `<ANTHROPIC_IP_RANGE>` + `<OPERATOR_IP>` | OAuth metadata discovery |
| `GET /.well-known/oauth-protected-resource` | `<ANTHROPIC_IP_RANGE>` + `<OPERATOR_IP>` | Protected resource metadata |
| `POST /register` | `<ANTHROPIC_IP_RANGE>` + `<OPERATOR_IP>` | RFC 7591 dynamic client registration — open |
| `GET /authorize` | **`<OPERATOR_IP>` only** | Single-tenant: only the operator's browser completes login |
| `POST /token` | `<ANTHROPIC_IP_RANGE>` | Token exchange after successful authorize |

### Single-tenant authorization

`/authorize` is restricted to `<OPERATOR_IP>` via a dedicated nginx `location = /authorize`
block with `allow <OPERATOR_IP>; deny all;`. This means:

- Any Claude.ai account can discover the MCP and register a client.
- Only requests from the operator's IP can complete the OAuth login step.
- Anyone else attempting to connect via Claude.ai receives 403 at the authorize step.

### Rollback

OpenResty config backup: `<ROLLBACK_BACKUP_PATH>`

To roll back:
```bash
sudo cp <ROLLBACK_BACKUP_PATH> \
  /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu
sudo /usr/local/openresty/nginx/sbin/nginx -t
sudo systemctl reload openresty
sudo systemctl start <PYTHON_SERVICE>
```

Python files are never deleted. The rollback restores traffic to the Python service without
data loss.

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

**Do not** add `<ANTHROPIC_IP_RANGE>` to the public config or documentation. The actual range
is maintained in the local nginx configuration.

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
| Go authoritative audit | `/var/log/mcp-runtime-go/audit.jsonl` | Permanent |
| Go shadow audit (historical) | `/var/log/mcp-runtime-go/audit-shadow.jsonl` | Preserved |
| OpenResty access log | `/var/log/nginx/mcp-hugo.access.log` | System rotation |
| OpenResty error log | `/var/log/nginx/mcp-hugo.error.log` | System rotation |

---

## Quick Diagnostics

```bash
# Service health
systemctl status mcp-runtime --no-pager

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

# Check for CrowdSec blocks (LAPI layer)
sudo cscli decisions list

# Run healthcheck
/home/jm/mcp-runtime-go/scripts/healthcheck-shadow.sh http://127.0.0.1:<GO_PORT>
```
