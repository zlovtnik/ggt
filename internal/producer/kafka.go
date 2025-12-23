package producer

import (
	"context"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// ProducerConfig holds minimal producer configuration.
type ProducerConfig struct {
	Brokers []string
	Acks    string
}

// parseAcks converts string acks value to kgo.Acks
func parseAcks(acks string) kgo.Acks {
	switch acks {
	case "0":
		return kgo.NoAck()
	case "1":
		return kgo.LeaderAck()
	case "all":
		return kgo.AllISRAcks()
	default:
		// Default to all acks for safety
		return kgo.AllISRAcks()
	}
}

// Producer is a scaffold for a franz-go backed producer. Methods are
// intentionally minimal and act as a starting point for batching,
// compression and retry logic.
type Producer struct {
	client *kgo.Client
	cfg    ProducerConfig
	logger *zap.Logger
}

// NewProducer constructs a producer backed by franz-go.
func NewProducer(cfg ProducerConfig, logger *zap.Logger) (*Producer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.RequiredAcks(parseAcks(cfg.Acks)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create franz-go client: %w", err)
	}

	return &Producer{client: client, cfg: cfg, logger: logger.With(zap.String("component", "producer"))}, nil
}

// Send publishes a message. This is a placeholder; batching, compression
// and retries will be implemented later.
func (p *Producer) Send(ctx context.Context, topic string, key, value []byte) error {
	if p == nil {
		return fmt.Errorf("producer is nil")
	}
	// TODO: implement production using p.client.ProduceSync or batching.
	_ = topic
	_ = key
	_ = value
	// simulate small delay for pipeline compatibility
	timer := time.NewTimer(1 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Close shuts down the producer client.
func (p *Producer) Close() error {
	if p == nil || p.client == nil {
		return nil
	}
	p.client.Close()
	return nil
}
