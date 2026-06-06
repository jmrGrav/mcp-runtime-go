# Documentation Cleanup Plan

Date: 2026-06-06

## Goal

Keep only four live documentation pages plus the README:

1. `README.md`
2. `docs/ARCHITECTURE.md`
3. `docs/OPERATIONS.md`
4. `docs/ROADMAP.md`

Everything else should live under `docs/archive/` unless it is explicitly kept as a live reference.

## Resulting Classification

| Source | Destination | Action | Justification |
|---|---|---|---|
| `README.md` | `README.md` | keep | Short entry point, quick start, live links |
| `docs/ARCHITECTURE.md` | `docs/ARCHITECTURE.md` | keep | Live architecture reference |
| `docs/OPERATIONS.md` | `docs/OPERATIONS.md` | keep | Live operator runbook |
| `docs/ROADMAP.md` | `docs/ROADMAP.md` | keep | Live status and future work |
| `docs/architecture/*.md` | `docs/archive/architecture/` | archive | Historical architecture material |
| `docs/migration/*.md` | `docs/archive/migration/` | archive | Historical migration material |
| `docs/operations/ROLLBACK*.md` | `docs/archive/operations/` | archive | Historical rollback notes after merging current procedure into `OPERATIONS.md` |
| `docs/operations/SHADOW_*.md` | `docs/archive/shadow/` | archive | Shadow-era runbooks no longer apply to production |
| `docs/deployment/SHADOW_MODE.md` | `docs/archive/shadow/` | archive | Shadow-era deployment reference |
| `docs/plans/*.md` | `docs/archive/plans/` | archive | Temporary planning notes with no live operator value |
| `docs/security/*.md` | `docs/archive/security/` | archive | Security audit history, not live operator guidance |
| `docs/testing/*.md` | `docs/archive/testing/` | archive | Historical testing policy / coverage policy notes |
| `docs/*_REVIEW.md` | `docs/archive/reviews/` | archive | Review artifacts are historical once merged |
| `docs/*_AUDIT.md` | `docs/archive/reports/` or a more specific archive dir | archive | Audit artifacts are historical evidence, not live guidance |
| `docs/*_REPORT.md` | `docs/archive/reports/` or a more specific archive dir | archive | Report artifacts are historical evidence, not live guidance |
| `docs/*_VALIDATION.md` | `docs/archive/reports/` | archive | Validation artifacts are historical evidence |
| `docs/INDEX.md` | `docs/archive/reports/` | archive | Superseded by the four live docs |
| `docs/V1_4_COVERAGE_STASH_ANALYSIS.md` | `docs/archive/reports/` | archive | Temporary analysis completed and no longer live |
| `BROOKS_*.md` | `docs/archive/reviews/` or `docs/archive/reports/` | archive | Review material only, not live guidance |

## Notes

- The current live docs now explain the repository directly without needing a sprawling index.
- Shadow-era documentation is preserved only as archive history.
- Migration and validation reports remain available under archive paths if an operator needs them.
- If a report still contains an actionable procedure, that procedure should be merged into `docs/OPERATIONS.md` first and only then archived.

## Current Live Docs

- [README.md](../README.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)
- [OPERATIONS.md](OPERATIONS.md)
- [ROADMAP.md](ROADMAP.md)

## Current Archive Layout

- `docs/archive/architecture/`
- `docs/archive/migration/`
- `docs/archive/operations/`
- `docs/archive/plans/`
- `docs/archive/reports/`
- `docs/archive/reviews/`
- `docs/archive/security/`
- `docs/archive/shadow/`
- `docs/archive/testing/`

