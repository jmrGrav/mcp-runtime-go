# Phase 3 Shadow Deployment

## Goal

Run the Go runtime in shadow mode for 48 hours while Python remains authoritative. The Go service must observe mirrored traffic, write audit logs to its own path, and be compared against Python with strict parity checks.

## Build

```bash
go build ./cmd/mcp-runtime
go build ./cmd/shadow-compare
```

Recommended binary locations:

```bash
install -m 0755 ./bin/mcp-runtime /usr/local/bin/mcp-runtime
install -m 0755 ./bin/shadow-compare /usr/local/bin/shadow-compare
```

## Create System User

Manual commands to validate:

```bash
sudo useradd --system --home /var/lib/mcp-runtime-shadow --shell /usr/sbin/nologin mcp-runtime
sudo mkdir -p /var/lib/mcp-runtime-shadow /var/log/mcp-runtime-shadow /etc/mcp-runtime-shadow
sudo chown -R mcp-runtime:mcp-runtime /var/lib/mcp-runtime-shadow /var/log/mcp-runtime-shadow
sudo chmod 0750 /var/lib/mcp-runtime-shadow /var/log/mcp-runtime-shadow
```

## Copy Env

```bash
sudo install -m 0640 deploy/env/mcp-runtime-shadow.env.example /etc/mcp-runtime-shadow/mcp-runtime-shadow.env
```

Edit `/etc/mcp-runtime-shadow/mcp-runtime-shadow.env` manually before enabling anything.

## Install Service

Manual commands to validate:

```bash
sudo install -m 0644 deploy/systemd/mcp-runtime-shadow.service /etc/systemd/system/mcp-runtime-shadow.service
sudo systemctl daemon-reload
sudo systemctl enable --now mcp-runtime-shadow.service
```

## Verification

```bash
systemctl status mcp-runtime-shadow.service --no-pager
journalctl -u mcp-runtime-shadow.service --no-pager -n 100
./scripts/healthcheck-shadow.sh http://127.0.0.1:8084
```

## Mirror Configuration

Use the example OpenResty/Nginx mirror config in `deploy/nginx/mcp-runtime-shadow-mirror.conf.example` as a manual patch to the authoritative edge configuration. Keep Python authoritative and mirror requests to the Go shadow service only.

## Launch Comparison

Run the comparison wrapper with the Python and Go audit logs and a report directory:

```bash
./scripts/shadow-compare-48h.sh /var/log/python/audit.log /var/log/mcp-runtime-shadow/audit.log reports/
```

## Rollback

See `docs/ROLLBACK.md`.

## Notes

- Go stays non-authoritative.
- The mirror is best-effort and should not block the user path.
- `shadow-compare` is strict and should fail on duplicate or missing critical `request_id` values, malformed JSON, and mismatches.
