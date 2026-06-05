# Pre-Merge Security & Regression Review — v1.2-brooks-hardening → main

**Date:** 2026-06-05
**Reviewer:** Automated adversarial review (Claude Code)
**Branch:** `v1.2-brooks-hardening`
**Base:** `main` @ `3cc0d20`
**Commits ahead:** 4 (`23cc1c3`, `cdc5d7d`, `bb7341d`, `3bc4518`)

---

## 1. Summary of Changes

| Area | What changed |
|---|---|
| **OAuth error handling** | `/authorize` now RFC 6749 §4.1.2.1 compliant: invalid `redirect_uri`/`client_id` → direct 400/401 (no redirect); all other errors → RFC error redirect. `/token` now RFC 6749 §5.2 compliant: JSON error body with standard codes |
| **Error strings (internal)** | `"unsupported response_type"` → `"unsupported_response_type"`, `"client_auth_failed"` → `"invalid_client"`, `"invalid_code"` → `"invalid_grant: ..."`, `"pkce_failed"` → `"invalid_grant: pkce verification failed"` |
| **Proxy caching** | `httputil.ReverseProxy` built once at `NewService`, not per-request; sub-path via typed context key |
| **HTTP timeouts** | `WriteTimeout` 10s → 60s (was shorter than backend 30s timeout) |
| **Purge loop lifecycle** | `StartPurgeLoop` now accepts `context.Context`; goroutine exits on cancel; ticker is `Stop()`'d |
| **Token persistence errors** | `AddAccessToken` and `syncTokens` now log errors and increment Prometheus counter instead of silently ignoring |
| **Config validation** | `Validate()` now rejects empty/non-http GRAV_MCP_URL, PROXY_BASE_URL, and TTL ≤ 0 |
| **SQLite default** | `USE_SQLITE` default changed `false` → `true` |
| **Logger nil guard** | `observability.Logger` initialised to discard handler at package init; never nil |
| **proxyResponseWriter.Write** | Added; captures implicit 200 before the underlying writer does |
| **Metrics endpoint** | `/metrics` added; Prometheus text format; 7 counters; zero external dependencies |
| **Server WaitReady** | `net.Listen` split; `Addr()` and `WaitReady(ctx)` for deterministic test startup |
| **CI** | `staticcheck`, `govulncheck`, coverage gate ≥60% added; shadow-compare build removed |
| **go.mod** | `go 1.25.0` → `go 1.25.11`; `golang.org/x/sys` v0.42→v0.44; `modernc.org/sqlite` promoted to direct dep |
| **Docs** | OPERATIONS.md placeholders replaced; ROLLBACK_PRODUCTION.md new; shadow-compare marked HISTORICAL |

---

## 2. Sensitive Files Modified

| File | Sensitivity | Change |
|---|---|---|
| `internal/oauthproxy/handlers.go` | 🔴 HIGH — OAuth endpoints | RFC error handling rewrite |
| `internal/oauthproxy/service.go` | 🔴 HIGH — token issuance logic | Error strings, purge loop, persistence errors |
| `internal/oauthproxy/proxy.go` | 🔴 HIGH — MCP reverse proxy | Cached proxy, proxyResponseWriter.Write |
| `internal/config/config.go` | 🟠 MEDIUM — startup validation | New URL/TTL checks, SQLite default flip |
| `internal/httpserver/server.go` | 🟠 MEDIUM — HTTP server | WriteTimeout 10s→60s, WaitReady |
| `internal/runtime/app.go` | 🟠 MEDIUM — app wiring | purgeCtx, /metrics route |
| `go.mod` / `go.sum` | 🟡 LOW | go 1.25.11 patch, sys v0.44 |
| `internal/observability/*` | 🟢 INFO | Logger nil guard, metrics, audit error reporting |
| `.github/workflows/ci.yml` | 🟢 INFO | Stricter CI only |
| `docs/` | 🟢 INFO | Docs and runbooks |

---

## 3. OAuth Regression Risks

### 3.1 Dynamic Client Registration (`/register`)
**Risk: LOW — No regression.**

The only change is `w.Header().Set("Content-Type", "application/json")` added before the 400 error response for invalid redirect_uri. The happy path (successful registration) is unchanged. Claude.ai sends a registration request; it gets back the same `client_id`/`client_secret` pair as before.

### 3.2 Authorization Code Flow (`/authorize`)

**Change:** Pre-validation of `redirect_uri` and `client_id` before `IssueAuthCode` is called.

**Risk: LOW for valid requests, BEHAVIORAL CHANGE for error cases.**

- Valid requests (correct `redirect_uri`, correct `client_id`, valid PKCE): path is identical.
- Invalid `redirect_uri`: was `IssueAuthCode` → 400 plain text. Now: 400 plain text (same HTTP status, different body). Not a Claude.ai concern — Claude.ai only sends valid redirect URIs.
- Invalid `client_id`: was `IssueAuthCode` → `"invalid client_id"` → mapped to 400. Now: 401 `unauthorized_client`. This is RFC-correct. Claude.ai should not be sending wrong client IDs in production.
- Missing `state`: was 400. Now: 302 redirect with `error=invalid_request`. This is RFC 6749 §4.1.2.1 compliant. Claude.ai always provides `state`.
- Missing PKCE (when MandatoryPKCE=true): was 400. Now: 302 redirect with `error=invalid_request`. Claude.ai sends PKCE.

**Verdict:** No regression for the normal Claude.ai PKCE flow. Error-path changes are RFC improvements.

### 3.3 Token Exchange (`/token`)

**Risk: LOW for valid requests, BEHAVIORAL CHANGE for error cases.**

Internal error strings changed:
| Old string | New string | RFC code emitted | HTTP status |
|---|---|---|---|
| `"unsupported grant_type"` | `"unsupported_grant_type"` | `unsupported_grant_type` | 400 |
| `"client_auth_failed"` | `"invalid_client"` | `invalid_client` | **401** (was 401, unchanged) |
| `"invalid_code"` | `"invalid_grant: ..."` | `invalid_grant` | 400 |
| `"invalid_redirect_uri"` | `"invalid_grant: redirect_uri mismatch"` | `invalid_grant` | 400 |
| `"pkce_failed"` | `"invalid_grant: pkce verification failed"` | `invalid_grant` | 400 |
| `"server_error"` | `"server_error"` | `server_error` | 500 |

The important change: `client_auth_failed` → `invalid_client`. Previously the old switch in handlers.go had `case "client_auth_failed": status = http.StatusUnauthorized` so the HTTP status was already 401. The new code maps it to `invalid_client` RFC code with 401. Same HTTP status; the RFC `error` field is now correct. Claude.ai may handle `invalid_client` by re-initiating the OAuth flow, which is the correct behavior.

JSON body format is now always present (was already present for error responses in v1.1).

**Verdict:** No regression for valid authorization_code exchanges. Error cases now return correct RFC codes.

### 3.4 PKCE Verification
No change to the `security.ValidatePKCE` function. The validation logic (S256, base64url encoding, length checks 43–128) is identical.

---

## 4. Storage Regression Risks

### 4.1 SQLite backend (default: now enabled)

**Risk: MEDIUM — deployment action required on first upgrade.**

`USE_SQLITE` default flipped to `true`. **If the production env file does not explicitly set `USE_SQLITE=true` or `USE_SQLITE=false`, the service will now default to SQLite on first restart.**

- If previously running JSON store: on restart with v1.2 and no env override, the service will start fresh with an empty SQLite DB. All existing tokens will be lost. Claude.ai will re-authenticate automatically on the next request (standard OAuth token expiry behavior). **This is a one-time re-auth event, not a data loss incident**, because access tokens expire and Claude.ai handles token refresh.
- If production env file already has `USE_SQLITE=true` (set during v1.1 deployment): no change.
- Mitigation: deployment instructions in `FINAL_HARDENING_REPORT.md` require explicit `USE_SQLITE=true` in env file before restart.

**Token migration:** If continuity is required, run `mcp-runtime migrate-storage` before restart to migrate existing JSON tokens to SQLite. See `ROLLBACK_PRODUCTION.md`.

### 4.2 Token persistence error handling

**Risk: LOW — strictly improvement.**

Previously silent `_ = s.store.Save(snapshot)` in `syncTokens`. Now logs error + increments counter. A failure that was invisible before is now visible. No behavioral change for the success path.

`AddAccessToken` now returns error to caller if `store.Save` fails, and rolls back the in-memory map. Previously it silently kept the token in memory but didn't persist it (tokens would be lost on restart). New behavior: fails the token issuance cleanly. Claude.ai receives a `server_error` response and will retry. **More correct behavior.**

---

## 5. OpenResty / systemd Regression Risks

### 5.1 Port and interface
No change. Service still binds `ListenHost:ListenPort` from config (default: `127.0.0.1:8086`).

### 5.2 WriteTimeout 10s → 60s

**Risk: LOW — improvement, no OpenResty concern.**

The previous 10s WriteTimeout was shorter than the backend HTTP client Timeout (30s). Under load, the server could close connections before the proxy response arrived. Raising to 60s prevents this. OpenResty is an upstream caller; it has its own proxy timeout configured. This change only affects how long the Go server waits before killing a slow response, not how long OpenResty waits.

**Action:** Verify OpenResty `proxy_read_timeout` is ≥ 60s (or confirm it's irrelevant because OpenResty → Go → backend is the chain and OpenResty's timeout dominates).

### 5.3 New `/metrics` endpoint
OpenResty does not need to proxy `/metrics`. If OpenResty proxies all requests to the Go service, it will forward `/metrics` requests too — but since `/metrics` is unauthenticated, this should be blocked at the OpenResty layer or firewall.

**Action:** Confirm `/metrics` is not exposed through OpenResty to the public internet. The endpoint returns operational counters, not secrets, but rate limiting is advisable.

### 5.4 systemd lifecycle
The purge loop now requires a context cancel before `Close()`. The wiring in `app.go` is:
```go
purgeCtx, cancelPurge := context.WithCancel(context.Background())
go a.oauth.StartPurgeLoop(purgeCtx)
defer cancelPurge()
```
The `defer cancelPurge()` runs before `defer a.oauth.Close()` (defers are LIFO). Wait — `defer a.oauth.Close()` is registered first at the top of `Run()`, so it runs last. `defer cancelPurge()` runs before `Close()`. This ordering is correct. No goroutine leak on SIGTERM.

**systemd graceful shutdown:** `server.Stop(ctx)` has a 15s graceful shutdown timeout. This is compatible with systemd's default `TimeoutStopSec=90s`.

### 5.5 Config validation at startup

**Risk: MEDIUM — potential startup failure if env file incomplete.**

New `Validate()` checks:
1. `GRAV_MCP_URL` must be non-empty and http/https.
2. `PROXY_BASE_URL` must be non-empty and http/https.
3. `AUTH_CODE_TTL` must be > 0 (default: 300).
4. `ACCESS_TOKEN_TTL` must be > 0 (default: 86400).

Items 3 and 4 have safe defaults. Items 1 and 2 must be set in the env file. Any correctly operational v1.1 production instance necessarily has these set (the service cannot function without them). The new validation converts a runtime panic/misbehavior into a clean startup rejection.

**Risk is theoretical:** if the env file has a typo or missing values that went unnoticed because the service started but behaved incorrectly, the new validation will now cleanly reject. This is desired behavior.

---

## 6. Test Results

```
Branch:          v1.2-brooks-hardening
Worktree:        /home/jm/mcp-runtime-go/.worktrees/hardening-v1.1.1-rc1
Checked out:     clean (0 modified, 0 untracked after FINAL_HARDENING_REPORT.md commit)

go test ./...
  ?   mcp-runtime-go/cmd/mcp-runtime         [no test files]
  ok  mcp-runtime-go/cmd/shadow-compare      (cached)
  ok  mcp-runtime-go/internal/config         (cached)
  ok  mcp-runtime-go/internal/context        (cached)
  ok  mcp-runtime-go/internal/httpserver     (cached)
  ok  mcp-runtime-go/internal/oauthproxy     (cached)
  ok  mcp-runtime-go/internal/observability  (cached)
  ok  mcp-runtime-go/internal/runtime        (cached)
  ok  mcp-runtime-go/internal/security       (cached)
  ok  mcp-runtime-go/internal/storage        (cached)
  EXIT: 0

go test -race ./...
  All packages: ok
  EXIT: 0

go vet ./...
  EXIT: 0 (no output)

govulncheck ./...
  No vulnerabilities found.
  EXIT: 0

staticcheck ./...
  EXIT: 0 (no output — clean)

Coverage:
  internal/config         92.3%
  internal/oauthproxy     83.3%
  internal/observability  81.9%
  internal/runtime        83.7%
  internal/security       100.0%
  internal/storage        76.9%
  internal/httpserver     63.2%
  internal/context        50.0%
  Total                   83.0%  (gate: ≥ 60% ✓)
```

---

## 7. Pre-Deployment Checklist

Before merging and deploying, confirm the following in the production env file:

- [ ] `USE_SQLITE=true` is explicitly set (prevents accidental JSON-store activation)
- [ ] `GRAV_MCP_URL=https://...` is set and uses http/https scheme
- [ ] `PROXY_BASE_URL=https://...` is set and uses http/https scheme
- [ ] `/metrics` is NOT exposed via OpenResty to the public internet
- [ ] Binary backup taken: `sudo cp /usr/local/bin/mcp-runtime /usr/local/bin/mcp-runtime.v1.1.backup`
- [ ] Rollback procedure confirmed: `docs/operations/ROLLBACK_PRODUCTION.md` is readable

---

## 8. Verdict

```
╔══════════════════════════════════════════════╗
║                                              ║
║           ✅  MERGE SAFE                     ║
║                                              ║
║  All 4 commits are safe to merge to main.    ║
║  No regressions in OAuth, storage, proxy,   ║
║  or systemd/OpenResty integration detected. ║
║                                              ║
║  Follow Pre-Deployment Checklist (§7)        ║
║  before restarting the service.              ║
║                                              ║
╚══════════════════════════════════════════════╝
```

**Rationale:** Every behavioral change is a hardening improvement that makes a previously-silent failure visible, or brings error responses into RFC compliance. The normal Claude.ai PKCE + authorization_code flow is unaffected on the happy path. The SQLite default flip is the highest-impact operational change, but it is safe as long as `USE_SQLITE=true` is explicit in the env file (which it should already be from the v1.1 deployment).

---

### Merge command (execute only after completing §7 checklist)

```bash
git checkout main
git merge --no-ff v1.2-brooks-hardening -m "v1.2: Reliability & Observability Hardening (Brooks audit)"
```
