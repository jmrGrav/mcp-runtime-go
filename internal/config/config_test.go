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
	origGravToken := os.Getenv("GRAV_TOKEN")
	defer func() {
		os.Setenv("CLIENT_ID", origClientID)
		os.Setenv("CLIENT_SECRET", origClientSecret)
		os.Setenv("GRAV_TOKEN", origGravToken)
	}()

	os.Setenv("CLIENT_ID", "test-client")
	os.Setenv("CLIENT_SECRET", "test-secret")
	os.Setenv("GRAV_TOKEN", "test-token")

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
					ClientID:     "id",
					ClientSecret: "secret",
					GravToken:    "token",
				},
			},
			false,
		},
		{
			"Missing ClientID",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientSecret: "secret",
					GravToken:    "token",
				},
			},
			true,
		},
		{
			"Missing ClientSecret",
			Config{
				OAuthProxy: OAuthProxyConfig{
					ClientID:  "id",
					GravToken: "token",
				},
			},
			true,
		},
		{
			"Missing GravToken",
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
