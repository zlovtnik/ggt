package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zlovtnik/ggt/internal/enrichment"
	"go.uber.org/zap"
)

// Client wraps a Redis client with connection pooling and caching.
type Client struct {
	client   *redis.Client
	cache    *enrichment.LRUCache
	config   *ClientConfig
	cacheTTL time.Duration
}

// ClientConfig holds Redis client configuration.
type ClientConfig struct {
	Addr       string
	Password   string
	DB         int
	MaxRetries *int
	PoolSize   *int
	CacheTTL   *time.Duration
	CacheSize  int
}

// NewClient creates a new Redis client with connection pooling and LRU cache.
func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg == nil {
		cfg = &ClientConfig{
			Addr:       "localhost:6379",
			DB:         0,
			MaxRetries: func() *int { v := 3; return &v }(),
			PoolSize:   func() *int { v := 10; return &v }(),
			CacheTTL:   func() *time.Duration { v := 5 * time.Minute; return &v }(),
			CacheSize:  1000,
		}
	} else {
		// Apply defaults for nil values
		if cfg.Addr == "" {
			cfg.Addr = "localhost:6379"
		}
		if cfg.MaxRetries == nil {
			cfg.MaxRetries = func() *int { v := 3; return &v }()
		}
		if cfg.PoolSize == nil {
			cfg.PoolSize = func() *int { v := 10; return &v }()
		}
		if cfg.CacheTTL == nil {
			cfg.CacheTTL = func() *time.Duration { v := 5 * time.Minute; return &v }()
		}
		if cfg.CacheSize == 0 {
			cfg.CacheSize = 1000
		}
	}

	// Create Redis client with pooling
	opts := &redis.Options{
		Addr:       cfg.Addr,
		Password:   cfg.Password,
		DB:         cfg.DB,
		MaxRetries: *cfg.MaxRetries,
		PoolSize:   *cfg.PoolSize,
	}

	client := redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis: connection test failed: %w", err)
	}

	zap.L().With(zap.String("component", "redis_client")).Info("redis client ready",
		zap.String("addr", opts.Addr),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", *cfg.PoolSize),
		zap.Duration("cache_ttl", *cfg.CacheTTL),
	)

	// Create cache
	cache := enrichment.NewLRUCache(cfg.CacheSize, *cfg.CacheTTL)

	return &Client{
		client:   client,
		cache:    cache,
		config:   cfg,
		cacheTTL: *cfg.CacheTTL,
	}, nil
}

// Get retrieves a value from Redis, with local caching using client's configured TTL.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	return c.GetWithTTL(ctx, key, c.cacheTTL)
}

// GetWithTTL retrieves a value from Redis, with local caching using a custom TTL.
func (c *Client) GetWithTTL(ctx context.Context, key string, ttl time.Duration) (string, error) {
	// Check local cache first
	if cached := c.cache.Get(key); cached != nil {
		if str, ok := cached.(string); ok {
			return str, nil
		}
		// Cache entry has wrong type, invalidate it
		c.cache.Delete(key)
	}

	// Query Redis
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("redis: get failed: %w", err)
	}

	// Cache the result with custom TTL
	c.cache.SetWithTTL(key, val, ttl)
	return val, nil
}

// GetJSON retrieves a JSON value from Redis and unmarshals it.
func (c *Client) GetJSON(ctx context.Context, key string, dest interface{}) error {
	// Check local cache first
	if cached := c.cache.Get(key); cached != nil {
		if str, ok := cached.(string); ok {
			return json.Unmarshal([]byte(str), dest)
		}
		// Cache entry has wrong type, invalidate it
		c.cache.Delete(key)
	}

	// Query Redis
	val, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}
		return fmt.Errorf("redis: getjson failed: %w", err)
	}

	// Unmarshal JSON
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return fmt.Errorf("redis: json unmarshal failed: %w", err)
	}

	// Cache the raw string value
	c.cache.Set(key, val)
	return nil
}

// Set stores a string value in Redis.
func (c *Client) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	if err := c.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("redis: set failed: %w", err)
	}
	c.cache.Delete(key)
	return nil
}

// SetJSON marshals and stores a JSON value in Redis.
func (c *Client) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis: json marshal failed: %w", err)
	}

	if err := c.client.Set(ctx, key, data, expiration).Err(); err != nil {
		return fmt.Errorf("redis: setjson failed: %w", err)
	}

	c.cache.Delete(key)
	return nil
}

// Delete removes a key from Redis.
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	if err := c.client.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("redis: delete failed: %w", err)
	}

	for _, key := range keys {
		c.cache.Delete(key)
	}
	return nil
}

// Exists checks if a key exists in Redis.
func (c *Client) Exists(ctx context.Context, keys ...string) (int64, error) {
	n, err := c.client.Exists(ctx, keys...).Result()
	if err != nil {
		return 0, fmt.Errorf("redis: exists failed: %w", err)
	}
	return n, nil
}

// HGet retrieves a field value from a Redis hash, with local caching using client's configured TTL.
func (c *Client) HGet(ctx context.Context, key, field string) (string, error) {
	return c.HGetWithTTL(ctx, key, field, c.cacheTTL)
}

// HGetWithTTL retrieves a field value from a Redis hash, with local caching using a custom TTL.
func (c *Client) HGetWithTTL(ctx context.Context, key, field string, ttl time.Duration) (string, error) {
	cacheKey := key + ":" + field
	if cached := c.cache.Get(cacheKey); cached != nil {
		if str, ok := cached.(string); ok {
			return str, nil
		}
		// Cache entry has wrong type, invalidate it
		c.cache.Delete(cacheKey)
	}

	val, err := c.client.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", fmt.Errorf("redis: hget failed: %w", err)
	}

	c.cache.SetWithTTL(cacheKey, val, ttl)
	return val, nil
}

// HSet sets a field value in a Redis hash.
func (c *Client) HSet(ctx context.Context, key, field, value string) error {
	if err := c.client.HSet(ctx, key, field, value).Err(); err != nil {
		return fmt.Errorf("redis: hset failed: %w", err)
	}
	// Invalidate cache for this field
	cacheKey := key + ":" + field
	c.cache.Delete(cacheKey)
	return nil
}

// HGetAll retrieves all fields and values from a Redis hash.
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	vals, err := c.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("redis: hgetall failed: %w", err)
	}
	return vals, nil
}

// Close closes the Redis connection.
func (c *Client) Close() error {
	c.cache.Clear()
	return c.client.Close()
}

// CacheStats returns cache statistics (size).
func (c *Client) CacheStats() map[string]int {
	return map[string]int{
		"size": c.cache.Size(),
	}
}
