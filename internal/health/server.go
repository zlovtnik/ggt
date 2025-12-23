package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CheckFunc examines the runtime state and reports an error when unhealthy.
type CheckFunc func(context.Context) error

// Server exposes health endpoints for Kubernetes probes.
type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
	port       int
	checks     []namedCheck
	mu         sync.RWMutex
}

type namedCheck struct {
	name  string
	check CheckFunc
}

// NewServer builds an HTTP server that listens on the provided port.
func NewServer(port int, logger *zap.Logger) *Server {
	if port <= 0 || logger == nil {
		return nil
	}
	mux := http.NewServeMux()
	hs := &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
			Handler:      mux,
		},
		logger: logger.With(zap.String("server", "health")),
		port:   port,
	}
	mux.HandleFunc("/healthz", hs.handleHealth)
	return hs
}

// AddCheck registers a named check that runs during /healthz requests.
func (s *Server) AddCheck(name string, check CheckFunc) error {
	if s == nil || check == nil {
		return fmt.Errorf("server or check is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("check name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// prevent duplicate names
	for _, nc := range s.checks {
		if nc.name == name {
			return fmt.Errorf("check with name %q already registered", name)
		}
	}
	s.checks = append(s.checks, namedCheck{name: name, check: check})
	return nil
}

// Start begins serving health endpoints in the background.
func (s *Server) Start(cancel context.CancelFunc) {
	if s == nil || s.httpServer == nil {
		return
	}
	go func() {
		s.logger.Info("starting health server", zap.Int("port", s.port))
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("health server terminated unexpectedly", zap.Error(err))
			if cancel != nil {
				cancel()
			}
		}
	}()
}

// Shutdown gracefully stops the health server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	s.mu.RLock()
	checks := make([]namedCheck, len(s.checks))
	copy(checks, s.checks)
	s.mu.RUnlock()
	for _, check := range checks {
		if err := check.check(ctx); err != nil {
			s.logger.Warn("health check failed", zap.String("check", check.name), zap.Error(err))
			http.Error(w, fmt.Sprintf("%s: %v", check.name, err), http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
