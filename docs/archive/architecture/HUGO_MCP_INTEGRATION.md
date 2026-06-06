# FUTURE_HUGO_MCP_INTEGRATION.md

## Strategy
`hugo-mcp` will be integrated as a separate domain within this runtime.

### Preparation
1. **Router**: The `internal/httpserver` is designed to mount multiple sub-routers. `hugo-mcp` will occupy its own path prefix or hostname.
2. **State**: The `internal/storage` abstraction will support the more complex needs of Hugo (Page state, Audit results) by adding a SQLite implementation.
3. **Execution**: The `internal/runtime` can manage multiple background tasks (e.g., Hugo's periodic audits) alongside the OAuth proxy's token purger.
4. **Shared Security**: Re-use `internal/security` for API token validation (bcrypt) and input sanitization.
