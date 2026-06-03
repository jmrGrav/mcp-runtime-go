# PHASE 2.6 REPORT — Red-Team Hardening

## Summary
Phase 2.6 focused on remediating critical security and reliability blockers identified by the Red-Team audit. The primary objectives were hardening the HTTP proxy against boundary bypass, ensuring reliable shadow parity comparison, and optimizing system observability and performance.

**Verdict: DEPLOY SHADOW AUTHORIZED**

## Remediated Findings

### 1. Hardened HTTP Proxy (Priority 1)
- **Implementation**: Replaced manual proxy logic with a robust `httputil.ReverseProxy` integration.
- **Path Validation**: Added `path.Clean` normalization and strict rejection of `@`, encoded dot segments (`%2e`), and out-of-boundary paths.
- **Header Security**: Implemented explicit removal of all hop-by-hop headers (`Connection`, `Keep-Alive`, `Proxy-Authenticate`, `Proxy-Authorization`, `TE`, `Trailer`, `Transfer-Encoding`, `Upgrade`).
- **Tests**: Added comprehensive unit tests in `proxy_test.go` covering bypass attempts, header suppression, and `X-Request-ID` propagation.

### 2. Reliable Shadow Comparison (Priority 2)
- **Primary Key**: `request_id` is now the mandatory primary join key for parity validation.
- **Failures**: The tool now exits with a non-zero code if `request_id` is missing on critical events (`token_issued`, `authorize_approved`, etc.), if duplicate IDs are found, or if matches are ambiguous.
- **Unsafe Fallback**: Time-based matching is only allowed via the explicit `--allow-unsafe-fallback` flag.

### 3. Request ID Entropy & Observability (Priority 3)
- **Entropy**: Increased `request_id` entropy to 16 bytes (128-bit) using `crypto/rand`.
- **Fail-Closed**: If random generation fails, the system panics (fail-closed) rather than proceeding with weak IDs.
- **Visibility**: `GetRequestID` now returns `"missing"` instead of an empty string to ensure visibility in audit logs.

### 4. Optimized Token Store Locking (Priority 4)
- **Snapshot Pattern**: Refactored `AddAccessToken` and `PurgeExpired` to take a snapshot of the token map under lock and perform the disk write (`fsync`) outside the critical section.
- **Concurrency**: Verified thread-safety with a new `TestService_Concurrency` test case.

### 5. Simplified Routing (Priority 5)
- **Ambiguity Removal**: Simplified `ServeMux` registration to use a single `/mcp/` handler, leveraging Go 1.22's routing behavior to handle both root and subpaths consistently.

## Validation Results
- `go test ./...`: **PASS**
- `go vet ./...`: **PASS**
- `go build ./cmd/mcp-runtime`: **SUCCESS**
- `go build ./cmd/shadow-compare`: **SUCCESS**
- `brooks-review`: **Score 98/100**
- `brooks-test`: **Clean**

## Files Modified
- `internal/oauthproxy/proxy.go`: Hardened proxy logic.
- `internal/oauthproxy/proxy_test.go`: New security and bypass tests.
- `cmd/shadow-compare/main.go`: Tightened comparison logic.
- `internal/runtime/app.go`: Simplified routing.
- `internal/runtime/middleware.go`: Verified 16-byte entropy.
- `internal/context/context.go`: Explicit "missing" ID.
- `internal/oauthproxy/service.go`: Optimized locking.
- `internal/oauthproxy/service_test.go`: Concurrency and TLS tests.

## Exit Criteria Check
- [x] No Critical Brooks remaining.
- [x] Proxy routing tested against bypass.
- [x] Hop-by-hop headers suppressed.
- [x] shadow-compare enforces request_id.
- [x] Request ID entropy >= 16 bytes.
- [x] Go remains non-authoritative.
