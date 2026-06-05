package httpserver

import (
	"context"
	"fmt"
	"mcp-runtime-go/internal/config"
	"mcp-runtime-go/internal/observability"
	"net"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	mu         sync.Mutex
	boundAddr  string
	ready      chan struct{}
}

func New(cfg *config.Config, handler http.Handler) *Server {
	addr := fmt.Sprintf("%s:%d", cfg.Runtime.ListenHost, cfg.Runtime.ListenPort)
	return &Server{
		cfg:   cfg,
		ready: make(chan struct{}),
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

// Start binds the listen socket and serves. The actual bound address (needed when
// ListenPort is 0) is available via Addr() once Start returns from its bind phase,
// which is signalled via the ready channel exposed to WaitReady.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.boundAddr = ln.Addr().String()
	s.mu.Unlock()
	close(s.ready)
	observability.Logger.Info("server starting", "addr", s.boundAddr)
	return s.httpServer.Serve(ln)
}

// Addr returns the actual bound address. Valid only after Start has progressed past bind.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.boundAddr
}

// WaitReady blocks until the server has bound its listen socket or the context is done.
func (s *Server) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) Stop(ctx context.Context) error {
	observability.Logger.Info("server stopping")
	return s.httpServer.Shutdown(ctx)
}
