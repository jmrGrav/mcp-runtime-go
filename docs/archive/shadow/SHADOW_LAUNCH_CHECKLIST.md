# Shadow Launch Checklist

## Pre-Launch

- [ ] Confirm `hugo-mcp-proxy.service` is active and serving the public path.
- [ ] Confirm `openresty.service` is active.
- [ ] Confirm `mcp-runtime-shadow.service` is installed but not yet authoritative.
- [ ] Confirm `/etc/mcp-runtime-go/mcp-runtime-shadow.env` exists and contains
      only approved shadow values.
- [ ] Confirm no secrets are printed in logs, docs, or shell history.
- [ ] Confirm the Go shadow port is available from the env file.
- [ ] Confirm the Python authoritative env file is untouched.

## Launch

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mcp-runtime-shadow.service
```

- [ ] Confirm `systemctl status mcp-runtime-shadow.service --no-pager` is active.
- [ ] Confirm `./scripts/healthcheck-shadow.sh` passes against the Go shadow URL.
- [ ] Confirm the edge mirror still routes public traffic to Python.

## Verify Python Remains Authority

```bash
systemctl status hugo-mcp-proxy.service --no-pager
systemctl status openresty.service --no-pager
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server'
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-protected-resource'
```

- [ ] Public responses still come from Python.
- [ ] Go does not replace the public route.

## Verify Go Is Shadow Only

```bash
PORT="$(awk -F= '/^LISTEN_PORT=/{print $2}' /etc/mcp-runtime-go/mcp-runtime-shadow.env)"
./scripts/healthcheck-shadow.sh "http://127.0.0.1:${PORT}"
journalctl -u mcp-runtime-shadow.service --no-pager -n 100
```

- [ ] Go health endpoints return 200.
- [ ] No crash loop is visible in the journal.
- [ ] Go audit entries appear only for shadow traffic.

## Verify Boundaries

```bash
sudo nginx -T 2>/dev/null | rg -n 'mirror|proxy_pass|mcp-runtime-shadow|hugo-mcp-proxy'
rg -n 'cloudflare|CLOUDFLARE|mutat|delete_page|update_page|create_page' . --glob '!vendor/**'
rg -n 'crowdsec|cscli|allowlist|ban|deban|decision|mutat' . --glob '!vendor/**'
rg -n 'AI_EXPLAIN|openai|anthropic|gemini|provider registry|llm' . --glob '!vendor/**'
```

- [ ] Cloudflare mutation path remains disabled.
- [ ] CrowdSec write boundary remains unchanged.
- [ ] No AI Explain runtime exists in this repository.

## Inspect MCP

```bash
PORT="$(awk -F= '/^LISTEN_PORT=/{print $2}' /etc/mcp-runtime-go/mcp-runtime-shadow.env)"
curl -fsS "http://127.0.0.1:${PORT}/healthz"
curl -fsS "http://127.0.0.1:${PORT}/readyz"
curl -fsS "http://127.0.0.1:${PORT}/.well-known/oauth-authorization-server"
curl -fsS "http://127.0.0.1:${PORT}/.well-known/oauth-protected-resource"
```

- [ ] MCP discovery endpoints respond.
- [ ] Read-only posture remains intact.

## Inspect Shadow Status

```bash
PY_AUDIT="/var/log/mcp-oauth/audit-hugo.log"
GO_AUDIT="/var/log/mcp-runtime-go/audit-shadow.jsonl"
REPORTS="/var/log/mcp-runtime-go/reports"
./scripts/shadow-status.sh "$PY_AUDIT" "$GO_AUDIT" "$REPORTS"
```

- [ ] Shadow status summary is readable.
- [ ] Go audit growth remains bounded.
- [ ] Python remains authoritative.

## Compare After Window

```bash
PY_AUDIT="/var/log/mcp-oauth/audit-hugo.log"
GO_AUDIT="/var/log/mcp-runtime-go/audit-shadow.jsonl"
REPORTS="/var/log/mcp-runtime-go/reports"
./scripts/shadow-compare-48h.sh "$PY_AUDIT" "$GO_AUDIT" "$REPORTS"
```

- [ ] No missing critical request IDs.
- [ ] No malformed JSON.
- [ ] No unexpected mismatches.

## Stop / Roll Back

```bash
sudo systemctl stop mcp-runtime-shadow.service
sudo systemctl disable mcp-runtime-shadow.service
sudo nginx -t
sudo systemctl reload openresty
```

- [ ] Go shadow is stopped.
- [ ] Python continues serving traffic.
- [ ] Edge mirror can be removed separately if needed.

## Final Operator Note

GO SHADOW.
