package oauthproxy

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/security"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"sync"
	"time"
)

const (
	mcpBasePath = "/mcp"
	mcpScope    = "mcp"
)

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
}

func NewService(cfg *config.Config, store storage.Store, audit *observability.AuditLogger, httpClient *http.Client) (*Service, error) {
	tokens, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load tokens: %w", err)
	}

	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Service{
		cfg:          cfg,
		store:        store,
		audit:        audit,
		authCodes:    make(map[string]AuthCode),
		accessTokens: tokens,
		httpClient:   httpClient,
	}, nil
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

	_ = s.store.Save(snapshot)
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

func (s *Service) StartPurgeLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		s.PurgeExpired()
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

func (s *Service) AddAccessToken(token string, expiresAt time.Time) {
	hash := s.HashToken(token)
	exp := float64(expiresAt.Unix())

	s.tokensMu.Lock()
	s.accessTokens[hash] = exp
	snapshot := make(map[string]float64, len(s.accessTokens))
	for k, v := range s.accessTokens {
		snapshot[k] = v
	}
	_ = s.store.Save(snapshot)
	s.tokensMu.Unlock()
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
	for _, uri := range req.RedirectURIs {
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
		return "", fmt.Errorf("unsupported response_type")
	}
	if subtle.ConstantTimeCompare([]byte(req.ClientID), []byte(s.cfg.OAuthProxy.ClientID)) != 1 {
		return "", fmt.Errorf("invalid client_id")
	}
	if !security.IsAllowedRedirect(req.RedirectURI) {
		return "", fmt.Errorf("invalid redirect_uri")
	}
	if req.State == "" {
		return "", fmt.Errorf("missing state parameter")
	}
	if req.CodeChallenge == "" && s.cfg.OAuthProxy.MandatoryPKCE {
		return "", fmt.Errorf("pkce_mandatory")
	}
	if req.CodeChallenge != "" {
		if req.CodeChallengeMethod != "S256" {
			return "", fmt.Errorf("pkce_method_unsupported")
		}
		if len(req.CodeChallenge) < 43 || len(req.CodeChallenge) > 128 {
			return "", fmt.Errorf("pkce_challenge_len_invalid")
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
		return nil, fmt.Errorf("unsupported grant_type")
	}

	if !s.authenticateClient(req.ClientID, req.ClientSecret) {
		return nil, fmt.Errorf("client_auth_failed")
	}

	data, ok := s.ConsumeAuthCode(req.Code)
	if !ok || data.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("invalid_code")
	}
	if req.RedirectURI == "" || req.RedirectURI != data.RedirectURI {
		return nil, fmt.Errorf("invalid_redirect_uri")
	}

	if data.CodeChallenge != "" {
		if !security.ValidatePKCE(data.CodeChallenge, req.CodeVerifier) {
			return nil, fmt.Errorf("pkce_failed")
		}
	}

	accessToken := security.GenerateRandomString(32)
	s.AddAccessToken(accessToken, time.Now().Add(time.Duration(s.cfg.OAuthProxy.AccessTokenTTL)*time.Second))

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
