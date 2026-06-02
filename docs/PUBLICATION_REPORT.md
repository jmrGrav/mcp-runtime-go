# Publication Report — mcp-runtime-go

**Date:** 2026-06-02
**Verdict:** PUBLICATION COMPLETE

---

## Publication Audit Results

See [PUBLICATION_AUDIT.md](PUBLICATION_AUDIT.md) for the full audit.

**Summary:**
- No secrets, credentials, API keys, private keys, or tokens found in any source file
- All configuration is environment-variable driven; env templates use `<placeholder>` values only
- Compiled binaries excluded via corrected `.gitignore` (anchored root patterns `/mcp-runtime`, `/shadow-compare`)
- Coverage files, `.claude/` directory, runtime data directories excluded
- Docs containing public hostname (`mcp-hugo.arleo.eu`) and server paths are operational context — not secrets

**Files excluded from publication:**
`bin/`, `/mcp-runtime`, `/shadow-compare`, `coverage*.out`, `*.env`, `logs/`, `reports/`, `audit*.jsonl`, `tokens*.json`, `*.db`, `*.sqlite`, `tmp/`, `backups/`, `.claude/`

---

## Go Repository — mcp-runtime-go

| Field | Value |
|---|---|
| URL | https://github.com/jmrGrav/mcp-runtime-go |
| Visibility | PUBLIC |
| Default branch | main |
| Initial commit | `3268718` |
| Commit signature | GPG-verified (key `33133FBFAFFFCA48AFFD3953E34BC7955D46431A`) |
| Files published | 76 files, 6403 lines |
| Topics | go, oauth, oauth2, mcp, model-context-protocol, proxy, security, shadow-deployment, openresty |

### Push Result

```
To https://github.com/jmrGrav/mcp-runtime-go.git
 * [new branch]      main -> main
```

---

## Python Repository — mcp-oauth-proxy

| Field | Value |
|---|---|
| URL | https://github.com/jmrGrav/mcp-oauth-proxy |
| Commit | `bea8774` |
| Commit signature | GPG-verified |
| Changes | README.md (Project Status section added), docs/MIGRATION_TO_GO.md (new) |

### Changes Made

**README.md** — added `## Project Status` section:
- States Python remains maintained and production-authoritative
- Links to `https://github.com/jmrGrav/mcp-runtime-go`
- Lists Go runtime advantages
- References `docs/MIGRATION_TO_GO.md`

**docs/MIGRATION_TO_GO.md** (new):
- Why Go was created
- Migration strategy (5 phases, current phase: shadow deployment)
- Shadow deployment detail (mirror architecture, comparison tool, gate criteria)
- Current recommendation table (Python authoritative now, Go for new work)

---

## GitHub Quality Checks

| Check | Result |
|---|---|
| Repository is public | ✓ |
| Description set | ✓ |
| Topics set (9) | ✓ |
| LICENSE present | ✓ (MIT) |
| README.md present | ✓ |
| No broken internal links | ✓ |
| No secrets in any committed file | ✓ |
| Commits GPG-signed | ✓ |
| Correct commit email (no-reply) | ✓ |

---

## Remaining Manual Tasks

- None required for publication.
- Optional future improvements:
  - Add GitHub Actions CI workflow (`go test ./...`, `go vet ./...`)
  - Add a `CONTRIBUTING.md`
  - Tag a `v1.0.0` release once Go passes the shadow gate and becomes authoritative

---

## Final Verdict

**PUBLICATION COMPLETE**

- `https://github.com/jmrGrav/mcp-runtime-go` — live, public, GPG-signed
- `https://github.com/jmrGrav/mcp-oauth-proxy` — updated with migration reference
- No secrets exposed. Both projects preserved. No Python deprecation.
