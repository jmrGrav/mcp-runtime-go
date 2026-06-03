# Phase 4 Cutover — Pre-Change Backup Record

**Date:** 2026-06-03T07:03:47Z
**Operator:** automated cutover

## Files Backed Up

| Original | Backup Path | Hash |
|---|---|---|
| `/usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu` | `/usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-cutover-20260603-070347` | — |
| `/etc/mcp-runtime-go/mcp-runtime.env` | `/etc/mcp-runtime-go/mcp-runtime.env.bak-20260603-070347` | — |

## Active Upstream Before Cutover

| Service | Host | Port | Role |
|---|---|---|---|
| `hugo-mcp-proxy.service` (Python) | 127.0.0.1 | 8084 | **Public authoritative** |
| `mcp-runtime-shadow.service` (Go) | 127.0.0.1 | 8085 | Shadow (mirror only) |
| `mcp-runtime.service` (Go) | 127.0.0.1 | 8086 | Not yet routed (pre-cutover) |

## Rollback Commands

```bash
# 1. Restore OpenResty config
sudo cp /usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-cutover-20260603-070347 \
  /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu

# 2. Test config
sudo /usr/local/openresty/nginx/sbin/nginx -t

# 3. Reload if test passes
sudo systemctl reload openresty

# 4. Start Python
sudo systemctl start hugo-mcp-proxy.service

# 5. Stop Go authoritative if needed
sudo systemctl stop mcp-runtime.service

# 6. Verify public endpoint
curl -si https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server | head -3
```
