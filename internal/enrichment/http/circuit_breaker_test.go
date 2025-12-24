package http

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_AllowClosed(t *testing.T) {
	cb := NewCircuitBreaker(30*time.Second, 5, 2)
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_TransitionToOpen(t *testing.T) {
	cb := NewCircuitBreaker(30*time.Second, 1, 2)

	assert.True(t, cb.Allow())
	cb.RecordFailure()

	assert.False(t, cb.Allow())
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_TransitionToHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(100*time.Millisecond, 1, 2)

	assert.True(t, cb.Allow())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(200 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_SuccessClosesCircuit(t *testing.T) {
	cb := NewCircuitBreaker(100*time.Millisecond, 1, 1)

	assert.True(t, cb.Allow())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(200 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	cb := NewCircuitBreaker(200*time.Millisecond, 1, 2)
	var wg sync.WaitGroup

	// Start with closed state
	assert.Equal(t, StateClosed, cb.State())

	// Induce failure to open
	assert.True(t, cb.Allow())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	// Wait for reset timeout
	time.Sleep(300 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())

	// Launch goroutines to record successes concurrently
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.RecordSuccess()
		}()
	}

	wg.Wait()
	// Should be closed after required successes
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_MultipleConsecutiveSuccesses(t *testing.T) {
	cb := NewCircuitBreaker(100*time.Millisecond, 1, 3)

	// Transition to HalfOpen
	assert.True(t, cb.Allow())
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	time.Sleep(200 * time.Millisecond)
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())

	// Record successes sequentially
	cb.RecordSuccess()
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
}

func TestRetrier_Success(t *testing.T) {
	retrier := NewRetrier(&RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	})

	callCount := 0
	resp, err := retrier.Do(context.Background(), func() (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 200}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 1, callCount)
}

func TestRetrier_Retry(t *testing.T) {
	retrier := NewRetrier(&RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	})

	callCount := 0
	resp, err := retrier.Do(context.Background(), func() (*http.Response, error) {
		callCount++
		if callCount < 3 {
			return &http.Response{StatusCode: 503}, nil
		}
		return &http.Response{StatusCode: 200}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, callCount)
}

func TestRetrier_FailAfterMaxRetries(t *testing.T) {
	retrier := NewRetrier(&RetryConfig{
		MaxRetries:        2,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	})

	callCount := 0
	resp, err := retrier.Do(context.Background(), func() (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 503}, nil
	})

	assert.Error(t, err)
	assert.Equal(t, 2, callCount)
	assert.Equal(t, 503, resp.StatusCode)
}

func TestRetrier_ErrorRetry(t *testing.T) {
	retrier := NewRetrier(&RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	})

	callCount := 0
	resp, err := retrier.Do(context.Background(), func() (*http.Response, error) {
		callCount++
		if callCount < 3 {
			return nil, errors.New("network error")
		}
		return &http.Response{StatusCode: 200}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 3, callCount)
}

func TestRetrier_NonRetryableError(t *testing.T) {
	retrier := NewRetrier(&RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    10 * time.Millisecond,
		MaxBackoff:        50 * time.Millisecond,
		BackoffMultiplier: 2.0,
	})

	callCount := 0
	resp, err := retrier.Do(context.Background(), func() (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: 400}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
	assert.Equal(t, 1, callCount)
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		err        error
		statusCode int
		expected   bool
	}{
		{errors.New("network error"), 200, true},
		{nil, 500, true},
		{nil, 503, true},
		{nil, 429, true},
		{nil, 408, true},
		{nil, 400, false},
		{nil, 404, false},
		{nil, 200, false},
	}

	for _, tt := range tests {
		result := isRetryable(tt.err, tt.statusCode)
		assert.Equal(t, tt.expected, result, "err=%v, statusCode=%d", tt.err, tt.statusCode)
	}
}
