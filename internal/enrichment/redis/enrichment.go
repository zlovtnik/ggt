package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// enrichConfig specifies how to enrich an event with Redis data.
type enrichConfig struct {
	KeyField    string `json:"key_field"`
	TargetField string `json:"target_field"`
	RedisConfig string `json:"redis_config"`
	HashField   string `json:"hash_field,omitempty"`
	CacheTTL    string `json:"cache_ttl,omitempty"`
	DropOnError bool   `json:"drop_on_error,omitempty"`
}

// enrichTransform enriches events with data from Redis.
type enrichTransform struct {
	cfg      enrichConfig
	client   *Client
	cacheTTL time.Duration
}

func (e *enrichTransform) Name() string { return "enrich.redis" }

func (e *enrichTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("enrich.redis: config required")
	}

	if err := json.Unmarshal(raw, &e.cfg); err != nil {
		return fmt.Errorf("enrich.redis: config parse failed: %w", err)
	}

	if e.cfg.KeyField == "" {
		return fmt.Errorf("enrich.redis: key_field required")
	}

	if e.cfg.TargetField == "" {
		return fmt.Errorf("enrich.redis: target_field required")
	}

	if e.cfg.RedisConfig == "" {
		e.cfg.RedisConfig = "default"
	}

	if e.cfg.CacheTTL != "" {
		ttl, err := time.ParseDuration(e.cfg.CacheTTL)
		if err != nil {
			return fmt.Errorf("enrich.redis: invalid cache_ttl: %w", err)
		}
		e.cacheTTL = ttl
	} else {
		e.cacheTTL = 5 * time.Minute
	}

	return nil
}

func (e *enrichTransform) Execute(ctx context.Context, payload interface{}) (interface{}, error) {
	if e.cfg.KeyField == "" {
		return nil, fmt.Errorf("enrich.redis: not configured")
	}

	ev, ok := payload.(event.Event)
	if !ok {
		return nil, fmt.Errorf("enrich.redis: unexpected payload type")
	}

	keyValue, exists := ev.GetField(e.cfg.KeyField)
	if !exists {
		if e.cfg.DropOnError {
			return nil, transform.ErrDrop
		}
		return ev, nil
	}

	key, ok := keyValue.(string)
	if !ok {
		key = fmt.Sprintf("%v", keyValue)
	}

	var enrichedData interface{}
	var err error

	if e.cfg.HashField != "" {
		enrichedData, err = e.client.HGetWithTTL(ctx, key, e.cfg.HashField, e.cacheTTL)
	} else {
		enrichedData, err = e.client.GetWithTTL(ctx, key, e.cacheTTL)
	}

	if err != nil {
		if e.cfg.DropOnError {
			return nil, fmt.Errorf("enrich.redis: enrichment failed: %w", err)
		}
		return ev, nil
	}

	if enrichedData == nil {
		return ev, nil
	}
	if str, ok := enrichedData.(string); ok && str == "" {
		return ev, nil
	}

	var parsed interface{}
	if str, ok := enrichedData.(string); ok {
		if str != "" && str[0] == '{' {
			if err := json.Unmarshal([]byte(str), &parsed); err == nil {
				enrichedData = parsed
			}
		}
	}

	return ev.SetField(e.cfg.TargetField, enrichedData), nil
}

// EnrichTransformFactory creates an enrich.redis transform with a configured client.
func EnrichTransformFactory(client *Client) func(json.RawMessage) (transform.Transform, error) {
	return func(raw json.RawMessage) (transform.Transform, error) {
		t := &enrichTransform{client: client}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	}
}

// RegisterRedisEnrichment registers the enrich.redis transform with the provided client.
func RegisterRedisEnrichment(cfg config.RedisEnrichment) error {
	ttl := 5 * time.Minute
	clientCfg := &ClientConfig{
		Addr:       cfg.Addr,
		Password:   cfg.Password,
		DB:         cfg.DB,
		MaxRetries: &cfg.MaxRetries,
		PoolSize:   &cfg.PoolSize,
		CacheTTL:   &ttl,
		CacheSize:  1000,
	}

	client, err := NewClient(clientCfg)
	if err != nil {
		return err
	}

	transform.Register("enrich.redis", EnrichTransformFactory(client))
	return nil
}
