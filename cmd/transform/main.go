package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/internal/consumer"
	"github.com/zlovtnik/ggt/internal/health"
	"github.com/zlovtnik/ggt/internal/logging"
	"github.com/zlovtnik/ggt/internal/metrics"
	"github.com/zlovtnik/ggt/internal/producer"
	"github.com/zlovtnik/ggt/internal/shutdown"
	"github.com/zlovtnik/ggt/internal/transform"
	_ "github.com/zlovtnik/ggt/internal/transform/data"
	_ "github.com/zlovtnik/ggt/internal/transform/field"
	_ "github.com/zlovtnik/ggt/internal/transform/filter"
	_ "github.com/zlovtnik/ggt/internal/transform/validate"
	"github.com/zlovtnik/ggt/pkg/event"
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

	worker := startPipelineWorker(ctx, logger, metricsCollector, &cfg)
	coordinator.Register("pipeline worker", shutdown.ShutdownFunc(worker.Stop))

	healthServer, err := health.NewServer(cfg.Service.HealthPort, logger)
	if err != nil {
		logger.Error("failed to create health server", zap.Error(err))
		os.Exit(1)
	}
	if healthServer != nil {
		if err := healthServer.AddCheck("pipeline worker", worker.HealthCheck); err != nil {
			logger.Warn("failed to register health check", zap.Error(err))
		}
		ready, errCh := healthServer.Start(cancel)
		go func() {
			select {
			case <-ready:
				logger.Info("health server ready", zap.Int("port", cfg.Service.HealthPort))
			case err := <-errCh:
				if err != nil {
					logger.Error("health server failed to start", zap.Error(err))
				}
				return
			}
			for err := range errCh {
				if err != nil {
					logger.Error("health server error", zap.Error(err))
				}
			}
		}()
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

type pipelineInfo struct {
	pipe *transform.Pipeline
	cfg  *config.PipelineConfig
}

type pipelineWorker struct {
	cancel  context.CancelFunc
	done    chan struct{}
	metrics *metrics.Metrics
	cons    *consumer.Consumer
	prod    *producer.Producer
}

func startPipelineWorker(parent context.Context, logger *zap.Logger, metricsCollector *metrics.Metrics, cfg *config.Config) *pipelineWorker {
	ctx, cancel := context.WithCancel(parent)
	w := &pipelineWorker{cancel: cancel, done: make(chan struct{}), metrics: metricsCollector}

	// Collect input topics
	inputTopics := make(map[string]bool)
	for _, p := range cfg.Transforms.Pipelines {
		for _, t := range p.InputTopics {
			inputTopics[t] = true
		}
	}
	topics := make([]string, 0, len(inputTopics))
	for t := range inputTopics {
		topics = append(topics, t)
	}

	// Create pipelines map
	pipelines := make(map[string]*pipelineInfo)
	for _, p := range cfg.Transforms.Pipelines {
		pipe, err := transform.NewPipeline(p.Name, p.Transforms)
		if err != nil {
			logger.Error("failed to create pipeline", zap.String("name", p.Name), zap.Error(err))
			return nil
		}
		info := &pipelineInfo{pipe: pipe, cfg: &p}
		for _, t := range p.InputTopics {
			pipelines[t] = info
		}
	}

	// Create consumer
	consumerCfg := consumer.ConsumerConfig{
		Brokers:         cfg.Kafka.Consumer.Brokers,
		GroupID:         cfg.Kafka.Consumer.GroupID,
		Topics:          topics,
		HandlerTimeout:  30 * time.Second,
		FetchMaxRecords: cfg.Kafka.Consumer.FetchMaxRecords,
		PollIntervalMs:  cfg.Kafka.Consumer.PollIntervalMs,
	}
	cons, err := consumer.NewConsumer(consumerCfg, logger)
	if err != nil {
		logger.Error("failed to create consumer", zap.Error(err))
		return nil
	}
	w.cons = cons

	// Create producer
	producerCfg := producer.ProducerConfig{
		Brokers: cfg.Kafka.Producer.Brokers,
		Acks:    cfg.Kafka.Producer.Acks,
	}
	prod, err := producer.NewProducer(producerCfg, logger)
	if err != nil {
		logger.Error("failed to create producer", zap.Error(err))
		return nil
	}
	w.prod = prod

	handler := func(hctx context.Context, r *kgo.Record) error {
		topic := r.Topic
		info, ok := pipelines[topic]
		if !ok {
			logger.Warn("no pipeline for topic", zap.String("topic", topic))
			return nil
		}

		ev := event.Event{
			Payload: make(map[string]interface{}),
			Metadata: event.Metadata{
				Topic:     r.Topic,
				Partition: r.Partition,
				Offset:    r.Offset,
				Key:       r.Key,
			},
			Timestamp: r.Timestamp,
			Headers:   make(map[string]string),
		}

		if err := json.Unmarshal(r.Value, &ev.Payload); err != nil {
			logger.Error("failed to unmarshal payload", zap.Error(err))
			return err
		}

		result, err := info.pipe.Execute(hctx, ev)
		if err != nil {
			if err == transform.ErrDrop {
				return nil
			}
			logger.Error("pipeline error", zap.Error(err))
			// Send to DLQ
			dlqTopic := info.cfg.DLQTopic
			if dlqTopic != "" {
				dlqValue, _ := json.Marshal(result.(event.Event).Payload)
				if sendErr := w.prod.Send(hctx, dlqTopic, r.Key, dlqValue); sendErr != nil {
					logger.Error("failed to send to DLQ", zap.Error(sendErr))
				}
			}
			return err
		}

		// Send to output
		outputTopic := info.cfg.OutputTopic
		outputValue, _ := json.Marshal(result.(event.Event).Payload)
		if err := w.prod.Send(hctx, outputTopic, r.Key, outputValue); err != nil {
			logger.Error("failed to send to output", zap.Error(err))
			return err
		}
		return nil
	}

	if err := cons.Start(handler); err != nil {
		logger.Error("failed to start consumer", zap.Error(err))
		return nil
	}

	go func() {
		logger.Info("pipeline worker started")
		defer close(w.done)
		<-ctx.Done()
		logger.Info("pipeline worker stopping")
		if w.cons != nil {
			w.cons.Stop(context.Background())
		}
		if w.prod != nil {
			w.prod.Close()
		}
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
