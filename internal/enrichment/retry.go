package enrichment

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryConfig specifies retry behavior for enrichment operations.
type RetryConfig struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns sensible defaults.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        2 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Do executes fn with retries and exponential backoff. fn should return an error
// which indicates whether the attempt failed. Do will not retry on context
// cancellation/deadline exceeded and will respect ctx during backoff sleeps.
func Do(ctx context.Context, cfg *RetryConfig, fn func() error) error {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1
	}

	backoff := cfg.InitialBackoff
	var lastErr error
	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			// Do not retry if context was cancelled or deadline exceeded.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			lastErr = err
		}

		if attempt+1 < cfg.MaxRetries {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * cfg.BackoffMultiplier)
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
		}
	}
	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxRetries, lastErr)
}

// DoWithPredicate is like Do but accepts a shouldRetry predicate. When fn
// returns an error, shouldRetry(err) is consulted; if it returns false the
// error is returned immediately without further retries.
func DoWithPredicate(ctx context.Context, cfg *RetryConfig, fn func() error, shouldRetry func(error) bool) error {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 1
	}

	backoff := cfg.InitialBackoff
	var lastErr error
	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		if err := fn(); err == nil {
			return nil
		} else {
			// Do not retry if context was cancelled or deadline exceeded.
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			// If caller provided a predicate and it says not to retry, return immediately.
			if shouldRetry != nil {
				if !shouldRetry(err) {
					return err
				}
			}
			lastErr = err
		}

		if attempt+1 < cfg.MaxRetries {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return ctx.Err()
			}
			backoff = time.Duration(float64(backoff) * cfg.BackoffMultiplier)
			if backoff > cfg.MaxBackoff {
				backoff = cfg.MaxBackoff
			}
		}
	}
	return fmt.Errorf("operation failed after %d attempts: %w", cfg.MaxRetries, lastErr)
}
