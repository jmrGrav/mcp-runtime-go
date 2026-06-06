# Roadmap

## Current State

- `v1.3` is the production baseline that validated OAuth, PKCE, SQLite WAL, observability, and cutover.
- `v1.4` work focused on cleanup, packaging, and documentation consolidation.
- The branch previously claimed as `100%` coverage is currently only `96.8%` on a clean tree; the missing gap is tracked separately and should not be described as solved in live docs.

## Delivered

- OAuth Authorization Code + PKCE is live and validated.
- Dynamic Client Registration is live.
- SQLite WAL token storage is live.
- Structured audit logging and metrics are live.
- Readiness and health probes are live.

## Remaining Debt

- Documentation was too fragmented and has been consolidated.
- Legacy docs and historical reports still exist in `docs/archive/` for reference.
- Coverage work still needs either a clean recommit of the missing coverage-driving changes or an explicit decision to stop chasing the last percentage points for now.

## v1.4 Cleanup / Packaging

Goals:

- keep production behavior unchanged
- simplify packaging and operator instructions
- keep archival history, but not as active documentation

## v1.5 Proposals

- remove remaining legacy env compatibility only when operator migration is complete
- reduce internal seam complexity where tests are now strong enough to simplify implementation
- trim old helper scripts that are only retained for historical reference

## v2 Ideas

- multi-backend MCP support
- tighter operational automation
- smaller production surface area with fewer compatibility fallbacks

## Coverage Decision

Decision: the `100%` coverage target should be treated as a follow-up technical task, not as a current documentation promise.

- If the missing coverage-driving changes are recommitted, finish the job cleanly and document the exact state.
- If the missing changes are intentionally left out because they belong to a different cleanup stream, record that as a deliberate stop point and do not present `100%` as achieved.

The live docs should reflect the verified state of the branch, not the aspirational result from an intermediate stash state.
