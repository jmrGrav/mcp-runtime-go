# BROOKS_REVIEW_ACTIONS.md

## Actions from Phase 2.5 Audit

### [FIXED] Critical: Shadow Matching
- **Issue**: time-based matching was fragile and collision-prone.
- **Fix**: Implemented `X-Request-ID` middleware and updated `shadow-compare` to use it as the primary join key.
- **Status**: Verified.

### [FIXED] Warning: JSON Store Durability
- **Issue**: Lack of `fsync` could lead to data loss on crash.
- **Fix**: Added `f.Sync()` before closing temp files and `df.Sync()` on the parent directory after rename.
- **Status**: Verified.

### [FIXED] Warning: Bloated Handlers
- **Issue**: `HandleToken` and `HandleAuthorize` were too complex.
- **Fix**: Extracted `validateAuthorizeRequest` and `authenticateClient` into separate methods.
- **Status**: Verified.

## Actions from Phase 2.6 Hardening

### [FIXED] Critical: Proxy Boundary Bypass
- **Issue**: Manual proxy logic was vulnerable to dot-segment and character-based bypass.
- **Fix**: Implemented `path.Clean` and strict character rejection in `HandleProxy`.
- **Status**: Verified with unit tests.

### [FIXED] Critical: Unreliable Shadow Comparison
- **Issue**: Fallback matching was collision-prone; missing IDs were silent.
- **Fix**: Enforced `request_id` as primary key; non-zero exit on missing critical IDs or duplicates.
- **Status**: Verified.

### [FIXED] Warning: Token Store Bottleneck
- **Issue**: Holding `tokensMu` during disk I/O.
- **Fix**: Implemented snapshot pattern to write outside the lock.
- **Status**: Verified with concurrency tests.

### [FIXED] Suggestion: Knowledge Duplication (Security)
- **Fix**: Centralized `GenerateRandomString` in `internal/security`.
- **Status**: Verified.

## Actions from Phase 2.8 Adversarial Remediation

### [FIXED] High: CSRF Vulnerability
- **Issue**: Missing `state` enforcement on `/authorize`.
- **Fix**: Mandatory `state` parameter check added to `validateAuthorizeRequest`.
- **Status**: Verified with tests.

### [FIXED] Medium: IP Spoofing
- **Issue**: Trusting `X-Forwarded-For` from untrusted sources.
- **Fix**: Implemented `TrustedProxies` whitelist for IP resolution in audit logs.
- **Status**: Verified.

### [FIXED] Medium: PKCE Downgrade
- **Issue**: PKCE was optional.
- **Fix**: Implemented `MandatoryPKCE` flag (default true) in configuration.
- **Status**: Verified.

### [FIXED] Medium: Coverage Gaps
- **Issue**: Background routines and metadata handlers were untested.
- **Fix**: Added comprehensive unit tests for `PurgeExpired` and Metadata handlers.
- **Status**: Verified (Coverage 38.7% -> 40.0%).

### [DEFERRED] Suggestion: Dynamic Cert Reload
- **Issue**: Certificate rotation requires a restart.
- **Reason for Deferral**: Low operational impact for the initial rollout. Restarting the service is currently acceptable.
- **Target**: Post-migration optimization.
