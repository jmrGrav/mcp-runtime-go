# SHADOW_MODE.md

## Strategy
To ensure zero-risk migration from Python to Go, the Go runtime implements a **Shadow Mode**.

### Operation
1. **Authoritative (Python)**: The existing Python service remains the source of truth, handling real traffic.
2. **Shadow (Go)**: The Go service runs in parallel (likely on a different port or receiving mirrored traffic).
3. **Comparison**:
    - The Go service processes requests but does **not** side-effect external systems (unless safe).
    - It logs the decision it *would* have made (e.g., "Would approve authorization for client X").
    - **Request ID Matching**: `X-Request-ID` is the mandatory primary key for matching Python and Go audit events.
    - Discrepancies between Python logs and Go logs are flagged for investigation.

### Execution Plan
1. Launch Go service with `SHADOW_MODE=true`.
2. Use a test client or traffic mirror to send identical requests to both.
3. Compare `audit.log` from Python with the audit trail in Go using `shadow-compare`.
4. **Enforcement**: `shadow-compare` will fail if critical events lack a matching `X-Request-ID`.
5. Once 100% parity is achieved over N days/requests, flip the authority.

### Criteria for Parity
- Identical JSON responses (structure and values).
- Identical error codes for invalid inputs.
- Identical behavior for PKCE and Redirect URI validation.
- Identical token hashing results.

### Infrastructure Setup (Nginx/OpenResty)
To enable accurate shadow comparison, the edge proxy must generate and propagate a unique Request ID.

Example Nginx configuration:
```nginx
server {
    listen 443 ssl;
    
    # Generate a unique ID if not present
    set $rid $http_x_request_id;
    if ($rid = "") {
        set $rid $request_id;
    }

    location / {
        proxy_set_header X-Request-ID $rid;
        
        # Mirror traffic to Go shadow service
        mirror /shadow;
        mirror_request_body on;

        proxy_pass http://python_authoritative;
    }

    location = /shadow {
        internal;
        proxy_set_header X-Request-ID $rid;
        proxy_pass http://go_shadow;
    }
}
```
