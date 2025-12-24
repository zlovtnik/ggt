package consumer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.uber.org/zap"
)

// ConsumerConfig contains basic configuration for the consumer wrapper.
type ConsumerConfig struct {
	Brokers []string
	GroupID string
	Topics  []string
	// HandlerTimeout, if set, limits how long a handler invocation may run.
	// If zero, a sensible default (30s) is used.
	HandlerTimeout  time.Duration
	FetchMaxRecords int
	PollIntervalMs  int
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
	rebalanceMu    sync.Mutex
	rebalanceCb    func(assigned []PartitionAssignment, revoked []PartitionAssignment)
	wg             sync.WaitGroup
	done           chan struct{}
	doneOnce       sync.Once
	closeOnce      sync.Once
	started        uint32
	handlerTimeout time.Duration
	errCh          chan error
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
			var assigned []PartitionAssignment
			for t, ps := range as {
				for _, p := range ps {
					assigned = append(assigned, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			c.notifyRebalance(assigned, nil)
		}),
		kgo.OnPartitionsRevoked(func(ctx context.Context, cl *kgo.Client, rev map[string][]int32) {
			var revoked []PartitionAssignment
			for t, ps := range rev {
				for _, p := range ps {
					revoked = append(revoked, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			c.notifyRebalance(nil, revoked)
		}),
		kgo.OnPartitionsLost(func(ctx context.Context, cl *kgo.Client, lost map[string][]int32) {
			var revoked []PartitionAssignment
			for t, ps := range lost {
				for _, p := range ps {
					revoked = append(revoked, PartitionAssignment{Topic: t, Partition: p})
				}
			}
			c.notifyRebalance(nil, revoked)
		}),
	}

	client, err := kgo.NewClient(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create franz-go client: %w", err)
	}

	c.client = client
	c.done = make(chan struct{})
	c.errCh = make(chan error, 10) // buffered to track multiple errors
	if cfg.HandlerTimeout > 0 {
		c.handlerTimeout = cfg.HandlerTimeout
	} else {
		c.handlerTimeout = 30 * time.Second
	}
	return c, nil
}

// Start registers the handler and starts the fetch loop. Multiple Start
// calls are guarded against and will return an error if already started.
func (c *Consumer) Start(handler func(context.Context, *kgo.Record) error) error {
	if c == nil {
		return fmt.Errorf("consumer is nil")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	// Prevent multiple starts
	if !atomic.CompareAndSwapUint32(&c.started, 0, 1) {
		return fmt.Errorf("consumer already started")
	}

	// Start fetch loop
	c.wg.Add(1)
	go func() {
		defer func() {
			c.doneOnce.Do(func() { close(c.done) })
			c.wg.Done()
		}()

		c.logger.Info("consumer fetch loop started")

	fetchLoop:
		for {
			select {
			case <-c.ctx.Done():
				c.logger.Info("consumer context cancelled; exiting fetch loop")
				return
			default:
			}

			// Poll for records (bounded number)
			fetchMax := c.cfg.FetchMaxRecords
			if fetchMax <= 0 {
				fetchMax = 1024
			}
			fetches := c.client.PollRecords(c.ctx, fetchMax)
			// log partition errors
			fetches.EachError(func(topic string, partition int32, err error) {
				c.logger.Error("fetch error", zap.String("topic", topic), zap.Int32("partition", partition), zap.Error(err))
			})

			var abortErr error

			// Process each topic/partition's records
			fetches.EachTopic(func(ft kgo.FetchTopic) {
				ft.EachPartition(func(fp kgo.FetchPartition) {
					fp.EachRecord(func(r *kgo.Record) {
						// Wrap handler invocation with a timeout derived from consumer context
						hctx, hcancel := context.WithTimeout(c.ctx, c.handlerTimeout)
						defer hcancel()
						// ensure we don't leak timers — call cancel after handler returns
						if err := handler(hctx, r); err != nil {
							c.logger.Error("handler error; record not committed", zap.Error(err), zap.String("topic", r.Topic), zap.Int32("partition", r.Partition), zap.Int64("offset", r.Offset))
							// record the error and abort after this fetch iteration
							abortErr = err
							return
						}

						// Commit this record on success (commit-on-success semantics)
						if err := c.client.CommitRecords(c.ctx, r); err != nil {
							c.logger.Error("failed to commit record", zap.Error(err), zap.String("topic", r.Topic), zap.Int32("partition", r.Partition), zap.Int64("offset", r.Offset))
						}
					})
				})
			})

			if abortErr != nil {
				// abort the fetch loop so callers can handle the error (via consumer stop or higher-level retry)
				c.logger.Warn("aborting fetch iteration due to handler error", zap.Error(abortErr))
				select {
				case c.errCh <- abortErr:
				default:
					c.logger.Warn("error channel buffer full, dropping error", zap.Error(abortErr))
				}
				break fetchLoop
			}

			// small sleep to avoid hot-loop in edge cases
			pollInterval := c.cfg.PollIntervalMs
			if pollInterval <= 0 {
				pollInterval = 10
			}
			time.Sleep(time.Duration(pollInterval) * time.Millisecond)
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

	// Cancel fetch loop first
	c.cancel()

	// Wait for fetch loop to exit or context cancellation
	if c.done != nil {
		select {
		case <-c.done:
			// continue to close client
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Close client exactly once after fetch loop finished
	c.closeOnce.Do(func() {
		if c.client != nil {
			c.client.Close()
		}
	})
	return nil
}

// Errors returns a channel that receives handler errors. Callers should
// select on this channel to detect and handle processing failures.
func (c *Consumer) Errors() <-chan error {
	return c.errCh
}
