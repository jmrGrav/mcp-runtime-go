# V1.4 Coverage Stash Analysis

Date: 2026-06-06

## Summary

- Branch at inspection time: `v1.4-cleanup-packaging`
- Current branch tip: `d4ebc5dd0a00e1eb175a26db2e0911905c068ea7`
- Active stash: `stash@{0}: wip: cleanup/packaging and local notes before push`
- Fresh clean-branch coverage: `96.8%`
- The previously claimed `100.0%` is not reproducible on the committed branch tip alone.

## Evidence

### Clean worktree

`git status` reported a clean tree:

```text
On branch v1.4-cleanup-packaging
nothing to commit, working tree clean
```

### Branch commits

`git log --oneline origin/main..HEAD`:

```text
d4ebc5d docs: add regression review for coverage sprint
2c22e09 test: reach 100% coverage with targeted seams
```

### Clean coverage run

`go test ./... -coverprofile=coverage.clean.out` followed by `go tool cover -func=coverage.clean.out | tail -20` produced:

```text
mcp-runtime-go/cmd/shadow-compare/main.go:24:		main				0.0%
mcp-runtime-go/cmd/shadow-compare/main.go:47:		runCompare			99.0%
mcp-runtime-go/cmd/shadow-compare/main.go:224:		findFallbackMatches		95.0%
mcp-runtime-go/internal/config/config.go:44:		Load				81.8%
mcp-runtime-go/internal/context/context.go:16:		WithClientRequestID		0.0%
mcp-runtime-go/internal/context/context.go:27:		GetClientRequestID		0.0%
mcp-runtime-go/internal/httpserver/server.go:43:	Start				88.9%
mcp-runtime-go/internal/httpserver/server.go:57:	Addr				0.0%
mcp-runtime-go/internal/httpserver/server.go:64:	WaitReady			0.0%
mcp-runtime-go/internal/runtime/app.go:28:		NewApp				96.3%
mcp-runtime-go/internal/runtime/middleware.go:31:	generateID			75.0%
total:							(statements)			96.8%
```

### Stash contents

`git stash show --include-untracked --name-status stash@{0}` showed the stash contains:

- coverage-related tracked modifications in:
  - `internal/config/config.go`
  - `internal/context/context_test.go`
  - `internal/httpserver/server.go`
  - `internal/httpserver/server_test.go`
  - `internal/runtime/app.go`
  - `internal/runtime/middleware.go`
  - `internal/runtime/middleware_test.go`
- cleanup/packaging changes in:
  - `cmd/shadow-compare/main.go`
  - `cmd/shadow-compare/compare_test.go`
  - shadow-mode docs/scripts
  - package/logrotate/systemd layout files
- local notes in:
  - `BROOKS_V1_3_PRODUCTION_REVIEW.md`
  - `MCP_HUGO_GO_MIGRATION_PLAN.md`

## Classification

### A. Necessary for the coverage sprint

These stashed files are the parts that actually explain the missing coverage on the clean branch:

- `internal/config/config.go`
- `internal/context/context_test.go`
- `internal/httpserver/server.go`
- `internal/httpserver/server_test.go`
- `internal/runtime/app.go`
- `internal/runtime/middleware.go`
- `internal/runtime/middleware_test.go`

Reason:

- They are the files whose absence leaves the clean branch at `96.8%`.
- They correspond to the undercovered functions reported by `go tool cover`.

### B. Cleanup/packaging files outside coverage

These are structurally relevant to the cleanup sprint, but they are not the reason the coverage delta exists:

- `cmd/shadow-compare/main.go`
- `cmd/shadow-compare/compare_test.go`
- shadow docs and scripts moved or deleted by the stash
- package/logrotate/systemd artifacts added by the stash

Reason:

- They belong to the cleanup/packaging workstream.
- They can remove the shadow-compare package from the denominator, but they do not by themselves explain the missing coverage in the remaining live code.

### C. Local parasite notes

- `BROOKS_V1_3_PRODUCTION_REVIEW.md`
- `MCP_HUGO_GO_MIGRATION_PLAN.md`

Reason:

- These are workspace notes, not production or coverage inputs.
- They should not be recommitted as part of the sprint unless explicitly intended.

## Conclusion

### Verdict

`100% commit incomplete`

### Why

- The clean committed branch stays at `96.8%`.
- The stash contains the internal code/test changes that were needed to raise coverage.
- The committed branch does not currently contain those coverage-driving file changes.

### What is missing

- The stashed changes to:
  - `internal/config/config.go`
  - `internal/context/context_test.go`
  - `internal/httpserver/server.go`
  - `internal/httpserver/server_test.go`
  - `internal/runtime/app.go`
  - `internal/runtime/middleware.go`
  - `internal/runtime/middleware_test.go`

### What is not worth recommitting for coverage

- The parasite notes listed above.

### Next decision

- Recommit the missing coverage-related files if the goal is to restore the 100% state.
- Keep the cleanup/packaging changes separate, because they are a different sprint.
