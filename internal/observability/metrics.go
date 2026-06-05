package observability

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Counter is a monotonically increasing counter exported in Prometheus text format.
type Counter struct {
	name  string
	help  string
	value atomic.Int64
}

func (c *Counter) Inc() {
	c.value.Add(1)
}

func (c *Counter) Get() int64 {
	return c.value.Load()
}

// Package-level counters wired into the runtime.
var (
	AuditWriteFailures       = &Counter{name: "mcp_audit_write_failures_total", help: "Total audit log write failures"}
	TokenPersistenceFailures = &Counter{name: "mcp_token_persistence_failures_total", help: "Total token store persistence failures"}
	ProxyRequestsTotal       = &Counter{name: "mcp_proxy_requests_total", help: "Total MCP proxy requests completed"}
	ProxyErrorsTotal         = &Counter{name: "mcp_proxy_errors_total", help: "Total MCP proxy backend errors"}
	TokensIssuedTotal        = &Counter{name: "mcp_tokens_issued_total", help: "Total access tokens successfully issued"}
	TokensRejectedTotal      = &Counter{name: "mcp_tokens_rejected_total", help: "Total token exchange requests rejected"}
	ReadinessFailuresTotal   = &Counter{name: "mcp_readiness_failures_total", help: "Total /readyz check failures"}
)

var allCounters = []*Counter{
	AuditWriteFailures,
	TokenPersistenceFailures,
	ProxyRequestsTotal,
	ProxyErrorsTotal,
	TokensIssuedTotal,
	TokensRejectedTotal,
	ReadinessFailuresTotal,
}

// HandleMetrics writes a Prometheus text format (version 0.0.4) metrics response.
func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	for _, c := range allCounters {
		fmt.Fprintf(w, "# HELP %s %s\n", c.name, c.help)
		fmt.Fprintf(w, "# TYPE %s counter\n", c.name)
		fmt.Fprintf(w, "%s %d\n", c.name, c.Get())
	}
}
