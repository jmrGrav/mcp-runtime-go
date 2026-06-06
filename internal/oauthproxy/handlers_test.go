package oauthproxy

import (
	"encoding/json"
	"fmt"
	"mcp-runtime-go/internal/config"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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
			ClientID:              "test-client",
			ClientSecret:          "test-secret",
			ProxyBaseURL:          "http://proxy",
			AuthCodeTTL:           300,
			AccessTokenTTL:        3600,
			MandatoryPKCE:         true,
			TrustedProxies:        []string{"127.0.0.1"},
			TrustedAuthorizeCIDRs: []string{"127.0.0.1/32", "::1/128"},
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

	t.Run("GET OAuth Metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rr := httptest.NewRecorder()
		s.HandleMetadata(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		var data map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&data)
		if got := data["service_documentation"]; got != "http://proxy/mcp" {
			t.Errorf("unexpected doc link: %v", got)
		}
	})

	t.Run("POST Metadata Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/.well-known/oauth-authorization-server", nil)
		rr := httptest.NewRecorder()
		s.HandleMetadata(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
}

func TestHandleProtectedResourceMetadata(t *testing.T) {
	s, _ := setupTestService(t)

	t.Run("GET Resource Metadata", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/.well-known/oauth-protected-resource", nil)
		rr := httptest.NewRecorder()
		s.HandleProtectedResourceMetadata(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		var data map[string]interface{}
		json.NewDecoder(rr.Body).Decode(&data)
		if got := data["resource"]; got != "http://proxy/mcp" {
			t.Errorf("unexpected resource: %v", got)
		}
	})

	t.Run("POST Resource Metadata Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/.well-known/oauth-protected-resource", nil)
		rr := httptest.NewRecorder()
		s.HandleProtectedResourceMetadata(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405, got %d", rr.Code)
		}
	})
}

func TestHandleRegister_DecodeError(t *testing.T) {
	s, _ := setupTestService(t)
	req := httptest.NewRequest("POST", "/register", strings.NewReader("invalid json"))
	rr := httptest.NewRecorder()
	s.HandleRegister(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleToken_ParseFormError(t *testing.T) {
	s, _ := setupTestService(t)
	// Force a form parsing error by using an invalid escape sequence in the body
	req := httptest.NewRequest("POST", "/token", strings.NewReader("client_id=1%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.HandleToken(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
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
			// RFC 6749 §4.1.2.1: other errors redirect to redirect_uri with error param.
			"Missing state",
			url.Values{
				"response_type":         {"code"},
				"client_id":             {cfg.OAuthProxy.ClientID},
				"redirect_uri":          {"https://claude.ai/callback"},
				"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
				"code_challenge_method": {"S256"},
			},
			http.StatusFound,
		},
		{
			// RFC 6749 §4.1.2.1: other errors redirect to redirect_uri with error param.
			"Missing PKCE when mandatory",
			url.Values{
				"response_type": {"code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"test-state"},
			},
			http.StatusFound,
		},
		{
			// RFC 6749 §4.1.2.1: invalid client_id must NOT redirect — return error directly.
			"Invalid client_id",
			url.Values{
				"response_type": {"code"},
				"client_id":     {"wrong"},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"test-state"},
			},
			http.StatusUnauthorized,
		},
		{
			// RFC 6749 §4.1.2.1: invalid redirect_uri must NOT redirect — return 400 directly.
			"Invalid redirect_uri",
			url.Values{
				"response_type": {"code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"redirect_uri":  {"http://evil.com/cb"},
				"state":         {"test-state"},
			},
			http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/authorize?"+tt.query.Encode(), nil)
			req.RemoteAddr = "127.0.0.1:1234"
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

func TestHandleAuthorize_CIDR(t *testing.T) {
	s, cfg := setupTestService(t)
	cfg.OAuthProxy.TrustedAuthorizeCIDRs = []string{"192.168.1.0/24"}

	tests := []struct {
		name     string
		sourceIP string
		expected int
	}{
		{"Allowed IP", "192.168.1.5", http.StatusFound},
		{"Forbidden IP", "10.0.0.1", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := url.Values{
				"response_type":         {"code"},
				"client_id":             {cfg.OAuthProxy.ClientID},
				"redirect_uri":          {"https://claude.ai/callback"},
				"state":                 {"test-state"},
				"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
				"code_challenge_method": {"S256"},
			}
			req := httptest.NewRequest("GET", "/authorize?"+query.Encode(), nil)
			req.RemoteAddr = tt.sourceIP + ":1234"
			rr := httptest.NewRecorder()
			s.HandleAuthorize(rr, req)

			if rr.Code != tt.expected {
				t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, rr.Code)
			}
		})
	}
}

func TestHandleRegister_MethodNotAllowed(t *testing.T) {
	s, _ := setupTestService(t)
	for _, method := range []string{"GET", "PUT", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/register", nil)
		rr := httptest.NewRecorder()
		s.HandleRegister(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /register: expected 405, got %d", method, rr.Code)
		}
		if rr.Header().Get("Allow") != "POST" {
			t.Errorf("%s /register: expected Allow: POST, got %q", method, rr.Header().Get("Allow"))
		}
	}
}

func TestHandleToken_MethodNotAllowed(t *testing.T) {
	s, _ := setupTestService(t)
	for _, method := range []string{"GET", "PUT", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/token", nil)
		rr := httptest.NewRecorder()
		s.HandleToken(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /token: expected 405, got %d", method, rr.Code)
		}
		if rr.Header().Get("Allow") != "POST" {
			t.Errorf("%s /token: expected Allow: POST, got %q", method, rr.Header().Get("Allow"))
		}
	}
}

func TestHandleAuthorize_MethodNotAllowed(t *testing.T) {
	s, cfg := setupTestService(t)
	cfg.OAuthProxy.TrustedAuthorizeCIDRs = []string{"0.0.0.0/0", "::/0"}
	for _, method := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		req := httptest.NewRequest(method, "/authorize", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		rr := httptest.NewRecorder()
		s.HandleAuthorize(rr, req)
		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /authorize: expected 405, got %d", method, rr.Code)
		}
		if rr.Header().Get("Allow") != "GET" {
			t.Errorf("%s /authorize: expected Allow: GET, got %q", method, rr.Header().Get("Allow"))
		}
	}
}

func TestHandleRegister_EmptyRedirectURIs(t *testing.T) {
	s, _ := setupTestService(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"Missing redirect_uris", `{}`},
		{"Empty array", `{"redirect_uris": []}`},
		{"Empty string entry", `{"redirect_uris": [""]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/register", strings.NewReader(tt.payload))
			rr := httptest.NewRecorder()
			s.HandleRegister(rr, req)
			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tt.name, rr.Code)
			}
		})
	}
}

func TestAuditLog_ClientRequestID(t *testing.T) {
	tmpDir := t.TempDir()
	auditPath := filepath.Join(tmpDir, "audit.log")
	tokenPath := filepath.Join(tmpDir, "tokens.json")
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:              "test-client",
			ClientSecret:          "test-secret",
			ProxyBaseURL:          "http://proxy",
			AuthCodeTTL:           300,
			AccessTokenTTL:        3600,
			MandatoryPKCE:         true,
			TrustedProxies:        []string{"127.0.0.1"},
			TrustedAuthorizeCIDRs: []string{"127.0.0.1/32"},
		},
	}
	audit := observability.NewAuditLogger(auditPath)
	store := storage.NewTokenStore(tokenPath, false)
	s, err := NewService(cfg, store, audit, nil)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("GET", "/authorize", nil)
	req = req.WithContext(mcpctx.WithRequestID(req.Context(), "rid-1"))
	req = req.WithContext(mcpctx.WithClientRequestID(req.Context(), "client-123"))

	s.auditLog(req, "test_event", nil)

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"client_request_id":"client-123"`) {
		t.Fatal("expected client_request_id in audit log")
	}
}

// failSaveStore accepts Load but fails on Save, for testing fail-closed token issuance.
type failSaveStore struct{}

func (f *failSaveStore) Load() (map[string]float64, error) { return map[string]float64{}, nil }
func (f *failSaveStore) Save(_ map[string]float64) error   { return fmt.Errorf("disk full") }
func (f *failSaveStore) Close() error                      { return nil }

func TestHandleToken_StoreFailure(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		OAuthProxy: config.OAuthProxyConfig{
			ClientID:       "test-client",
			ClientSecret:   "test-secret",
			AccessTokenTTL: 3600,
			AuthCodeTTL:    300,
		},
	}
	audit := observability.NewAuditLogger(filepath.Join(tmpDir, "audit.log"))
	s, err := NewService(cfg, &failSaveStore{}, audit, nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}

	code := "store-fail-code"
	s.AddAuthCode(code, AuthCode{
		RedirectURI: "https://claude.ai/callback",
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	})

	req := httptest.NewRequest("POST", "/token", strings.NewReader(url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {"test-client"},
		"client_secret": {"test-secret"},
		"redirect_uri":  {"https://claude.ai/callback"},
		"code":          {code},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	s.HandleToken(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when store fails, got %d", rr.Code)
	}

	// Token must not be in memory after a failed save
	if s.ValidateAccessToken("any-token") {
		t.Error("token should not be valid after store failure")
	}
}

func TestHandleAuthorize_CIDR_XFF(t *testing.T) {
	s, cfg := setupTestService(t)
	cfg.OAuthProxy.TrustedProxies = []string{"127.0.0.1"}
	cfg.OAuthProxy.TrustedAuthorizeCIDRs = []string{"192.168.1.0/24"}

	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.OAuthProxy.ClientID},
		"redirect_uri":          {"https://claude.ai/callback"},
		"state":                 {"test-state"},
		"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
		"code_challenge_method": {"S256"},
	}
	req := httptest.NewRequest("GET", "/authorize?"+query.Encode(), nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "192.168.1.5")
	rr := httptest.NewRecorder()

	s.HandleAuthorize(rr, req)
	if rr.Code != http.StatusFound {
		t.Errorf("expected 302, got %d: %s", rr.Code, rr.Body.String())
	}
}

// TestHandleAuthorize_RFC6749_ErrorRedirect verifies that errors other than invalid
// redirect_uri / unauthorized_client produce a 302 to the redirect_uri with an
// RFC 6749 §4.1.2.1 error parameter.
func TestHandleAuthorize_RFC6749_ErrorRedirect(t *testing.T) {
	s, cfg := setupTestService(t)

	tests := []struct {
		name      string
		query     url.Values
		wantError string
	}{
		{
			name: "missing state",
			query: url.Values{
				"response_type":         {"code"},
				"client_id":             {cfg.OAuthProxy.ClientID},
				"redirect_uri":          {"https://claude.ai/callback"},
				"code_challenge":        {"JBbiqONGWPaAmwXk_8bT6UnlPfrn65D32eZlJS-zGG0"},
				"code_challenge_method": {"S256"},
			},
			wantError: "invalid_request",
		},
		{
			name: "pkce mandatory",
			query: url.Values{
				"response_type": {"code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"s"},
			},
			wantError: "invalid_request",
		},
		{
			name: "unsupported response type",
			query: url.Values{
				"response_type": {"token"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"redirect_uri":  {"https://claude.ai/callback"},
				"state":         {"s"},
			},
			wantError: "unsupported_response_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/authorize?"+tt.query.Encode(), nil)
			req.RemoteAddr = "127.0.0.1:1234"
			rr := httptest.NewRecorder()
			s.HandleAuthorize(rr, req)

			if rr.Code != http.StatusFound {
				t.Errorf("expected 302 redirect, got %d: %s", rr.Code, rr.Body.String())
				return
			}
			loc := rr.Header().Get("Location")
			u, err := url.Parse(loc)
			if err != nil {
				t.Fatalf("invalid Location header %q: %v", loc, err)
			}
			if errParam := u.Query().Get("error"); errParam != tt.wantError {
				t.Errorf("expected error=%s, got %q in Location: %s", tt.wantError, errParam, loc)
			}
		})
	}
}

// TestHandleToken_RFC6749_JSONErrors verifies /token returns RFC 6749 §5.2 JSON error bodies.
func TestHandleToken_RFC6749_JSONErrors(t *testing.T) {
	s, cfg := setupTestService(t)

	tests := []struct {
		name      string
		form      url.Values
		wantCode  int
		wantError string
	}{
		{
			name: "unsupported grant type",
			form: url.Values{
				"grant_type":    {"implicit"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"client_secret": {cfg.OAuthProxy.ClientSecret},
			},
			wantCode:  http.StatusBadRequest,
			wantError: "unsupported_grant_type",
		},
		{
			name: "invalid client",
			form: url.Values{
				"grant_type":    {"authorization_code"},
				"client_id":     {"wrong"},
				"client_secret": {"wrong"},
				"code":          {"x"},
			},
			wantCode:  http.StatusUnauthorized,
			wantError: "invalid_client",
		},
		{
			name: "invalid grant (bad code)",
			form: url.Values{
				"grant_type":    {"authorization_code"},
				"client_id":     {cfg.OAuthProxy.ClientID},
				"client_secret": {cfg.OAuthProxy.ClientSecret},
				"redirect_uri":  {"https://claude.ai/callback"},
				"code":          {"nonexistent"},
			},
			wantCode:  http.StatusBadRequest,
			wantError: "invalid_grant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/token", strings.NewReader(tt.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rr := httptest.NewRecorder()
			s.HandleToken(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rr.Code)
			}
			ct := rr.Header().Get("Content-Type")
			if !strings.Contains(ct, "application/json") {
				t.Errorf("expected JSON Content-Type, got %q", ct)
			}
			var body map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode JSON error response: %v", err)
			}
			if body["error"] != tt.wantError {
				t.Errorf("expected error=%q, got %q", tt.wantError, body["error"])
			}
		})
	}
}
