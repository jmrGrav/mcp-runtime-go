package oauthproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/oauthcore"
	"mcp-runtime-go/internal/observability"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"
	"time"
)

const anonymousInspectionLimitBytes = 1 << 20

type anonymousToolsListContextKey struct{}

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
			// req.URL.RawQuery is preserved from the original request by httputil.ReverseProxy.

			req.Host = s.cfg.OAuthProxy.HugoHost

			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.cfg.OAuthProxy.HugoToken))
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
		ModifyResponse: func(resp *http.Response) error {
			anonymousToolsList, _ := resp.Request.Context().Value(anonymousToolsListContextKey{}).(bool)
			if !anonymousToolsList {
				return nil
			}
			return s.filterAnonymousToolsListResponse(resp)
		},
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
	if auth != "" {
		if !strings.HasPrefix(auth, "Bearer ") {
			s.unauthorized(w, "Bearer token required")
			return
		}
		token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
		if !s.ValidateAccessToken(token) {
			s.unauthorized(w, "Invalid or expired token")
			return
		}
	} else if !s.cfg.OAuthProxy.AnonymousEnabled {
		s.unauthorized(w, "Bearer token required")
		return
	} else {
		allowed, toolsList := s.allowAnonymousMCPRequest(w, r)
		if !allowed {
			return
		}
		if toolsList {
			r = r.WithContext(context.WithValue(r.Context(), anonymousToolsListContextKey{}, true))
		}
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

func (s *Service) allowAnonymousMCPRequest(w http.ResponseWriter, r *http.Request) (allowed bool, toolsList bool) {
	if r.Method != http.MethodPost {
		s.auditLog(r, "anonymous_proxy_rejected", map[string]interface{}{"reason": "method_not_allowed", "method": r.Method})
		http.Error(w, "anonymous MCP requires POST", http.StatusMethodNotAllowed)
		return false, false
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, anonymousInspectionLimitBytes))
	if err != nil {
		s.auditLog(r, "anonymous_proxy_rejected", map[string]interface{}{"reason": "body_too_large_or_unreadable"})
		http.Error(w, "invalid anonymous MCP request", http.StatusBadRequest)
		return false, false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	allowed, toolsList, reason := s.anonymousPayloadAllowed(body)
	if !allowed {
		s.auditLog(r, "anonymous_proxy_rejected", map[string]interface{}{"reason": reason})
		http.Error(w, "anonymous MCP request is not allowed", http.StatusForbidden)
		return false, false
	}

	s.auditLog(r, "anonymous_proxy_allowed", nil)
	return true, toolsList
}

func (s *Service) anonymousPayloadAllowed(body []byte) (allowed bool, toolsList bool, reason string) {
	return oauthcore.AnonymousMCPPolicy{PublicTools: s.cfg.OAuthProxy.AnonymousPublicTools}.PayloadAllowed(body)
}

func (s *Service) isAnonymousPublicTool(name string) bool {
	return oauthcore.AnonymousMCPPolicy{PublicTools: s.cfg.OAuthProxy.AnonymousPublicTools}.IsPublicTool(name)
}

func (s *Service) filterAnonymousToolsListResponse(resp *http.Response) error {
	body, err := io.ReadAll(io.LimitReader(resp.Body, anonymousInspectionLimitBytes+1))
	if err != nil {
		return err
	}
	if err := resp.Body.Close(); err != nil {
		return err
	}
	if len(body) > anonymousInspectionLimitBytes {
		return fmt.Errorf("anonymous tools/list response exceeds inspection limit")
	}

	filtered := s.filterAnonymousToolsListBody(body)
	resp.Body = io.NopCloser(bytes.NewReader(filtered))
	resp.ContentLength = int64(len(filtered))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(filtered)))
	return nil
}

func (s *Service) filterAnonymousToolsListBody(body []byte) []byte {
	var value interface{}
	if err := json.Unmarshal(body, &value); err == nil {
		s.filterAnonymousToolsListJSON(value)
		if filtered, err := json.Marshal(value); err == nil {
			return filtered
		}
		return body
	}

	lines := bytes.Split(body, []byte("\n"))
	for i, line := range lines {
		data, ok := bytes.CutPrefix(line, []byte("data: "))
		if !ok {
			continue
		}
		var event interface{}
		if err := json.Unmarshal(data, &event); err != nil {
			continue
		}
		s.filterAnonymousToolsListJSON(event)
		filtered, err := json.Marshal(event)
		if err != nil {
			continue
		}
		lines[i] = append([]byte("data: "), filtered...)
	}
	return bytes.Join(lines, []byte("\n"))
}

func (s *Service) filterAnonymousToolsListJSON(value interface{}) {
	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			s.filterAnonymousToolsListJSON(item)
		}
	case map[string]interface{}:
		result, ok := typed["result"].(map[string]interface{})
		if !ok {
			return
		}
		tools, ok := result["tools"].([]interface{})
		if !ok {
			return
		}
		filtered := make([]interface{}, 0, len(tools))
		for _, tool := range tools {
			toolObj, ok := tool.(map[string]interface{})
			if !ok {
				continue
			}
			name, ok := toolObj["name"].(string)
			if ok && s.isAnonymousPublicTool(name) {
				filtered = append(filtered, tool)
			}
		}
		result["tools"] = filtered
	}
}
