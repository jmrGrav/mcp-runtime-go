# Phase 4 Cutover Report

**Date:** 2026-06-01
**Operator:** Claude Code (automated)
**Verdict:** CUTOVER BLOCKED

---

## Pre-Cutover Verification Results

### Healthcheck (STEP 1)

```
$ ./scripts/healthcheck-shadow.sh http://127.0.0.1:8085
OK: healthz
OK: readyz
OK: oauth metadata
OK: protected resource metadata
Shadow healthcheck passed for http://127.0.0.1:8085
```

Go shadow service responds correctly to all health endpoints. **PASS.**

### Service Status at Verification Time

| Service | Status |
|---|---|
| `mcp-runtime-shadow.service` | active/running (PID 993705, since 2026-06-01T02:33:00Z) |
| `openresty.service` | active/running (since 2026-05-24, multiple successful reloads) |
| `hugo-mcp-proxy.service` | active/running (Python authoritative, port 8084, PID 1942, since 2026-05-13) |
| `mcp-oauth-proxy.service` | inactive/dead, disabled (Grav proxy, out of scope) |

Python (`hugo-mcp-proxy.service`) remains authoritative. **PASS.**

### Shadow Comparison

```
$ sudo bin/shadow-compare \
    -python /var/log/mcp-oauth/audit-hugo.log \
    -go /var/log/mcp-runtime-go/audit-shadow.jsonl
```

```
Comparing 915 Python entries with 8 Go entries

[CRITICAL] Missing request_id for critical event "token_issued" (multiple — May/June)
[CRITICAL] Missing request_id for critical event "authorize_approved" (multiple)
[CRITICAL] Missing request_id for critical event "client_registered" (multiple)

Summary:
  Matched by ID:       0
  Matched by Fallback: 0
  Mismatches:          0
  Unmatched Python:    915
  Unmatched Go:        8
  Missing ID:          915
  Critical Missing ID: 45
  Duplicate ID:        0
  Ambiguous Fallback:  0

[RESULT] Parity check FAILED
```

---

## Blockers — Three Independent ABSOLUTE RULE Violations

### BLOCKER 1: 48h Shadow Window Not Complete

- **T0:** 2026-06-01T00:33:08Z
- **Window closes:** 2026-06-03T00:33:08Z
- **Elapsed at check:** ~23h (48% of required window)
- **Rule violated:** "Do not cut over if 48h shadow comparison is not clean."

Waiting for 48h alone is **not sufficient** — blockers 2 and 3 must also be resolved.

### BLOCKER 2: Python Audit Log Has No `request_id` Fields

The `hugo-mcp-proxy.service` (Python) does not emit `request_id` in its audit log.
All 915 Python entries have `ts` + `event` but no `request_id`.

The `shadow-compare` tool matches entries exclusively by `request_id`. With 0 Python
IDs, the tool is **structurally incapable** of confirming parity — regardless of
whether Go and Python made identical decisions. The 45 critical missing-ID failures
are not noise; they represent real token issuance and authorization events for which
parity cannot be established.

**Fix required before retry:**
Add `request_id` to Python audit log emission in `mcp_oauth_proxy.py`, or implement
a validated time+event+IP fallback matching method in `shadow-compare` that is
explicitly approved for the parity gate.

### BLOCKER 3: Go Shadow Emits No Audit Entries for Production Mirror Traffic

The Go shadow log has **8 entries total** — all from test/healthcheck calls made at
T0 (02:33:08Z) and this verification run (23:33:27Z). Zero entries from production
traffic.

Python served real authenticated MCP traffic on June 1:

```
07:13 — mcp_forward (3× calls from Claude.ai Anthropic IPs)
10:59 — mcp_forward (3× calls)
12:36 — mcp_forward
13:02 — mcp_forward
19:24 — mcp_forward (3× calls)
23:24 — mcp_forward (3× calls)
```

None of this produced Go shadow audit entries. The shadow is running (healthcheck
passes), but **the mirror traffic is not generating Go audit log output for `/mcp`
POST requests or the OAuth flows that precede them**.

Additional issue found in Go journal: at 02:15:48, Go briefly ran with:
```
[ERROR] audit log open failed: open /var/log/mcp-runtime-go/audit-shadow.jsonl: read-only file system
```
Go restarted at 02:16:30 and again at 02:32:59 (T0). The audit log appears writable
from that point (healthcheck entries exist), but production mirror calls still do not
produce entries.

**Root cause to investigate:**
- Does the Go shadow handler for `/mcp` POST write an audit event? If Go is a
  transparent proxy and only logs OAuth events (authorize, token, register), MCP
  forward calls would not appear. Verify whether Go's `mcp_forward` equivalent
  produces an audit entry.
- Are mirrored requests reaching Go? Confirm via `access_log` or by tailing the Go
  audit log during a live production call.
- The OpenResty mirror for `/mcp` sends to `http://127.0.0.1:8085/mcp/` — confirm
  the Go server handles that path and logs it.

Without Go audit entries for real production traffic, there is **zero usable parity
evidence** even if IDs were present.

---

## Current State Summary

| Check | Result |
|---|---|
| Go healthcheck (healthz, readyz, metadata) | PASS |
| Python still authoritative | PASS |
| OpenResty serving public route | PASS (→ 8084 Python) |
| 48h window elapsed | FAIL — 23h of 48h |
| Shadow comparison | FAIL — 45 critical missing IDs |
| Go audit coverage of production traffic | FAIL — 0 production entries |
| Go crashes | None observed |
| TLS/backend errors | None observed |
| Duplicate request_id in Go log | 0 |
| Malformed JSON in Go log | 0 |

---

## No Changes Made

No OpenResty config was modified. No systemd units were changed. No Python service
was stopped. No production service unit (`mcp-runtime.service`) was created. The
system is in the same state as before this verification run.

Python remains authoritative. Rollback not required.

---

## Must Fix Before Retry

The following must be resolved before re-running the 48h shadow comparison:

1. **Python `request_id`**: Add `request_id` (UUID4, per-request) to all Python
   audit log entries — especially `authorize_approved`, `token_issued`,
   `client_registered`. This requires a `hugo-mcp-proxy.service` restart.
   Restart resets T0 for the new shadow window.

2. **Go `/mcp` audit coverage**: Confirm Go logs an audit event for every mirrored
   `/mcp` POST (or explicitly document that MCP proxy calls are out of scope for
   the shadow comparison). If Go only audits OAuth operations, the comparison
   scope must be narrowed to OAuth events only, and the Python log must contain
   `request_id` for those events.

3. **Mirror validation**: After both fixes above, manually trigger a test OAuth flow
   through the public endpoint and confirm:
   - Python audit log shows event with `request_id`
   - Go shadow audit log shows matching event with same `request_id`
   - `shadow-compare` exits 0 on those entries

4. **Restart T0**: Once fixes are deployed, record a new T0 and run the full 48h
   window before retrying Phase 4.

---

## Rollback Instructions (Unchanged from Phase 3)

No changes were made; rollback is not needed. For reference, rollback procedure is
documented in `docs/ROLLBACK.md`.

If future cutover needs to be rolled back:
```bash
# 1. Restore OpenResty backup (from /usr/local/openresty/nginx/conf/backups/ or
#    /usr/local/openresty/nginx/conf/sites-available/mcp-hugo.arleo.eu.bak-*)
# 2. sudo nginx -t
# 3. sudo systemctl reload openresty
# 4. sudo systemctl stop mcp-runtime.service (if started)
# 5. sudo systemctl start hugo-mcp-proxy.service
# 6. Verify public endpoint returns 200 on /mcp POST with valid token
```

---

## Final Verdict

**CUTOVER BLOCKED**

Three independent violations of ABSOLUTE RULES. No production changes were made.
Python authoritative service (`hugo-mcp-proxy.service`, port 8084) continues to
serve all traffic. Go shadow (`mcp-runtime-shadow.service`, port 8085) continues
in shadow-only mode.

Next action: resolve the three blockers above, establish a new T0, run 48h, then
re-run Phase 4.
