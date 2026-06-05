package config

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type Config struct {
	OAuthProxy OAuthProxyConfig
	Runtime    RuntimeConfig
}

type OAuthProxyConfig struct {
	HugoMCPURL              string   `env:"HUGO_MCP_URL" envAlias:"GRAV_MCP_URL" envDefault:"http://127.0.0.1/api/mcp"`
	HugoHost                string   `env:"HUGO_HOST" envAlias:"GRAV_HOST" envDefault:"www.arleo.eu"`
	HugoToken               string   `env:"HUGO_TOKEN" envAlias:"GRAV_TOKEN"`
	ClientID                string   `env:"CLIENT_ID" envAlias:"MCP_CLIENT_ID"`
	ClientSecret            string   `env:"CLIENT_SECRET" envAlias:"MCP_CLIENT_SECRET"`
	ProxyBaseURL            string   `env:"PROXY_BASE_URL" envAlias:"MCP_PROXY_BASE_URL" envDefault:"https://www.arleo.eu"`
	TokensFile              string   `env:"TOKENS_FILE" envAlias:"MCP_TOKEN_STORE" envDefault:"/opt/mcp-oauth-proxy/tokens.json"`
	TokensDB                string   `env:"TOKENS_DB" envAlias:"MCP_TOKEN_DB" envDefault:"/opt/mcp-oauth-proxy/tokens.db"`
	UseSQLite               bool     `env:"USE_SQLITE" envDefault:"true"`
	AuditLogFile            string   `env:"AUDIT_LOG_FILE" envAlias:"MCP_AUDIT_LOG" envDefault:"/var/log/mcp-oauth/audit.log"`
	MCPCACert               string   `env:"MCP_CA_CERT" envAlias:"MCP_CA_CERT"`
	AuthCodeTTL             int      `env:"AUTH_CODE_TTL" envDefault:"300"`
	AccessTokenTTL          int      `env:"ACCESS_TOKEN_TTL" envDefault:"86400"`
	TrustedProxies          []string `env:"TRUSTED_PROXIES" envDefault:"127.0.0.1,::1"`
	TrustedAuthorizeCIDRs   []string `env:"TRUSTED_AUTHORIZE_CIDRS" envDefault:"127.0.0.1/32,::1/128"`
	MandatoryPKCE           bool     `env:"MANDATORY_PKCE" envDefault:"true"`
	AllowTokenStoreRecovery bool     `env:"ALLOW_TOKEN_STORE_RECOVERY" envDefault:"false"`
}

type RuntimeConfig struct {
	ListenHost string `env:"LISTEN_HOST" envDefault:"127.0.0.1"`
	ListenPort int    `env:"LISTEN_PORT" envDefault:"8083"`
	ShadowMode bool   `env:"SHADOW_MODE" envDefault:"false"`
	LogLevel   string `env:"LOG_LEVEL" envDefault:"info"`
}

func Load() (*Config, error) {
	c := &Config{}
	if err := bindEnv(c); err != nil {
		return nil, err
	}

	// Emit deprecation warnings when legacy Grav env vars are used without their Hugo replacements.
	for _, pair := range [][2]string{
		{"GRAV_MCP_URL", "HUGO_MCP_URL"},
		{"GRAV_TOKEN", "HUGO_TOKEN"},
		{"GRAV_HOST", "HUGO_HOST"},
	} {
		legacy, replacement := pair[0], pair[1]
		if _, legacySet := os.LookupEnv(legacy); legacySet {
			if _, newSet := os.LookupEnv(replacement); !newSet {
				fmt.Fprintf(os.Stderr, "[WARN] %s is deprecated, use %s\n", legacy, replacement)
			}
		}
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Config) Validate() error {
	if c.OAuthProxy.ClientID == "" {
		return fmt.Errorf("CLIENT_ID is required")
	}
	if c.OAuthProxy.ClientSecret == "" {
		return fmt.Errorf("CLIENT_SECRET is required")
	}
	if c.OAuthProxy.HugoToken == "" {
		return fmt.Errorf("HUGO_TOKEN is required")
	}

	// Validate URLs: must be non-empty, parseable, and have http/https scheme.
	for envName, raw := range map[string]string{
		"HUGO_MCP_URL":   c.OAuthProxy.HugoMCPURL,
		"PROXY_BASE_URL": c.OAuthProxy.ProxyBaseURL,
	} {
		if raw == "" {
			return fmt.Errorf("%s must not be empty", envName)
		}
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("%s is not a valid URL: %w", envName, err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("%s must use http or https scheme, got %q", envName, u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("%s must include a host", envName)
		}
	}

	if c.OAuthProxy.AuthCodeTTL <= 0 {
		return fmt.Errorf("AUTH_CODE_TTL must be > 0, got %d", c.OAuthProxy.AuthCodeTTL)
	}
	if c.OAuthProxy.AccessTokenTTL <= 0 {
		return fmt.Errorf("ACCESS_TOKEN_TTL must be > 0, got %d", c.OAuthProxy.AccessTokenTTL)
	}

	return nil
}

// bindEnv is a lightweight reflection-based environment binder to address Change Propagation debt.
func bindEnv(iface interface{}) error {
	v := reflect.ValueOf(iface)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := v.Type().Field(i)

		if field.Kind() == reflect.Struct {
			if err := bindEnv(field.Addr().Interface()); err != nil {
				return err
			}
			continue
		}

		envKey := fieldType.Tag.Get("env")
		if envKey == "" {
			continue
		}

		envVal, exists := os.LookupEnv(envKey)
		if !exists {
			if alias := fieldType.Tag.Get("envAlias"); alias != "" {
				envVal, exists = os.LookupEnv(alias)
			}
		}
		if !exists {
			envVal = fieldType.Tag.Get("envDefault")
		}

		if !exists && envVal == "" {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			field.SetString(envVal)
		case reflect.Int:
			iv, err := strconv.Atoi(envVal)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", envKey, err)
			}
			field.SetInt(int64(iv))
		case reflect.Bool:
			bv, err := strconv.ParseBool(envVal)
			if err != nil {
				return fmt.Errorf("invalid value for %s: %w", envKey, err)
			}
			field.SetBool(bv)
		case reflect.Slice:
			if fieldType.Type.Elem().Kind() == reflect.String {
				parts := strings.Split(envVal, ",")
				var cleaned []string
				for _, p := range parts {
					cleaned = append(cleaned, strings.TrimSpace(p))
				}
				field.Set(reflect.ValueOf(cleaned))
			}
		}
	}
	return nil
}
