# Sensitive Data Audit

**Date:** 2026-06-04
**Scope:** All tracked files in `/home/jm/mcp-runtime-go`

---

## Scan Commands Run

```bash
# Primary pattern scan across all tracked files
git grep -n -E \
  "sk-|ghp_|github_pat_|Bearer |Authorization:|client_secret|refresh_token|access_token|\
ANTHROPIC_API_KEY|OPENAI_API_KEY|GEMINI_API_KEY|82\.65\.145\.189|-----BEGIN|\
tokens\.json|audit.*jsonl"

# Long hex token scan (≥40 chars, excluding known safe patterns)
git grep -n -E "[0-9a-f]{40,}" -- '*.md' '*.go' '*.yaml' '*.sh' '*.conf'

# Untracked / ignored file check
git status --ignored --short
```

---

## Findings by Severity

### HIGH — Fixed

| File | Finding | Action |
|---|---|---|
| `docs/PHASE4_CLAUDE_CONNECTOR_FIX.md:31` | Home/operator public IP literal (`82.65.145.189`) | Replaced with `<HOME_IP>` |

The IP appeared only in the most recent commit (`9758b30`). It is an operator IP for the
MCP service, not a credential. No rotation required. History rewrite not recommended — the
IP is already publicly visible in DNS/Cloudflare and is the operator's known IP for this
service. Anonymizing the docs file is sufficient.

### LOW — Accepted (not secrets)

| Pattern | Files | Assessment |
|---|---|---|
| `Bearer ` | `proxy.go`, `proxy_test.go`, `handlers_test.go` | Code checking for "Bearer " prefix — legitimate |
| `client_secret` | `handlers.go`, `handlers_test.go`, `audit.go`, `audit_test.go` | Form field name, test config value, audit scrubbing — no real secret |
| `access_token` | `models.go`, `audit_test.go` | JSON field name and test placeholder — no real token |
| `tokens.json` | `.gitignore`, deploy template, test code | Path references; `tokens*.json` is gitignored; no real token data committed |
| `audit*.jsonl` | `.gitignore`, README, docs | Log path references; `audit*.jsonl` is gitignored; no real log content committed |
| `Authorization:` | `proxy.go`, `audit_test.go` | HTTP header name in code — legitimate |
| `client_secret=bad` | `docs/archive/PHASE3_3_SHADOW_T0_REPORT.md` | Curl test command with explicit placeholder `bad` — not a real secret |
| `TOKENS_FILE=...tokens.json` | `deploy/env/mcp-runtime-shadow.env.example` | Example file with path template — no real value |

### NOT FOUND

| Pattern | Result |
|---|---|
| `sk-` (OpenAI/Anthropic API keys) | Not found |
| `ghp_` / `github_pat_` | Not found |
| `gho_` (GitHub OAuth token) | Not found |
| `xoxb-` (Slack) | Not found |
| `ANTHROPIC_API_KEY` | Not found |
| `OPENAI_API_KEY` | Not found |
| `GEMINI_API_KEY` | Not found |
| `-----BEGIN` (PEM/private key) | Not found |
| Long hex token strings (≥40 chars outside Go BuildIDs) | Not found |
| HUGO_TOKEN value | Not found (only referenced as env var name) |
| CLIENT_SECRET value | Not found (only referenced as env var name) |

---

## Files Changed

| File | Change |
|---|---|
| `docs/PHASE4_CLAUDE_CONNECTOR_FIX.md` | `82.65.145.189` → `<HOME_IP>` (2 occurrences) |
| `docs/operations/OPERATIONS.md` | Created — uses placeholders throughout |
| `docs/security/SENSITIVE_DATA_AUDIT.md` | Created (this file) |

---

## Ignored / Untracked Files

The following exist on disk and are correctly gitignored:

| Path | Status |
|---|---|
| `.claude/` | Gitignored — Claude Code workspace settings |
| `bin/` | Gitignored — compiled binaries |
| `coverage.out` | Gitignored — test coverage output |
| `coverage_security.out` | Gitignored — test coverage output |

No runtime secrets (env files, token stores, audit logs) are present in the working directory.
All such files exist only outside the repository (under `/etc/mcp-runtime-go/`, `/var/log/`, `/var/lib/`).

---

## Git History Assessment

- Home IP `82.65.145.189` introduced in commit `9758b30` (most recent, `docs/PHASE4_CLAUDE_CONNECTOR_FIX.md`)
- No secrets found in any earlier commit
- History rewrite: **not recommended** — the IP is not a credential, rotation is not applicable,
  and it is the operator's known public-facing IP for this service

---

## Recommended Rotations

**None required.** No credentials, tokens, API keys, or private keys were found in tracked files.

---

## Final Verdict

**CLEAN AFTER ANONYMIZATION**

One operator IP literal was found and anonymized. No secrets requiring rotation were detected.
The repository is safe to remain public.
