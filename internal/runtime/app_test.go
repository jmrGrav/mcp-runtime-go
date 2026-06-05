package runtime

import (
	"context"
	"encoding/pem"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/oauthproxy"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
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

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	if app == nil {
		t.Fatal("NewApp returned nil app")
	}

	if app.server == nil {
		t.Error("App.server is nil")
	}
	if app.oauth == nil {
		t.Error("App.oauth is nil")
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "id",
			ClientSecret:   "secret",
			HugoToken:      "token",
			HugoMCPURL:     "http://127.0.0.1/api/mcp",
			ProxyBaseURL:   "https://example.com",
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			TrustedProxies: []string{"127.0.0.1"},
			AuthCodeTTL:    300,
			AccessTokenTTL: 86400,
		},
	}
	observability.InitLogger(0)
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	svc, err := oauthproxy.NewService(cfg, store, audit, nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	return newHandler(svc)
}

func TestNewHandler_HealthAliases(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "id",
			ClientSecret:   "secret",
			HugoToken:      "token",
			HugoMCPURL:     "http://127.0.0.1/api/mcp",
			ProxyBaseURL:   "https://example.com",
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			TrustedProxies: []string{"127.0.0.1"},
			AuthCodeTTL:    300,
			AccessTokenTTL: 86400,
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

	for _, path := range []string{"/health", "/healthz", "/readyz"} {
		resp, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s expected 200, got %d", path, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestNewHandler_ReadyzUnready(t *testing.T) {
	tmpDir := t.TempDir()
	// HugoMCPURL intentionally empty → Ready() returns error → /readyz must 503
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			HugoToken:    "token",
			HugoMCPURL:   "",
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

	resp, err := http.Get(server.URL + "/readyz")
	if err != nil {
		t.Fatalf("GET /readyz failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when backend URL missing, got %d", resp.StatusCode)
	}

	// /healthz must still return 200 even when not ready
	resp2, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz failed: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("expected /healthz 200, got %d", resp2.StatusCode)
	}
}

func TestApp_Run(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "id",
			ClientSecret:   "secret",
			HugoToken:      "token",
			HugoMCPURL:     "http://127.0.0.1/api/mcp",
			ProxyBaseURL:   "http://127.0.0.1",
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			AuthCodeTTL:    300,
			AccessTokenTTL: 86400,
		},
		Runtime: config.RuntimeConfig{
			ListenHost: "127.0.0.1",
			ListenPort: 0,
		},
	}

	app, err := NewApp(cfg)
	if err != nil {
		t.Fatalf("NewApp failed: %v", err)
	}

	// Run in goroutine
	done := make(chan error, 1)
	go func() {
		done <- app.Run()
	}()

	// Wait for the server to bind its listen socket (no time.Sleep).
	readyCtx, readyCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer readyCancel()
	if err := app.server.WaitReady(readyCtx); err != nil {
		t.Fatalf("server did not become ready: %v", err)
	}

	// Send SIGINT to ourselves.
	p, _ := os.FindProcess(os.Getpid())
	p.Signal(syscall.SIGINT)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("App.Run failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Error("App.Run did not stop after SIGINT")
	}
}

func TestCreateHTTPClient(t *testing.T) {
	t.Run("Default client", func(t *testing.T) {
		client, err := createHTTPClient("")
		if err != nil {
			t.Fatalf("failed to create client: %v", err)
		}
		if client.Transport.(*http.Transport).TLSClientConfig.RootCAs != nil {
			t.Error("expected nil RootCAs")
		}
	})
}

func TestBackendVerification(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer ts.Close()

	// Get the cert in PEM format
	cert := ts.Certificate()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})

	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "server.crt")
	if err := os.WriteFile(caPath, certPEM, 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Verification success", func(t *testing.T) {
		client, err := createHTTPClient(caPath)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Get(ts.URL)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("Verification failure with no CA", func(t *testing.T) {
		client, err := createHTTPClient("")
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Get(ts.URL)
		if err == nil {
			t.Error("expected error for untrusted cert")
		}
	})
}

func TestNewHandler_MetricsEndpoint(t *testing.T) {
	handler := newTestHandler(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "text/plain; version=0.0.4; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %q", ct)
	}
}
