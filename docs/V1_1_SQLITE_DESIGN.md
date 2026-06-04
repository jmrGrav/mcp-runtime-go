# SQLite Storage Design Specification — v1.1

**Project:** mcp-runtime-go
**Status:** Production Blueprint
**Goal:** Replace synchronous JSON persistence with high-concurrency SQLite WAL.

---

## 1. Executive Summary

The current `TokenStore` implementation in `json_store.go` performs synchronous, blocking I/O (fsync) on the request path for every token issuance. This design provides durability but limits concurrency and reliability under load.

This specification proposes a **SQLite-backed storage engine** using **Write-Ahead Logging (WAL)** mode. This transition maintains existing durability guarantees while decoupling disk latency from the request path and enabling efficient $O(1)$ token validation.

---

## 2. Comparison: JSON vs. SQLite WAL

| Feature | Current JSON Store | Proposed SQLite WAL |
|---|---|---|
| **Concurrency** | Single-writer (Exclusive Lock) | Multi-reader, Single-writer (Parallel) |
| **I/O Pattern** | Full Rewrite + 2x fsync | Append-only (WAL) + Background Checkpoint |
| **Search Time** | $O(N)$ (requires full load) | $O(1)$ (indexed lookup) |
| **Memory Risk** | High (Entire store in RAM) | Low (Buffer cache only) |
| **Recovery** | Backup-and-Fresh (on corruption) | Robust (journaling/integrity checks) |

---

## 3. Schema Design

### Table: `access_tokens`
Stores the hashed access tokens and their expiration timestamps.

```sql
CREATE TABLE access_tokens (
    token_hash TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
) WITHOUT ROWID;

CREATE INDEX idx_access_tokens_expiry ON access_tokens(expires_at);
```

**Rationale:** Unix timestamps are stored as `INTEGER` to avoid floating-point conversion risk and align with Go `time.Unix()` and SQLite `strftime('%s')`.

### Table: `metadata`
Stores internal versioning and migration state.

```sql
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY
);
```

---

## 4. Concurrency & Locking Model

### WAL Mode (Write-Ahead Logging)
- **Parallelism:** Readers do not block writers; writers do not block readers.
- **Synchronous:** `PRAGMA synchronous = NORMAL;`
    - Provides durability while minimizing fsync calls on the critical path.
    - Transaction commits are written to the WAL file; periodic "checkpoints" move data to the main DB.
- **Journaling:** `PRAGMA journal_mode = WAL;`
- **Bloat Prevention:** `PRAGMA journal_size_limit = 10485760;` (10MB).

### Connection Management
- **Single Node:** The application maintains a single persistent connection pool.
- **Busy Timeout:** `PRAGMA busy_timeout = 5000;` (5 seconds).
- **Go Pool Tuning:** The `sql.DB` connection pool must be configured with `SetMaxOpenConns(1)` to avoid concurrent writer contention and `SQLITE_BUSY` errors in this single-node configuration.

---

## 5. Migration Strategy

### Zero-Downtime Transition
1. **Side-by-Side:** The new `TokenStore` implementation will attempt to open the SQLite database.
2. **One-Way Import:**
    - If `tokens.db` is empty AND `tokens.json` exists:
        - Load `tokens.json` into memory.
        - Start a single SQLite transaction.
        - Insert all valid tokens into `access_tokens`.
        - Commit.
        - **Atomic Sequence:** Rename `tokens.json` to `tokens.json.migrated` ONLY after the import transaction commits successfully AND either the SQLite connection is cleanly closed or `PRAGMA wal_checkpoint(TRUNCATE);` succeeds.
3. **Interface Parity:** The `internal/storage` package will provide a factory that returns an implementation of a new `Store` interface, ensuring zero changes to the `oauthproxy` package logic.

### Rollback Strategy
1. **Binary Revert:** If the SQLite binary is reverted to the JSON-capable version.
2. **JSON Recovery:** Rename `tokens.json.migrated` back to `tokens.json`. Any tokens issued *during* the SQLite window will be lost, but session continuity for the majority is preserved.

---

## 6. Purge & Retention Strategy

### Background Purge
- Instead of the hourly full-map purge in `oauthproxy/service.go`, the SQLite backend will handle expiration.
- **Operation:** `DELETE FROM access_tokens WHERE expires_at < strftime('%s', 'now');`
- **Schedule:** Run every 1 hour via a background goroutine.
- **Optimization:** Use `VACUUM` occasionally (e.g., once a week) if fragmentation is observed.

---

## 7. Backup & Recovery

### Backup Strategy
- **Online Backup:** Use `sqlite3_backup` API or simply copy the file while the application is running (WAL handles consistency).
- **Frequency:** Nightly file-level backup.

### Corruption Recovery
- **Integrity Check:** Run `PRAGMA integrity_check;` on startup.
- **Recovery:** If corrupted, move `tokens.db` to `tokens.db.corrupt` and start fresh. Since tokens are transient, the cost of a full reset is low (users simply re-authenticate).

---

## 8. Operational Risks

| Risk | Impact | Mitigation |
|---|---|---|
| **CGO Dependency** | High | Use a pure-Go SQLite driver (e.g., `modernc.org/sqlite`) to maintain zero-dependency builds. |
| **File Locking** | Medium | Ensure SQLite is only accessed by a single process; use WAL mode. |
| **Disk Space** | Low | WAL files can grow; set `journal_size_limit` to 10MB. |

---

## 9. Testing Strategy

1. **Interface Compliance:** Run existing `storage/json_store_test.go` logic against the SQLite implementation.
2. **Concurrency Test:** Simulate 50 parallel writers and 200 parallel readers to verify WAL performance.
3. **Migration Test:** Mock a large `tokens.json` (10k tokens) and verify the 100% accurate import to SQLite.
4. **Crash Consistency:** Kill the process during a write and verify no database corruption occurs.

---

## 10. Definition of Done

1. `Store` interface defined in `internal/storage`.
2. `SQLiteStore` implementation complete with WAL and WAL-checkpoint logic.
3. One-way migration logic from JSON to SQLite verified.
4. Concurrency and durability tests pass.
5. Documentation updated (ROADMAP.md, OPERATIONS.md).
6. Performance benchmark showing $>10x$ improvement in `POST /token` p99 latency.

---

## 11. Implementation Phases

1. **Phase 1:** Define `Store` interface and refactor `Service` to use it.
2. **Phase 2:** Implement `SQLiteStore` with pure-Go driver.
3. **Phase 3:** Implement migration logic.
4. **Phase 4:** Verify in Shadow Mode.

---

## 12. Brooks Storage Review Outcome

- **Verdict:** GO
- **Required Revisions Applied:** All five mandatory revisions (Schema types, Connection tuning, WAL limits, Migration atomicity, Retention precision) have been incorporated into this blueprint.
- **Remaining Implementation Caution:** Do not start SQLite coding until tests and a comprehensive migration plan are derived from this blueprint.
