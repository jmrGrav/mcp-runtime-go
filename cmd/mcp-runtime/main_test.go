package main

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/runtime"
	"mcp-runtime-go/internal/storage"
)

type stubApp struct {
	runErr error
}

func (s stubApp) Run() error {
	return s.runErr
}

func restoreMainHooks() {
	loadConfigFn = func() (*config.Config, error) { return config.Load() }
	newAppFn = func(cfg *config.Config) (appRunner, error) { return runtime.NewApp(cfg) }
	migrateFn = storage.MigrateJSONToSQLite
	exitFn = os.Exit
	stderr = os.Stderr
}

func TestMain(m *testing.M) {
	code := m.Run()
	restoreMainHooks()
	os.Exit(code)
}

func TestRun_MigrateStorage(t *testing.T) {
	tmpDir := t.TempDir()
	jsonPath := filepath.Join(tmpDir, "tokens.json")
	sqlitePath := filepath.Join(tmpDir, "tokens.db")

	// Create dummy JSON store
	os.WriteFile(jsonPath, []byte("{}"), 0600)

	os.Setenv("CLIENT_ID", "id")
	os.Setenv("CLIENT_SECRET", "secret")
	os.Setenv("HUGO_TOKEN", "token")
	os.Setenv("TOKENS_FILE", jsonPath)
	os.Setenv("TOKENS_DB", sqlitePath)
	defer os.Unsetenv("CLIENT_ID")
	defer os.Unsetenv("CLIENT_SECRET")
	defer os.Unsetenv("HUGO_TOKEN")
	defer os.Unsetenv("TOKENS_FILE")
	defer os.Unsetenv("TOKENS_DB")

	// Test migration command
	args := []string{"mcp-runtime", "migrate-storage"}
	if err := run(args); err != nil {
		t.Fatalf("run(migrate-storage) failed: %v", err)
	}

	// Verify SQLite exists
	if _, err := os.Stat(sqlitePath); os.IsNotExist(err) {
		t.Error("SQLite database not created after migration")
	}
}

func TestRun_AppSuccess(t *testing.T) {
	origLoadConfig := loadConfigFn
	origNewApp := newAppFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		newAppFn = origNewApp
	})

	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{}, nil
	}
	newAppFn = func(cfg *config.Config) (appRunner, error) {
		return stubApp{runErr: nil}, nil
	}

	if err := run([]string{"mcp-runtime"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestRun_AppSuccess_WithCommandArg(t *testing.T) {
	origLoadConfig := loadConfigFn
	origNewApp := newAppFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		newAppFn = origNewApp
	})

	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{}, nil
	}
	newAppFn = func(cfg *config.Config) (appRunner, error) {
		return stubApp{runErr: nil}, nil
	}

	if err := run([]string{"mcp-runtime", "serve"}); err != nil {
		t.Fatalf("expected success for non-migration command, got %v", err)
	}
}

func TestRun_DefaultNewAppFn(t *testing.T) {
	origLoadConfig := loadConfigFn
	origNewApp := newAppFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		newAppFn = origNewApp
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		t.Fatal(err)
	}

	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{
			OAuthProxy: config.OAuthProxyConfig{
				ClientID:       "id",
				ClientSecret:   "secret",
				HugoToken:      "token",
				HugoMCPURL:     "http://127.0.0.1/api/mcp",
				ProxyBaseURL:   "http://127.0.0.1",
				TokensFile:     filepath.Join(t.TempDir(), "tokens.json"),
				AuditLogFile:   filepath.Join(t.TempDir(), "audit.log"),
				AuthCodeTTL:    300,
				AccessTokenTTL: 86400,
			},
			Runtime: config.RuntimeConfig{
				ListenHost: "127.0.0.1",
				ListenPort: port,
			},
		}, nil
	}

	if err := run([]string{"mcp-runtime"}); err == nil {
		t.Fatal("expected run to fail when default newAppFn hits occupied port")
	}
}

func TestRun_MigrateStorageError(t *testing.T) {
	origLoadConfig := loadConfigFn
	origMigrate := migrateFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		migrateFn = origMigrate
	})

	tmpDir := t.TempDir()
	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{
			OAuthProxy: config.OAuthProxyConfig{
				TokensFile: filepath.Join(tmpDir, "tokens.json"),
				TokensDB:   filepath.Join(tmpDir, "tokens.db"),
			},
		}, nil
	}
	migrateFn = func(_, _ string) error {
		return errors.New("boom")
	}

	if err := run([]string{"mcp-runtime", "migrate-storage"}); err == nil {
		t.Fatal("expected migration error")
	}
}

func TestRun_ConfigError(t *testing.T) {
	origLoadConfig := loadConfigFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
	})

	loadConfigFn = func() (*config.Config, error) {
		return nil, errors.New("load failed")
	}

	if err := run([]string{"mcp-runtime"}); err == nil {
		t.Error("expected error due to invalid config")
	}
}

func TestRun_AppInitError(t *testing.T) {
	origLoadConfig := loadConfigFn
	origNewApp := newAppFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		newAppFn = origNewApp
	})

	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{}, nil
	}
	newAppFn = func(cfg *config.Config) (appRunner, error) {
		return nil, errors.New("init failed")
	}

	if err := run([]string{"mcp-runtime"}); err == nil {
		t.Error("expected error during app initialization")
	}
}

func TestRun_AppRunError(t *testing.T) {
	origLoadConfig := loadConfigFn
	origNewApp := newAppFn
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		newAppFn = origNewApp
	})

	loadConfigFn = func() (*config.Config, error) {
		return &config.Config{}, nil
	}
	newAppFn = func(cfg *config.Config) (appRunner, error) {
		return stubApp{runErr: fmt.Errorf("run failed")}, nil
	}

	if err := run([]string{"mcp-runtime"}); err == nil {
		t.Error("expected error during app execution")
	}
}

func TestMain_ExitsOnRunError(t *testing.T) {
	origLoadConfig := loadConfigFn
	origExit := exitFn
	origStderr := stderr
	t.Cleanup(func() {
		loadConfigFn = origLoadConfig
		exitFn = origExit
		stderr = origStderr
	})

	loadConfigFn = func() (*config.Config, error) {
		return nil, errors.New("load failed")
	}

	var buf bytes.Buffer
	stderr = &buf
	called := false
	exitFn = func(code int) {
		called = true
		if code != 1 {
			t.Fatalf("expected exit code 1, got %d", code)
		}
	}

	main()

	if !called {
		t.Fatal("expected main to call exitFn")
	}
	if got := buf.String(); !strings.Contains(got, "[FATAL]") {
		t.Fatalf("expected fatal log on stderr, got %q", got)
	}
}
