# Phase 4 Cutover Report

**Date:** 2026-06-03
**Verdict:** GO LIVE COMPLETE

---

## Shadow Gate Result

See [PHASE3_6_24H_SHADOW_OBSERVATION_REPORT.md](PHASE3_6_24H_SHADOW_OBSERVATION_REPORT.md)
for the full shadow comparison.

| Metric | Value |
|---|---|
| Verdict | **SHADOW PASS WITH LIMITATION** |
| Critical mismatches | 0 |
| Critical unmatched Python | 0 |
| Critical unmatched Go | 0 |
| Malformed JSON | 0 |
| Go crashes | 0 |
| Python crashes | 0 |
| OpenResty errors | 0 |
| Fallback used | yes — documented and accepted |

**Limitation accepted:** Token store divergence in shadow mode means Go's `/mcp` proxy
rejections go unaudited. OAuth flow parity (the critical path) was validated via T1 test
flows with 0 mismatches.

---

## Commands Executed

```bash
# Phase A — comparison
sudo /home/jm/mcp-runtime-go/scripts/shadow-compare-48h.sh \
  /var/log/mcp-oauth/audit-hugo.log \
  /var/log/mcp-runtime-go/audit-shadow.jsonl \
  /var/log/mcp-runtime-go/reports 24

# Phase B — Go authoritative service
sudo systemctl daemon-reload
sudo systemctl enable mcp-runtime
sudo systemctl start mcp-runtime

# Phase C — backups (timestamp 20260603-070347)
# OpenResty: /usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-cutover-20260603-070347
# Go env:    /etc/mcp-runtime-go/mcp-runtime.env.bak-20260603-070347

# Phase D — OpenResty switch
sudo /usr/local/openresty/nginx/sbin/nginx -t   # PASS
sudo systemctl reload openresty

# Phase E — verification (public endpoint)
curl -si https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server   # 200
curl -si https://mcp-hugo.arleo.eu/.well-known/oauth-protected-resource     # 200
curl -si https://mcp-hugo.arleo.eu/mcp -X POST -H "Content-Type: application/json" -d '{}'  # 401 + WWW-Authenticate
curl -si "https://mcp-hugo.arleo.eu/authorize?...&client_id=bad"            # 400
curl -si -X POST https://mcp-hugo.arleo.eu/token -d "...client_id=bad"     # 401

# Phase F — stop Python and shadow
sudo systemctl stop hugo-mcp-proxy.service
sudo systemctl disable hugo-mcp-proxy.service
sudo systemctl stop mcp-runtime-shadow.service
sudo systemctl disable mcp-runtime-shadow.service
```

---

## Files Changed

| File | Change |
|---|---|
| `/usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu` | Removed mirror directives, shadow locations; proxy_pass → 8086 |
| `/etc/systemd/system/mcp-runtime.service` | Created — Go authoritative service unit |
| `/etc/mcp-runtime-go/mcp-runtime.env` | Created — production env (SHADOW_MODE=false, port 8086, correct PROXY_BASE_URL) |

---

## Backups Created

| Backup | Path |
|---|---|
| OpenResty config (pre-cutover) | `/usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-cutover-20260603-070347` |
| Go production env | `/etc/mcp-runtime-go/mcp-runtime.env.bak-20260603-070347` |

---

## OpenResty Validation

```
nginx -t: OK
systemctl reload openresty: OK
```

No merge commits. Config switched from Python 8084 (+ shadow 8085 mirror) to Go 8086 (direct, no mirror).

---

## Service Status at Cutover Completion

| Service | Status | Notes |
|---|---|---|
| `mcp-runtime.service` (Go) | **active/running** since 2026-06-03T07:03:21 CEST | Port 8086, SHADOW_MODE=false |
| `openresty.service` | **active/running** | Routes all public traffic to Go 8086 |
| `hugo-mcp-proxy.service` (Python) | **inactive/disabled** | Stopped 2026-06-03T07:04:48 — clean shutdown |
| `mcp-runtime-shadow.service` (Go shadow) | **inactive/disabled** | Stopped 2026-06-03T07:04:51 — clean SIGTERM |

Python files: preserved at `/opt/mcp-oauth-proxy/` — not deleted.

---

## Public Endpoint Checks

| Endpoint | Expected | Result |
|---|---|---|
| `GET /.well-known/oauth-authorization-server` | 200, issuer=mcp-hugo.arleo.eu | ✓ 200, correct issuer |
| `GET /.well-known/oauth-protected-resource` | 200 | ✓ 200 |
| `POST /mcp` (no auth) | 401 + WWW-Authenticate | ✓ 401, header present |
| `GET /authorize?client_id=bad` | 400 | ✓ 400 |
| `POST /token` (bad client) | 401 | ✓ 401 |

---

## Audit Log Proof

Go authoritative log (`/var/log/mcp-runtime-go/audit.jsonl`) received live production entries
during and after cutover:

```json
{"event":"metadata_served","request_id":"771583b9b1f678f38cd9c6039a4080ec","src_ip":"127.0.0.1","ts":"2026-06-03T07:04:21+0200","ua":"curl/8.5.0"}
{"event":"resource_metadata_served","request_id":"5ce2b2d807b544def2b1f7d51989a9a2","src_ip":"127.0.0.1","ts":"2026-06-03T07:04:21+0200","ua":"curl/8.5.0"}
{"event":"authorize_rejected","reason":"invalid client_id","request_id":"a6a24fa4f0c944bac11f47ac29a381bf","src_ip":"127.0.0.1","ts":"2026-06-03T07:04:30+0200","ua":"curl/8.5.0"}
{"event":"token_rejected","reason":"client_auth_failed","request_id":"bfae2ecbae2416b2fe3e5c3c0728acf7","src_ip":"127.0.0.1","ts":"2026-06-03T07:04:30+0200","ua":"curl/8.5.0"}
{"event":"resource_metadata_served","request_id":"a55c3cb406a38a61321e36acf9ae2865","src_ip":"127.0.0.1","ts":"2026-06-03T20:35:58+0200","ua":"python-httpx/0.28.1"}
{"event":"metadata_served","request_id":"2348ffb003da04a1553a57f5a138ffd2","src_ip":"127.0.0.1","ts":"2026-06-03T20:35:58+0200","ua":"python-httpx/0.28.1"}
```

The `python-httpx/0.28.1` entry at 20:35:58 is a Claude.ai OAuth discovery request — live
production traffic hitting Go 13h after cutover.

Python audit log stopped receiving authoritative entries at 2026-06-03T06:11 (last `mcp_forward`
before cutover routing change).

---

## Rollback Procedure

```bash
# 1. Restore OpenResty config
sudo cp \
  /usr/local/openresty/nginx/conf/backups/mcp-hugo.arleo.eu.pre-cutover-20260603-070347 \
  /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu

# 2. Validate
sudo /usr/local/openresty/nginx/sbin/nginx -t

# 3. Reload if test passes
sudo systemctl reload openresty

# 4. Start Python
sudo systemctl start hugo-mcp-proxy.service

# 5. Stop Go authoritative if needed
sudo systemctl stop mcp-runtime.service

# 6. Verify
curl -si https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server | head -3
```

---

## Final Verdict

**GO LIVE COMPLETE**

- Public route → Go authoritative (port 8086, SHADOW_MODE=false)
- Python stopped only after Go verified live
- Rollback documented and backup present
- No critical parity mismatches
- No service health failures
- Live Claude.ai traffic confirmed in Go audit log
