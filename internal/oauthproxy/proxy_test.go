package oauthproxy

import (
	"mcp-runtime-go/internal/config"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleProxy_AuditLog(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted) // 202
	}))
	defer backend.Close()

	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoMCPURL: backend.URL,
			HugoToken:  "test-token",
			ClientID:   "hugo-mcp",
		},
	}
	audit := observability.NewAuditLogger(auditPath)
	store := storage.NewTokenStore(filepath.Join(tmpDir, "tokens.json"), false)

	s, _ := NewService(cfg, store, audit, nil)
	s.AddAccessToken("valid-token", time.Now().Add(1*time.Hour))

	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("X-Request-ID", "test-rid")

	ctx := mcpctx.WithRequestID(req.Context(), "test-rid")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	s.HandleProxy(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rr.Code)
	}

	// Verify audit log
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	logStr := string(data)
	if !strings.Contains(logStr, `"event":"proxy_hit"`) {
		t.Error("audit log missing proxy_hit event")
	}
	if !strings.Contains(logStr, `"path":"/mcp/tools"`) {
		t.Error("audit log missing correct path")
	}
	if !strings.Contains(logStr, `"status":202`) {
		t.Error("audit log missing correct status")
	}
	if !strings.Contains(logStr, `"request_id":"test-rid"`) {
		t.Error("audit log missing request_id")
	}
	if !strings.Contains(logStr, `"client_id":"hugo-mcp"`) {
		t.Error("audit log missing client_id")
	}
}

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
			HugoMCPURL: backend.URL + "/api/mcp",
			HugoToken:  "test-token",
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
			HugoMCPURL: backend.URL,
			HugoToken:  "test-token",
			HugoHost:   "backend-host",
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

func TestHandleProxy_AuthFailures(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoMCPURL: backend.URL,
			HugoToken:  "test-token",
			ClientID:   "hugo-mcp",
		},
	}
	audit := observability.NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
	store := storage.NewTokenStore(filepath.Join(tmpDir, "tokens.json"), false)
	s, _ := NewService(cfg, store, audit, nil)

	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/mcp/tools", nil)
		rr := httptest.NewRecorder()
		s.HandleProxy(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("invalid bearer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/mcp/tools", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rr := httptest.NewRecorder()
		s.HandleProxy(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})
}

func TestHandleProxy_BackendMissing(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoToken: "test-token",
			ClientID:  "hugo-mcp",
		},
	}
	audit := observability.NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
	store := storage.NewTokenStore(filepath.Join(tmpDir, "tokens.json"), false)
	s, _ := NewService(cfg, store, audit, nil)
	s.AddAccessToken("valid-token", time.Now().Add(1*time.Hour))

	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()
	s.HandleProxy(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHandleProxy_ProxyError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoMCPURL: backend.URL,
			HugoToken:  "test-token",
			ClientID:   "hugo-mcp",
		},
	}
	audit := observability.NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
	store := storage.NewTokenStore(filepath.Join(tmpDir, "tokens.json"), false)
	s, _ := NewService(cfg, store, audit, nil)
	s.AddAccessToken("valid-token", time.Now().Add(1*time.Hour))

	backend.Close()

	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	req.Header.Set("X-Request-ID", "req-1")
	ctx := mcpctx.WithRequestID(req.Context(), "req-1")
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	s.HandleProxy(rr, req)
	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rr.Code)
	}
}

func TestProxyResponseWriter_Implicit200(t *testing.T) {
	rr := httptest.NewRecorder()
	rw := &proxyResponseWriter{ResponseWriter: rr, statusCode: 0}

	// Write without calling WriteHeader
	n, err := rw.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != 5 {
		t.Errorf("expected 5 bytes written, got %d", n)
	}
	if rw.statusCode != http.StatusOK {
		t.Errorf("expected statusCode 200, got %d", rw.statusCode)
	}
	if rr.Code != http.StatusOK {
		t.Errorf("expected recorder code 200, got %d", rr.Code)
	}
}
