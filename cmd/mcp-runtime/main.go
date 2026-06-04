package main

import (
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/runtime"
	"mcp-runtime-go/internal/storage"
	"os"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Explicit administrative migration command
	if len(os.Args) > 1 && os.Args[1] == "migrate-storage" {
		fmt.Printf("[INFO] starting storage migration: %s -> %s\n", cfg.OAuthProxy.TokensFile, cfg.OAuthProxy.TokensDB)
		if err := storage.MigrateJSONToSQLite(cfg.OAuthProxy.TokensFile, cfg.OAuthProxy.TokensDB); err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] migration failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[INFO] migration successful")
		return
	}

	app, err := runtime.NewApp(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] failed to initialize app: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] app exited with error: %v\n", err)
		os.Exit(1)
	}
}
