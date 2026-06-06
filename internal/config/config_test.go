package config

import (
	"os"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	// Clear env after test
	origClientID := os.Getenv("CLIENT_ID")
	origClientSecret := os.Getenv("CLIENT_SECRET")
	origHugoToken := os.Getenv("HUGO_TOKEN")
	defer func() {
		os.Setenv("CLIENT_ID", origClientID)
		os.Setenv("CLIENT_SECRET", origClientSecret)
		os.Setenv("HUGO_TOKEN", origHugoToken)
	}()

	os.Setenv("CLIENT_ID", "test-client")
	os.Setenv("CLIENT_SECRET", "test-secret")
	os.Setenv("HUGO_TOKEN", "test-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.OAuthProxy.ClientID != "test-client" {
		t.Errorf("expected client id test-client, got %s", cfg.OAuthProxy.ClientID)
	}

	// Test default values
	if cfg.Runtime.ListenPort != 8083 {
		t.Errorf("expected default port 8083, got %d", cfg.Runtime.ListenPort)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			"Valid config",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "http://127.0.0.1/api/mcp",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    300,
					AccessTokenTTL: 86400,
				},
			},
			false,
		},
		{
			"Invalid HUGO_MCP_URL scheme",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "ftp://127.0.0.1/mcp",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    300,
					AccessTokenTTL: 86400,
				},
			},
			true,
		},
		{
			"Invalid HUGO_MCP_URL no host",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "http:///api/mcp",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    300,
					AccessTokenTTL: 86400,
				},
			},
			true,
		},
		{
			"Invalid PROXY_BASE_URL",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "http://127.0.0.1/api/mcp",
					ProxyBaseURL:   "://invalid",
					AuthCodeTTL:    300,
					AccessTokenTTL: 86400,
				},
			},
			true,
		},
		{
			"Empty HUGO_MCP_URL",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    300,
					AccessTokenTTL: 86400,
				},
			},
			true,
		},
		{
			"Zero AuthCodeTTL",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "http://127.0.0.1/api/mcp",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    0,
					AccessTokenTTL: 86400,
				},
			},
			true,
		},
		{
			"Negative AccessTokenTTL",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:       "id",
					ClientSecret:   "secret",
					HugoToken:      "token",
					HugoMCPURL:     "http://127.0.0.1/api/mcp",
					ProxyBaseURL:   "https://example.com",
					AuthCodeTTL:    300,
					AccessTokenTTL: -1,
				},
			},
			true,
		},
		{
			"Missing ClientID",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientSecret: "secret",
					HugoToken:    "token",
				},
			},
			true,
		},
		{
			"Missing ClientSecret",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:  "id",
					HugoToken: "token",
				},
			},
			true,
		},
		{
			"Missing HugoToken",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:     "id",
					ClientSecret: "secret",
				},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBindEnv(t *testing.T) {
	type testConfig struct {
		Str   string   `env:"TEST_STR" envDefault:"def"`
		Int   int      `env:"TEST_INT" envDefault:"123"`
		Bool  bool     `env:"TEST_BOOL" envDefault:"false"`
		Slice []string `env:"TEST_SLICE" envDefault:"a,b"`
	}

	t.Run("Defaults", func(t *testing.T) {
		os.Clearenv()
		c := &testConfig{}
		if err := bindEnv(c); err != nil {
			t.Fatal(err)
		}
		if c.Str != "def" || c.Int != 123 || c.Bool != false || !reflect.DeepEqual(c.Slice, []string{"a", "b"}) {
			t.Errorf("defaults not applied correctly: %+v", c)
		}
	})

	t.Run("Override", func(t *testing.T) {
		os.Setenv("TEST_STR", "new")
		os.Setenv("TEST_INT", "456")
		os.Setenv("TEST_BOOL", "true")
		os.Setenv("TEST_SLICE", "x, y ")

		c := &testConfig{}
		if err := bindEnv(c); err != nil {
			t.Fatal(err)
		}
		if c.Str != "new" || c.Int != 456 || c.Bool != true || !reflect.DeepEqual(c.Slice, []string{"x", "y"}) {
			t.Errorf("overrides not applied correctly: %+v", c)
		}
	})
}

func TestLoad_InvalidPort(t *testing.T) {
	os.Setenv("LISTEN_PORT", "not-a-number")
	defer os.Unsetenv("LISTEN_PORT")

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid LISTEN_PORT")
	}
}

func TestLoad_ValidationError(t *testing.T) {
	t.Setenv("CLIENT_ID", "id")
	t.Setenv("CLIENT_SECRET", "secret")
	t.Setenv("HUGO_TOKEN", "token")
	t.Setenv("HUGO_MCP_URL", "http://")

	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error from Load")
	}
}

func TestBindEnv_InvalidBoolFails(t *testing.T) {
	type testConfig struct {
		Bool bool `env:"TEST_BOOL" envDefault:"false"`
	}

	os.Setenv("TEST_BOOL", "definitely-not-bool")
	defer os.Unsetenv("TEST_BOOL")

	c := &testConfig{}
	if err := bindEnv(c); err == nil {
		t.Fatal("expected error for invalid boolean value")
	}
}

func TestBindEnv_SkipsUntaggedField(t *testing.T) {
	type testConfig struct {
		Tagged   string `env:"TEST_TAGGED" envDefault:"value"`
		Skipped  string
		Override string `env:"TEST_OVERRIDE"`
	}

	t.Setenv("TEST_TAGGED", "set")
	t.Setenv("TEST_OVERRIDE", "override")

	c := &testConfig{}
	if err := bindEnv(c); err != nil {
		t.Fatal(err)
	}

	if c.Tagged != "set" {
		t.Fatalf("expected tagged field to be populated, got %q", c.Tagged)
	}
	if c.Skipped != "" {
		t.Fatalf("expected untagged field to remain empty, got %q", c.Skipped)
	}
	if c.Override != "override" {
		t.Fatalf("expected override field to be populated, got %q", c.Override)
	}
}

func TestBindEnv_EmptyBoolFailsWhenSet(t *testing.T) {
	type testConfig struct {
		Bool bool `env:"TEST_BOOL" envDefault:"true"`
	}

	os.Setenv("TEST_BOOL", "")
	defer os.Unsetenv("TEST_BOOL")

	c := &testConfig{}
	if err := bindEnv(c); err == nil {
		t.Fatal("expected error for explicit empty boolean value")
	}
}

func TestBindEnv_InvalidInt(t *testing.T) {
	type testConfig struct {
		Int int `env:"TEST_INT" envDefault:"0"`
	}

	os.Setenv("TEST_INT", "not-an-int")
	defer os.Unsetenv("TEST_INT")

	c := &testConfig{}
	if err := bindEnv(c); err == nil {
		t.Fatal("expected error for invalid int value")
	}
}
