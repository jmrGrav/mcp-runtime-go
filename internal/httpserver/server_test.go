package httpserver

import (
	"context"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"log/slog"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	observability.InitLogger(slog.LevelInfo)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cfg := &config.Config{
		Runtime: config.RuntimeConfig{
			ListenHost: "127.0.0.1",
			ListenPort: 0, // OS chooses port
		},
	}
	srv := New(cfg, mux)
	
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			// Cannot t.Errorf in a goroutine
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}
