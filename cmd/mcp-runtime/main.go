package main

import (
	"fmt"
	"io"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/runtime"
	"mcp-runtime-go/internal/storage"
	"os"
)

type appRunner interface {
	Run() error
}

var (
	loadConfigFn           = config.Load
	newAppFn               = func(cfg *config.Config) (appRunner, error) { return runtime.NewApp(cfg) }
	migrateFn              = storage.MigrateJSONToSQLite
	exitFn                 = os.Exit
	stderr       io.Writer = os.Stderr
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(stderr, "[FATAL] %v\n", err)
		exitFn(1)
	}
}

func run(args []string) error {
	cfg, err := loadConfigFn()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Explicit administrative migration command
	if len(args) > 1 && args[1] == "migrate-storage" {
		fmt.Printf("[INFO] starting storage migration: %s -> %s\n", cfg.OAuthProxy.TokensFile, cfg.OAuthProxy.TokensDB)
		if err := migrateFn(cfg.OAuthProxy.TokensFile, cfg.OAuthProxy.TokensDB); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		fmt.Println("[INFO] migration successful")
		return nil
	}

	app, err := newAppFn(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize app: %w", err)
	}

	if err := app.Run(); err != nil {
		return fmt.Errorf("app exited with error: %w", err)
	}
	return nil
}
