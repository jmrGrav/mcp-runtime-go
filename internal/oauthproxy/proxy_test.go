package oauthproxy

import (
	"mcp-runtime-go/internal/config"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleProxy_PathValidation(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")
	tokenPath := filepath.Join(tmpDir, "tokens.json")

	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			GravMCPURL: backend.URL + "/api/mcp",
			GravToken:  "test-token",
		},
	}
	audit := observability.NewAuditLogger(auditPath)
	store := storage.NewTokenStore(tokenPath, false)

	s, _ := NewService(cfg, store, audit, nil)
	s.AddAccessToken("valid-token", time.Now().Add(1*time.Hour))

	tests := []struct {
		path     string
		expected int
	}{
		{"/mcp", 200},
		{"/mcp/", 200},
		{"/mcp/tools", 200},
		{"/mcp_admin", 404},
		{"/mcp@evil.com", 400},
		{"/mcp/../admin", 404}, // path.Clean makes it /admin
		{"/mcp//tools", 200},   // path.Clean makes it /mcp/tools
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Authorization", "Bearer valid-token")
			rr := httptest.NewRecorder()

			s.HandleProxy(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("path %s: expected %d, got %d", tt.path, tt.expected, rr.Code)
			}
		})
	}
}

func TestHandleProxy_Security(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify hop-by-hop headers are removed
		for _, h := range hopByHopHeaders {
			if r.Header.Get(h) != "" {
				t.Errorf("hop-by-hop header %s found in backend request", h)
			}
		}
		for _, h := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-Server"} {
			if r.Header.Get(h) != "" {
				t.Errorf("provenance header %s found in backend request", h)
			}
		}
		// Verify X-Request-ID propagation
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("X-Request-ID missing in backend request")
		}
		// Verify Authorization
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer test-token") {
			t.Errorf("Authorization header missing or invalid: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			GravMCPURL: backend.URL,
			GravToken:  "test-token",
			GravHost:   "backend-host",
		},
	}
	audit := observability.NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
	store := storage.NewTokenStore(filepath.Join(tmpDir, "tokens.json"), false)

	s, _ := NewService(cfg, store, audit, nil)
	s.AddAccessToken("valid-token", time.Now().Add(1*time.Hour))

	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("Connection", "close")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("X-Request-ID", "test-rid")

	ctx := mcpctx.WithRequestID(req.Context(), "test-rid")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.HandleProxy(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
