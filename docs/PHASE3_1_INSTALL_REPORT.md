# Phase 3.1 Install and Start Shadow Mode

## Outcome

SHADOW INSTALLED WITHOUT MIRROR

## What Was Installed

- User: `mcp-runtime`
- Directories:
  - `/etc/mcp-runtime-go`
  - `/var/lib/mcp-runtime-go`
  - `/var/log/mcp-runtime-go`
  - `/var/log/mcp-runtime-go/reports`
- Binaries:
  - `/usr/local/bin/mcp-runtime`
  - `/usr/local/bin/shadow-compare`
- Systemd unit:
  - `/etc/systemd/system/mcp-runtime-shadow.service`
- Shadow env:
  - `/etc/mcp-runtime-go/mcp-runtime-shadow.env`

## Port Choice

- `8084` was already occupied by an unrelated active shadow service (`cf-shadow.service`), so this install used `127.0.0.1:8085` for the Go shadow.

## Commands Executed

```bash
pwd
git status --short --branch
./scripts/test-all.sh
go test -race ./...
go vet ./...
go build ./cmd/mcp-runtime
go build ./cmd/shadow-compare
shellcheck scripts/*.sh
systemctl status hugo-mcp-proxy.service --no-pager
systemctl status cf-shadow.service --no-pager
ss -ltnp | rg ':8084\\b|:8085\\b'
sudo useradd --system --home-dir /var/lib/mcp-runtime-go --create-home --shell /usr/sbin/nologin mcp-runtime
sudo install -d -o mcp-runtime -g mcp-runtime /var/lib/mcp-runtime-go /var/log/mcp-runtime-go /var/log/mcp-runtime-go/reports
sudo install -d -o root -g root -m 0750 /etc/mcp-runtime-go
sudo install -o root -g root -m 0600 /dev/stdin /etc/mcp-runtime-go/mcp-runtime-shadow.env
sudo install -o root -g root -m 0644 deploy/systemd/mcp-runtime-shadow.service /etc/systemd/system/mcp-runtime-shadow.service
sudo systemctl daemon-reload
sudo systemctl enable mcp-runtime-shadow
sudo systemctl restart mcp-runtime-shadow
sudo systemctl status mcp-runtime-shadow --no-pager
sudo journalctl -u mcp-runtime-shadow -n 100 --no-pager
./scripts/healthcheck-shadow.sh http://127.0.0.1:8085
curl -fsS -o /dev/null -w 'healthz=%{http_code}\\n' http://127.0.0.1:8085/healthz
curl -fsS -o /dev/null -w 'authorize=%{http_code}\\n' 'http://127.0.0.1:8085/authorize?response_type=code&client_id=bad&redirect_uri=https://claude.ai/callback&state=x'
sudo ss -ltnp | rg ':8085\\b'
sudo ls -l /var/log/mcp-runtime-go/audit-shadow.jsonl /var/log/mcp-runtime-go/SHADOW_T0.txt
sudo tail -n 5 /var/log/mcp-runtime-go/audit-shadow.jsonl
sudo systemctl status hugo-mcp-proxy.service --no-pager
sudo systemctl status cf-shadow.service --no-pager
```

## Files Installed Or Updated

- [`deploy/systemd/mcp-runtime-shadow.service`](/home/jm/mcp-runtime-go/deploy/systemd/mcp-runtime-shadow.service)
- `/etc/systemd/system/mcp-runtime-shadow.service`
- `/etc/mcp-runtime-go/mcp-runtime-shadow.env`
- `/usr/local/bin/mcp-runtime`
- `/usr/local/bin/shadow-compare`
- `/var/log/mcp-runtime-go/SHADOW_T0.txt`
- `/var/log/mcp-runtime-go/audit-shadow.jsonl`

## Service Status

- `mcp-runtime-shadow.service`: active/running
- Main PID: `980875`
- Listen address: `127.0.0.1:8085`
- Journald logging: enabled

## Healthcheck Result

`./scripts/healthcheck-shadow.sh http://127.0.0.1:8085`

Result:

- `OK: healthz`
- `OK: readyz`
- `OK: oauth metadata`
- `OK: protected resource metadata`
- `Shadow healthcheck passed for http://127.0.0.1:8085`

## Python Authoritative Confirmation

- `hugo-mcp-proxy.service` remained active and running throughout the install.
- No public route was changed to point to Go.
- No Python service was stopped or replaced.

## Nginx / OpenResty

- No mirror configuration was applied.
- No Nginx/OpenResty reload was performed.
- This install phase stayed local and did not modify production routing.

## Shadow Audit Log

- Go audit log path: `/var/log/mcp-runtime-go/audit-shadow.jsonl`
- The file exists and received an event after an invalid authorize request.
- Example event type observed: `authorize_rejected`

## T0 Shadow Marker

- `/var/log/mcp-runtime-go/SHADOW_T0.txt` was created.
- It contains the UTC timestamp, Python audit log path, Go audit log path, and the 48h comparison command.

## Comparison Commands

### T+1h control

```bash
./scripts/shadow-status.sh /var/log/mcp-oauth/audit.log /var/log/mcp-runtime-go/audit-shadow.jsonl /var/log/mcp-runtime-go/reports
```

### T+48h comparison

```bash
./scripts/shadow-compare-48h.sh /var/log/mcp-oauth/audit.log /var/log/mcp-runtime-go/audit-shadow.jsonl /var/log/mcp-runtime-go/reports
```

## Rollback Readiness

- `systemctl stop mcp-runtime-shadow`
- `systemctl disable mcp-runtime-shadow`
- Remove `/etc/systemd/system/mcp-runtime-shadow.service` if desired
- Remove `/etc/mcp-runtime-go/mcp-runtime-shadow.env` if desired
- Leave Python authoritative service untouched
- Keep Nginx/OpenResty unchanged because no mirror was applied

## Issues Encountered

- `8084` was already occupied by `cf-shadow.service`, so the Go shadow could not safely use the originally requested port.
- The shadow unit initially had a path mismatch for the environment file; it was corrected to `/etc/mcp-runtime-go/mcp-runtime-shadow.env`.
- The service needed an explicit `ReadWritePaths` allowance for `/var/log/mcp-runtime-go` so the audit log could be created under the hardened systemd sandbox.
- `git status` is not usable here because the current workspace is not a Git repository in this environment.

## Verdict

SHADOW INSTALLED WITHOUT MIRROR
