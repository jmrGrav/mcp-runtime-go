# SQLite Migration Execution Report — v1.3

**Date:** 2026-06-06
**Operator:** Post-deployment SQLite migration run
**Base commit:** `bf0fecd6a57d6fa0f94ae7e582e77e3ee8219dd5` (main)
**Migration window:** 06:38–06:40 CEST (service downtime ~36s)

---

## Objective

Migrate token store from JSON (`tokens.json`) to SQLite WAL (`tokens.db`).
Constraint: `TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db` — never `/opt/mcp-oauth-proxy/tokens.db`.

---

## Pre-Migration State

| Item | Value |
|---|---|
| Service state | `active (running)` — PID 2745310 (since 2026-06-05 22:54 CEST) |
| `tokens.json` | 84 bytes — **1 token** |
| `tokens.db` | absent |
| `USE_SQLITE` | `false` |
| `TOKENS_FILE` | `/var/lib/mcp-runtime-go/tokens.json` |
| `TOKENS_DB` | not set in env file |

---

## Steps Executed

### Step 1 — Pre-check ✅
Confirmed: 1 active token in `tokens.json`, no `tokens.db`, service healthy.

### Step 2 — Backups ✅
```
/etc/mcp-runtime-go/mcp-runtime.env.pre-sqlite.20260606-063837
/var/lib/mcp-runtime-go/tokens.json.pre-sqlite.20260606-063918
```

### Step 3 — Service stopped ✅
```
sudo systemctl stop mcp-runtime.service
→ inactive
```

### Step 4 — Migration ✅
```bash
sudo -u mcp-runtime bash -c \
  'set -a; source /etc/mcp-runtime-go/mcp-runtime.env; \
   TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db; \
   /usr/local/bin/mcp-runtime migrate-storage'
```
Output:
```
[INFO] starting storage migration: /var/lib/mcp-runtime-go/tokens.json -> /var/lib/mcp-runtime-go/tokens.db
[INFO] migration successful
```

Post-migration filesystem:
```
-rw-r--r-- mcp-runtime  16384  tokens.db          (SQLite 3.x, WAL, version 3053001)
-rw------- mcp-runtime     84  tokens.json.migrated  (original renamed — not deleted)
-rw------- root            84  tokens.json.pre-sqlite.20260606-063918  (backup copy)
```

Token count in SQLite: **1** (matches pre-migration JSON count ✅)

### Step 5 — Env file updated ✅

Changes made:
```diff
-USE_SQLITE=false
+USE_SQLITE=true
+TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db
```

`TOKENS_FILE` retained (legacy fallback, service ignores it when `USE_SQLITE=true`).

### Step 6 — Service started ✅
```
sudo systemctl start mcp-runtime.service
→ active (running) since 2026-06-06 06:40:23 CEST — PID 2838611
```

Startup log (key lines):
```json
{"level":"INFO","msg":"token store: SQLite WAL","path":"/var/lib/mcp-runtime-go/tokens.db"}
{"level":"INFO","msg":"server starting","addr":"127.0.0.1:8086"}
```

No WARN JSON store. No startup errors.

### Step 7 — Health checks ✅
```
GET /healthz  → 200 OK
GET /readyz   → 200 OK
GET /metrics  → 200 OK
```

All 7 Prometheus counters at 0:
| Counter | Value |
|---|---|
| `mcp_audit_write_failures_total` | 0 |
| `mcp_token_persistence_failures_total` | 0 |
| `mcp_proxy_requests_total` | 0 |
| `mcp_proxy_errors_total` | 0 |
| `mcp_tokens_issued_total` | 0 |
| `mcp_tokens_rejected_total` | 0 |
| `mcp_readiness_failures_total` | 0 |

### Step 8 — Token count verified ✅
```sql
SELECT COUNT(*) FROM access_tokens;  -- → 1
```
Matches pre-migration count.

### Step 9 — Audit log clean ✅
Last 5 entries: `authorize_approved`, `token_issued`, then 3× `proxy_hit` (status 200).
No `authorize_rejected`, `token_rejected`, or `proxy_error` events.

---

## Post-Migration State

| Item | Value |
|---|---|
| Service state | `active (running)` — PID 2838611 |
| Token store | SQLite WAL — `/var/lib/mcp-runtime-go/tokens.db` |
| Token count | 1 (Claude.ai session preserved) |
| `USE_SQLITE` | `true` |
| `TOKENS_DB` | `/var/lib/mcp-runtime-go/tokens.db` |
| JSON store | renamed to `tokens.json.migrated` (not deleted) |
| Startup WARN | none |

---

## Env File Post-Migration (anonymized)

```
SHADOW_MODE=false
LISTEN_HOST=127.0.0.1
LISTEN_PORT=8086
HUGO_MCP_URL=https://<BACKEND_IP>:8000/mcp
PROXY_BASE_URL=https://mcp-hugo.arleo.eu
HUGO_HOST=<BACKEND_IP>
HUGO_TOKEN=[REDACTED]
CLIENT_ID=hugo-mcp
CLIENT_SECRET=[REDACTED]
MCP_CA_CERT=/etc/hugo-mcp/vm-ca.crt
TOKENS_FILE=/var/lib/mcp-runtime-go/tokens.json
TOKENS_DB=/var/lib/mcp-runtime-go/tokens.db
USE_SQLITE=true
AUDIT_LOG_FILE=/var/log/mcp-runtime-go/audit.jsonl
TRUSTED_PROXIES=127.0.0.1,::1
MANDATORY_PKCE=true
ALLOW_TOKEN_STORE_RECOVERY=false
LOG_LEVEL=info
```

---

## Rollback Procedure (not used — preserved for reference)

If the service fails to start or `readyz` returns non-200 after the migration:

```bash
# 1. Stop service
sudo systemctl stop mcp-runtime.service

# 2. Restore env file
sudo cp /etc/mcp-runtime-go/mcp-runtime.env.pre-sqlite.20260606-063837 \
        /etc/mcp-runtime-go/mcp-runtime.env

# 3. Restore tokens.json from backup
sudo cp /var/lib/mcp-runtime-go/tokens.json.pre-sqlite.20260606-063918 \
        /var/lib/mcp-runtime-go/tokens.json
sudo chown mcp-runtime:mcp-runtime /var/lib/mcp-runtime-go/tokens.json
sudo chmod 600 /var/lib/mcp-runtime-go/tokens.json

# 4. Start service
sudo systemctl start mcp-runtime.service
# Expect: "token store: JSON (legacy)" WARN — acceptable for rollback
```

---

## Remaining Operational Notes

| Priority | Item | Action |
|---|---|---|
| 🟡 LOW | `tokens.json.migrated` on disk | Can be deleted after 2 weeks of stable SQLite operation |
| 🟡 LOW | `TOKENS_FILE` in env still set | Harmless (ignored when `USE_SQLITE=true`); remove in future cleanup |
| 🟡 LOW | Audit log rotation | Configure `logrotate` for `/var/log/mcp-runtime-go/audit.jsonl` |
| 🟢 INFO | Prometheus counters reset at restart | Expected — counters are in-memory, not persisted |

---

## Verdict

```
╔═══════════════════════════════════════════════════╗
║                                                   ║
║   ✅  SQLite MIGRATION SUCCESSFUL                 ║
║                                                   ║
║  Token store: JSON → SQLite WAL                   ║
║  Downtime: ~36 seconds                            ║
║  Tokens preserved: 1 (Claude.ai session intact)   ║
║  All health checks: green                         ║
║  No data loss. No credential exposure.            ║
║                                                   ║
╚═══════════════════════════════════════════════════╝
```
