package oauthproxy

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	mcpctx "mcp-runtime-go/internal/context"
	"mcp-runtime-go/internal/oauthcore"
	"mcp-runtime-go/internal/observability"
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
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	s.auditLog(r, "metadata_served", nil)
	data := oauthcore.AuthorizationServerMetadata(s.oauthCoreConfig())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Service) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	s.auditLog(r, "resource_metadata_served", nil)
	data := oauthcore.ProtectedResourceMetadata(s.oauthCoreConfig())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Service) oauthCoreConfig() oauthcore.Config {
	return oauthcore.Config{
		Issuer:          s.cfg.OAuthProxy.ProxyBaseURL,
		Resource:        mcpServiceURL(s.cfg.OAuthProxy.ProxyBaseURL),
		ScopesSupported: []string{mcpScope},
	}
}

func (s *Service) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid_request", http.StatusBadRequest)
		return
	}

	resp, err := s.RegisterClient(req)
	if err != nil {
		s.auditLog(r, "register_rejected", map[string]interface{}{"reason": err.Error()})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid_redirect_uri"})
		return
	}

	s.auditLog(r, "client_registered", map[string]interface{}{"redirect_uris": req.RedirectURIs})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// HandleAuthorize implements RFC 6749 §4.1.1 / §4.1.2.1 error handling:
//   - Invalid redirect_uri or invalid client_id → direct HTTP error (no redirect).
//   - All other errors → redirect to redirect_uri with RFC-standard error codes.
func (s *Service) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

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
	redirectURI := q.Get("redirect_uri")
	clientID := q.Get("client_id")
	state := q.Get("state")

	// RFC 6749 §4.1.2.1: invalid redirect_uri or invalid client_id → must NOT redirect.
	if !security.IsAllowedRedirect(redirectURI) {
		s.auditLog(r, "authorize_rejected", map[string]interface{}{"reason": "invalid_redirect_uri"})
		http.Error(w, "invalid_redirect_uri", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(s.cfg.OAuthProxy.ClientID)) != 1 {
		s.auditLog(r, "authorize_rejected", map[string]interface{}{"reason": "unauthorized_client"})
		http.Error(w, "unauthorized_client", http.StatusUnauthorized)
		return
	}

	req := AuthorizeRequest{
		ResponseType:        q.Get("response_type"),
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		State:               state,
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
	}

	code, err := s.IssueAuthCode(req)
	if err != nil {
		s.auditLog(r, "authorize_rejected", map[string]interface{}{"reason": err.Error()})
		// Redirect with RFC-standard error code (redirect_uri and client_id already validated).
		rfcErr, rfcDesc := oauthcore.MapAuthorizeError(err)
		params := url.Values{}
		params.Set("error", rfcErr)
		if rfcDesc != "" {
			params.Set("error_description", rfcDesc)
		}
		if state != "" {
			params.Set("state", state)
		}
		http.Redirect(w, r, redirectURI+"?"+params.Encode(), http.StatusFound)
		return
	}

	params := url.Values{}
	params.Add("code", code)
	if state != "" {
		params.Add("state", state)
	}

	s.auditLog(r, "authorize_approved", map[string]interface{}{
		"redirect_uri": req.RedirectURI,
		"pkce":         req.CodeChallenge != "",
	})
	http.Redirect(w, r, fmt.Sprintf("%s?%s", req.RedirectURI, params.Encode()), http.StatusFound)
}

// HandleToken implements RFC 6749 §4.1.3 / §5.2 with RFC-standard JSON error bodies.
func (s *Service) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		writeTokenError(w, "invalid_request", "invalid form data", http.StatusBadRequest)
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
		observability.TokensRejectedTotal.Inc()
		rfcErr, status := oauthcore.MapTokenError(err)
		writeTokenError(w, rfcErr, "", status)
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

func writeTokenError(w http.ResponseWriter, errCode, desc string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]string{"error": errCode}
	if desc != "" {
		body["error_description"] = desc
	}
	json.NewEncoder(w).Encode(body)
}
