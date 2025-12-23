package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/twmb/franz-go/pkg/kgo"
)

// ConsumerConfig contains basic configuration for the consumer wrapper.
type ConsumerConfig struct {
	Brokers []string
	GroupID string
	Topics  []string
}

// PartitionAssignment is a lightweight representation of a topic/partition
// assigned to this consumer. We avoid exposing franz-go types in the
// public consumer API so callers can remain decoupled from the client.
type PartitionAssignment struct {
	Topic     string
	Partition int32
}

// Consumer is a light wrapper around a franz-go client. Methods are
// intentionally minimal — the heavy lifting (rebalancing, offset
// management) lives in other packages and will be wired into this
// wrapper.
type Consumer struct {
	client *kgo.Client
	cfg    ConsumerConfig
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc
	// optional rebalance callback registered by callers
	rebalanceMu sync.Mutex
	rebalanceCb func(assigned []PartitionAssignment, revoked []PartitionAssignment)
}

// NewConsumer constructs a consumer wrapper backed by franz-go. It
// validates inputs and returns an error on invalid args.
func NewConsumer(cfg ConsumerConfig, logger *zap.Logger) (*Consumer, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("at least one broker is required")
	}
	if cfg.GroupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	if len(cfg.Topics) == 0 {
		return nil, fmt.Errorf("at least one topic is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	c := &Consumer{cfg: cfg, logger: logger.With(zap.String("component", "consumer")), ctx: ctx, cancel: cancel}

	// Build franz-go options and capture consumer pointer in callbacks so
	// we can notify registered handlers on rebalance events.
	opts := []kgo.Opt{
		kgo.SeedBrokers(cfg.Brokers...),
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(cfg.Topics...),
		// We'll manage commits manually (commit-on-success).
		kgo.DisableAutoCommit(),
		kgo.OnPartitionsAssigned(func(ctx context.Context, cl *kgo.Client, as map[string][]int32) {
			// convert assignment map to slice
			var assigned []PartitionAssignment
			for t, ps := range as {
				for _, p := range ps {
					assigned = append(assigned, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			if c != nil {
				c.notifyRebalance(assigned, nil)
			}
		}),
		kgo.OnPartitionsRevoked(func(ctx context.Context, cl *kgo.Client, rev map[string][]int32) {
			var revoked []PartitionAssignment
			for t, ps := range rev {
				for _, p := range ps {
					revoked = append(revoked, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			if c != nil {
				c.notifyRebalance(nil, revoked)
			}
			// OnPartitionsRevoked: users may want to commit in their handler.
			// We don't perform commits here; the registered handler should do so
			// if desired (e.g., via an OffsetManager).
		}),
		kgo.OnPartitionsLost(func(ctx context.Context, cl *kgo.Client, lost map[string][]int32) {
			var revoked []PartitionAssignment
			for t, ps := range lost {
				for _, p := range ps {
					revoked = append(revoked, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			if c != nil {
				c.notifyRebalance(nil, revoked)
			}
		}),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create franz-go client: %w", err)
	}

	c.client = client
	return c, nil
}

// Start currently registers the handler and returns immediately. The
// implementation is a placeholder — real consumption loop will use
// franz-go fetch APIs and the offset manager.
func (c *Consumer) Start(handler func(context.Context, []byte) error) error {
	if c == nil {
		return fmt.Errorf("consumer is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	// Start fetch loop
	go func() {
		defer func() {
			// ensure client closed when context cancels
			c.client.Close()
		}()

		c.logger.Info("consumer fetch loop started")

		for {
			select {
			case <-c.ctx.Done():
				c.logger.Info("consumer context cancelled; exiting fetch loop")
				return
			default:
			}

			// Poll for records (bounded number)
			fetches := c.client.PollRecords(c.ctx, 1024)
			// log partition errors
			fetches.EachError(func(topic string, partition int32, err error) {
				c.logger.Error("fetch error", zap.String("topic", topic), zap.Int32("partition", partition), zap.Error(err))
			})

			// Process each topic/partition's records
			fetches.EachTopic(func(ft kgo.FetchTopic) {
				ft.EachPartition(func(fp kgo.FetchPartition) {
					fp.EachRecord(func(r *kgo.Record) {
						// Call handler synchronously; handler is expected to provide its own queuing/backpressure.
						if err := handler(c.ctx, r.Value); err != nil {
							c.logger.Error("handler error; record not committed", zap.Error(err), zap.String("topic", r.Topic), zap.Int32("partition", r.Partition), zap.Int64("offset", r.Offset))
							return
						}

						// Commit this record on success (commit-on-success semantics)
						if err := c.client.CommitRecords(c.ctx, r); err != nil {
							c.logger.Error("failed to commit record", zap.Error(err), zap.String("topic", r.Topic), zap.Int32("partition", r.Partition), zap.Int64("offset", r.Offset))
						}
					})
				})
			})

			// small sleep to avoid hot-loop in edge cases
			time.Sleep(10 * time.Millisecond)
		}
	}()

	return nil
}

// RegisterRebalanceHandler registers a callback that will be invoked when
// partition assignments change. For now this is a local callback; when the
// franz-go balanced consumer is wired this will be invoked from the
// appropriate rebalance callbacks.
func (c *Consumer) RegisterRebalanceHandler(cb func(assigned []PartitionAssignment, revoked []PartitionAssignment)) {
	if c == nil {
		return
	}
	c.rebalanceMu.Lock()
	defer c.rebalanceMu.Unlock()
	c.rebalanceCb = cb
}

// notifyRebalance invokes the registered rebalance callback if present.
func (c *Consumer) notifyRebalance(assigned []PartitionAssignment, revoked []PartitionAssignment) {
	c.rebalanceMu.Lock()
	cb := c.rebalanceCb
	c.rebalanceMu.Unlock()
	if cb != nil {
		cb(assigned, revoked)
	}
}

// Stop stops the consumer and closes resources.
func (c *Consumer) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.cancel()
	// Allow franz-go client to close gracefully.
	c.client.Close()
	return nil
}
