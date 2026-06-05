package oauthproxy

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/security"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

const (
	mcpBasePath = "/mcp"
	mcpScope    = "mcp"
)

// subPathContextKey is a typed context key for the per-request MCP sub-path.
type subPathContextKey struct{}

func mcpServiceURL(baseURL string) string {
	return fmt.Sprintf("%s%s", baseURL, mcpBasePath)
}

type Service struct {
	cfg          *config.Config
	store        storage.Store
	audit        *observability.AuditLogger
	authCodes    map[string]AuthCode
	codesMu      sync.RWMutex
	accessTokens map[string]float64
	tokensMu     sync.RWMutex
	httpClient   *http.Client
	backendURL   *url.URL
	proxy        *httputil.ReverseProxy
}

func NewService(cfg *config.Config, store storage.Store, audit *observability.AuditLogger, httpClient *http.Client) (*Service, error) {
	tokens, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Pre-parse backend URL at startup to fail fast on misconfiguration.
	var backendURL *url.URL
	if cfg.OAuthProxy.HugoMCPURL != "" {
		backendURL, err = url.Parse(cfg.OAuthProxy.HugoMCPURL)
		if err != nil {
			return nil, fmt.Errorf("invalid backend URL %q: %w", cfg.OAuthProxy.HugoMCPURL, err)
		}
	}

	svc := &Service{
		cfg:          cfg,
		store:        store,
		audit:        audit,
		authCodes:    make(map[string]AuthCode),
		accessTokens: tokens,
		httpClient:   httpClient,
		backendURL:   backendURL,
	}
	svc.proxy = svc.buildReverseProxy()
	return svc, nil
}

func (s *Service) HashToken(token string) string {
	h := sha256.New()
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) syncTokens() {
	var snapshot map[string]float64
	s.tokensMu.RLock()
	snapshot = make(map[string]float64, len(s.accessTokens))
	for k, v := range s.accessTokens {
		snapshot[k] = v
	}
	s.tokensMu.RUnlock()

	if err := s.store.Save(snapshot); err != nil {
		observability.Logger.Error("token persistence failed during purge sync", "error", err)
		observability.TokenPersistenceFailures.Inc()
	}
}

func (s *Service) PurgeExpired() {
	now := float64(time.Now().Unix())

	s.codesMu.Lock()
	for code, data := range s.authCodes {
		if data.ExpiresAt.Before(time.Now()) {
			delete(s.authCodes, code)
		}
	}
	s.codesMu.Unlock()

	s.tokensMu.Lock()
	changed := false
	for hash, expiresAt := range s.accessTokens {
		if expiresAt <= now {
			delete(s.accessTokens, hash)
			changed = true
		}
	}
	s.tokensMu.Unlock()

	if changed {
		s.syncTokens()
	}
}

// StartPurgeLoop runs the periodic expiry sweep until ctx is cancelled.
// The caller must cancel ctx before calling Close to avoid a store write after Close.
func (s *Service) StartPurgeLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.PurgeExpired()
		}
	}
}

func (s *Service) AddAuthCode(code string, data AuthCode) {
	s.codesMu.Lock()
	defer s.codesMu.Unlock()
	s.authCodes[code] = data
}

func (s *Service) GetAuthCode(code string) (AuthCode, bool) {
	s.codesMu.RLock()
	defer s.codesMu.RUnlock()
	data, ok := s.authCodes[code]
	return data, ok
}

func (s *Service) RemoveAuthCode(code string) {
	s.codesMu.Lock()
	defer s.codesMu.Unlock()
	delete(s.authCodes, code)
}

func (s *Service) ConsumeAuthCode(code string) (AuthCode, bool) {
	s.codesMu.Lock()
	defer s.codesMu.Unlock()
	data, ok := s.authCodes[code]
	if ok {
		delete(s.authCodes, code)
	}
	return data, ok
}

// AddAccessToken adds a token to the in-memory map and persists it.
// Returns an error if persistence fails; on error the token is rolled back from memory.
func (s *Service) AddAccessToken(token string, expiresAt time.Time) error {
	hash := s.HashToken(token)
	exp := float64(expiresAt.Unix())

	s.tokensMu.Lock()
	s.accessTokens[hash] = exp
	snapshot := make(map[string]float64, len(s.accessTokens))
	for k, v := range s.accessTokens {
		snapshot[k] = v
	}
	err := s.store.Save(snapshot)
	if err != nil {
		delete(s.accessTokens, hash)
		observability.Logger.Error("token persistence failed on issuance", "error", err)
		observability.TokenPersistenceFailures.Inc()
	}
	s.tokensMu.Unlock()
	return err
}

func (s *Service) ValidateAccessToken(token string) bool {
	s.tokensMu.RLock()
	defer s.tokensMu.RUnlock()
	hash := s.HashToken(token)
	expiresAt, ok := s.accessTokens[hash]
	if !ok {
		return false
	}
	return expiresAt > float64(time.Now().Unix())
}

// RegisterClient performs business validation and returns a registration response.
func (s *Service) RegisterClient(req RegistrationRequest) (*RegistrationResponse, error) {
	if len(req.RedirectURIs) == 0 {
		return nil, fmt.Errorf("invalid_request: redirect_uris missing or empty")
	}
	for _, uri := range req.RedirectURIs {
		if uri == "" {
			return nil, fmt.Errorf("invalid_redirect_uri: empty URI")
		}
		if !security.IsAllowedRedirect(uri) {
			return nil, fmt.Errorf("invalid_redirect_uri")
		}
	}

	resp := &RegistrationResponse{
		ClientID:                      s.cfg.OAuthProxy.ClientID,
		ClientIDIssuedAt:              time.Now().Unix(),
		RedirectURIs:                  req.RedirectURIs,
		GrantTypes:                    []string{"authorization_code"},
		ResponseTypes:                 []string{"code"},
		TokenEndpointAuthMethod:       "none",
		CodeChallengeMethodsSupported: []string{"S256"},
		Scope:                         "mcp",
	}

	return resp, nil
}

type AuthorizeRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// IssueAuthCode validates the request and issues a new auth code.
func (s *Service) IssueAuthCode(req AuthorizeRequest) (string, error) {
	if req.ResponseType != "code" {
		return "", fmt.Errorf("unsupported_response_type")
	}
	if subtle.ConstantTimeCompare([]byte(req.ClientID), []byte(s.cfg.OAuthProxy.ClientID)) != 1 {
		return "", fmt.Errorf("unauthorized_client")
	}
	if !security.IsAllowedRedirect(req.RedirectURI) {
		return "", fmt.Errorf("invalid_redirect_uri")
	}
	if req.State == "" {
		return "", fmt.Errorf("invalid_request: missing state parameter")
	}
	if req.CodeChallenge == "" && s.cfg.OAuthProxy.MandatoryPKCE {
		return "", fmt.Errorf("invalid_request: pkce_mandatory")
	}
	if req.CodeChallenge != "" {
		if req.CodeChallengeMethod != "S256" {
			return "", fmt.Errorf("invalid_request: unsupported code_challenge_method")
		}
		if len(req.CodeChallenge) < 43 || len(req.CodeChallenge) > 128 {
			return "", fmt.Errorf("invalid_request: code_challenge length invalid")
		}
	}

	code := security.GenerateRandomString(32)
	s.AddAuthCode(code, AuthCode{
		RedirectURI:         req.RedirectURI,
		ExpiresAt:           time.Now().Add(time.Duration(s.cfg.OAuthProxy.AuthCodeTTL) * time.Second),
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
	})

	return code, nil
}

type TokenExchangeRequest struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Code         string
	CodeVerifier string
}

// ExchangeToken performs client authentication, code validation, and issues an access token.
func (s *Service) ExchangeToken(req TokenExchangeRequest) (*TokenResponse, error) {
	if req.GrantType != "authorization_code" {
		return nil, fmt.Errorf("unsupported_grant_type")
	}

	if !s.authenticateClient(req.ClientID, req.ClientSecret) {
		return nil, fmt.Errorf("invalid_client")
	}

	data, ok := s.ConsumeAuthCode(req.Code)
	if !ok || data.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("invalid_grant: invalid or expired code")
	}
	if req.RedirectURI == "" || req.RedirectURI != data.RedirectURI {
		return nil, fmt.Errorf("invalid_grant: redirect_uri mismatch")
	}

	if data.CodeChallenge != "" {
		if !security.ValidatePKCE(data.CodeChallenge, req.CodeVerifier) {
			return nil, fmt.Errorf("invalid_grant: pkce verification failed")
		}
	}

	accessToken := security.GenerateRandomString(32)
	if err := s.AddAccessToken(accessToken, time.Now().Add(time.Duration(s.cfg.OAuthProxy.AccessTokenTTL)*time.Second)); err != nil {
		return nil, fmt.Errorf("server_error")
	}

	observability.TokensIssuedTotal.Inc()

	return &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   s.cfg.OAuthProxy.AccessTokenTTL,
		Scope:       "mcp",
	}, nil
}

func (s *Service) authenticateClient(clientID, clientSecret string) bool {
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(s.cfg.OAuthProxy.ClientID)) != 1 {
		return false
	}
	// Public client (no secret provided): allowed — PKCE in ExchangeToken provides the security.
	if clientSecret == "" {
		return true
	}
	// Confidential client: validate secret.
	if s.cfg.OAuthProxy.ClientSecret == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(clientSecret), []byte(s.cfg.OAuthProxy.ClientSecret)) == 1
}

// Ready checks critical runtime dependencies and returns a non-nil error if any are unready.
func (s *Service) Ready() error {
	if s.cfg.OAuthProxy.ClientID == "" {
		return fmt.Errorf("config incomplete: missing client_id")
	}
	if s.cfg.OAuthProxy.HugoMCPURL == "" {
		return fmt.Errorf("config incomplete: missing backend MCP URL")
	}
	if _, err := s.store.Load(); err != nil {
		observability.ReadinessFailuresTotal.Inc()
		return fmt.Errorf("token store not ready: %w", err)
	}
	if err := s.audit.Ping(); err != nil {
		observability.ReadinessFailuresTotal.Inc()
		return fmt.Errorf("audit writer not ready: %w", err)
	}
	return nil
}

// Close closes the underlying token store.
func (s *Service) Close() error {
	return s.store.Close()
}
