package consumer

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// OffsetManager provides a small interface for committing offsets. This
// is intentionally simple for the initial scaffold; later it will be
// implemented to coordinate commits with franz-go and durable storage.
type OffsetManager struct {
	mu      sync.Mutex
	commits map[string]int64 // map[topic-partition key] -> offset
	logger  *zap.Logger
}

// NewOffsetManager creates a new offset manager instance.
func NewOffsetManager(logger *zap.Logger) *OffsetManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &OffsetManager{commits: make(map[string]int64), logger: logger.With(zap.String("component", "offset_manager"))}
}

// Commit records an offset for a given topic-partition key. This is a
// local in-memory commit; persistence/interaction with the broker will
// be added when wiring with the consumer.
func (o *OffsetManager) Commit(ctx context.Context, key string, offset int64) error {
	if o == nil {
		return fmt.Errorf("offset manager is nil")
	}
	// ctx may be used in future for persistence/backoff/cancellation
	_ = ctx
	o.mu.Lock()
	defer o.mu.Unlock()
	o.commits[key] = offset
	o.logger.Debug("offset committed", zap.String("key", key), zap.Int64("offset", offset))
	return nil
}

// Get returns the last committed offset for a key or false when unknown.
func (o *OffsetManager) Get(key string) (int64, bool) {
	if o == nil {
		return 0, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	v, ok := o.commits[key]
	return v, ok
}
