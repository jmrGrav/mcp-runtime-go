# CI and Security Audit

Date: 2026-06-06

## Scope

Reviewed:

- `.github/workflows/ci.yml`
- `Makefile`
- `scripts/*`
- release-oriented documentation

## Existing Controls

### `.github/workflows/ci.yml`

Already present before the hardening pass:

- `go vet ./...`
- `staticcheck ./...`
- `go test -coverprofile=coverage.out ./...`
- coverage gate at `60%`
- `go test -race ./...`
- `go build ./cmd/mcp-runtime`
- `govulncheck ./...`

### `Makefile`

Already present as convenience targets:

- `build`
- `test`
- `race`
- `vet`
- `coverage`

### `scripts/*`

Current scripts are operational helpers:

- `scripts/test-all.sh`
- `scripts/shadow-compare-48h.sh`
- `scripts/shadow-status.sh`
- `scripts/healthcheck-shadow.sh`

Only `scripts/test-all.sh` overlaps with validation concerns. The shadow scripts are historical.

### Release Documentation

Before this change, there was no explicit release gate document that enumerated the required
security and validation checks before a tag-based release.

## Gaps Found

- No secret scanning in CI.
- No historical secret scan in CI.
- No explicit multi-arch Linux build validation.
- No artifact upload path for tagged builds.
- No release gate document that states the required checks.

## Duplicates / Non-Gates

- `Makefile` duplicates some CI commands, but it is a convenience layer rather than the gate.
- `staticcheck` remains present as an extra quality signal, but it is not the release gate itself.
- Shadow-era scripts are not part of the live release path.

## Hardening Decision

One workflow remains the authoritative gate:

- tests
- race detector
- vet
- coverage gate
- `govulncheck`
- `gitleaks detect`
- `gitleaks git`
- `trufflehog git file://.`
- Linux `amd64` and `arm64` builds with `CGO_ENABLED=0`

## Scan Results

Local validation on this repository state produced:

- `gitleaks detect --no-banner --redact` -> no leaks found
- `gitleaks git --no-banner --redact --log-opts="--all" .` -> no leaks found
- `trufflehog git file://. --results=verified,unknown --fail` -> no secrets found

No exclusions were added.

## Notes

- `gitleaks` is installed through `go install github.com/zricethezav/gitleaks/v8@v8.30.1`.
- `govulncheck` is installed through `go install golang.org/x/vuln/cmd/govulncheck@latest`.
- `trufflehog` is installed from the official release tarball because direct `go install` is not
  usable for the current module layout.
- `fetch-depth: 0` is required for the history-oriented scans.
