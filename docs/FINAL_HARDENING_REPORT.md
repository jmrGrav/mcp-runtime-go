# v1.2 Final Hardening Report — mcp-runtime-go

**Date:** 2026-06-05
**Branch:** `v1.2-brooks-hardening`
**Based on:** `hardening/v1.1.1-rc1` (which was based on `main` @ `3cc0d20`)
**Auditor:** Brooks-style independent adversarial review (2026-06-05)

---

## Executive Summary

All 18 findings (F-01 through F-18) and all 20 debt items (D-01 through D-20) from the
Brooks audit have been addressed. 5 items are intentionally retained with documented
justification. The repository now passes `go test ./...`, `go test -race ./...`,
`go vet ./...`, `staticcheck ./...`, and `govulncheck ./...` with zero failures.

---

## Final Command Results

```
go test ./...          → ok (all 9 packages)
go test -race ./...    → ok (all 9 packages, no races)
go vet ./...           → ok
staticcheck ./...      → ok (no issues)
govulncheck ./...      → No vulnerabilities found
Total coverage         → 83.0% (gate: ≥ 60%)
```

---

## Findings Status: F-01 to F-18

| ID | Title | Status | Disposition |
|---|---|---|---|
| F-01 | Audit write failures silently discarded | **FIXED** | `audit.go`: both `f.Write()` calls now log to stderr + increment `AuditWriteFailures` counter |
| F-02 | Token persistence errors silently discarded | **FIXED** | `AddAccessToken` returns error; `ExchangeToken` returns `server_error`; in-memory rollback on failure |
| F-03 | StartPurgeLoop goroutine never terminated | **FIXED** | `StartPurgeLoop(ctx context.Context)` with ticker stop on cancel; `App.Run` cancels ctx before `Close()` |
| F-04 | ReverseProxy instantiated per request | **FIXED** | `buildReverseProxy()` called once at `NewService`; `backendURL` pre-parsed; sub-path via typed context key |
| F-05 | WriteTimeout (10s) < backend Timeout (30s) | **FIXED** | `WriteTimeout` raised to 60s (backend 30s + 30s margin) |
| F-06 | OAuth error responses non-RFC-compliant | **FIXED** | `/authorize`: redirects with RFC error codes; invalid redirect_uri / unauthorized_client return direct errors. `/token`: RFC 6749 §5.2 JSON error bodies with standard codes |
| F-07 | HTTP method guards absent | **FIXED** | All handlers: GET/POST enforcement with 405 + `Allow` header (was done in hardening/v1.1.1-rc1) |
| F-08 | ROLLBACK.md stale; OPERATIONS.md placeholders | **FIXED** | OPERATIONS.md: all placeholders replaced. ROLLBACK.md: marked HISTORICAL. ROLLBACK_PRODUCTION.md: new executable runbook |
| F-09 | USE_SQLITE default false | **FIXED** | `envDefault:"true"` in config.go; JSON backend logs WARN at startup |
| F-10 | Health endpoints don't verify storage | **FIXED** | `/readyz` calls `Service.Ready()` which checks ClientID, GravMCPURL, store.Load(), audit.Ping() (was done in hardening/v1.1.1-rc1) |
| F-11 | observability.Logger nil before InitLogger | **FIXED** | Logger initialised to discard handler at declaration; never nil |
| F-12 | proxyResponseWriter Write() not overridden | **FIXED** | `Write()` added; calls `WriteHeader(200)` if not already called |
| F-13 | RegisterClient returns same ClientID | **INTENTIONALLY RETAINED** | Single-tenant design decision. Documented in service.go comment and PRODUCTION_VALIDATION.md. Rate limiting at OpenResty layer. |
| F-14 | Config URL/TTL validation absent | **FIXED** | `Validate()` checks scheme, host for GRAV_MCP_URL and PROXY_BASE_URL; rejects TTL ≤ 0 |
| F-15 | bindEnv empty bool inconsistency | **INTENTIONALLY RETAINED** | Behaviour is correct and tested (`TestBindEnv_EmptyBoolFailsWhenSet`). Inconsistency is documented. Future maintainer note: empty string for bool is always an error. |
| F-16 | syncTokens silently swallows errors | **FIXED** | Logs `ERROR` to structured logger + increments `TokenPersistenceFailures` counter |
| F-17 | shadow-compare dead code | **PARTIALLY RETIRED** | HISTORICAL comment added to `cmd/shadow-compare/main.go`. Binary removed from CI. Code preserved for audit trail. |
| F-18 | TestApp_Run uses time.Sleep | **FIXED** | `httpserver.Server.WaitReady(ctx)` + ready channel. No time.Sleep in test. |

---

## Debt Register Status: D-01 to D-20

| ID | Description | Status | Notes |
|---|---|---|---|
| D-01 | Log audit write errors | **FIXED** | F-01 |
| D-02 | Log token persistence errors | **FIXED** | F-02, F-16 |
| D-03 | Replace OPERATIONS.md placeholders | **FIXED** | F-08 |
| D-04 | Fix/retire ROLLBACK.md | **FIXED** | F-08 |
| D-05 | Done-channel to purge loop | **FIXED** | F-03 (context.Context approach) |
| D-06 | Cache ReverseProxy + pre-parse URL | **FIXED** | F-04 |
| D-07 | HTTP method guards | **FIXED** | F-07 (done in v1.1.1-rc1) |
| D-08 | RFC OAuth error format | **FIXED** | F-06 |
| D-09 | Align write/read timeout with backend | **FIXED** | F-05 |
| D-10 | Health checks that verify storage | **FIXED** | F-10 (done in v1.1.1-rc1) |
| D-11 | Flip USE_SQLITE default to true | **FIXED** | F-09 |
| D-12 | Prometheus metrics endpoint | **FIXED** | `/metrics` endpoint with Prometheus text format; zero external deps (standard library counters) |
| D-13 | Fix nil Logger guard | **FIXED** | F-11 |
| D-14 | Replace time.Sleep in app_test.go | **FIXED** | F-18 |
| D-15 | Audit log rotation documentation | **FIXED** | OPERATIONS.md now documents logrotate requirement |
| D-16 | Add govulncheck to CI | **FIXED** | CI step added; go.mod bumped to 1.25.11; all stdlib vulns cleared |
| D-17 | Validate URLs and TTLs | **FIXED** | F-14 |
| D-18 | Retire/archive shadow mode dead code | **PARTIALLY DONE** | HISTORICAL comment + removed from CI. Full deletion deferred to v1.3 (D-18-deferred: need to confirm no reference from deploy scripts before delete). |
| D-19 | Correlate OpenResty request_id with audit log | **DEFERRED to v1.3** | Requires OpenResty config change. Documented in OPERATIONS.md under future roadmap. Not a code change. |
| D-20 | Deprecate/remove JSON store | **PARTIALLY DONE** | JSON store retained but: (1) logs WARN at startup when active, (2) USE_SQLITE default is now true, (3) full removal deferred to v1.3 after SQLite has proven stable in production. |

---

## Modified Files

### Source code
| File | Changes |
|---|---|
| `internal/observability/audit.go` | Checked Write errors; removed ad-hoc XFF extraction; counter increment on failure |
| `internal/observability/logger.go` | Logger initialised to discard handler, not nil |
| `internal/observability/metrics.go` | **NEW** Prometheus-text counter endpoint, 7 counters |
| `internal/oauthproxy/service.go` | StartPurgeLoop(ctx); syncTokens logs errors; AddAccessToken logs error; metrics increments; backendURL + proxy cached in struct |
| `internal/oauthproxy/proxy.go` | Cached ReverseProxy via buildReverseProxy(); proxyResponseWriter.Write(); sub-path via context key; removed self-assignment |
| `internal/oauthproxy/handlers.go` | RFC 6749 §4.1.2.1 authorize error redirect; RFC 6749 §5.2 token JSON errors; error code mapping; method guard on metadata handlers |
| `internal/config/config.go` | USE_SQLITE default true; URL scheme/host validation; TTL > 0 validation |
| `internal/runtime/app.go` | purgeCtx lifecycle; storage backend log; /metrics route |
| `internal/httpserver/server.go` | WriteTimeout 60s; net.Listener bind; Addr(); WaitReady(ctx) |
| `go.mod` | go 1.25.0 → go 1.25.11; golang.org/x/sys v0.42 → v0.44.0 |

### Tests
| File | Changes |
|---|---|
| `internal/observability/audit_test.go` | WriteFailureIncrementsCounter; Logger_DefaultNotNil; HandleMetrics format; HandleMetrics 405 |
| `internal/oauthproxy/service_test.go` | TestStartPurgeLoop_Cancellation; TestTokenPersistenceFailureMetric |
| `internal/oauthproxy/handlers_test.go` | RFC6749_ErrorRedirect; RFC6749_JSONErrors; updated authorize test expectations |
| `internal/config/config_test.go` | URL scheme validation; zero TTL; negative TTL |
| `internal/runtime/app_test.go` | WaitReady replaces time.Sleep; newTestHandler helper; MetricsEndpoint test |

### CI
| File | Changes |
|---|---|
| `.github/workflows/ci.yml` | staticcheck; govulncheck; coverage gate ≥60%; shadow-compare build removed |

### Documentation
| File | Changes |
|---|---|
| `docs/operations/OPERATIONS.md` | All placeholders replaced; /readyz and /metrics in diagnostics; logrotate note; rollback reference updated |
| `docs/operations/ROLLBACK.md` | HISTORICAL banner added at top |
| `docs/operations/ROLLBACK_PRODUCTION.md` | **NEW** Executable production rollback runbook |
| `cmd/shadow-compare/main.go` | HISTORICAL comment |

---

## Remaining Risks

| Risk | Severity | Notes |
|---|---|---|
| OpenResty `$lan` bypass out-of-repo | High | Documented in OPERATIONS.md. Mitigation: document the file path and what it must contain, without exposing the actual IP range. |
| JSON store retained (default disabled) | Low | Startup WARN + USE_SQLITE=true default mitigate accidental activation. Full removal in v1.3. |
| Audit log no rotation | Low | Documented in OPERATIONS.md. Requires logrotate config outside this repo. |
| No per-request context timeout for proxy | Low | WriteTimeout 60s is a blunt instrument. A proper per-request `context.WithTimeout` in the Director would be cleaner but requires refactoring `HandleProxy`. Deferred to v1.3. |
| shadow-compare code still in repo | Info | Retired from CI, marked HISTORICAL. Delete in v1.3 after confirming no deploy scripts reference it. |

---

## Deployment Instructions

### Prerequisites
- Go runtime running on production (mcp-runtime.service on port 8086)
- Backup of current binary: `sudo cp /usr/local/bin/mcp-runtime /usr/local/bin/mcp-runtime.v1.1.backup`

### Steps

1. Build on the server (or copy from CI artifact):
   ```bash
   go build -o bin/mcp-runtime ./cmd/mcp-runtime
   ```

2. Update the env file to set SQLite as the backend (if not already set):
   ```bash
   sudo grep USE_SQLITE /etc/mcp-runtime/mcp-runtime.env || \
     echo "USE_SQLITE=true" | sudo tee -a /etc/mcp-runtime/mcp-runtime.env
   ```

3. If migrating from JSON to SQLite for the first time:
   ```bash
   sudo -u mcp-runtime /usr/local/bin/mcp-runtime migrate-storage
   ```

4. Install the new binary and restart:
   ```bash
   sudo cp bin/mcp-runtime /usr/local/bin/mcp-runtime
   sudo systemctl restart mcp-runtime.service
   ```

5. Verify readiness:
   ```bash
   curl -s http://127.0.0.1:8086/readyz   # expect: OK
   curl -s http://127.0.0.1:8086/metrics  # expect: Prometheus text with all 0 counters
   ```

6. Monitor for 5 minutes:
   ```bash
   sudo tail -f /var/log/mcp-runtime-go/audit.jsonl
   ```
   Look for `metadata_served`, `client_registered`, `token_issued` events.
   No `mcp_token_persistence_failures_total > 0` in `/metrics`.

### Rollback
If anything goes wrong, follow [ROLLBACK_PRODUCTION.md](docs/operations/ROLLBACK_PRODUCTION.md).

---

## Maturity Level

**Production Ready** (upgraded from "Production Candidate")

The two silent failure modes (F-01, F-02) that blocked this rating are resolved. All critical
operational paths now produce observable errors. Test coverage is 83%. All CI checks pass.
The remaining risks are documented and have mitigations in place.

To reach **Mature Production System**: add per-request proxy context timeout, implement log
rotation, add the OpenResty correlation ID, and remove the shadow mode dead code.
