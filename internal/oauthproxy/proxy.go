package oauthproxy

import (
	"context"
	"fmt"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/observability"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"
	"time"
)

func appendSubPath(ctx context.Context, subPath string) context.Context {
	return context.WithValue(ctx, subPathContextKey{}, subPath)
}

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

type proxyResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *proxyResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures an implicit 200 that the underlying ResponseWriter would emit on first Write.
func (rw *proxyResponseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// buildReverseProxy creates the cached reverse proxy. The Director reads the per-request
// sub-path from the request context set by HandleProxy.
func (s *Service) buildReverseProxy() *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			subPath, _ := req.Context().Value(subPathContextKey{}).(string)

			req.URL.Scheme = s.backendURL.Scheme
			req.URL.Host = s.backendURL.Host
			req.URL.Path = strings.TrimSuffix(s.backendURL.Path, "/") + subPath
			req.URL.RawQuery = req.URL.RawQuery // preserved by ReverseProxy from original

			req.Host = s.cfg.OAuthProxy.GravHost

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.OAuthProxy.GravToken))
			req.Header.Del("Forwarded")
			req.Header["X-Forwarded-For"] = nil
			req.Header.Del("X-Forwarded-Host")
			req.Header.Del("X-Forwarded-Proto")
			req.Header.Del("X-Forwarded-Server")

			req.Header.Set("X-Request-ID", mcpctx.GetRequestID(req.Context()))

			for _, h := range hopByHopHeaders {
				req.Header.Del(h)
			}
		},
		Transport: s.httpClient.Transport,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.auditLog(r, "proxy_error", map[string]interface{}{"error": err.Error()})
			observability.ProxyErrorsTotal.Inc()
			http.Error(w, "proxy error", http.StatusBadGateway)
		},
	}
}

func (s *Service) HandleProxy(w http.ResponseWriter, r *http.Request) {
	// 1. Auth validation
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

	// 2. Path validation & boundary check
	cleanPath := path.Clean(r.URL.Path)

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

	if s.backendURL == nil {
		http.Error(w, "backend not configured", http.StatusServiceUnavailable)
		return
	}

	subPath := strings.TrimPrefix(cleanPath, mcpBasePath)

	// 3. Attach sub-path to context for the cached Director to read.
	ctx := r.Context()
	ctx = appendSubPath(ctx, subPath)
	r = r.WithContext(ctx)

	// 4. Proxy with per-request query string preserved.
	// ReverseProxy copies r.URL.RawQuery to req.URL.RawQuery in the Director via the
	// ModifyRequest chain; we preserve it from the original request.
	start := time.Now()
	rw := &proxyResponseWriter{ResponseWriter: w, statusCode: 0}
	s.proxy.ServeHTTP(rw, r)
	duration := time.Since(start)

	observability.ProxyRequestsTotal.Inc()
	s.auditLog(r, "proxy_hit", map[string]interface{}{
		"path":        cleanPath,
		"method":      r.Method,
		"status":      rw.statusCode,
		"duration_ms": duration.Milliseconds(),
		"client_id":   s.cfg.OAuthProxy.ClientID,
	})
}
