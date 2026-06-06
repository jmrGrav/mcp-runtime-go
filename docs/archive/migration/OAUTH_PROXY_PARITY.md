# OAUTH_PROXY_PARITY.md

| Feature | Python Status | Go Status | Test Coverage | Discrepancy / Notes |
| :--- | :--- | :--- | :--- | :--- |
| **GET /.well-known/...** | Active | **Implemented** | Unit | 1:1 Parity |
| **POST /register** | Active | **Implemented** | Unit | 1:1 Parity |
| **GET /authorize** | Active | **Implemented** | Unit | 1:1 Parity |
| **POST /token** | Active | **Implemented** | Unit | 1:1 Parity |
| **PKCE S256 Validation** | Active | **Implemented** | Unit | Verified via RFC 7636 tests |
| **Redirect URI Whitelist**| Active | **Implemented** | Unit | Same domain/suffix logic |
| **Token Hashing (SHA256)**| Active | **Implemented** | Unit | hex(sha256) parity |
| **Audit Logging (JSONL)** | Active | **Implemented** | Unit | Includes X-Request-ID |
| **Atomic JSON Store** | Active | **Implemented** | Unit | Hardened with fsync |
| **Token Purge Loop** | Active | **Implemented** | Unit | 1h interval |
| **WWW-Authenticate** | Active | **Implemented** | Unit | RFC 6750 compliant |
| **Backend TLS (MCP_CA_CERT)**| Active | **Implemented** | Unit | Secure transport with RootCAs |
| **Hardened Proxy (httputil)**| Active | **Implemented** | Unit | Strips hop-by-hop headers, normalizes paths |
| **Request ID Matching** | Active | **Mandatory** | Unit | Primary key for shadow comparison |
