package runtime

import (
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/oauthproxy"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestNewHandler_MCPRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			HugoToken:    "token",
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
		Runtime: config.RuntimeConfig{
			ListenHost: "127.0.0.1",
			ListenPort: 0,
		},
	}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	svc, err := oauthproxy.NewService(cfg, store, audit, nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	handler := newHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()

	client := server.Client()
	// Disable following redirects to catch 301/307
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	for _, path := range []string{"/mcp", "/mcp/"} {
		t.Run(path, func(t *testing.T) {
			resp, err := client.Post(server.URL+path, "application/json", nil)
			if err != nil {
				t.Fatalf("POST %s failed: %v", path, err)
			}
			defer resp.Body.Close()

			// We expect 401 Unauthorized (because no token),
			// but definitely NOT 404 Not Found or 301 Moved Permanently
			if resp.StatusCode == http.StatusNotFound {
				t.Errorf("POST %s got 404, expected proxy handler (likely 401)", path)
			}
			if resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusPermanentRedirect {
				t.Errorf("POST %s got redirect %d, expected direct reach to proxy handler", path, resp.StatusCode)
			}

			t.Logf("POST %s got status %d", path, resp.StatusCode)
		})
	}
}
