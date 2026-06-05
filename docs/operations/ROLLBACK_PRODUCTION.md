# Production Rollback — Go → Python

**Goal:** Return authoritative traffic to the Python service (`hugo-mcp-proxy.service`) within
60 seconds, without data loss.

**When to use:** Go runtime (`mcp-runtime.service`) is failing, crashing, or producing
wrong results. Python service has been verified to be on disk and stopped (not deleted).

---

## Pre-checks (30 seconds)

```bash
# Confirm Python service exists and is stopped (not deleted)
systemctl status hugo-mcp-proxy.service --no-pager

# Confirm the OpenResty pre-cutover backup config exists
ls -la /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu.pre-cutover-*
```

If the Python service unit is missing or the backup config is absent, **stop here** and
escalate — the rollback prerequisites are not met.

---

## Rollback Procedure

### Step 1: Activate the Python service

```bash
sudo systemctl start hugo-mcp-proxy.service
```

Verify it is running:

```bash
systemctl status hugo-mcp-proxy.service --no-pager
```

### Step 2: Revert the OpenResty routing config

Identify the pre-cutover backup (the most recent `.pre-cutover-*` file):

```bash
BACKUP=$(ls -t /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu.pre-cutover-* | head -1)
echo "Will restore: $BACKUP"
```

Copy it back:

```bash
sudo cp "$BACKUP" /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu
```

### Step 3: Test and reload OpenResty

```bash
sudo /usr/local/openresty/nginx/sbin/nginx -t
sudo systemctl reload openresty
```

If `nginx -t` fails: the backup config is corrupted. Do NOT reload. Restore the file manually
from git (`deploy/nginx/`) or contact the operator.

### Step 4: Verify traffic is flowing to Python

```bash
# Health check via loopback — Python service should return 200
curl -s http://127.0.0.1:8084/healthz

# Check last access log line — port 8084 should appear
sudo tail -3 /var/log/nginx/mcp-hugo.access.log
```

### Step 5: Stop the Go runtime

```bash
sudo systemctl stop mcp-runtime.service
```

---

## Post-Rollback

1. Capture audit logs before they rotate:
   ```bash
   sudo cp /var/log/mcp-runtime-go/audit.jsonl /tmp/audit-$(date +%Y%m%d-%H%M%S).jsonl
   ```

2. File a post-mortem with the last 50 lines of the Go audit log and the `/metrics` snapshot
   taken before the stop:
   ```bash
   curl -s http://127.0.0.1:8086/metrics > /tmp/metrics-snapshot.txt  # before stopping
   sudo tail -50 /var/log/mcp-runtime-go/audit.jsonl
   ```

3. Do NOT delete the SQLite token store. Re-migration from Python back to Go is possible via
   `mcp-runtime migrate-storage` once the root cause is resolved.

---

## Token continuity

- Python uses a separate in-memory token store (no shared state with Go).
- Any tokens issued by Go while authoritative will be **invalid** under Python.
- Claude.ai will re-authenticate automatically on the next request (token expiry → re-auth flow).
- No user data is lost; at most one MCP session must be re-established.

---

## Re-cutover to Go (after fix)

Once the root cause is resolved and the Go binary is updated:

```bash
# Verify Go runtime is healthy
curl -s http://127.0.0.1:8086/readyz   # must return 200

# Restore Go routing config
sudo cp /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu.go-live \
     /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu
sudo /usr/local/openresty/nginx/sbin/nginx -t
sudo systemctl reload openresty

# Stop Python (now idle)
sudo systemctl stop hugo-mcp-proxy.service
```
