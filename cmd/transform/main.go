package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
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
	_ "github.com/zlovtnik/ggt/internal/transform/aggregate"
	_ "github.com/zlovtnik/ggt/internal/transform/data"
	_ "github.com/zlovtnik/ggt/internal/transform/field"
	_ "github.com/zlovtnik/ggt/internal/transform/filter"
	_ "github.com/zlovtnik/ggt/internal/transform/split"
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
	zap.ReplaceGlobals(logger)
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

	worker, err := startPipelineWorker(ctx, logger, metricsCollector, &cfg)
	if err != nil {
		logger.Error("failed to start pipeline worker", zap.Error(err))
		os.Exit(1)
	}
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

func startPipelineWorker(parent context.Context, logger *zap.Logger, metricsCollector *metrics.Metrics, cfg *config.Config) (*pipelineWorker, error) {
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
			return nil, err
		}
		info := &pipelineInfo{pipe: pipe, cfg: &p}
		for _, t := range p.InputTopics {
			if existing, ok := pipelines[t]; ok {
				return nil, fmt.Errorf("duplicate input topic %q used by pipelines %q and %q", t, existing.cfg.Name, p.Name)
			}
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
		return nil, err
	}
	w.cons = cons
	var consForDefer = cons
	defer func() {
		if consForDefer != nil {
			consForDefer.Stop(context.Background())
		}
	}()

	// Create producer
	producerCfg := producer.ProducerConfig{
		Brokers: cfg.Kafka.Producer.Brokers,
		Acks:    cfg.Kafka.Producer.Acks,
	}
	prod, err := producer.NewProducer(producerCfg, logger)
	if err != nil {
		logger.Error("failed to create producer", zap.Error(err))
		return nil, err
	}
	w.prod = prod
	var prodForDefer = prod
	defer func() {
		if prodForDefer != nil {
			prodForDefer.Close()
		}
	}()

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
			logger.Error("failed to unmarshal payload", zap.Error(err), zap.String("topic", topic), zap.Int32("partition", r.Partition), zap.Int64("offset", r.Offset))

			dlqTopic := strings.TrimSpace(info.cfg.DLQTopic)
			if dlqTopic != "" {
				if sendErr := w.prod.Send(hctx, dlqTopic, r.Key, r.Value); sendErr != nil {
					logger.Error("failed to send malformed payload to DLQ", zap.Error(sendErr), zap.String("dlq_topic", dlqTopic))
					return sendErr
				}
				if w.metrics != nil {
					w.metrics.MessagesSentToDLQ.Inc()
				}
				return nil
			}

			if w.metrics != nil {
				w.metrics.MessagesDropped.Inc()
			}
			return nil
		}

		logger.Debug("processing message", zap.String("topic", topic), zap.Int64("offset", r.Offset))

		result, err := info.pipe.Execute(hctx, ev)
		if err != nil {
			if err == transform.ErrDrop {
				if w.metrics != nil {
					w.metrics.MessagesDropped.Inc()
				}
				return nil
			}

			logger.Error("pipeline error", zap.Error(err), zap.String("pipeline", info.cfg.Name))

			dlqTopic := strings.TrimSpace(info.cfg.DLQTopic)
			if dlqTopic != "" {
				var dlqPayload []byte
				switch evt := result.(type) {
				case event.Event:
					marshaled, marshalErr := json.Marshal(evt.Payload)
					if marshalErr != nil {
						logger.Error("failed to marshal DLQ payload", zap.Error(marshalErr), zap.String("pipeline", info.cfg.Name))
						dlqPayload = r.Value
					} else {
						dlqPayload = marshaled
					}
				default:
					dlqPayload = r.Value
				}

				if sendErr := w.prod.Send(hctx, dlqTopic, r.Key, dlqPayload); sendErr != nil {
					logger.Error("failed to send to DLQ", zap.Error(sendErr), zap.String("dlq_topic", dlqTopic))
					return sendErr
				}
				if w.metrics != nil {
					w.metrics.MessagesSentToDLQ.Inc()
				}
				return nil
			}

			if w.metrics != nil {
				w.metrics.MessagesDropped.Inc()
			}
			return nil
		}

		// Send to output - handle both single and multiple events
		outputTopic := strings.TrimSpace(info.cfg.OutputTopic)
		if outputTopic == "" {
			logger.Error("output topic is not configured or empty")
			return fmt.Errorf("output topic is required but not configured")
		}

		appendIndexToKey := func(base []byte, idx int) []byte {
			if base == nil {
				return strconv.AppendInt([]byte("multi-"), int64(idx), 10)
			}
			copied := make([]byte, len(base))
			copy(copied, base)
			copied = append(copied, '-')
			return strconv.AppendInt(copied, int64(idx), 10)
		}

		switch result := result.(type) {
		case event.Event:
			// Check if event should go to DLQ
			if dlqReason, hasDLQ := result.Headers["_dlq_reason"]; hasDLQ && dlqReason != "" {
				dlqTopic := info.cfg.DLQTopic
				if dlqTopic != "" {
					dlqValue, err := json.Marshal(result.Payload)
					if err != nil {
						logger.Error("failed to marshal DLQ payload", zap.Error(err), zap.Any("payload", result.Payload))
						return err
					}
					if err := w.prod.Send(hctx, dlqTopic, r.Key, dlqValue); err != nil {
						logger.Error("failed to send to DLQ", zap.Error(err))
						return err
					}
					if w.metrics != nil {
						w.metrics.MessagesSentToDLQ.Inc()
					}
					return nil
				}
			}
			// Single event output
			outputValue, err := json.Marshal(result.Payload)
			if err != nil {
				logger.Error("failed to marshal output payload", zap.Error(err), zap.Any("payload", result.Payload))
				return err
			}
			// Check for routing header
			sendTopic := outputTopic
			if routeTarget, ok := result.Headers["_route_target"]; ok && routeTarget != "" {
				sendTopic = routeTarget
				// Increment routing metrics
				if w.metrics != nil {
					w.metrics.RoutingMetrics.WithLabelValues("filter.route", sendTopic).Inc()
				}
			}
			if err := w.prod.Send(hctx, sendTopic, r.Key, outputValue); err != nil {
				logger.Error("failed to send to output", zap.Error(err))
				return err
			}
		case []event.Event:
			// Multiple event output - send each to the output topic
			var failedIndices []int
			dlqTopic := info.cfg.DLQTopic
			for i, evt := range result {
				// Check if event should go to DLQ
				if dlqReason, hasDLQ := evt.Headers["_dlq_reason"]; hasDLQ && dlqReason != "" {
					if dlqTopic != "" {
						dlqValue, err := json.Marshal(evt.Payload)
						if err != nil {
							logger.Error("failed to marshal DLQ payload", zap.Error(err), zap.Int("index", i), zap.Any("payload", evt.Payload))
							failedIndices = append(failedIndices, i)
							continue
						}
						key := r.Key
						if len(result) > 1 {
							key = appendIndexToKey(key, i)
						}
						if err := w.prod.Send(hctx, dlqTopic, key, dlqValue); err != nil {
							logger.Error("failed to send to DLQ", zap.Error(err), zap.Int("index", i))
							failedIndices = append(failedIndices, i)
							continue
						}
						if w.metrics != nil {
							w.metrics.MessagesSentToDLQ.Inc()
						}
						continue
					}
				}
				outputValue, err := json.Marshal(evt.Payload)
				if err != nil {
					logger.Error("failed to marshal output payload", zap.Error(err), zap.Int("index", i), zap.Any("payload", evt.Payload))
					failedIndices = append(failedIndices, i)
					continue
				}
				// For multiple messages, append index to key to avoid duplicates (using "multi-%d" for nil keys)
				key := r.Key
				if len(result) > 1 {
					// Append index to key to make it unique
					key = appendIndexToKey(key, i)
				}
				// Check for routing header
				sendTopic := outputTopic
				if routeTarget, ok := evt.Headers["_route_target"]; ok && routeTarget != "" {
					sendTopic = routeTarget
					// Increment routing metrics
					if w.metrics != nil {
						w.metrics.RoutingMetrics.WithLabelValues("filter.route", sendTopic).Inc()
					}
				}
				if err := w.prod.Send(hctx, sendTopic, key, outputValue); err != nil {
					logger.Error("failed to send to output", zap.Error(err), zap.Int("index", i))
					// Send to DLQ if configured
					if dlqTopic != "" {
						dlqValue, marshalErr := json.Marshal(evt.Payload)
						if marshalErr != nil {
							logger.Error("failed to marshal DLQ payload", zap.Error(marshalErr), zap.Int("index", i))
						} else if sendErr := w.prod.Send(hctx, dlqTopic, key, dlqValue); sendErr != nil {
							logger.Error("failed to send to DLQ", zap.Error(sendErr), zap.Int("index", i))
						}
					}
					failedIndices = append(failedIndices, i)
					continue
				}
			}
			if len(failedIndices) > 0 {
				return fmt.Errorf("send failures at indices: %v", failedIndices)
			}
		default:
			logger.Error("unexpected result type from pipeline", zap.String("type", fmt.Sprintf("%T", result)))
			return fmt.Errorf("unexpected result type: %T", result)
		}

		if w.metrics != nil {
			w.metrics.MessagesProcessed.Inc()
		}

		return nil
	}

	if err := cons.Start(handler); err != nil {
		logger.Error("failed to start consumer", zap.Error(err))
		return nil, err
	}

	// Cancel deferred cleanup since startup succeeded
	consForDefer = nil
	prodForDefer = nil

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
	return w, nil
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
