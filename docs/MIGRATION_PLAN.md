# MIGRATION_PLAN.md

## Phase 1: Foundation & OAuth Proxy
- [x] Initialize Go repository and architecture.
- [x] Implement `oauthproxy` domain logic with 1:1 behavioral parity.
- [x] Implement Shadow Mode logging.
- [x] Achieve parity on:
    - Metadata Discovery (RFC 8414, RFC 9728).
    - Dynamic Client Registration (RFC 7591).
    - Authorization Code Flow with PKCE S256.
    - Token storage (compatible with current JSON format).
    - **Backend TLS validation (MCP_CA_CERT).**

## Phase 2: Parity Validation
- [ ] Run Go in Shadow Mode alongside Python.
- [x] **Create shadow-compare tool.**
- [ ] Validate audit logs comparison (Pending real traffic).
- [ ] Perform security stress tests (invalid PKCE, path traversal, etc.).

## Phase 3: Transition
- [ ] Switch Go to authoritative mode for `mcp-oauth-proxy`.
- [ ] Decommission Python version after burn-in period.

## Phase 4: Future - Hugo MCP
- [ ] Integrate `hugo-mcp` as a second domain.
- [ ] Leverage existing `runtime` (Scheduler, SQLite, FSM) from `security-automation-go`.
