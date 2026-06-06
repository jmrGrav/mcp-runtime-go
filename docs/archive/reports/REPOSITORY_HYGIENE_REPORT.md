# Repository Hygiene Report

**Date:** 2026-06-03
**Verdict:** CLEAN WITH RECOMMENDATIONS

---

## Markdown Files Reviewed

35 markdown files reviewed total (including README.md).

| Disposition | Count |
|---|---|
| Kept — moved to structured subfolder | 9 |
| Archived to `docs/archive/` | 26 |
| Newly created (this session) | 4 (INDEX.md, DOCUMENTATION_AUDIT.md, TREE_HYGIENE_REPORT.md, this file) |

## Files Kept (Permanent Documentation)

Moved to structured subfolders under `docs/`:

| File | New Location |
|---|---|
| `ARCHITECTURE.md` | `docs/architecture/ARCHITECTURE.md` |
| `FUTURE_HUGO_MCP_INTEGRATION.md` | `docs/architecture/HUGO_MCP_INTEGRATION.md` |
| `SHADOW_MODE.md` | `docs/deployment/SHADOW_MODE.md` |
| `ROLLBACK.md` | `docs/operations/ROLLBACK.md` |
| `MIGRATION_PLAN.md` | `docs/migration/MIGRATION_PLAN.md` |
| `OAUTH_PROXY_PARITY.md` | `docs/migration/OAUTH_PROXY_PARITY.md` |
| `COVERAGE_POLICY.md` | `docs/testing/COVERAGE_POLICY.md` |
| `operations/SHADOW_LAUNCH_CHECKLIST.md` | unchanged |
| `operations/SHADOW_RUNBOOK.md` | unchanged |

## Files Archived

All phase reports (Phase 2, Phase 3, Phase 4), historical audit evidence, publication records, and tooling-specific reports moved to `docs/archive/`. See [DOCUMENTATION_AUDIT.md](DOCUMENTATION_AUDIT.md) for the full list with justifications.

## Files Recommended for Deletion

None. No files were deleted. Archive is preferred over deletion for traceability.

The following items exist on disk but are correctly excluded from git and can be safely deleted from the filesystem when needed:

| Path | Reason |
|---|---|
| `/mcp-runtime` (root) | Old compiled binary, superseded by `bin/mcp-runtime` |
| `/shadow-compare` (root) | Old compiled binary, superseded by `bin/shadow-compare` |

## Tree Improvements

| Improvement | Done |
|---|---|
| Created `docs/architecture/` | ✓ |
| Created `docs/deployment/` | ✓ |
| Created `docs/operations/` (pre-existing) | ✓ |
| Created `docs/testing/` | ✓ |
| Created `docs/migration/` | ✓ |
| Created `docs/archive/` | ✓ |
| Removed empty `docs/security/` | ✓ |
| 26 phase reports moved to archive | ✓ |

## README Improvements

| Improvement | Done |
|---|---|
| Removed verbose config table (belongs in deploy docs) | ✓ |
| Added Documentation section → INDEX.md | ✓ |
| Architecture section tightened to essentials | ✓ |
| Shadow deployment strategy section clarified | ✓ |
| Security model section made concise | ✓ |
| Current status table updated | ✓ |

## Git Hygiene Status

### .gitignore Coverage

| Pattern | Present | Coverage |
|---|---|---|
| `.env` | ✓ | Environment files |
| `*.env` | ✓ | All env files |
| `!*.env.example` | ✓ | Example files still tracked |
| `logs/` | ✓ | Runtime logs |
| `reports/` | ✓ | Shadow comparison reports |
| `audit*.jsonl` | ✓ | Audit log files |
| `tokens*.json` | ✓ | Token persistence files |
| `*.db` / `*.sqlite` | ✓ | Database files |
| `coverage*.out` | ✓ | Test coverage output |
| `tmp/` / `backups/` | ✓ | Temporary files |
| `.claude/` | ✓ | Claude Code workspace |
| `bin/` | ✓ | Compiled binaries directory |
| `/mcp-runtime` | ✓ | Root-level binary (anchored) |
| `/shadow-compare` | ✓ | Root-level binary (anchored) |

**All required patterns present. No gaps found.**

### Secrets scan

- Go source files: 0 hardcoded secrets
- Deploy templates: placeholders only
- Docs: no credentials, tokens, or private keys
- Archive docs: no credentials (operational paths and hostnames only)

## Final Verdict

**CLEAN WITH RECOMMENDATIONS**

The repository is publication-safe with good structure after this cleanup. Remaining recommendations:

1. **Delete root-level old binaries** (`/mcp-runtime`, `/shadow-compare`) from the filesystem — they are not tracked but occupy space.
2. **Add a `Makefile`** with `build`, `test`, `clean` targets — would improve developer UX and handle `coverage*.out` cleanup.
3. **Add GitHub Actions CI** (`.github/workflows/ci.yml`) — `go test ./...`, `go vet ./...`, `go build ./...` on push/PR.
4. **Tag `v1.0.0`** after Go becomes authoritative — the current codebase quality warrants a release tag.
