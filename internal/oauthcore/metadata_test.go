package oauthcore

import "testing"

func TestAuthorizationServerMetadata(t *testing.T) {
	cfg := Config{
		Issuer:          "https://mcp.example.test",
		Resource:        "https://mcp.example.test/mcp",
		ScopesSupported: []string{"mcp"},
	}

	got := AuthorizationServerMetadata(cfg)

	if got["issuer"] != cfg.Issuer {
		t.Fatalf("issuer = %v, want %q", got["issuer"], cfg.Issuer)
	}
	if got["authorization_endpoint"] != "https://mcp.example.test/authorize" {
		t.Fatalf("authorization_endpoint = %v", got["authorization_endpoint"])
	}
	if got["token_endpoint"] != "https://mcp.example.test/token" {
		t.Fatalf("token_endpoint = %v", got["token_endpoint"])
	}
	if got["registration_endpoint"] != "https://mcp.example.test/register" {
		t.Fatalf("registration_endpoint = %v", got["registration_endpoint"])
	}
	if got["service_documentation"] != cfg.Resource {
		t.Fatalf("service_documentation = %v, want %q", got["service_documentation"], cfg.Resource)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	cfg := Config{
		Issuer:          "https://mcp.example.test",
		Resource:        "https://mcp.example.test/mcp",
		ScopesSupported: []string{"mcp"},
	}

	got := ProtectedResourceMetadata(cfg)

	if got["resource"] != cfg.Resource {
		t.Fatalf("resource = %v, want %q", got["resource"], cfg.Resource)
	}
	servers, ok := got["authorization_servers"].([]string)
	if !ok || len(servers) != 1 || servers[0] != cfg.Issuer {
		t.Fatalf("authorization_servers = %#v", got["authorization_servers"])
	}
	if got["resource_documentation"] != cfg.Resource {
		t.Fatalf("resource_documentation = %v, want %q", got["resource_documentation"], cfg.Resource)
	}
}
