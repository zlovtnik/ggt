package http

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	transformPkg "github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestHTTPEnrichment_Configure(t *testing.T) {
	cfg := &enrichConfig{
		URL:         "https://api.example.com/users/{{.user_id}}",
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/users/{{.user_id}}", transform.config.URL)
	assert.Equal(t, "user_data", transform.config.TargetField)
	assert.Equal(t, "GET", transform.config.Method)
}

func TestHTTPEnrichment_Configure_MissingURL(t *testing.T) {
	cfg := &enrichConfig{
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestHTTPEnrichment_Configure_MissingTargetField(t *testing.T) {
	cfg := &enrichConfig{
		URL: "https://api.example.com/users",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestHTTPEnrichment_InterpolateTemplate(t *testing.T) {
	tests := []struct {
		template  string
		payload   map[string]interface{}
		expected  string
		wantError bool
	}{
		{
			"https://api.example.com/users/{{.user_id}}",
			map[string]interface{}{"user_id": "123"},
			"https://api.example.com/users/123",
			false,
		},
		{
			"https://api.example.com/users/{{.user_id}}/posts/{{.post_id}}",
			map[string]interface{}{"user_id": "123", "post_id": "456"},
			"https://api.example.com/users/123/posts/456",
			false,
		},
		{
			"https://api.example.com/users/{{.missing}}",
			map[string]interface{}{"user_id": "123"},
			"",
			true,
		},
		{
			"https://api.example.com/users/{{.user_id}}/posts/{{.missing_post}}",
			map[string]interface{}{"user_id": "123"},
			"",
			true,
		},
	}

	for _, tt := range tests {
		ev := event.Event{Payload: tt.payload}
		result, err := interpolateTemplate(tt.template, ev)
		if tt.wantError {
			assert.Error(t, err, "expected error for template: %s", tt.template)
			assert.Contains(t, err.Error(), "template interpolation failed")
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		}
	}
}

func TestHTTPEnrichment_Name(t *testing.T) {
	transform := &enrichTransform{}
	assert.Equal(t, "enrich.http", transform.Name())
}

func TestHTTPEnrichment_Execute_NotConfigured(t *testing.T) {
	transform := &enrichTransform{}

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHTTPEnrichment_Execute_InvalidPayload(t *testing.T) {
	cfg := &enrichConfig{
		URL:         "https://api.example.com/users",
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{client: NewClient(nil)}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	result, err := transform.Execute(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestHTTPEnrichment_Execute_DropOnError(t *testing.T) {
	cfg := &enrichConfig{
		URL:         "http://localhost:0/invalid",
		TargetField: "user_data",
		DropOnError: true,
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{client: NewClient(nil)}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.ErrorIs(t, err, transformPkg.ErrDrop)
	assert.Nil(t, result)
}

func TestHTTPEnrichment_DefaultMethod(t *testing.T) {
	cfg := &enrichConfig{
		URL:         "https://api.example.com/users",
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)
	assert.Equal(t, "GET", transform.config.Method)
}

func TestClient_PostWithOptions_IdempotentOverride(t *testing.T) {
	// Test that IdempotentVerbOverride allows retries for POST without idempotency key
	client := NewClient(&ClientConfig{
		DefaultTimeout:   10 * time.Second,
		MaxIdleConns:     10,
		MaxConnsPerHost:  10,
		CacheTTL:         5 * time.Minute,
		CacheSize:        100,
		MaxRetries:       3,
		CircuitOpenTime:  30 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		MaxResponseSize:  10 * 1024 * 1024,
	})

	// This should not return an error due to missing idempotency key
	// because IdempotentVerbOverride is true
	opts := &PostOptions{
		AllowRetries:           true,
		IdempotentVerbOverride: true,
	}

	// We expect this to fail at the network level (no server running)
	// but not at the validation level
	_, err := client.PostWithOptions(context.Background(), "http://nonexistent.example.com/test", "application/json", []byte("{}"), opts)
	assert.Error(t, err)
	// The error should be network-related, not validation-related
	assert.NotContains(t, err.Error(), "retries requested for POST without idempotency key")
}

func TestClient_PostWithOptions_RequiresIdempotencyKey(t *testing.T) {
	// Test that retries for POST require either an idempotency key or IdempotentVerbOverride
	client := NewClient(&ClientConfig{
		DefaultTimeout:   10 * time.Second,
		MaxIdleConns:     10,
		MaxConnsPerHost:  10,
		CacheTTL:         5 * time.Minute,
		CacheSize:        100,
		MaxRetries:       3,
		CircuitOpenTime:  30 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		MaxResponseSize:  10 * 1024 * 1024,
	})

	// This should fail validation because AllowRetries is true but neither
	// IdempotencyKey nor IdempotentVerbOverride is set
	opts := &PostOptions{
		AllowRetries: true,
	}

	_, err := client.PostWithOptions(context.Background(), "http://example.com/test", "application/json", []byte("{}"), opts)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "retries requested for POST without idempotency key")
}
