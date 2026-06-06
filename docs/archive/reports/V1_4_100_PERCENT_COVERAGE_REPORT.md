# V1_4_100_PERCENT_COVERAGE_REPORT.md

**Date:** 2026-06-06
**Branch:** `v1.4-cleanup-packaging`

## Outcome

- **Coverage before:** 97.3%
- **Coverage after:** 100.0%
- **Verdict:** **A) 100% COVERAGE ACHIEVED**

## What Changed

### Code removed

- Removed the dead `sql.Open("sqlite", path)` error branch from `internal/storage/sqlite_store.go`.

### Refactorings for testability

- `cmd/mcp-runtime/main.go`
  - Added injectable hooks for config loading, app creation, migration, stderr, and exit.
  - This made `main()` and the non-migration `run()` path testable without subprocess gymnastics.
- `internal/oauthproxy/service.go`
  - Added an injectable purge ticker factory so `StartPurgeLoop` can be exercised without waiting an hour.
- `internal/observability/audit.go`
  - Added an injectable write hook so audit write and newline failures can be forced deterministically.
- `internal/storage/json_store.go`
  - Added injectable directory-open and directory-sync hooks to cover parent-fsync error paths.
- `internal/storage/migration.go`
  - Added a storage factory seam and a `Checkpoint()` abstraction so migration failure modes can be tested cleanly.
- `internal/storage/sqlite_store.go`
  - Added `Checkpoint()` and injectable SQLite init helpers for schema/error-path coverage.

### New tests added

- `cmd/mcp-runtime/main_test.go`
  - Covers `main()` exit handling, `run()` success, migration success/error, app init error, app run error, and the real `newAppFn` path.
- `internal/config/config_test.go`
  - Covers `Load()` validation failures and reflection binding of untagged fields.
- `internal/oauthproxy/service_test.go`
  - Covers `NewService` load errors, `Ready()` load errors, purge-loop tick and cancel paths, `IssueAuthCode` RFC failures, `ExchangeToken` RFC failures, and error mapping defaults.
- `internal/oauthproxy/handlers_test.go`
  - Covers `client_request_id` audit propagation.
- `internal/oauthproxy/proxy_test.go`
  - Covers missing bearer auth, invalid bearer auth, missing backend config, and proxy transport failures.
- `internal/observability/audit_test.go`
  - Covers `RemoteAddr` fallback, JSON marshal failure, write failure, and newline failure.
- `internal/runtime/app_test.go`
  - Covers log-level branches, SQLite startup branch, invalid backend URL, and server start failure.
- `internal/storage/json_store_test.go`
  - Covers corruption recovery failure, rename failure, parent open/sync failures, marshal failure, and write/fsync errors.
- `internal/storage/migration_test.go`
  - Covers migration load/save/checkpoint/close/rename failures using a fake store seam.
- `internal/storage/sqlite_store_test.go`
  - Covers pragma error, schema error, query error, scan error, rows error, delete/prepare/insert save failures, and the existing happy path.

## Validation Results

- `go test ./...`
  - PASS
- `go test -race ./...`
  - PASS
- `go vet ./...`
  - PASS
- `govulncheck ./...`
  - PASS
  - Output: `No vulnerabilities found.`

## Complexity Reduction

- Removed one unreachable branch instead of testing around it.
- Replaced a few fragile filesystem assumptions with narrow, explicit seams.
- Kept the production behavior unchanged while making the error paths observable under test.

## Risks

- The new seams are internal testability hooks only; they do not change runtime behavior, but they do add a small amount of indirection in low-level infrastructure code.
- Some storage and audit tests rely on injected failure hooks rather than real OS faults. That is intentional and keeps the coverage real without turning the suite into flaky environment-dependent tests.

