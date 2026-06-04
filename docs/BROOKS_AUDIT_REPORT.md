# Brooks-Lint — Health Dashboard & Audit Report

**Project:** mcp-runtime-go
**Date:** 2026-06-04
**Review Mode:** BROOKS-AUDIT (Adversarial)
**Maturity Level:** Production Ready

---

## 1. Executive Dashboard

| Dimension | Score | Verdict |
|---|---|---|
| **Architecture** | 92/100 | 🟢 Solid boundaries; minor dependency leakage. |
| **Security** | 94/100 | 🟢 Defense-in-depth implemented; strong redaction. |
| **Reliability** | 78/100 | 🟡 **CRITICAL BOTTLENECK**: Request-path I/O blocking. |
| **Observability** | 82/100 | 🟡 **AUDIT GAP**: No success logging for proxy calls. |
| **Maintainability** | 90/100 | 🟢 High readability; minor "clever" config logic. |
| **Production Readiness** | 88/100 | **PRODUCTION READY** (with critical debt) |

---

## 2. Classified Findings

### 🔴 Critical — Persistence Bottleneck: Synchronous I/O on Request Path
- **Affected Files:** `internal/storage/json_store.go`, `internal/oauthproxy/service.go`
- **Explanation:** Every OAuth token exchange (`POST /token`) triggers a synchronous write to the JSON token store. The current implementation performs `json.Marshal`, writes to a temporary file, calls `fsync` on the file, renames, and finally calls `fsync` on the parent directory—all while holding a write lock on the token map.
- **Operational Impact:** Under high connection churn (e.g., many AI sessions starting simultaneously), disk I/O latency will directly block the Go runtime's ability to issue tokens. This creates a "Stop-the-World" event for the OAuth service that can lead to 504 timeouts at the edge proxy.
- **Remedy:** Decouple persistence from the request path. Implement an asynchronous writer or migrate to a database (SQLite with WAL mode) that supports concurrent reads and background writes.

### 🟠 High — Observability Gap: Blind Proxy Successes
- **Affected Files:** `internal/oauthproxy/proxy.go`
- **Explanation:** `HandleProxy` audits `proxy_rejected` and `proxy_error` but provides zero audit trail for successful tool calls.
- **Operational Impact:** Forensics is impossible. If the AI performs a sensitive action via the MCP proxy, the Go audit logs will only show that a token was issued earlier, with no record of the specific call or its parameters.
- **Remedy:** Implement `proxy_hit` auditing in `HandleProxy` that logs the request method, subpath, and request ID for every successful proxying event.

### 🟡 Medium — Memory & Disk Growth: $O(N)$ JSON Store
- **Affected Files:** `internal/storage/json_store.go`
- **Explanation:** The entire token store is loaded into memory and the entire file is rewritten for every update.
- **Operational Impact:** As the system scales to thousands of registered clients or active sessions, memory consumption and write size grow linearly. This is a scalability ceiling that will eventually cause "Accidental Complexity" in memory management.
- **Remedy:** Migrate to a keyed storage engine (SQLite) where updates are incremental and only active tokens are cached.

### 🟢 Low — Security: Proxy-Layer Dependency for Authorization
- **Affected Files:** `internal/httpserver/server.go`, OpenResty config
- **Explanation:** The restriction of `/authorize` to the owner IP is implemented solely in OpenResty. The Go application itself does not enforce this restriction.
- **Operational Impact:** If the Go application is accidentally exposed (e.g., misconfigured internal routing), the authorization endpoint is globally accessible, increasing the attack surface for session hijacking or brute-forcing (though PKCE mitigates this).
- **Remedy:** Implement CIDR-based allowlisting within the Go application as a secondary defense layer.

### ℹ️ Observation — Tactical Reflection in Config
- **Affected Files:** `internal/config/config.go`
- **Explanation:** The use of `reflect` for environment binding is a "Tactical Programming" choice. It reduces boilerplate but obscures the mapping between environment variables and struct fields for static analysis tools.
- **Impact:** Low. The implementation is self-contained and well-commented.

---

## 3. Explicit Evaluation

| Item | Evaluation |
|---|---|
| **JSON vs SQLite** | **SQLite Required.** JSON store is durable (fsync is correct) but architecturally fragile for high-concurrency production use. |
| **Audit Coverage** | **Incomplete.** Critical gap in MCP proxy successes. RFC 7591/6749 flows are well-audited, but the data-plane is silent. |
| **Request ID** | **Robust.** Strategy correctly propagates `X-Request-ID` from edge to backend services, enabling unified tracing. |
| **RFC Compliance** | **High.** OAuth 2.0 and PKCE (RFC 7636) implementations are disciplined and follow best practices. |
| **Dynamic Registration** | **Functional.** Minimal RFC 7591 compliance is sufficient for Claude.ai's current connector pattern. |
| **OpenResty Dependency** | **High Risk.** The system relies on the proxy for both security (`/authorize`) and request-id generation. |
| **CrowdSec Integration** | **Exemplary.** AppSec integration provides the necessary WAF layer for public-facing OAuth endpoints. |
| **Rollback Realism** | **Manual.** The path back to Python is "copy-paste and restart." Acceptable for current scale but lacks automation. |
| **Test Coverage** | **High Quality.** Use of `testutil` and comprehensive coverage in `oauthproxy` ensures regression safety. |
| **Documentation** | **Excellent.** Migration and production validation docs are detailed and professional. |

---

## 4. Technical Debt Register

| Category | Priority | Effort | Risk | Suggested Milestone |
|---|---|---|---|---|
| **Persistence** | 🔴 Critical | Medium | High | Post-Migration Cleanup |
| **Observability** | 🟠 High | Low | Medium | Immediate Patch |
| **Architecture** | 🟡 Medium | Medium | Low | Scalability Phase |
| **Security** | 🟢 Low | Low | Low | Security Hardening |

---

## 5. Final Verdict

**Health Score: 88/100**

**Top 5 Strengths:**
1. Disciplined Go package structure and boundary isolation.
2. Correct implementation of durable filesystem writes (fsync + rename).
3. Robust PKCE and OAuth RFC compliance.
4. Comprehensive audit log redaction preventing secret leaks.
5. Unified request ID propagation across layers.

**Top 5 Risks:**
1. **Synchronous I/O Blocking**: Disk latency can freeze the entire request handler.
2. **Audit Blind Spot**: No visibility into successful AI tool calls.
3. **Linear Persistence Growth**: Entire store rewrites don't scale $O(N)$.
4. **Proxy Dependency**: Security relies too heavily on OpenResty configuration.
5. **Memory State**: Token maps in memory grow unchecked until purged (hourly).

**Recommended Next Milestone:** **Persistence Refactoring (SQLite Transition).**

**Final Verdict:**
The system is **PRODUCTION READY** and represents a significant upgrade over the legacy Python implementation in terms of performance and maintainability. However, the **synchronous I/O bottleneck** in the token exchange path is a latent failure point that will manifest under load. This must be remediated before the system is considered a **Mature Production System**.
