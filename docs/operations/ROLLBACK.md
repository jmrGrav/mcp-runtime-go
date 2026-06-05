> **HISTORICAL — SHADOW MODE ONLY**
> This document describes how to roll back from **Go shadow → Python authoritative**.
> Shadow mode was retired on 2026-06-03. Go is now the authoritative runtime.
>
> **For the current production rollback procedure**, see [ROLLBACK_PRODUCTION.md](ROLLBACK_PRODUCTION.md).

---

# Shadow Rollback (Historical)

## Goal

Return the environment to Python-only authoritative operation without losing user service.

**This procedure applies only if the Go runtime was running in shadow mode alongside
the Python authoritative service. It is NOT applicable to the current production setup.**

## Steps

1. Stop the shadow service:

```bash
sudo systemctl stop mcp-runtime-shadow.service
```

2. Disable the shadow service:

```bash
sudo systemctl disable mcp-runtime-shadow.service
```

3. Optionally remove the unit file after validation:

```bash
sudo rm -f /etc/systemd/system/mcp-runtime-shadow.service
sudo systemctl daemon-reload
```

4. Optionally remove shadow env and logs after validation:

```bash
sudo rm -f /etc/mcp-runtime-shadow/mcp-runtime-shadow.env
sudo rm -rf /var/lib/mcp-runtime-shadow /var/log/mcp-runtime-shadow
```

5. Verify Python remains authoritative:

```bash
systemctl status python-authoritative.service --no-pager || true
```

6. Remove the Nginx/OpenResty mirror block from the edge config and reload it:

```bash
sudo nginx -t
sudo systemctl reload nginx
```

## Expected Outcome

- No user-visible outage.
- Python continues serving traffic.
- Go shadow stops cleanly.
- Mirror traffic is removed.
