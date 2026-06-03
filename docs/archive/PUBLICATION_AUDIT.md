# Publication Audit — mcp-runtime-go

**Date:** 2026-06-02
**Auditor:** pre-publication automated scan

## Audit Summary

**Publication status: SAFE**

No secrets, credentials, tokens, or private keys were found in the publishable file set.

---

## Files Excluded from Publication (via .gitignore)

| Pattern | Reason |
|---|---|
| `bin/` | Compiled binaries |
| `mcp-runtime` (root) | Old compiled binary |
| `shadow-compare` (root) | Old compiled binary |
| `coverage*.out` | Test coverage output |
| `*.env`, `.env` | Environment files with real values |
| `logs/` | Runtime log files |
| `reports/` | Shadow comparison reports |
| `audit*.jsonl` | Structured audit logs (operational data) |
| `tokens*.json` | Token persistence files |
| `*.db`, `*.sqlite` | Database files |
| `tmp/`, `backups/` | Temporary and backup files |
| `.claude/` | Claude Code workspace settings |

## Secrets Scan Results

### Go source files (`*.go`)
- Scanned: all files in `cmd/` and `internal/`
- Hardcoded secrets found: **0**
- All configuration is read from environment variables at runtime
- No API keys, tokens, passwords, or private keys in source

### Deploy files
- `deploy/env/mcp-runtime-shadow.env.example`: contains only `<placeholder>` values, no real credentials
- `deploy/systemd/mcp-runtime-shadow.service`: references env file path, no real values
- `deploy/nginx/mcp-runtime-shadow-mirror.conf.example`: uses `example.org`, no real values

### Documentation
- Docs reference `mcp-hugo.arleo.eu` — this is the public-facing domain name, not a secret
- Docs reference server filesystem paths (e.g. `/usr/local/openresty/...`) — operational context, not credentials
- No tokens, passwords, client secrets, or private keys appear in any document

### Binary files
- Two root-level compiled binaries (`mcp-runtime`, `shadow-compare`) identified and excluded via `.gitignore`
- `bin/` directory with built binaries excluded via `.gitignore`

## Files Safe to Publish

- All Go source (`cmd/`, `internal/`)
- `deploy/env/*.example`, `deploy/nginx/*.example`, `deploy/systemd/`
- `scripts/` (shell scripts — no hardcoded credentials)
- `docs/` (architecture, migration, phase reports — operational context only)
- `go.mod`
- `.gitignore`
- `README.md`
- `LICENSE`

## Conclusion

Publication is **APPROVED**. No secrets removal was required.

The `.gitignore` was rewritten from a broken state (listed source directories as ignored) to correct patterns that exclude binaries, coverage data, environment files, and runtime artifacts.
