package httpserver

import (
	"context"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"net/http"
	"time"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
}

func New(cfg *config.Config, handler http.Handler) *Server {
	addr := fmt.Sprintf("%s:%d", cfg.Runtime.ListenHost, cfg.Runtime.ListenPort)
	return &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Addr:    addr,
			Handler: handler,
			// ReadTimeout covers reading the request headers and body.
			ReadTimeout: 10 * time.Second,
			// WriteTimeout must exceed the backend client Timeout (30 s) to avoid
			// the server closing the connection before the proxy response arrives.
			WriteTimeout: 60 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	observability.Logger.Info("server starting", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	observability.Logger.Info("server stopping")
	return s.httpServer.Shutdown(ctx)
}
