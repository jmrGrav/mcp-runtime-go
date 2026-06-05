# mcp-runtime-go Hardening Plan

> **For Gemini:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Apply minimal production-hardening patches to mcp-runtime-go to improve security, reliability, and correctness before candidate release.

**Architecture:** Surgical updates to HTTP handlers, OAuth service, and observability layers to enforce strict validation and error handling without altering core system design or data formats.

**Tech Stack:** Go, standard library (net/http, crypto/subtle).

---

### Task 1: Route /mcp and /mcp/

**Files:**
- Modify: `internal/runtime/app.go`
- Test: `internal/runtime/app_test.go`

**Step 1: Write the failing test**
Update `internal/runtime/app_test.go` to verify that both `/mcp` and `/mcp/` reach the proxy handler without redirect or 404.

**Step 2: Run test to verify it fails**
Run: `go test -v internal/runtime/app_test.go`
Expected: Fail (likely 301 or 404 for `/mcp`).

**Step 3: Write minimal implementation**
In `internal/runtime/app.go`, add an explicit route for `/mcp`.

```go
	// MCP Proxy
	mux.HandleFunc("/mcp", oauthSvc.HandleProxy)
	mux.HandleFunc("/mcp/", oauthSvc.HandleProxy)
```

**Step 4: Run test to verify it passes**
Run: `go test -v internal/runtime/app_test.go`

**Step 5: Commit**
`git add internal/runtime/app.go internal/runtime/app_test.go && git commit -m "fix: route both /mcp and /mcp/ to proxy handler"`

---

### Task 2: Do not issue tokens when persistence fails

**Files:**
- Modify: `internal/oauthproxy/service.go`
- Test: `internal/oauthproxy/service_test.go`

**Step 1: Write the failing test**
Create a test in `internal/oauthproxy/service_test.go` with a mock store that returns an error on `Save`. Verify `ExchangeToken` returns an error and no access token is issued.

**Step 2: Run test to verify it fails**
Run: `go test -v internal/oauthproxy/service_test.go`
Expected: Fail (currently ignores Save error).

**Step 3: Write minimal implementation**
1. Update `AddAccessToken` to return `error`.
2. In `ExchangeToken`, check the error from `AddAccessToken`.
3. If `AddAccessToken` fails, return a "server_error" or similar.

**Step 4: Run test to verify it passes**
Run: `go test -v internal/oauthproxy/service_test.go`

**Step 5: Commit**
`git commit -am "fix: fail token issuance if persistence fails"`

---

### Task 3: Enforce HTTP methods

**Files:**
- Modify: `internal/oauthproxy/handlers.go`
- Test: `internal/oauthproxy/handlers_test.go`

**Step 1: Write the failing test**
Update `internal/oauthproxy/handlers_test.go` to send `GET` to `/token` and `/register`, and `POST` to `/authorize`. Verify they return `405 Method Not Allowed` with `Allow` header.

**Step 2: Run test to verify it fails**
Run: `go test -v internal/oauthproxy/handlers_test.go`
Expected: Fail (currently allows any method).

**Step 3: Write minimal implementation**
Add method checks at the beginning of `HandleRegister`, `HandleToken`, and `HandleAuthorize`.

**Step 4: Run test to verify it passes**
Run: `go test -v internal/oauthproxy/handlers_test.go`

**Step 5: Commit**
`git commit -am "fix: enforce strict HTTP methods on OAuth endpoints"`

---

### Task 4: Strengthen readiness

**Files:**
- Modify: `internal/runtime/app.go`
- Modify: `internal/oauthproxy/service.go`
- Test: `internal/runtime/app_test.go`

**Step 1: Write the failing test**
Verify `/readyz` fails if dependencies are broken (e.g., token store unreadable).

**Step 2: Run test to verify it fails**
Run: `go test -v internal/runtime/app_test.go`
Expected: Fail (currently always returns OK).

**Step 3: Write minimal implementation**
1. Add `Ready()` method to `Service` and `Store`.
2. In `newHandler`, implement a proper `readyz` handler that calls `oauthSvc.Ready()`.

**Step 4: Run test to verify it passes**
Run: `go test -v internal/runtime/app_test.go`

**Step 5: Commit**
`git commit -am "fix: implement robust readiness check"`

---

### Task 5: PKCE constant-time compare

**Files:**
- Modify: `internal/security/pkce.go`
- Test: `internal/security/pkce_test.go`

**Step 1: Write the failing test**
(This is hard to test "failure" for timing, but I'll ensure behavior remains correct).

**Step 2: Write minimal implementation**
Replace `==` with `subtle.ConstantTimeCompare` in `ValidatePKCE`.

**Step 3: Run test to verify it passes**
Run: `go test -v internal/security/pkce_test.go`

**Step 4: Commit**
`git commit -am "security: use constant-time comparison for PKCE"`

---

### Task 6: Registration validation

**Files:**
- Modify: `internal/oauthproxy/service.go`
- Test: `internal/oauthproxy/service_test.go`

**Step 1: Write the failing test**
Test `RegisterClient` with empty `redirect_uris`. Verify it fails.

**Step 2: Run test to verify it fails**
Run: `go test -v internal/oauthproxy/service_test.go`
Expected: Fail (currently allows empty).

**Step 3: Write minimal implementation**
Add check for `len(req.RedirectURIs) == 0` in `RegisterClient`.

**Step 4: Run test to verify it passes**
Run: `go test -v internal/oauthproxy/service_test.go`

**Step 5: Commit**
`git commit -am "fix: validate redirect_uris in client registration"`

---

### Task 7: Audit file permissions

**Files:**
- Modify: `internal/observability/audit.go`
- Test: `internal/observability/audit_test.go`

**Step 1: Write the failing test**
Write a test that creates an audit log and checks its file permissions.

**Step 2: Run test to verify it fails**
Run: `go test -v internal/observability/audit_test.go`
Expected: Fail (currently 0644).

**Step 3: Write minimal implementation**
Change `0644` to `0600` in `os.OpenFile`.

**Step 4: Run test to verify it passes**
Run: `go test -v internal/observability/audit_test.go`

**Step 5: Commit**
`git commit -am "security: restrict audit log file permissions to 0600"`
