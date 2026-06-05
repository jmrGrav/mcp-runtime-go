package oauthproxy

import (
	"context"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
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
