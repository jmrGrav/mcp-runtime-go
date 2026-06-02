package oauthproxy

import (
	"encoding/json"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestService(t *testing.T) (*Service, *config.Config) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")
	tokenPath := filepath.Join(tmpDir, "tokens.json")

	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "test-client",
			ClientSecret:   "test-secret",
			ProxyBaseURL:   "http://proxy",
			AuthCodeTTL:    300,
			AccessTokenTTL: 3600,
			MandatoryPKCE:  true,
			TrustedProxies: []string{"127.0.0.1"},
		},
	}
	audit := observability.NewAuditLogger(auditPath)
	store := storage.NewTokenStore(tokenPath, false)
	s, err := NewService(cfg, store, audit, nil)
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}
	return s, cfg
}

func TestHandleMetadata(t *testing.T) {
	s, _ := setupTestService(t)

	t.Run("OAuth Metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rr := httptest.NewRecorder()
		s.HandleMetadata(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}

		var data map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&data); err != nil {
			t.Fatalf("decode metadata: %v", err)
		}
		if got := data["service_documentation"]; got != "http://proxy/mcp" {
			t.Fatalf("unexpected service_documentation: %v", got)
		}
	})

	t.Run("Resource Metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
		rr := httptest.NewRecorder()
		s.HandleProtectedResourceMetadata(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}

		var data map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&data); err != nil {
			t.Fatalf("decode metadata: %v", err)
		}
		if got := data["resource"]; got != "http://proxy/mcp" {
			t.Fatalf("unexpected resource: %v", got)
		}
		if got := data["resource_documentation"]; got != "http://proxy/mcp" {
			t.Fatalf("unexpected resource_documentation: %v", got)
		}
	})
}

func TestHandleRegister(t *testing.T) {
	s, _ := setupTestService(t)

	tests := []struct {
		name     string
		payload  string
		expected int
	}{
		{"Valid registration", `{"redirect_uris": ["https://claude.ai/callback"]}`, http.StatusCreated},
		{"Invalid redirect URI", `{"redirect_uris": ["http://evil.com"]}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/register", strings.NewReader(tt.payload))
			rr := httptest.NewRecorder()
			s.HandleRegister(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, rr.Code)
			}
		})
	}
}

func TestHandleAuthorize(t *testing.T) {
	s, cfg := setupTestService(t)

	tests := []struct {
		name     string
		query    url.Values
		expected int
	}{
		{
			"Valid request",
			url.Values{
				"response_type":         {"code"},
				"client_id":             {cfg.OAuthProxy.ClientID},
				"redirect_uri":          {"https://claude.ai/callback"},
				"state":                 {"test-state"},
				"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
				"code_challenge_method": {"S256"},
			},
			http.StatusFound,
		},
		{
			"Missing state",
			url.Values{
				"response_type":         {"code"},
				"client_id":             {cfg.OAuthProxy.ClientID},
				"redirect_uri":          {"https://claude.ai/callback"},
				"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
				"code_challenge_method": {"S256"},
			},
			http.StatusBadRequest,
		},
		{
			"Missing PKCE when mandatory",
			url.Values{
				"response_type": {"code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"test-state"},
			},
			http.StatusBadRequest,
		},
		{
			"Invalid client_id",
			url.Values{
				"response_type": {"code"},
				"client_id":     {"wrong"},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"test-state"},
			},
			http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/authorize?"+tt.query.Encode(), nil)
			rr := httptest.NewRecorder()
			s.HandleAuthorize(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, rr.Code)
			}
		})
	}
}

func TestHandleToken(t *testing.T) {
	s, cfg := setupTestService(t)

	// Pre-seed an auth code
	code := "valid-code"
	s.AddAuthCode(code, AuthCode{
		RedirectURI: "https://claude.ai/callback",
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	tests := []struct {
		name     string
		form     url.Values
		expected int
	}{
		{
			"Valid token request",
			url.Values{
				"grant_type":    {"authorization_code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"client_secret": {cfg.OAuthProxy.ClientSecret},
				"redirect_uri":  {"https://claude.ai/callback"},
				"code":          {code},
			},
			http.StatusOK,
		},
		{
			"Invalid code",
			url.Values{
				"grant_type":    {"authorization_code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"client_secret": {cfg.OAuthProxy.ClientSecret},
				"redirect_uri":  {"https://claude.ai/callback"},
				"code":          {"wrong"},
			},
			http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/token", strings.NewReader(tt.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			s.HandleToken(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, rr.Code)
			}

			if rr.Code == http.StatusOK {
				var resp TokenResponse
				json.NewDecoder(rr.Body).Decode(&resp)
				if resp.AccessToken == "" {
					t.Error("expected access token in response")
				}
			}
		})
	}
}

func TestHandleToken_PKCE(t *testing.T) {
	s, cfg := setupTestService(t)

	// Pre-seed an auth code with PKCE
	code := "pkce-code"
	challenge := "JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"
	s.AddAuthCode(code, AuthCode{
		RedirectURI:         "https://claude.ai/callback",
		ExpiresAt:           time.Now().Add(5 * time.Minute),
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})

	t.Run("Valid PKCE", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/token", strings.NewReader(url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {cfg.OAuthProxy.ClientID},
			"client_secret": {cfg.OAuthProxy.ClientSecret},
			"redirect_uri":  {"https://claude.ai/callback"},
			"code":          {code},
			"code_verifier": {"test-verifier"}, // matches JBbiq...
		}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		s.HandleToken(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestUnauthorized(t *testing.T) {
	s, _ := setupTestService(t)
	rr := httptest.NewRecorder()
	s.unauthorized(rr, "test detail")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
	if !strings.Contains(rr.Header().Get("WWW-Authenticate"), "Bearer realm=") {
		t.Errorf("missing or invalid WWW-Authenticate header: %s", rr.Header().Get("WWW-Authenticate"))
	}
}

func TestHandleToken_ClientAuth(t *testing.T) {
	s, cfg := setupTestService(t)
	code := "auth-code"
	s.AddAuthCode(code, AuthCode{ExpiresAt: time.Now().Add(5 * time.Minute)})

	t.Run("Invalid secret", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/token", strings.NewReader(url.Values{
			"grant_type":    {"authorization_code"},
			"client_id":     {cfg.OAuthProxy.ClientID},
			"client_secret": {"wrong"},
			"redirect_uri":  {"https://claude.ai/callback"},
			"code":          {code},
		}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		s.HandleToken(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rr.Code)
		}
	})
}
