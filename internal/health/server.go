package health

import (
	"context"
	"errors"
	"fmt"
	"net"
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
func NewServer(port int, logger *zap.Logger) (*Server, error) {
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid port: must be 1-65535")
	}
	if logger == nil {
		return nil, fmt.Errorf("nil logger provided")
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
	return hs, nil
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

// Start attempts to bind to the configured address and, on success, returns
// a ready channel that is closed when listening begins along with an
// err channel that reports any startup or runtime failures encountered by
// the server. Callers can wait on the channels to detect readiness or
// failure before proceeding.
func (s *Server) Start(cancel context.CancelFunc) (<-chan struct{}, <-chan error) {
	ready := make(chan struct{})
	errCh := make(chan error, 1)
	if s == nil || s.httpServer == nil {
		if s != nil && s.logger != nil {
			s.logger.Error("cannot start health server: server or httpServer is nil")
		}
		errCh <- fmt.Errorf("health server not configured")
		close(errCh)
		return ready, errCh
	}

	listener, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		errCh <- err
		close(errCh)
		return ready, errCh
	}
	close(ready)
	go func() {
		defer close(errCh)
		s.logger.Info("starting health server", zap.Int("port", s.port))
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
			s.logger.Error("health server terminated unexpectedly", zap.Error(serveErr))
			if cancel != nil {
				cancel()
			}
		}
	}()
	return ready, errCh
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
			http.Error(w, "unhealthy", http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
