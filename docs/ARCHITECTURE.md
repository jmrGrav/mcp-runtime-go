# ARCHITECTURE.md

## Overview
`mcp-runtime-go` is a unified runtime designed to host multiple MCP-related domains (OAuth Proxy, Hugo MCP, etc.) with a focus on security, observability, and robust state management.

## Core Principles
1. **Domain Isolation**: Each functional area (e.g., `oauthproxy`) is isolated in its own package under `internal/`.
2. **Runtime Unity**: Shared concerns like configuration, HTTP serving, logging, and metrics are managed by the central `internal/runtime`.
3. **Shadow Mode Readiness**: All domains must support a non-authoritative "shadow mode" for parity validation.
4. **Explicit Lifecycle**: Proper startup and shutdown sequences using `context.Context`.

## Structure
- `cmd/mcp-runtime`: Application entry point.
- `internal/runtime`: Orchestrates domains and shared services.
- `internal/httpserver`: Unified HTTP server with domain-based routing.
- `internal/oauthproxy`: OAuth 2.0 / OIDC logic (First domain to be migrated).
- `internal/storage`: Abstraction for persistence (JSON, SQLite).
- `internal/security`: Security primitives (PKCE, Redirect URI validation, Secrets).
- `internal/observability`: Structured logging (slog), audit trails, and metrics.
