# Phase 3 48h Comparison Plan

## Window

- T0: launch shadow
- T+1h: first quick control
- T+6h: intermediate control
- T+24h: partial comparison
- T+48h: final comparison

## Success Criteria

- 0 Critical mismatch
- 0 token issuance mismatch
- 0 authorize decision mismatch
- 0 proxy auth decision mismatch
- 0 duplicate request_id
- 0 missing request_id critical
- 0 malformed JSON
- unmatched rate is explained and only acceptable when caused by mirror fire-and-forget, never on critical events
- no secret leakage in logs
- Go remains stable without unexpected crash or restart

## Failure Criteria

- OAuth mismatch
- token accepted by Go but refused by Python or inverse
- redirect_uri / PKCE / state divergence
- duplicate or missing critical request_id
- Go logs containing raw token, code, or secret
- Go crash
- repeated TLS backend error
- abnormal memory or log growth

## Comparison Procedure

1. Keep Python authoritative.
2. Mirror traffic to Go shadow only.
3. Keep the Go audit log separate.
4. Run `shadow-compare` on the collected logs at the checkpoints.
5. Record each result in a timestamped report under `reports/`.

## Interpretation

- Critical events must match strictly.
- Missing IDs on critical events are failures.
- Duplicate request IDs are failures.
- Malformed JSON is a failure.
- Unmatched non-critical events require explanation and operator review.
