package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestEncodeTransform_Name(t *testing.T) {
	transform := &EncodeTransform{}
	assert.Equal(t, "data.encode", transform.Name())
}

func TestEncodeTransform_Configure(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name:        "valid base64 encode",
			config:      `{"source_field": "data", "operation": "encode", "format": "base64"}`,
			expectError: false,
		},
		{
			name:        "valid url decode with target",
			config:      `{"source_field": "url", "target_field": "decoded_url", "operation": "decode", "format": "url"}`,
			expectError: false,
		},
		{
			name:        "missing source_field",
			config:      `{"operation": "encode", "format": "base64"}`,
			expectError: true,
		},
		{
			name:        "missing operation",
			config:      `{"source_field": "data", "format": "base64"}`,
			expectError: true,
		},
		{
			name:        "invalid operation",
			config:      `{"source_field": "data", "operation": "invalid", "format": "base64"}`,
			expectError: true,
		},
		{
			name:        "missing format",
			config:      `{"source_field": "data", "operation": "encode"}`,
			expectError: true,
		},
		{
			name:        "invalid format",
			config:      `{"source_field": "data", "operation": "encode", "format": "invalid"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &EncodeTransform{}
			err := transform.Configure([]byte(tt.config))
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEncodeTransform_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		input       map[string]interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "base64 encode",
			config:   `{"source_field": "text", "operation": "encode", "format": "base64"}`,
			input:    map[string]interface{}{"text": "hello world"},
			expected: map[string]interface{}{"text": "aGVsbG8gd29ybGQ="},
		},
		{
			name:     "base64 decode",
			config:   `{"source_field": "encoded", "operation": "decode", "format": "base64"}`,
			input:    map[string]interface{}{"encoded": "aGVsbG8gd29ybGQ="},
			expected: map[string]interface{}{"encoded": "hello world"},
		},
		{
			name:     "url encode",
			config:   `{"source_field": "url", "operation": "encode", "format": "url"}`,
			input:    map[string]interface{}{"url": "hello world"},
			expected: map[string]interface{}{"url": "hello+world"},
		},
		{
			name:     "url decode",
			config:   `{"source_field": "encoded_url", "operation": "decode", "format": "url"}`,
			input:    map[string]interface{}{"encoded_url": "hello%20world"},
			expected: map[string]interface{}{"encoded_url": "hello world"},
		},
		{
			name:     "hex encode",
			config:   `{"source_field": "data", "operation": "encode", "format": "hex"}`,
			input:    map[string]interface{}{"data": "hello"},
			expected: map[string]interface{}{"data": "68656c6c6f"},
		},
		{
			name:     "hex decode",
			config:   `{"source_field": "hex_data", "operation": "decode", "format": "hex"}`,
			input:    map[string]interface{}{"hex_data": "68656c6c6f"},
			expected: map[string]interface{}{"hex_data": "hello"},
		},
		{
			name:     "encode with target field",
			config:   `{"source_field": "text", "target_field": "encoded_text", "operation": "encode", "format": "base64"}`,
			input:    map[string]interface{}{"text": "test"},
			expected: map[string]interface{}{"text": "test", "encoded_text": "dGVzdA=="},
		},
		{
			name:        "invalid base64 decode",
			config:      `{"source_field": "invalid", "operation": "decode", "format": "base64"}`,
			input:       map[string]interface{}{"invalid": "invalid-base64!"},
			expectError: true,
		},
		{
			name:        "invalid hex decode",
			config:      `{"source_field": "invalid", "operation": "decode", "format": "hex"}`,
			input:       map[string]interface{}{"invalid": "invalid-hex"},
			expectError: true,
		},
		{
			name:        "non-string field",
			config:      `{"source_field": "number", "operation": "encode", "format": "base64"}`,
			input:       map[string]interface{}{"number": 123},
			expectError: true,
		},
		{
			name:        "missing field",
			config:      `{"source_field": "missing", "operation": "encode", "format": "base64"}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &EncodeTransform{}
			err := transform.Configure([]byte(tt.config))
			require.NoError(t, err)

			evt := event.Event{Payload: tt.input}
			result, err := transform.Execute(context.Background(), evt)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				resultEvt, ok := result.(event.Event)
				require.True(t, ok)
				for key, expectedVal := range tt.expected {
					actualVal, exists := resultEvt.GetField(key)
					assert.True(t, exists)
					assert.Equal(t, expectedVal, actualVal)
				}
			}
		})
	}
}
