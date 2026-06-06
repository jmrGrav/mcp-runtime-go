# Tree Hygiene Report

**Date:** 2026-06-03

## Repository Tree (post-cleanup)

```
mcp-runtime-go/
├── .gitignore
├── LICENSE
├── README.md
├── go.mod
│
├── cmd/
│   ├── mcp-runtime/main.go        — binary entry point
│   └── shadow-compare/            — audit log comparison tool
│       ├── main.go
│       └── compare_test.go
│
├── deploy/
│   ├── env/mcp-runtime-shadow.env.example     — env template (no real values)
│   ├── nginx/mcp-runtime-shadow-mirror.conf.example
│   └── systemd/mcp-runtime-shadow.service
│
├── docs/
│   ├── INDEX.md
│   ├── DOCUMENTATION_AUDIT.md
│   ├── TREE_HYGIENE_REPORT.md
│   ├── REPOSITORY_HYGIENE_REPORT.md
│   ├── architecture/
│   ├── deployment/
│   ├── migration/
│   ├── operations/
│   ├── testing/
│   └── archive/
│
├── internal/
│   ├── config/         — env-driven configuration
│   ├── context/        — request ID propagation
│   ├── httpserver/     — HTTP server lifecycle
│   ├── observability/  — structured audit logger
│   ├── oauthproxy/     — OAuth 2.0 domain (handlers, service, proxy)
│   ├── runtime/        — app wiring, middleware
│   ├── security/       — PKCE, redirect URI, random generation
│   └── storage/        — token store (JSON)
│
└── scripts/
    ├── healthcheck-shadow.sh
    ├── shadow-compare-48h.sh
    ├── shadow-status.sh
    └── test-all.sh
```

## Findings

### Items on disk but NOT tracked (correctly excluded by .gitignore)

| Path | Type | Status |
|---|---|---|
| `bin/mcp-runtime` | Compiled binary | Excluded by `bin/` in .gitignore ✓ |
| `bin/shadow-compare` | Compiled binary | Excluded by `bin/` in .gitignore ✓ |
| `/mcp-runtime` (root) | Old compiled binary | Excluded by `/mcp-runtime` in .gitignore ✓ |
| `/shadow-compare` (root) | Old compiled binary | Excluded by `/shadow-compare` in .gitignore ✓ |
| `coverage.out` | Test coverage output | Excluded by `coverage*.out` ✓ |
| `coverage_security.out` | Test coverage output | Excluded by `coverage*.out` ✓ |
| `.claude/` | Claude Code settings | Excluded by `.claude/` ✓ |

**Action:** None required. All artifacts are correctly excluded.

### Empty directories created for structure

| Directory | Purpose |
|---|---|
| `docs/security/` | Reserved for future security documentation |

**Recommendation:** The `security/` directory is empty. Either add a placeholder or remove it.
Decision: remove it — no content yet and INDEX.md documents the security approach inline.

### `migrations/` directory

The full file listing from the initial inventory showed a `migrations/` directory entry. Confirmed absent from tracked files — does not exist.

### scripts/ — all retained

| Script | Purpose | Keep |
|---|---|---|
| `healthcheck-shadow.sh` | Shadow health probe | ✓ |
| `shadow-compare-48h.sh` | Comparison runner | ✓ |
| `shadow-status.sh` | Status summary | ✓ |
| `test-all.sh` | Test suite runner | ✓ |

No stale scripts identified.

### deploy/ — all retained

All three deploy artifacts use placeholders only, no real values. All correct.

## Recommendations

| Item | Recommendation | Priority |
|---|---|---|
| `docs/security/` empty dir | Remove (security docs inline in source) | Low |
| Root-level old binaries (`/mcp-runtime`, `/shadow-compare`) | Delete from filesystem (not tracked, just waste space) | Low |
| `bin/` directory binaries | Delete from filesystem after each rebuild | Low |
| `coverage*.out` files | Add to `make clean` target when Makefile is added | Low |
