package oauthproxy

import (
	"fmt"
	mcpctx "mcp-runtime-go/internal/context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

var hopByHopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"TE",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func (s *Service) HandleProxy(w http.ResponseWriter, r *http.Request) {
	// 1. Auth Validation
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		s.unauthorized(w, "Bearer token required")
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	if !s.ValidateAccessToken(token) {
		s.unauthorized(w, "Invalid or expired token")
		return
	}

	// 2. Path Validation & Boundary Check
	// Goal: mcpBasePath -> /api/mcp, mcpBasePath + "/foo" -> /api/mcp/foo
	// Reject: /mcp_admin, /mcp@evil.com, /mcp/../admin

	// Normalize path to prevent dot-segment bypass
	cleanPath := path.Clean(r.URL.Path)

	// Prevent "@" or other characters that could lead to misrouting
	if strings.Contains(r.URL.Path, "@") || strings.Contains(r.URL.RawPath, "%2e") || strings.Contains(r.URL.RawPath, "%2E") {
		s.auditLog(r, "proxy_rejected", map[string]interface{}{"reason": "malformed_path", "path": r.URL.Path})
		http.Error(w, "malformed path", http.StatusBadRequest)
		return
	}

	if cleanPath != mcpBasePath && !strings.HasPrefix(cleanPath, mcpBasePath+"/") {
		s.auditLog(r, "proxy_rejected", map[string]interface{}{"reason": "invalid_path", "path": r.URL.Path})
		http.Error(w, "invalid proxy path", http.StatusNotFound)
		return
	}

	subPath := strings.TrimPrefix(cleanPath, mcpBasePath)
	// subPath is now either "" or starting with "/"

	targetURL, err := url.Parse(s.cfg.OAuthProxy.GravMCPURL)
	if err != nil {
		http.Error(w, "invalid backend configuration", http.StatusInternalServerError)
		return
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetURL.Scheme
			req.URL.Host = targetURL.Host
			// targetURL.Path is probably /api/mcp
			req.URL.Path = strings.TrimSuffix(targetURL.Path, "/") + subPath
			req.URL.RawQuery = r.URL.RawQuery

			// Host Header
			req.Host = s.cfg.OAuthProxy.GravHost

			// Auth Header
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.OAuthProxy.GravToken))
			req.Header.Del("Forwarded")
			req.Header["X-Forwarded-For"] = nil
			req.Header.Del("X-Forwarded-Host")
			req.Header.Del("X-Forwarded-Proto")
			req.Header.Del("X-Forwarded-Server")

			// Propagation
			req.Header.Set("X-Request-ID", mcpctx.GetRequestID(r.Context()))

			// Strip all hop-by-hop headers
			for _, h := range hopByHopHeaders {
				req.Header.Del(h)
			}
		},
		Transport: s.httpClient.Transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.auditLog(r, "proxy_error", map[string]interface{}{"error": err.Error()})
			http.Error(w, "proxy error", http.StatusBadGateway)
		},
	}

	proxy.ServeHTTP(w, r)
}
