package main

import (
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/runtime"
	"os"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] failed to load config: %v\n", err)
		os.Exit(1)
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
