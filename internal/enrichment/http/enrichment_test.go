package http

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestHTTPEnrichment_InterpolateURL(t *testing.T) {
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
		result, err := interpolateURL(tt.template, ev)
		if tt.wantError {
			assert.Error(t, err, "expected error for template: %s", tt.template)
			assert.Contains(t, err.Error(), "url interpolation failed")
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
		URL:         "https://api.example.com/invalid",
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
	assert.NoError(t, err)
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
