# Pre-Merge Review Report: v1.2-brooks-hardening

**Date:** 2026-06-05
**Reviewer:** Gemini CLI
**Branch under review:** `v1.2-brooks-hardening`
**Base branch:** `main`

---

## 1. Git Log (main..v1.2-brooks-hardening)

```
3bc4518 docs: v1.2 final hardening report
bb7341d v1.2: CI hardening, docs fixes, shadow retirement
cdc5d7d v1.2: test suite hardening
23cc1c3 v1.2: core reliability and observability hardening
```

---

## 2. Git Diff Stat (main..v1.2-brooks-hardening)

```
 .github/workflows/ci.yml               |  21 +++++++--
 cmd/shadow-compare/main.go             |   3 ++
 docs/FINAL_HARDENING_REPORT.md         | 193 +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 docs/PRODUCTION_VALIDATION.md          |  29 +++----------
 docs/V1_1_FINAL_REPORT.md              |  56 ------------------------
 docs/operations/OPERATIONS.md          |  80 +++++++++++++++++++++-------------
 docs/operations/ROLLBACK.md            |  13 +++++-
 docs/operations/ROLLBACK_PRODUCTION.md | 127 +++++++++++++++++++++++++++++++++++++++++++++++++++++
 docs/plans/2026-06-04-hardening.md     | 183 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
 go.mod                                 |   7 +--
 go.sum                                 |  34 ++++++++++++++-
 internal/config/config.go              |  31 ++++++++++++-
 internal/config/config_test.go         |  55 +++++++++++++++++++++--
 internal/httpserver/server.go          |  51 +++++++++++++++++++---
 internal/oauthproxy/handlers.go        | 111 +++++++++++++++++++++++++++++++++++++++--------
 internal/oauthproxy/handlers_test.go   | 155 ++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++-
 internal/oauthproxy/proxy.go           | 113 +++++++++++++++++++++++++++--------------------
 internal/oauthproxy/service.go         |  73 +++++++++++++++++++++++--------
 internal/oauthproxy/service_test.go    |  56 ++++++++++++++++++++++++
 internal/observability/audit.go        |  25 +++++++----
 internal/observability/audit_test.go   |  51 ++++++++++++++++++++++
 internal/observability/logger.go       |   5 ++-
 internal/observability/metrics.go      |  58 +++++++++++++++++++++++++
 internal/runtime/app.go                |  21 ++++++---
 internal/runtime/app_test.go           |  81 +++++++++++++++++++++++++++++-----
 25 files changed, 1392 insertions(+), 240 deletions(-)
```

---

## 3. Modified Files List

1.  `.github/workflows/ci.yml`
2.  `cmd/shadow-compare/main.go`
3.  `docs/FINAL_HARDENING_REPORT.md`
4.  `docs/PRODUCTION_VALIDATION.md`
5.  `docs/V1_1_FINAL_REPORT.md`
6.  `docs/operations/OPERATIONS.md`
7.  `docs/operations/ROLLBACK.md`
8.  `docs/operations/ROLLBACK_PRODUCTION.md`
9.  `docs/plans/2026-06-04-hardening.md`
10. `go.mod`
11. `go.sum`
12. `internal/config/config.go`
13. `internal/config/config_test.go`
14. `internal/httpserver/server.go`
15. `internal/oauthproxy/handlers.go`
16. `internal/oauthproxy/handlers_test.go`
17. `internal/oauthproxy/proxy.go`
18. `internal/oauthproxy/service.go`
19. `internal/oauthproxy/service_test.go`
20. `internal/observability/audit.go`
21. `internal/observability/audit_test.go`
22. `internal/observability/logger.go`
23. `internal/observability/metrics.go`
24. `internal/runtime/app.go`
25. `internal/runtime/app_test.go`

---

## 4. Commit Summaries

- **23cc1c3 (Core Hardening):** Implemented the bulk of technical fixes: fail-closed token issuance, RFC-compliant OAuth errors, improved ReverseProxy efficiency, SQLITE by default, structured audit failure logging, and enhanced observability.
- **cdc5d7d (Test Hardening):** Significantly expanded the test suite to cover all new hardening behaviors and removed flaky `time.Sleep` calls in favor of proper synchronization (`WaitReady`).
- **bb7341d (CI & Docs Hardening):** Hardened CI with `staticcheck`, `govulncheck`, and coverage gates. Retired shadow mode. Produced a high-quality production rollback runbook (`ROLLBACK_PRODUCTION.md`).
- **3bc4518 (Final Report):** Documented the entire hardening process and disposition of all audit findings.

---

## 5. Risk Assessment

### OAuth Risks (LOW)
- **Improvement:** Error responses now strictly follow RFC 6749, reducing ambiguity for clients like Claude.ai.
- **Residual Risk:** Single-client registration (Dynamic Client Registration always returns the same `client_id`) is intentional for this single-tenant proxy. This is mitigated by CIDR enforcement on `/authorize` and rate limiting at the Nginx layer.

### PKCE Risks (NONE)
- **Improvement:** Switched to `crypto/subtle.ConstantTimeCompare` for PKCE verifier validation, eliminating potential timing attacks.

### Dynamic Client Registration Risks (LOW)
- **Improvement:** Now strictly validates `redirect_uris` (must be non-empty and matching allowed patterns).
- **Residual Risk:** Since it doesn't persist new clients (stateless), any client other than the pre-configured one will fail during the subsequent `/authorize` or `/token` phases due to `client_id` mismatch.

### SQLite Risks (LOW)
- **Improvement:** Defaulted to SQLite with WAL mode and sensible pragmas (`busy_timeout=5000`).
- **Residual Risk:** SQLite is file-based; ensure the target directory exists and is writable in production (addressed in `OPERATIONS.md`).

### OpenResty/CrowdSec Risks (NONE)
- **Status:** No changes were made to the Go runtime that break the Nginx integration. The `/readyz` and `/metrics` endpoints added are compatible with standard monitoring/probing.

### Rollback Python Risks (LOW)
- **Status:** A new executable runbook (`ROLLBACK_PRODUCTION.md`) has been created.
- **Mitigation:** Token continuity remains a minor friction (tokens issued by Go are not valid in Python), but Claude.ai handles this via automatic re-authentication.

---

## 6. Verification Results

- **go test ./...:** ✅ PASS (9/9 packages)
- **go test -race ./...:** ✅ PASS (no races detected)
- **go vet ./...:** ✅ PASS
- **govulncheck ./...:** ✅ PASS (No vulnerabilities found)
- **staticcheck ./...:** ✅ PASS (per `FINAL_HARDENING_REPORT.md`)

---

## 7. Verdict

# **MERGE SAFE**

The branch `v1.2-brooks-hardening` is exceptionally well-prepared. It not only fixes the requested hardening items but significantly improves the observability, testability, and documentation of the project. The transition to SQLite by default and the RFC-compliant OAuth errors are major stability wins.

