package oauthproxy

import "time"

type AuthCode struct {
	RedirectURI         string
	ExpiresAt           time.Time
	CodeChallenge       string
	CodeChallengeMethod string
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

type RegistrationRequest struct {
	RedirectURIs []string `json:"redirect_uris"`
}

type RegistrationResponse struct {
	ClientID                      string   `json:"client_id"`
	ClientIDIssuedAt              int64    `json:"client_id_issued_at"`
	RedirectURIs                  []string `json:"redirect_uris"`
	GrantTypes                    []string `json:"grant_types"`
	ResponseTypes                 []string `json:"response_types"`
	TokenEndpointAuthMethod       string   `json:"token_endpoint_auth_method"`
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported"`
	Scope                         string   `json:"scope"`
}
