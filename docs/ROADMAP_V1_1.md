# Roadmap v1.1 — mcp-runtime-go

**Target:** Mature Production System
**Focus:** Reliability, Observability, Scalability

---

## 1. Executive Summary

The v1.1 cycle focuses on transforming the `mcp-runtime-go` from a successful migration into a **Mature Production System**. The primary objectives are to eliminate synchronous I/O bottlenecks that threaten reliability under load and to close critical observability gaps in the proxy data plane.

---

## 2. Current State
- **Status:** Production Ready (with Technical Debt)
- **Strengths:** Robust OAuth RFC compliance, secure WAF integration, clean architecture.
- **Weaknesses:** Synchronous JSON persistence, blind proxy successes, linear scalability limits.

---

## 3. Workstreams

### Workstream A — Observability (Audit Parity)
- **Objective:** Achieve 100% audit coverage for the MCP data plane.
- **Rationale:** Forensic analysis of AI interactions is impossible without success logs.
- **Risk if Ignored:** Undetectable malicious tool usage; inability to debug data-plane issues.
- **Effort:** Low (2-3 days).
- **Production Impact:** Minimal log volume increase; significant forensic value.

### Workstream B — Persistence & Scalability (SQLite)
- **Objective:** Replace synchronous JSON store with a decoupled SQLite backend.
- **Rationale:** Current `fsync` on the request path is a latent failure point.
- **Risk if Ignored:** 504 timeouts under high concurrency; "Stop-the-World" writes.
- **Effort:** Medium (5-7 days).
- **Production Impact:** Drastic improvement in request latency and concurrency; zero-downtime migration required.

### Workstream C — Security Hardening (Defense-in-Depth)
- **Objective:** Implement secondary IP-based authorization checks in Go.
- **Rationale:** Reduces reliance on OpenResty configuration correctness.
- **Risk if Ignored:** Single point of failure (OpenResty config) for /authorize protection.
- **Effort:** Low (1-2 days).
- **Production Impact:** Enhanced security posture; no change to valid user flows.

---

## 4. Design Candidate Changes

### 4.1. proxy_hit Auditing
- **Current State:** Only `proxy_rejected` and `proxy_error` are logged.
- **Target State:** Every successful `HandleProxy` call logs a `proxy_hit` event.
- **Migration Strategy:** Update `HandleProxy` to call `s.auditLog` before `proxy.ServeHTTP`.
- **Rollback Strategy:** Revert Go binary.
- **Tests Required:** Unit tests in `proxy_test.go` verifying the audit event is emitted.

### 4.2. Asynchronous SQLite Backend
- **Current State:** `TokenStore` marshals JSON and calls `fsync` on every write.
- **Target State:** `TokenStore` uses SQLite with WAL (Write-Ahead Logging) and `PRAGMA synchronous=NORMAL`.
- **Migration Strategy:** 
    1. Implement SQLite `TokenStore`.
    2. Add "One-way Import": On startup, if `tokens.json` exists and SQLite is empty, import tokens.
    3. Keep `tokens.json` as read-only legacy for one release.
- **Rollback Strategy:** Revert binary; original `tokens.json` remains untouched.
- **Tests Required:** Concurrency tests for multiple writers; persistence recovery tests.

### 4.3. Internal CIDR Enforcement
- **Current State:** `/authorize` protected by Nginx `allow` directives.
- **Target State:** `AuthorizeRequest` includes a source IP check against `TRUSTED_AUTHORIZE_CIDRS`.
- **Migration Strategy:** Add config field; update `HandleAuthorize` to check `security.GetRequestInfo`.
- **Rollback Strategy:** Revert binary.
- **Tests Required:** Test unauthorized IP rejection in `handlers_test.go`.

---

## 5. Prioritization Matrix

### P0 — Mandatory for "Mature Production System"
1. **Async Persistence / SQLite Migration:** Critical for reliability.
2. **proxy_hit Auditing:** Critical for observability/compliance.

### P1 — v1.1 Hardening
1. **Internal CIDR Enforcement:** Security defense-in-depth.
2. **Token Store Purge Optimization:** Background purging for memory hygiene.

### P2 — Future Enhancements
1. **Prometheus Metrics:** Internal request/latency counters.
2. **Automatic Rollback Scripts:** CLI tools for one-command fallback.

---

## 6. Success Criteria

- **Observability:** 100% of successful MCP requests are traceable in `audit.jsonl` via `X-Request-ID`.
- **Reliability:** `POST /token` latency is independent of `fsync` performance (monitored via p99).
- **Scalability:** Token store supports >10,000 active sessions with $O(1)$ access time.
- **Security:** `/authorize` returns 403 when accessed directly (bypassing Nginx) from untrusted IPs.

---

## 7. Definition of Done
- All P0 and P1 items implemented and verified.
- 100% test coverage for new storage and audit logic.
- Migration script/logic verified with production-sized `tokens.json`.
- Documentation updated (OPERATIONS.md, SECURITY.md).

---

## 8. Recommended Release Sequence
1. **v1.1-beta1:** Audit parity (`proxy_hit`) + Security hardening.
2. **v1.1-beta2:** SQLite storage implementation (side-by-side with JSON).
3. **v1.1-rc1:** Migration logic active; JSON store deprecated.
4. **v1.1-stable:** Final v1.1 release.

**Final Maturity Classification:** **Production Ready**
*Note: To achieve **Mature Production System**, all P0 items must be verified in a production environment.*
