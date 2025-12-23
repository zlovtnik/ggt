package metrics

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server wraps an HTTP server that exposes Prometheus metrics.
type Server struct {
	httpServer *http.Server
	logger     *zap.Logger
	port       int
}

// NewServer builds a metrics HTTP server that listens on the provided port.
func NewServer(port int, logger *zap.Logger) (*Server, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("metrics port %d is out of range (1-65535)", port)
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &Server{
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  120 * time.Second,
			Handler:      mux,
		},
		logger: logger.With(zap.String("server", "metrics")),
		port:   port,
	}, nil
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
		go func() {
			errCh <- fmt.Errorf("metrics server not configured")
			close(errCh)
		}()
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
		s.logger.Info("starting metrics server", zap.Int("port", s.port))
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errCh <- serveErr
			s.logger.Error("metrics server terminated unexpectedly", zap.Error(serveErr))
			if cancel != nil {
				cancel()
			}
		}
	}()
	return ready, errCh
}

// Shutdown gracefully stops the metrics server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
