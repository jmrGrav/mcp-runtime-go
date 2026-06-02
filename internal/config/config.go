package config

import (
	"fmt"
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
	GravMCPURL              string   `env:"GRAV_MCP_URL" envAlias:"MCP_BACKEND_URL" envDefault:"http://127.0.0.1/api/mcp"`
	GravHost                string   `env:"GRAV_HOST" envDefault:"www.arleo.eu"`
	GravToken               string   `env:"GRAV_TOKEN"`
	ClientID                string   `env:"CLIENT_ID" envAlias:"MCP_CLIENT_ID"`
	ClientSecret            string   `env:"CLIENT_SECRET" envAlias:"MCP_CLIENT_SECRET"`
	ProxyBaseURL            string   `env:"PROXY_BASE_URL" envAlias:"MCP_PROXY_BASE_URL" envDefault:"https://www.arleo.eu"`
	TokensFile              string   `env:"TOKENS_FILE" envAlias:"MCP_TOKEN_STORE" envDefault:"/opt/mcp-oauth-proxy/tokens.json"`
	AuditLogFile            string   `env:"AUDIT_LOG_FILE" envAlias:"MCP_AUDIT_LOG" envDefault:"/var/log/mcp-oauth/audit.log"`
	MCPCACert               string   `env:"MCP_CA_CERT" envAlias:"MCP_CA_CERT"`
	AuthCodeTTL             int      `env:"AUTH_CODE_TTL" envDefault:"300"`
	AccessTokenTTL          int      `env:"ACCESS_TOKEN_TTL" envDefault:"86400"`
	TrustedProxies          []string `env:"TRUSTED_PROXIES" envDefault:"127.0.0.1,::1"`
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
	if c.OAuthProxy.GravToken == "" {
		return fmt.Errorf("GRAV_TOKEN is required")
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
