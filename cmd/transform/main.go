package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/internal/health"
	"github.com/zlovtnik/ggt/internal/logging"
	"github.com/zlovtnik/ggt/internal/metrics"
	"github.com/zlovtnik/ggt/internal/shutdown"
	"go.uber.org/zap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.ApplyEnvOverrides(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply env overrides: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "invalid configuration: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.NewLogger(cfg.Service.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("transform service booting", zap.String("service", cfg.Service.Name))

	metricsCollector := metrics.NewWithNamespace(cfg.Metrics.Namespace, cfg.Metrics.Subsystem, cfg.Metrics.Buckets)
	coordinator := shutdown.NewCoordinator(logger)

	var metricsServer *metrics.Server
	if cfg.Service.MetricsPort > 0 {
		var err error
		metricsServer, err = metrics.NewServer(cfg.Service.MetricsPort, logger)
		if err != nil {
			logger.Warn("metrics server disabled", zap.Error(err))
		}
	}
	if metricsServer != nil {
		ready, errCh := metricsServer.Start(cancel)
		go func() {
			select {
			case <-ready:
				logger.Info("metrics server ready", zap.Int("port", cfg.Service.MetricsPort))
			case err := <-errCh:
				if err != nil {
					logger.Error("metrics server failed to start", zap.Error(err))
				}
				return
			}
			for err := range errCh {
				if err != nil {
					logger.Error("metrics server error", zap.Error(err))
				}
			}
		}()
		coordinator.Register("metrics server", metricsServer)
	}

	worker := startPipelineWorker(ctx, logger, metricsCollector)
	coordinator.Register("pipeline worker", shutdown.ShutdownFunc(worker.Stop))

	healthServer := health.NewServer(cfg.Service.HealthPort, logger)
	if healthServer != nil {
		if err := healthServer.AddCheck("pipeline worker", worker.HealthCheck); err != nil {
			logger.Warn("failed to register health check", zap.Error(err))
		}
		healthServer.Start(cancel)
		coordinator.Register("health server", healthServer)
	}

	logger.Info("entering shutdown wait", zap.String("timeout", cfg.Service.ShutdownTimeout.String()))
	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer shutdownCancel()
	coordinator.Shutdown(shutdownCtx)

	logger.Info("shutdown complete")
}

type pipelineWorker struct {
	cancel  context.CancelFunc
	done    chan struct{}
	metrics *metrics.Metrics
}

func startPipelineWorker(parent context.Context, logger *zap.Logger, metricsCollector *metrics.Metrics) *pipelineWorker {
	ctx, cancel := context.WithCancel(parent)
	w := &pipelineWorker{cancel: cancel, done: make(chan struct{}), metrics: metricsCollector}
	go func() {
		logger.Info("pipeline worker started")
		defer close(w.done)
		<-ctx.Done()
		logger.Info("pipeline worker stopping")
	}()
	return w
}

func (w *pipelineWorker) Stop(ctx context.Context) error {
	if w == nil {
		return nil
	}
	w.cancel()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *pipelineWorker) HealthCheck(_ context.Context) error {
	select {
	case <-w.done:
		return fmt.Errorf("pipeline worker has stopped")
	default:
		return nil
	}
}
