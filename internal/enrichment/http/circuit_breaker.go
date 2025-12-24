package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker.
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateOpen
	StateHalfOpen
)

// CircuitBreaker implements a simple circuit breaker pattern.
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            CircuitBreakerState
	failureCount     int32
	successCount     int32
	lastFailureTime  time.Time
	openTimeout      time.Duration
	failureThreshold int32
	successThreshold int32
}

// NewCircuitBreaker creates a new circuit breaker with specified thresholds.
func NewCircuitBreaker(openTimeout time.Duration, failureThreshold, successThreshold int32) *CircuitBreaker {
	if openTimeout <= 0 {
		openTimeout = 30 * time.Second
	}
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if successThreshold <= 0 {
		successThreshold = 2
	}

	return &CircuitBreaker{
		state:            StateClosed,
		openTimeout:      openTimeout,
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
	}
}

// Allow checks if a request is allowed based on circuit state.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	state := cb.state
	lastFailureTime := cb.lastFailureTime
	cb.mu.RUnlock()

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(lastFailureTime) > cb.openTimeout {
			cb.mu.Lock()
			defer cb.mu.Unlock()
			if time.Since(cb.lastFailureTime) > cb.openTimeout {
				cb.state = StateHalfOpen
				cb.successCount = 0
				return true
			}
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0

	if cb.state == StateHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.successThreshold {
			cb.state = StateClosed
			cb.successCount = 0
		}
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	cb.failureCount++
	failures := cb.failureCount
	if failures >= cb.failureThreshold {
		cb.state = StateOpen
	}
}

// State returns the current circuit breaker state.
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// RetryConfig specifies retry behavior.
type RetryConfig struct {
	MaxRetries        int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	}
}

// Retrier handles retry logic with exponential backoff.
type Retrier struct {
	config *RetryConfig
}

// NewRetrier creates a new retrier with the given config.
func NewRetrier(cfg *RetryConfig) *Retrier {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}
	return &Retrier{config: cfg}
}

// isRetryable determines if an error should be retried.
func isRetryable(err error, statusCode int) bool {
	if err != nil {
		// Don't retry context cancellation or deadline exceeded
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		return true
	}
	return statusCode >= 500 || statusCode == 408 || statusCode == 429
}

// Do executes a request with retries and exponential backoff.
func (r *Retrier) Do(fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response
	backoff := r.config.InitialBackoff

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		resp, err := fn()

		if err == nil && resp != nil && resp.StatusCode < 500 && resp.StatusCode != 408 && resp.StatusCode != 429 {
			return resp, nil
		}

		// Before overwriting lastResp, drain and close the old response body to prevent leaks
		if lastResp != nil && lastResp.Body != nil {
			_, _ = io.CopyN(io.Discard, lastResp.Body, 512)
			_ = lastResp.Body.Close()
		}

		lastResp = resp
		lastErr = err

		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}

		if !isRetryable(lastErr, statusCode) {
			return resp, err
		}

		// If we're retrying, drain and close the current response body
		if resp != nil && resp.Body != nil {
			_, _ = io.CopyN(io.Discard, resp.Body, 512)
			_ = resp.Body.Close()
		}

		if attempt < r.config.MaxRetries {
			time.Sleep(backoff)
			backoff = time.Duration(float64(backoff) * r.config.BackoffMultiplier)
			if backoff > r.config.MaxBackoff {
				backoff = r.config.MaxBackoff
			}
		}
	}

	return lastResp, fmt.Errorf("http: max retries exceeded: %w", lastErr)
}
