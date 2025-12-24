package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zlovtnik/ggt/internal/enrichment"
)

// Client provides HTTP requests with caching, retries, and circuit breaking.
type Client struct {
	httpClient      *http.Client
	cache           *enrichment.LRUCache
	retrier         *Retrier
	circuitBreaker  *CircuitBreaker
	maxResponseSize int64
}

// ClientConfig holds HTTP client configuration.
type ClientConfig struct {
	DefaultTimeout   time.Duration
	MaxIdleConns     int
	MaxConnsPerHost  int
	CacheTTL         time.Duration
	CacheSize        int
	MaxRetries       int
	CircuitOpenTime  time.Duration
	FailureThreshold int32
	SuccessThreshold int32
	MaxResponseSize  int64
}

// PostOptions controls POST request behavior.
type PostOptions struct {
	AllowRetries           bool   // Whether to allow retries (default: false for non-idempotent POSTs)
	IdempotencyKey         string // Optional idempotency key for safe retries
	IdempotentVerbOverride bool   // Override POST non-idempotent behavior for naturally idempotent operations
}

// DefaultClientConfig returns sensible HTTP client defaults.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		DefaultTimeout:   10 * time.Second,
		MaxIdleConns:     100,
		MaxConnsPerHost:  10,
		CacheTTL:         5 * time.Minute,
		CacheSize:        1000,
		MaxRetries:       3,
		CircuitOpenTime:  30 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		MaxResponseSize:  10 * 1024 * 1024, // 10MB
	}
}

// NewClient creates a new HTTP client with caching, retries, and circuit breaking.
func NewClient(cfg *ClientConfig) *Client {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}

	transport := &http.Transport{
		MaxIdleConns:       cfg.MaxIdleConns,
		MaxConnsPerHost:    cfg.MaxConnsPerHost,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
		DisableKeepAlives:  false,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.DefaultTimeout,
	}

	cache := enrichment.NewLRUCache(cfg.CacheSize, cfg.CacheTTL)

	retryCfg := &RetryConfig{
		MaxRetries:        cfg.MaxRetries,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        10 * time.Second,
		BackoffMultiplier: 2.0,
	}

	return &Client{
		httpClient:      httpClient,
		cache:           cache,
		retrier:         NewRetrier(retryCfg),
		circuitBreaker:  NewCircuitBreaker(cfg.CircuitOpenTime, cfg.FailureThreshold, cfg.SuccessThreshold),
		maxResponseSize: cfg.MaxResponseSize,
	}
}

// Get performs an HTTP GET request with caching, retries, and circuit breaking.
func (c *Client) Get(ctx context.Context, url string) (string, error) {
	if cached := c.cache.Get(url); cached != nil {
		if s, ok := cached.(string); ok {
			return s, nil
		}
		// Invalid cache entry, remove it
		c.cache.Delete(url)
	}

	if !c.circuitBreaker.Allow() {
		return "", fmt.Errorf("http: circuit breaker is open")
	}

	resp, err := c.retrier.Do(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		return c.httpClient.Do(req)
	})

	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: get failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: get returned status %d", resp.StatusCode)
	}

	c.circuitBreaker.RecordSuccess()

	limitedReader := io.LimitReader(resp.Body, c.maxResponseSize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("http: read body failed: %w", err)
	}
	if int64(len(body)) > c.maxResponseSize {
		return "", fmt.Errorf("http: response body exceeds maximum size of %d bytes", c.maxResponseSize)
	}

	bodyStr := string(body)

	if int64(len(body)) > c.maxResponseSize {
		return "", fmt.Errorf("http: response body too large (size %d exceeds max %d)", len(body), c.maxResponseSize)
	}

	c.cache.Set(url, bodyStr)

	return bodyStr, nil
}

// Post performs an HTTP POST request with optional retries and circuit breaking.
// By default, retries are disabled to prevent duplicate side effects on non-idempotent requests.
// Use PostWithOptions to enable retries or provide an idempotency key for safe retry behavior.
// For naturally idempotent POST operations, use PostWithOptions with IdempotentVerbOverride=true.
func (c *Client) Post(ctx context.Context, url string, contentType string, body []byte) (string, error) {
	return c.PostWithOptions(ctx, url, contentType, body, &PostOptions{AllowRetries: false})
}

// PostWithOptions performs an HTTP POST request with configurable retry behavior.
// If AllowRetries is true, the retrier will be used. If IdempotencyKey is provided,
// it will be sent with the request to enable safe retries on the server side.
// For naturally idempotent POST operations, set IdempotentVerbOverride to true
// to allow retries without requiring an idempotency key.
func (c *Client) PostWithOptions(ctx context.Context, url string, contentType string, body []byte, opts *PostOptions) (string, error) {
	if opts == nil {
		opts = &PostOptions{AllowRetries: false}
	}

	if opts.AllowRetries && opts.IdempotencyKey == "" && !opts.IdempotentVerbOverride {
		return "", fmt.Errorf("http: retries requested for POST without idempotency key")
	}

	if !c.circuitBreaker.Allow() {
		return "", fmt.Errorf("http: circuit breaker is open")
	}

	// If retries are allowed, use the retrier; otherwise perform a single request
	if opts.AllowRetries {
		resp, err := c.retrier.Do(ctx, func() (*http.Response, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", contentType)
			if opts.IdempotencyKey != "" {
				req.Header.Set("Idempotency-Key", opts.IdempotencyKey)
			}
			return c.httpClient.Do(req)
		})

		if err != nil {
			c.circuitBreaker.RecordFailure()
			return "", fmt.Errorf("http: post failed: %w", err)
		}

		return c.handlePostResponse(resp)
	}

	// Perform single request without retries but still honor circuit breaker
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	if opts.IdempotencyKey != "" {
		req.Header.Set("Idempotency-Key", opts.IdempotencyKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: post failed: %w", err)
	}

	return c.handlePostResponse(resp)
}

// handlePostResponse handles the response from a POST request.
func (c *Client) handlePostResponse(resp *http.Response) (string, error) {
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: post returned status %d", resp.StatusCode)
	}

	c.circuitBreaker.RecordSuccess()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("http: read body failed: %w", err)
	}

	if int64(len(respBody)) > c.maxResponseSize {
		return "", fmt.Errorf("http: response body too large (size %d exceeds max %d)", len(respBody), c.maxResponseSize)
	}

	return string(respBody), nil
}

// GetJSON performs a GET request and returns the response body as a string.
func (c *Client) GetJSON(ctx context.Context, url string) (string, error) {
	if cached := c.cache.Get(url); cached != nil {
		if s, ok := cached.(string); ok {
			return s, nil
		}
		// Invalid cache entry, remove it
		c.cache.Delete(url)
	}

	if !c.circuitBreaker.Allow() {
		return "", fmt.Errorf("http: circuit breaker is open")
	}

	resp, err := c.retrier.Do(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		return c.httpClient.Do(req)
	})

	if err != nil {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: get failed: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.circuitBreaker.RecordFailure()
		return "", fmt.Errorf("http: get returned status %d", resp.StatusCode)
	}

	c.circuitBreaker.RecordSuccess()

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.maxResponseSize+1))
	if err != nil {
		return "", fmt.Errorf("http: read body failed: %w", err)
	}

	if int64(len(body)) > c.maxResponseSize {
		return "", fmt.Errorf("http: response body too large (size %d exceeds max %d)", len(body), c.maxResponseSize)
	}

	bodyStr := string(body)
	c.cache.Set(url, bodyStr)

	return bodyStr, nil
}

// Close closes the HTTP client.
func (c *Client) Close() {
	c.cache.Clear()
	c.httpClient.CloseIdleConnections()
}

// Stats returns client statistics including cache and circuit breaker state.
func (c *Client) Stats() map[string]interface{} {
	return map[string]interface{}{
		"cache_size":            c.cache.Size(),
		"circuit_breaker_state": c.circuitBreaker.State(),
	}
}

// CircuitBreakerState returns the current circuit breaker state.
func (c *Client) CircuitBreakerState() CircuitBreakerState {
	return c.circuitBreaker.State()
}
