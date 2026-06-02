# Phase 3.4 Shadow Monitoring

## T0

2026-06-01T00:33:08Z

## Verdict

SHADOW HEALTHY

## Checks Performed

- Verified `mcp-runtime-shadow.service` status.
- Verified `openresty.service` status.
- Checked Go audit log growth and contents.
- Confirmed Python authoritative path is still serving public responses.
- Checked for Go crashes and repeated backend/TLS errors.
- Confirmed no OpenResty reload failures or error-log spikes during this observation window.

## Observation Window Status

- T+1h: not due yet at the time of this check.
- T+6h: not due yet.
- T+24h: not due yet.
- T+48h: not due yet.

## Service Status

- `mcp-runtime-shadow.service`: active/running, no crash observed.
- `openresty.service`: active/running.
- Python authoritative service remains in place and unchanged.

## Go Audit Log

- Path: `/var/log/mcp-runtime-go/audit-shadow.jsonl`
- Size: `1033` bytes
- Lines: `6`
- Last observed events:
  - `metadata_served`
  - `resource_metadata_served`

## Python Log / Public Path

- Public MCP responses continued to return the expected authoritative behavior.
- Latest public access log entries include the mirrored OAuth checks and earlier `/mcp` traffic.
- No evidence of Go replacing the public authoritative backend.

## Mismatches / Unmatched Events / Integrity Checks

- No mismatch comparison was run yet because the 48h window is not complete.
- No duplicate `request_id` was observed in the newly added Go audit lines.
- No missing `request_id` was observed in the newly added Go audit lines.
- No malformed JSON was observed in the Go audit log tail checked during this window.

## OpenResty Reloads / Errors

- Reloads were observed and succeeded previously during mirror enablement.
- No reload failures were observed during this monitoring check.
- No new OpenResty error-log burst was observed during this monitoring check.

## Go Restarts

- No unexpected Go restarts were observed.
- The service remained stable after the mirror expansion and audit observability changes.

## Final Comparison

- Not due yet.
- `scripts/shadow-compare-48h.sh` should be run when the 48h window closes.

## Notes

- This report is observation-only.
- No code, OpenResty, or Python service changes were made in this monitoring turn.
