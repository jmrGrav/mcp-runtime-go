# Shadow Rollback

## Goal

Return the environment to Python-only authoritative operation without losing user service.

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
