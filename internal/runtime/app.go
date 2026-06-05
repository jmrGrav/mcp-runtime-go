package runtime

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/httpserver"
	"mcp-runtime-go/internal/oauthproxy"
	"mcp-runtime-go/internal/observability"
	"mcp-runtime-go/internal/storage"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type App struct {
	cfg    *config.Config
	server *httpserver.Server
	oauth  *oauthproxy.Service
}

func NewApp(cfg *config.Config) (*App, error) {
	// Initialize observability
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Runtime.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		if cfg.Runtime.ShadowMode {
			level = slog.LevelDebug
		}
	}
	observability.InitLogger(level)

	// Initialize infrastructure (HTTP Client with TLS)
	httpClient, err := createHTTPClient(cfg.OAuthProxy.MCPCACert)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	// Initialize components
	audit := observability.NewAuditLogger(cfg.OAuthProxy.AuditLogFile)

	var store storage.Store
	if cfg.OAuthProxy.UseSQLite {
		sqliteStore, err := storage.NewSQLiteStore(cfg.OAuthProxy.TokensDB)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize sqlite store: %w", err)
		}
		store = sqliteStore
	} else {
		store = storage.NewTokenStore(cfg.OAuthProxy.TokensFile, cfg.OAuthProxy.AllowTokenStoreRecovery)
	}

	oauthSvc, err := oauthproxy.NewService(cfg, store, audit, httpClient)
	if err != nil {
		return nil, err
	}

	handler := newHandler(oauthSvc)
	server := httpserver.New(cfg, handler)

	return &App{
		cfg:    cfg,
		server: server,
		oauth:  oauthSvc,
	}, nil
}

func newHandler(oauthSvc *oauthproxy.Service) http.Handler {
	mux := http.NewServeMux()

	// OAuth Discovery
	mux.HandleFunc("/.well-known/oauth-authorization-server", oauthSvc.HandleMetadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource", oauthSvc.HandleProtectedResourceMetadata)
	mux.HandleFunc("/.well-known/oauth-protected-resource/", oauthSvc.HandleProtectedResourceMetadata) // Simple suffixed handler

	// OAuth Endpoints
	mux.HandleFunc("/register", oauthSvc.HandleRegister)
	mux.HandleFunc("/authorize", oauthSvc.HandleAuthorize)
	mux.HandleFunc("/token", oauthSvc.HandleToken)

	// MCP Proxy
	mux.HandleFunc("/mcp", oauthSvc.HandleProxy)
	mux.HandleFunc("/mcp/", oauthSvc.HandleProxy)
	health := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
	readyz := func(w http.ResponseWriter, r *http.Request) {
		if err := oauthSvc.Ready(); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}

	// Health and readiness aliases for shadow deployment probes.
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/readyz", readyz)

	return RequestIDMiddleware(mux)
}

func (a *App) Run() error {
	defer a.oauth.Close()

	// Start purge loop
	go a.oauth.StartPurgeLoop()

	errChan := make(chan error, 1)
	go func() {
		errChan <- a.server.Start()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		observability.Logger.Info("received signal", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	return a.server.Stop(ctx)
}

func createHTTPClient(caCertPath string) (*http.Client, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA cert: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = caCertPool
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}, nil
}
