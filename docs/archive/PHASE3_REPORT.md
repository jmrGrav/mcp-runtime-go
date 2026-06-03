# Phase 3 Shadow Deployment Preparation Report

## Files Created

- `deploy/systemd/mcp-runtime-shadow.service`
- `deploy/env/mcp-runtime-shadow.env.example`
- `deploy/nginx/mcp-runtime-shadow-mirror.conf.example`
- `scripts/healthcheck-shadow.sh`
- `scripts/shadow-compare-48h.sh`
- `scripts/shadow-status.sh`
- `docs/PHASE3_SHADOW_DEPLOYMENT.md`
- `docs/PHASE3_48H_COMPARISON_PLAN.md`
- `docs/ROLLBACK.md`

## Files Updated

- `internal/runtime/app.go`
- `internal/runtime/app_test.go`
- `internal/config/config.go`

## Shadow Port

- `8084`

## Validation Commands

```bash
./scripts/test-all.sh
go test -race ./...
go vet ./...
go build ./cmd/mcp-runtime
go build ./cmd/shadow-compare
shellcheck scripts/*.sh
```

## Validation Results

- `./scripts/test-all.sh`: PASS
- `go test -race ./...`: PASS
- `go vet ./...`: PASS
- `go build ./cmd/mcp-runtime`: PASS
- `go build ./cmd/shadow-compare`: PASS
- `shellcheck scripts/*.sh`: PASS

## Manual Procedure Summary

1. Build binaries.
2. Create the non-root system user and directories.
3. Copy the env example to `/etc/mcp-runtime-shadow/mcp-runtime-shadow.env` and fill placeholders manually.
4. Install the systemd unit manually and reload systemd.
5. Enable and start the shadow service only after manual validation.
6. Apply the Nginx/OpenResty mirror example manually.
7. Run the healthcheck and compare scripts during the 48h window.
8. Roll back using `docs/ROLLBACK.md` if any critical condition appears.

## Risks Known

- Mirror traffic is best-effort and can drop some non-critical events.
- The Go shadow service must remain non-authoritative at all times.
- Manual configuration of Nginx/OpenResty is required.

## Human Validation Required

- Install and start the systemd unit.
- Apply the mirror config to the authoritative edge.
- Confirm the shadow service only binds localhost.
- Confirm the env file contains real, approved values.

## Verdict

READY TO INSTALL SHADOW
