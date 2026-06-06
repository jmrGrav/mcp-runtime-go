# Release Gates

This document defines the minimum checks that must pass before any release tag is considered safe.

## Mandatory Gates

Run all of the following and block the release if any step fails:

```bash
go test ./...
go test -race ./...
go vet ./...
govulncheck ./...
gitleaks detect --no-banner --redact
gitleaks git --no-banner --redact --log-opts="--all" .
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/mcp-runtime-linux-amd64 ./cmd/mcp-runtime
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o /tmp/mcp-runtime-linux-arm64 ./cmd/mcp-runtime
```

## Recommended Gate

```bash
trufflehog git file://. --results=verified,unknown --fail
```

## GitHub Actions Behavior

- The repository uses a single workflow file: `.github/workflows/ci.yml`.
- The workflow runs on pushes to `main`, pull requests targeting `main`, and tag pushes matching `v*`.
- Tagged builds upload Linux `amd64` and `arm64` binaries as GitHub Actions artifacts.
- The workflow does **not** create a GitHub Release automatically.

## Policy

- If tests fail, the release is blocked.
- If the race detector finds a race, the release is blocked.
- If `govulncheck` finds an reachable vulnerability, the release is blocked.
- If `gitleaks` or `trufflehog` detects a secret, the release is blocked.
- If either multi-arch Linux build fails, the release is blocked.

## Notes

- `CGO_ENABLED=0` is required for the release build matrix.
- The current codebase uses a pure Go SQLite driver, so CGO is not required for release builds.
- The release gate is intentionally explicit and strict; it is expected to be slower than the old CI.
