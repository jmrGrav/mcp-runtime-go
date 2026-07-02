package oauthcore

import "fmt"

func AuthorizationServerMetadata(cfg Config) map[string]interface{} {
	return map[string]interface{}{
		"issuer":                                cfg.Issuer,
		"authorization_endpoint":                fmt.Sprintf("%s/authorize", cfg.Issuer),
		"token_endpoint":                        fmt.Sprintf("%s/token", cfg.Issuer),
		"registration_endpoint":                 fmt.Sprintf("%s/register", cfg.Issuer),
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none", "client_secret_post"},
		"scopes_supported":                      cfg.ScopesSupported,
		"service_documentation":                 cfg.Resource,
	}
}

func ProtectedResourceMetadata(cfg Config) map[string]interface{} {
	return map[string]interface{}{
		"resource":                 cfg.Resource,
		"authorization_servers":    []string{cfg.Issuer},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         cfg.ScopesSupported,
		"resource_documentation":   cfg.Resource,
	}
}
