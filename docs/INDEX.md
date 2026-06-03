# Documentation Index

## Architecture

| Document | Description |
|---|---|
| [Architecture](architecture/ARCHITECTURE.md) | Module layout, core principles, package responsibilities |
| [Hugo MCP Integration](architecture/HUGO_MCP_INTEGRATION.md) | Future multi-domain roadmap (Hugo MCP second domain) |

## Deployment

| Document | Description |
|---|---|
| [Shadow Mode](deployment/SHADOW_MODE.md) | Shadow deployment strategy, criteria, infrastructure setup |

## Operations

| Document | Description |
|---|---|
| [Rollback](operations/ROLLBACK.md) | Rollback procedure for cutover reversal |
| [Shadow Launch Checklist](operations/SHADOW_LAUNCH_CHECKLIST.md) | Pre-launch checklist for shadow deployment |
| [Shadow Runbook](operations/SHADOW_RUNBOOK.md) | Step-by-step shadow monitoring runbook |

## Security

Security-relevant design is documented inline in the source:
- PKCE S256: `internal/security/pkce.go`
- Redirect URI validation: `internal/security/redirect_uri.go`
- Token hashing: `internal/oauthproxy/service.go`
- Audit logging: `internal/observability/audit.go`

See also: [OAuth Proxy Parity](migration/OAUTH_PROXY_PARITY.md) for security feature comparison with Python.

## Testing

| Document | Description |
|---|---|
| [Coverage Policy](testing/COVERAGE_POLICY.md) | Test coverage standards and enforcement rules |

## Migration

| Document | Description |
|---|---|
| [Migration Plan](migration/MIGRATION_PLAN.md) | Phase-by-phase migration roadmap from Python to Go |
| [OAuth Proxy Parity](migration/OAUTH_PROXY_PARITY.md) | Feature-by-feature parity table (Python ↔ Go) |

## Archive

Historical phase reports and point-in-time audit evidence are stored in [`docs/archive/`](archive/).
These are preserved for traceability but are not part of the living documentation.

Notable archive entries:
- `PHASE2_*.md` — Parity validation and security hardening phases
- `PHASE3_*.md` — Shadow deployment phases
- `PHASE4_CUTOVER_REPORT.md` — First cutover attempt (BLOCKED — shadow gate not passed)
- `PUBLICATION_*.md` — Initial GitHub publication audit
