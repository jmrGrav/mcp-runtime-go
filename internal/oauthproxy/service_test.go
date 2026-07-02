package oauthproxy

import (
	"context"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/oauthcore"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestService_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			AccessTokenTTL: 3600,
		},
	}
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			s.AddAccessToken(fmt.Sprintf("token-%d", i), time.Now().Add(1*time.Hour))
		}(i)
	}
	wg.Wait()

	// Verify we can load them back
	tokens, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != n {
		t.Errorf("expected %d tokens, got %d", n, len(tokens))
	}
}

func TestConsumeAuthCode_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)

	code := "replay-code"
	s.AddAuthCode(code, AuthCode{ExpiresAt: time.Now().Add(1 * time.Minute)})

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	results := make(chan bool, n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, ok := s.ConsumeAuthCode(code)
			results <- ok
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	for ok := range results {
		if ok {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful consumption, got %d", successCount)
	}
}

func TestService_PurgeExpired(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)

	// Add expired code
	s.AddAuthCode("expired-code", AuthCode{ExpiresAt: time.Now().Add(-1 * time.Minute)})
	// Add valid code
	s.AddAuthCode("valid-code", AuthCode{ExpiresAt: time.Now().Add(1 * time.Minute)})

	// Add expired token
	s.tokensMu.Lock()
	s.accessTokens[s.HashToken("expired-token")] = float64(time.Now().Add(-1 * time.Minute).Unix())
	s.accessTokens[s.HashToken("valid-token")] = float64(time.Now().Add(1 * time.Minute).Unix())
	s.tokensMu.Unlock()

	s.PurgeExpired()

	if _, ok := s.GetAuthCode("expired-code"); ok {
		t.Error("expired code not purged")
	}
	if _, ok := s.GetAuthCode("valid-code"); !ok {
		t.Error("valid code purged")
	}

	if s.ValidateAccessToken("expired-token") {
		t.Error("expired token not purged")
	}
	if !s.ValidateAccessToken("valid-token") {
		t.Error("valid token purged")
	}
}

// TestStartPurgeLoop_Cancellation verifies the purge goroutine exits when context is cancelled.
func TestStartPurgeLoop_Cancellation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	store := storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false)
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.StartPurgeLoop(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(2 * time.Second):
		t.Error("StartPurgeLoop did not exit after context cancellation")
	}
}

func TestStartPurgeLoop_Tick(t *testing.T) {
	origTicker := newPurgeTicker
	t.Cleanup(func() {
		newPurgeTicker = origTicker
	})

	newPurgeTicker = func(time.Duration) *time.Ticker {
		return time.NewTicker(5 * time.Millisecond)
	}

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	store := &failSaveStore{}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)
	s.tokensMu.Lock()
	s.accessTokens[s.HashToken("expired")] = float64(time.Now().Add(-1 * time.Minute).Unix())
	s.tokensMu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.StartPurgeLoop(ctx)
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartPurgeLoop did not exit after cancel")
	}
}

// TestTokenPersistenceFailureMetric verifies the counter increments when store.Save fails.
func TestTokenPersistenceFailureMetric(t *testing.T) {
	before := observability.TokenPersistenceFailures.Get()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			AccessTokenTTL: 3600,
		},
	}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	// failSaveStore defined in handlers_test.go (same package)
	s, err := NewService(cfg, &failSaveStore{}, audit, nil)
	if err != nil {
		t.Fatal(err)
	}

	_ = s.AddAccessToken("sometoken", time.Now().Add(time.Hour))

	if observability.TokenPersistenceFailures.Get() <= before {
		t.Error("expected TokenPersistenceFailures counter to increment on save failure")
	}
}

func TestService_Ready(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")
	tokenPath := filepath.Join(tmpDir, "tokens.json")

	t.Run("Ready success", func(t *testing.T) {
		cfg := &config.Config{
			OAuthProxy: config.OAuthProxyConfig{
				ClientID:     "id",
				HugoMCPURL:   "http://backend",
				TokensFile:   tokenPath,
				AuditLogFile: auditPath,
			},
		}
		audit := observability.NewAuditLogger(auditPath)
		store := storage.NewTokenStore(tokenPath, false)
		s, _ := NewService(cfg, store, audit, nil)
		if err := s.Ready(); err != nil {
			t.Errorf("expected ready, got error: %v", err)
		}
	})

	t.Run("Unready missing ClientID", func(t *testing.T) {
		cfg := &config.Config{OAuthProxy: config.OAuthProxyConfig{HugoMCPURL: "http://backend"}}
		s := &Service{cfg: cfg}
		if err := s.Ready(); err == nil {
			t.Error("expected error for missing client_id")
		}
	})

	t.Run("Unready missing HugoMCPURL", func(t *testing.T) {
		cfg := &config.Config{OAuthProxy: config.OAuthProxyConfig{ClientID: "id"}}
		s := &Service{cfg: cfg}
		if err := s.Ready(); err == nil {
			t.Error("expected error for missing backend URL")
		}
	})
}

func TestService_Close(t *testing.T) {
	tmpDir := t.TempDir()
	tokenPath := filepath.Join(tmpDir, "tokens.json")
	cfg := &config.Config{OAuthProxy: config.OAuthProxyConfig{TokensFile: tokenPath}}
	store := storage.NewTokenStore(tokenPath, false)
	s, _ := NewService(cfg, store, nil, nil)
	if err := s.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestService_AuthenticateClient_Public(t *testing.T) {
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "public-client",
			ClientSecret: "secret",
		},
	}
	s := &Service{cfg: cfg}

	t.Run("Public client allowed", func(t *testing.T) {
		if !s.authenticateClient("public-client", "") {
			t.Error("expected authentication to succeed for public client")
		}
	})

	t.Run("Wrong clientID", func(t *testing.T) {
		if s.authenticateClient("wrong", "") {
			t.Error("expected authentication to fail for wrong clientID")
		}
	})
}

func TestService_AuthenticateClient_NoConfigSecret(t *testing.T) {
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "client",
			ClientSecret: "", // Defensive path
		},
	}
	s := &Service{cfg: cfg}
	if s.authenticateClient("client", "some-secret") {
		t.Error("expected failure when no secret configured but one provided")
	}
}

func TestService_Ready_AuditFailure(t *testing.T) {
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "id",
			HugoMCPURL:   "http://backend",
			AuditLogFile: "/nonexistent/path/audit.log",
		},
	}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, &failSaveStore{}, audit, nil)
	if err := s.Ready(); err == nil {
		t.Error("expected error when audit logger not ready")
	}
}

type failLoadStore struct{}

func (f *failLoadStore) Load() (map[string]float64, error) { return nil, fmt.Errorf("load failed") }
func (f *failLoadStore) Save(map[string]float64) error     { return nil }
func (f *failLoadStore) Close() error                      { return nil }

func TestNewService_InvalidBackendURL(t *testing.T) {
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoMCPURL: "://invalid",
		},
	}
	_, err := NewService(cfg, &failSaveStore{}, nil, nil)
	if err == nil {
		t.Error("expected error for invalid backend URL")
	}
}

func TestNewService_LoadError(t *testing.T) {
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			HugoMCPURL: "http://backend",
		},
	}
	_, err := NewService(cfg, &failLoadStore{}, observability.NewAuditLogger(""), nil)
	if err == nil {
		t.Fatal("expected load error")
	}
}

func TestService_Ready_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:     "id",
			HugoMCPURL:   "http://backend",
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s := &Service{cfg: cfg, store: &failLoadStore{}, audit: audit}
	if err := s.Ready(); err == nil {
		t.Fatal("expected readiness error when store.Load fails")
	}
}

func TestService_PurgeExpired_PersistFailure(t *testing.T) {
	before := observability.TokenPersistenceFailures.Get()

	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			TokensFile:   filepath.Join(tmpDir, "tokens.json"),
			AuditLogFile: filepath.Join(tmpDir, "audit.log"),
		},
	}
	store := &failSaveStore{}
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)
	s, _ := NewService(cfg, store, audit, nil)

	s.tokensMu.Lock()
	s.accessTokens[s.HashToken("expired")] = float64(time.Now().Add(-1 * time.Minute).Unix())
	s.tokensMu.Unlock()

	s.PurgeExpired()

	if observability.TokenPersistenceFailures.Get() <= before {
		t.Fatal("expected TokenPersistenceFailures to increment")
	}
}

func TestIssueAuthCode_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "client",
			ClientSecret:   "secret",
			HugoToken:      "token",
			HugoMCPURL:     "http://backend",
			ProxyBaseURL:   "https://proxy",
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AuthCodeTTL:    300,
			AccessTokenTTL: 3600,
			MandatoryPKCE:  true,
		},
	}
	s, err := NewService(cfg, storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false), observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile), nil)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		req  AuthorizeRequest
	}{
		{"unsupported response type", AuthorizeRequest{ResponseType: "token", ClientID: "client", RedirectURI: "https://claude.ai/callback", State: "s"}},
		{"unauthorized client", AuthorizeRequest{ResponseType: "code", ClientID: "wrong", RedirectURI: "https://claude.ai/callback", State: "s"}},
		{"invalid redirect", AuthorizeRequest{ResponseType: "code", ClientID: "client", RedirectURI: "http://evil.com", State: "s"}},
		{"missing state", AuthorizeRequest{ResponseType: "code", ClientID: "client", RedirectURI: "https://claude.ai/callback"}},
		{"pkce mandatory", AuthorizeRequest{ResponseType: "code", ClientID: "client", RedirectURI: "https://claude.ai/callback", State: "s"}},
		{"unsupported code_challenge_method", AuthorizeRequest{ResponseType: "code", ClientID: "client", RedirectURI: "https://claude.ai/callback", State: "s", CodeChallenge: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", CodeChallengeMethod: "plain"}},
		{"invalid code_challenge length", AuthorizeRequest{ResponseType: "code", ClientID: "client", RedirectURI: "https://claude.ai/callback", State: "s", CodeChallenge: "short", CodeChallengeMethod: "S256"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.IssueAuthCode(tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestExchangeToken_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "client",
			ClientSecret:   "secret",
			HugoToken:      "token",
			HugoMCPURL:     "http://backend",
			ProxyBaseURL:   "https://proxy",
			AuditLogFile:   filepath.Join(tmpDir, "audit.log"),
			TokensFile:     filepath.Join(tmpDir, "tokens.json"),
			AccessTokenTTL: 3600,
		},
	}
	s, err := NewService(cfg, storage.NewTokenStore(cfg.OAuthProxy.TokensFile, false), observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile), nil)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	s.AddAuthCode("valid", AuthCode{RedirectURI: "https://claude.ai/callback", ExpiresAt: now.Add(time.Hour)})
	s.AddAuthCode("pkce", AuthCode{RedirectURI: "https://claude.ai/callback", ExpiresAt: now.Add(time.Hour), CodeChallenge: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", CodeChallengeMethod: "S256"})

	tests := []struct {
		name string
		req  TokenExchangeRequest
	}{
		{"unsupported grant", TokenExchangeRequest{GrantType: "implicit", ClientID: "client", ClientSecret: "secret"}},
		{"invalid client", TokenExchangeRequest{GrantType: "authorization_code", ClientID: "wrong", ClientSecret: "bad", Code: "valid"}},
		{"invalid grant missing code", TokenExchangeRequest{GrantType: "authorization_code", ClientID: "client", ClientSecret: "secret", Code: "missing"}},
		{"redirect mismatch", TokenExchangeRequest{GrantType: "authorization_code", ClientID: "client", ClientSecret: "secret", Code: "valid", RedirectURI: "https://other.example.com"}},
		{"pkce failed", TokenExchangeRequest{GrantType: "authorization_code", ClientID: "client", ClientSecret: "secret", Code: "pkce", RedirectURI: "https://claude.ai/callback", CodeVerifier: "wrong"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.ExchangeToken(tt.req)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMapErrorDefaults(t *testing.T) {
	if code, desc := oauthcore.MapAuthorizeError(fmt.Errorf("unexpected")); code != "invalid_request" || desc != "unexpected" {
		t.Fatalf("unexpected authorize mapping: %s %q", code, desc)
	}
	if code, status := oauthcore.MapTokenError(fmt.Errorf("unexpected")); code != "invalid_request" || status != http.StatusBadRequest {
		t.Fatalf("unexpected token mapping: %s %d", code, status)
	}
}
