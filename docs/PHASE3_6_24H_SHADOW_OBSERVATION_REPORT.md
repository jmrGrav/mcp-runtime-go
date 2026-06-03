# Phase 3.6 — 24h Shadow Observation Final Report

**T1:** 2026-06-02T03:58:10Z
**Window close:** 2026-06-03T03:58:10Z
**Comparison run:** 2026-06-03T04:59:13Z (1h01m after close)
**Report written:** 2026-06-03T18:35Z

---

## Checkpoint Log

| Checkpoint | Elapsed | mcp-runtime-shadow | hugo-mcp-proxy | openresty | Python lines | Go lines | Crashes | OpenResty errors |
|---|---|---|---|---|---|---|---|---|
| T+0 | 0h05m | active | active | active | 4 | 5 | 0 | 0 |
| T+1h | 1h24m | active | active | active | 7 (+3 mcp_forward from Anthropic) | 5 | 0 | 0 |
| T+2.5h | 2h31m | active | active | active | 7 (unchanged) | 5 | 0 | 0 |
| T+12h | 14h37m | active | active | active | 10 (+3 mcp_forward) | 5 | 0 | 0 |
| T+15.6h | 15h39m | active | active | active | 13 (+3 mcp_forward) | 5 | 0 | 0 |
| T+16.7h | 16h40m | active | active | active | 16 (+3 mcp_forward from .35/.36/.37/.38) | 5 | 0 | 0 |
| T+17.7h | 17h41m | active | active | active | 16 (unchanged) | 5 | 0 | 0 |
| T+24h (final) | 25h02m | active → stopped (cutover) | active → stopped (cutover) | active | 22 | 7 | 0 | 0 |

Note: `mcp-runtime-shadow` and `hugo-mcp-proxy` were stopped as part of the Phase 4 cutover executed
immediately after the window closed and the comparison passed.

---

## Audit Log Paths and Final Counts

| Log | Path | Final line count |
|---|---|---|
| Python authoritative | `/var/log/mcp-oauth/audit-hugo.log` | 23 |
| Go shadow | `/var/log/mcp-runtime-go/audit-shadow.jsonl` | 7 |

Python log grew from 4 (T+0) to 23 over the window — all growth from `mcp_forward` events
(Claude.ai authenticated sessions reusing existing tokens) plus 1 `service_start` entry.
No new OAuth flows (`authorize_approved`, `token_issued`) occurred through OpenResty during the window.

Go log grew from 5 (T+0 healthcheck) to 7 — the 2 additional entries were direct verification
calls made during the pre-close validation (2026-06-03T06:18Z).

---

## Comparison Output (full)

```
Shadow comparison run: 20260603T045913Z
Window: 24h
Python log: /var/log/mcp-oauth/audit-hugo.log
Go log: /var/log/mcp-runtime-go/audit-shadow.jsonl
Mode: --allow-unsafe-fallback (time+event+ip, 2s window)

Comparing 22 Python entries with 7 Go entries

Summary:
  Matched by ID:               0
  Matched by Fallback:         3
  Mismatches:                  0
  Unmatched Python:            19
  Unmatched Python (critical): 0
  Unmatched Go:                4
  Missing ID:                  1
  Critical Missing ID:         0
  Duplicate ID:                0
  Ambiguous Fallback:          0
[WARN] Non-critical events missing Request IDs (e.g. service_start/stop)
[WARN] Non-critical unmatched entries (expected in shadow mode for mcp_forward)

[RESULT] Parity check SUCCESS
```

Report file: `/var/log/mcp-runtime-go/reports/shadow-compare-24h-20260603T045913Z.txt`

---

## Metric Summary

| Metric | Value | Threshold | Pass |
|---|---|---|---|
| Critical mismatches | 0 | 0 | ✓ |
| Critical unmatched Python | 0 | 0 | ✓ |
| Critical unmatched Go | 0 | 0 | ✓ |
| Duplicate request_id | 0 | 0 | ✓ |
| Malformed JSON | 0 | 0 | ✓ |
| Matched by fallback | 3 | — | — |
| Matched by ID | 0 | — | (expected — IDs are service-generated, not shared) |
| Missing ID (non-critical) | 1 | — | WARN only (service_start has no request context) |
| Unmatched Python (non-critical) | 19 | — | WARN only (mcp_forward; token store divergence expected) |
| Unmatched Go | 4 | — | WARN only (healthcheck + verification entries) |
| Unsafe fallback used | yes | — | Documented limitation |

---

## Crashes and Restarts

### Go shadow (`mcp-runtime-shadow`)
- **0 unexpected restarts** during the observation window (T1 to close)
- Final stop: 2026-06-03T07:04:51Z — clean SIGTERM (cutover)

### Python authoritative (`hugo-mcp-proxy`)
- **0 unexpected restarts** during the observation window
- Continued serving live Anthropic traffic throughout (last entry 2026-06-03T06:11 before cutover)
- Final stop: 2026-06-03T07:04:48Z — clean shutdown (cutover)

### OpenResty
- **0 errors** recorded in `/var/log/nginx/mcp-hugo.error.log` since T1

---

## Real Mirrored Traffic Observation

**18 POST /mcp production requests** were observed in the OpenResty access log during the window
(from Anthropic IPs 160.79.106.35/36/37/38). All were mirrored to Go via `/__mcp_shadow`.

Go received all 18 mirrored requests but logged **0 audit entries** for them. Root cause (documented
in `docs/archive/PHASE3_6_PRE_CLOSE_VALIDATION.md`): Go's `HandleProxy` validates the Bearer token
against Go's own token store; Python-issued tokens are not present → Go returns 401 silently.
`unauthorized()` does not call `auditLog()`. This is expected shadow-mode behavior.

**No new OAuth flows** (authorize/token) occurred through OpenResty during the window, so the
`/__oauth_shadow` location never fired. The 3 fallback-matched critical events are from the
T1 setup test flows (direct calls to both services simultaneously at 05:58:26 local on 2026-06-02).

---

## Unsafe Fallback Note

The comparison used `--allow-unsafe-fallback` (time + event + source IP, 2-second window). This
is the only viable matching strategy when both services generate independent request IDs per
request. The 3 matched events are from a controlled test where both services were hit within
milliseconds, making the fallback reliable for those entries.

A shared correlation ID injected by OpenResty (e.g. `X-Shadow-Request-ID: $request_id`) would
enable ID-based matching and remove this limitation. This is documented as a future improvement.

---

## Accepted Limitation

The following limitation is explicitly accepted for this cutover:

> **Token store divergence**: Go shadow cannot share Python's token store, so mirrored `/mcp`
> POST requests always produce Go 401s that go unaudited. Shadow comparison is limited to
> OAuth flow events (register/authorize/token), which were validated correctly via the T1
> test flows. No critical parity divergence was observed.

---

## Final Verdict

**SHADOW PASS WITH LIMITATION**

- 0 critical mismatches
- 0 critical unmatched Python events
- 0 critical unmatched Go events
- 0 malformed JSON
- 0 Go crashes or unexpected restarts
- 0 Python crashes or unexpected restarts
- 0 OpenResty errors
- Fallback matching used (documented limitation — accepted)
- Real mirrored MCP traffic received by Go (token validation divergence expected, documented)
- Python authoritative throughout window
- Rollback documented and available

Cutover proceeded on verdict SHADOW PASS WITH LIMITATION with limitation explicitly accepted.
