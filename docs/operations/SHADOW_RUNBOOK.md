# Shadow Runbook

## Purpose

Run the Go runtime in shadow mode while the Python service remains authoritative.
This runbook is intentionally read-only: no new providers, no mutation paths, no
runtime refactor, and no new `shadow-status` mode.

## Services Involved

- `hugo-mcp-proxy.service` - Python authoritative service. It must remain the
  source of truth for public traffic.
- `openresty.service` - edge proxy / mirror layer.
- `mcp-runtime-shadow.service` - Go shadow service.
- `mcp-oauth-proxy.service` - legacy service, if still present, is out of scope
  for this shadow launch.

## Environment Files

- `/etc/mcp-runtime-go/mcp-runtime-shadow.env` - Go shadow runtime env file.
- `/home/jm/hugo-mcp/.env` - Python authoritative service env file, if that is
  the host's installed Python deployment.
- `/etc/mcp-oauth-proxy/secrets.env` - legacy Python env file, only if the host
  still uses the legacy `mcp-oauth-proxy.service`.

## Build and Install

```bash
go build ./cmd/mcp-runtime
go build ./cmd/shadow-compare

install -m 0755 ./bin/mcp-runtime /usr/local/bin/mcp-runtime
install -m 0755 ./bin/shadow-compare /usr/local/bin/shadow-compare
```

## Shadow Start

```bash
sudo install -m 0640 deploy/env/mcp-runtime-shadow.env.example /etc/mcp-runtime-go/mcp-runtime-shadow.env
sudo install -m 0644 deploy/systemd/mcp-runtime-shadow.service /etc/systemd/system/mcp-runtime-shadow.service
sudo systemctl daemon-reload
sudo systemctl enable --now mcp-runtime-shadow.service
```

If you need the shadow port dynamically, read it from the env file:

```bash
PORT="$(awk -F= '/^LISTEN_PORT=/{print $2}' /etc/mcp-runtime-go/mcp-runtime-shadow.env)"
```

## Shadow Stop

```bash
sudo systemctl stop mcp-runtime-shadow.service
sudo systemctl disable mcp-runtime-shadow.service
```

If rollback is needed, also remove the installed unit and env file only after
the shadow is fully stopped:

```bash
sudo rm -f /etc/systemd/system/mcp-runtime-shadow.service
sudo rm -f /etc/mcp-runtime-go/mcp-runtime-shadow.env
sudo systemctl daemon-reload
```

## Verify Python Remains Authoritative

```bash
systemctl status hugo-mcp-proxy.service --no-pager
systemctl status openresty.service --no-pager
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server'
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-protected-resource'
```

If the host still carries the legacy Python proxy, verify it is not the public
authoritative unit:

```bash
systemctl status mcp-oauth-proxy.service --no-pager || true
```

## Verify Go Remains Read-Only Shadow

```bash
PORT="$(awk -F= '/^LISTEN_PORT=/{print $2}' /etc/mcp-runtime-go/mcp-runtime-shadow.env)"
./scripts/healthcheck-shadow.sh "http://127.0.0.1:${PORT}"
systemctl status mcp-runtime-shadow.service --no-pager
journalctl -u mcp-runtime-shadow.service --no-pager -n 100
```

## Verify Cloudflare Mutations Stay Disabled

This repository does not ship a Cloudflare mutator. The operational check is to
confirm the edge still points public traffic at the Python service and mirrors
to Go only:

```bash
sudo nginx -T 2>/dev/null | rg -n 'mirror|proxy_pass|mcp-runtime-shadow|hugo-mcp-proxy'
rg -n 'cloudflare|CLOUDFLARE|mutat|delete_page|update_page|create_page' . --glob '!vendor/**'
```

## Verify CrowdSec Write Boundary Is Unchanged

This repository does not add a CrowdSec writer. The operational check is to
confirm no new CrowdSec write path exists in the repo checkout:

```bash
rg -n 'crowdsec|cscli|allowlist|ban|deban|decision|mutat' . --glob '!vendor/**'
```

## Inspect the UI

The Go runtime does not provide a separate UI service in this repository. Inspect
the authoritative and shadow endpoints instead:

```bash
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-authorization-server'
curl -skI 'https://mcp-hugo.arleo.eu/.well-known/oauth-protected-resource'
```

## Inspect MCP

```bash
PORT="$(awk -F= '/^LISTEN_PORT=/{print $2}' /etc/mcp-runtime-go/mcp-runtime-shadow.env)"
curl -fsS "http://127.0.0.1:${PORT}/healthz"
curl -fsS "http://127.0.0.1:${PORT}/readyz"
curl -fsS "http://127.0.0.1:${PORT}/.well-known/oauth-authorization-server"
curl -fsS "http://127.0.0.1:${PORT}/.well-known/oauth-protected-resource"
```

## Inspect AI Explain State

There is no AI Explain runtime in this repository. The correct operator check is
negative proof:

```bash
rg -n 'AI_EXPLAIN|openai|anthropic|gemini|provider registry|llm' . --glob '!vendor/**'
```

If that search returns no runtime implementation, the AI Explain system is not
present and remains out of scope for shadow launch.

## Shadow Status

There is no new `shadow-status` mode in this repository. Use the existing helper
instead:

```bash
PY_AUDIT="/var/log/mcp-oauth/audit-hugo.log"
GO_AUDIT="/var/log/mcp-runtime-go/audit-shadow.jsonl"
REPORTS="/var/log/mcp-runtime-go/reports"
./scripts/shadow-status.sh "$PY_AUDIT" "$GO_AUDIT" "$REPORTS"
```

## Metrics to Watch

- `journalctl -u mcp-runtime-shadow.service`
- Go audit log growth in `/var/log/mcp-runtime-go/audit-shadow.jsonl`
- Python audit log growth in `/var/log/mcp-oauth/audit-hugo.log`
- `./scripts/shadow-status.sh` output
- `./scripts/shadow-compare-48h.sh` parity output

## Logs to Watch

- `journalctl -u mcp-runtime-shadow.service --no-pager -n 100`
- `journalctl -u openresty.service --no-pager -n 100`
- `journalctl -u hugo-mcp-proxy.service --no-pager -n 100`
- `/var/log/mcp-oauth/audit-hugo.log`
- `/var/log/mcp-runtime-go/audit-shadow.jsonl`

## Rollback Procedure

Use the existing rollback steps:

```bash
sudo systemctl stop mcp-runtime-shadow.service
sudo systemctl disable mcp-runtime-shadow.service
sudo nginx -t
sudo systemctl reload openresty
```

If you need to fully remove the shadow install:

```bash
sudo rm -f /etc/systemd/system/mcp-runtime-shadow.service
sudo rm -f /etc/mcp-runtime-go/mcp-runtime-shadow.env
sudo systemctl daemon-reload
```

Then verify Python remains authoritative:

```bash
systemctl status hugo-mcp-proxy.service --no-pager
systemctl status openresty.service --no-pager
```

## Final Operator Note

GO SHADOW.
