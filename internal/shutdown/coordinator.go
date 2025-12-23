package shutdown

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// Task represents a component that can be stopped gracefully.
type Task interface {
	Shutdown(context.Context) error
}

// ShutdownFunc wraps a function so it satisfies Task.
type ShutdownFunc func(context.Context) error

// Shutdown executes the wrapped function.
func (f ShutdownFunc) Shutdown(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

// Coordinator orchestrates the shutdown of registered components.
type Coordinator struct {
	mu     sync.Mutex
	logger *zap.Logger
	tasks  []namedTask
	once   sync.Once
}

type namedTask struct {
	name string
	task Task
}

// NewCoordinator creates a coordinator that logs lifecycle events.
func NewCoordinator(logger *zap.Logger) *Coordinator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Coordinator{logger: logger}
}

// Register keeps track of a component that should be shut down.
func (c *Coordinator) Register(name string, task Task) {
	if task == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tasks = append(c.tasks, namedTask{name: name, task: task})
}

// Shutdown executes registered components in reverse order.
func (c *Coordinator) Shutdown(ctx context.Context) {
	c.once.Do(func() {
		c.logger.Info("commencing graceful shutdown", zap.Int("components", len(c.tasks)))
		c.mu.Lock()
		tasks := append([]namedTask(nil), c.tasks...)
		c.mu.Unlock()

		for i := len(tasks) - 1; i >= 0; i-- {
			entry := tasks[i]
			if err := entry.task.Shutdown(ctx); err != nil {
				c.logger.Warn("component shutdown error", zap.String("component", entry.name), zap.Error(err))
				continue
			}
			c.logger.Debug("component shutdown complete", zap.String("component", entry.name))
		}
	})
}
