package oauthproxy

import (
	"encoding/json"
	"fmt"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/security"
	"net/http"
	"net/url"
)

func (s *Service) auditLog(r *http.Request, event string, fields map[string]interface{}) {
	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields["request_id"] = mcpctx.GetRequestID(r.Context())
	if clientID := mcpctx.GetClientRequestID(r.Context()); clientID != "" {
		fields["client_request_id"] = clientID
	}

	info := security.GetRequestInfo(r, s.cfg.OAuthProxy.TrustedProxies)
	s.audit.LogWithIP(event, info.SourceIP, r, fields)
}

func (s *Service) HandleMetadata(w http.ResponseWriter, r *http.Request) {
	s.auditLog(r, "metadata_served", nil)
	data := map[string]interface{}{
		"issuer":                                s.cfg.OAuthProxy.ProxyBaseURL,
		"authorization_endpoint":                fmt.Sprintf("%s/authorize", s.cfg.OAuthProxy.ProxyBaseURL),
		"token_endpoint":                        fmt.Sprintf("%s/token", s.cfg.OAuthProxy.ProxyBaseURL),
		"registration_endpoint":                 fmt.Sprintf("%s/register", s.cfg.OAuthProxy.ProxyBaseURL),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"scopes_supported":                      []string{mcpScope},
		"service_documentation":                 mcpServiceURL(s.cfg.OAuthProxy.ProxyBaseURL),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Service) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	s.auditLog(r, "resource_metadata_served", nil)
	data := map[string]interface{}{
		"resource":                 mcpServiceURL(s.cfg.OAuthProxy.ProxyBaseURL),
		"authorization_servers":    []string{s.cfg.OAuthProxy.ProxyBaseURL},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{mcpScope},
		"resource_documentation":   mcpServiceURL(s.cfg.OAuthProxy.ProxyBaseURL),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Service) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}

	resp, err := s.RegisterClient(req)
	if err != nil {
		s.auditLog(r, "register_rejected", map[string]interface{}{"reason": err.Error()})
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_redirect_uri"})
		return
	}

	s.auditLog(r, "client_registered", map[string]interface{}{"redirect_uris": req.RedirectURIs})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Service) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	// Defense in Depth: Go-level CIDR check for /authorize
	info := security.GetRequestInfo(r, s.cfg.OAuthProxy.TrustedProxies)
	if !security.IsIPAllowed(info.SourceIP, s.cfg.OAuthProxy.TrustedAuthorizeCIDRs) {
		s.auditLog(r, "authorize_forbidden", map[string]interface{}{
			"reason":    "ip_not_allowed",
			"src_ip":    info.SourceIP,
			"client_id": r.URL.Query().Get("client_id"),
		})
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	q := r.URL.Query()

	req := AuthorizeRequest{
		ResponseType:        q.Get("response_type"),
		ClientID:            q.Get("client_id"),
		RedirectURI:         q.Get("redirect_uri"),
		State:               q.Get("state"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
	}

	code, err := s.IssueAuthCode(req)
	if err != nil {
		s.auditLog(r, "authorize_rejected", map[string]interface{}{"reason": err.Error()})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	params := url.Values{}
	params.Add("code", code)
	if req.State != "" {
		params.Add("state", req.State)
	}

	s.auditLog(r, "authorize_approved", map[string]interface{}{
		"redirect_uri": req.RedirectURI,
		"pkce":         req.CodeChallenge != "",
	})
	http.Redirect(w, r, fmt.Sprintf("%s?%s", req.RedirectURI, params.Encode()), http.StatusFound)
}

func (s *Service) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	req := TokenExchangeRequest{
		GrantType:    r.FormValue("grant_type"),
		ClientID:     r.FormValue("client_id"),
		ClientSecret: r.FormValue("client_secret"),
		RedirectURI:  r.FormValue("redirect_uri"),
		Code:         r.FormValue("code"),
		CodeVerifier: r.FormValue("code_verifier"),
	}

	resp, err := s.ExchangeToken(req)
	if err != nil {
		s.auditLog(r, "token_rejected", map[string]interface{}{"reason": err.Error()})
		status := http.StatusBadRequest
		if err.Error() == "client_auth_failed" {
			status = http.StatusUnauthorized
		}
		http.Error(w, err.Error(), status)
		return
	}

	s.auditLog(r, "token_issued", map[string]interface{}{"pkce": req.CodeVerifier != ""})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *Service) unauthorized(w http.ResponseWriter, detail string) {
	w.Header().Set("WWW-Authenticate", s.wwwAuthenticateHeader())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{
		"error":             "unauthorized",
		"error_description": detail,
	})
}

func (s *Service) wwwAuthenticateHeader() string {
	return fmt.Sprintf("Bearer realm=\"%s\", resource_metadata=\"%s/.well-known/oauth-protected-resource\"",
		s.cfg.OAuthProxy.ProxyBaseURL, s.cfg.OAuthProxy.ProxyBaseURL)
}
