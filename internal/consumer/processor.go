package consumer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/zlovtnik/ggt/pkg/event"
	"go.uber.org/zap"
)

// Processor runs the message processing loop and applies backpressure
// via a semaphore-limited worker pool.
type Processor struct {
	workerSem chan struct{}
	wg        sync.WaitGroup
	handler   func(context.Context, event.Event) error
	logger    *zap.Logger
	shutdown  uint32
}

// NewProcessor creates a processor with a given concurrency limit.
func NewProcessor(concurrency int, handler func(context.Context, event.Event) error, logger *zap.Logger) *Processor {
	if handler == nil {
		panic("handler cannot be nil")
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Processor{workerSem: make(chan struct{}, concurrency), handler: handler, logger: logger.With(zap.String("component", "processor"))}
}

// Enqueue schedules an event for processing. This method blocks when the
// processor is at capacity. Handler errors are logged but not returned.
func (p *Processor) Enqueue(ctx context.Context, evt event.Event) error {
	if p == nil {
		return fmt.Errorf("processor is nil")
	}
	if atomic.LoadUint32(&p.shutdown) == 1 {
		return fmt.Errorf("processor is shutting down")
	}
	select {
	case p.workerSem <- struct{}{}:
		// acquired slot
	case <-ctx.Done():
		return ctx.Err()
	}

	p.wg.Add(1)
	go func() {
		defer func() { <-p.workerSem; p.wg.Done() }()
		if err := p.handler(ctx, evt); err != nil {
			p.logger.Error("handler error", zap.Error(err))
		}
	}()
	return nil
}

// Stop waits for in-flight work to complete or until ctx is done.
func (p *Processor) Stop(ctx context.Context) error {
	if p == nil {
		return nil
	}
	// prevent new enqueues
	atomic.StoreUint32(&p.shutdown, 1)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
