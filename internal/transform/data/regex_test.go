package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestRegexTransform_Name(t *testing.T) {
	transform := &RegexTransform{}
	assert.Equal(t, "data.regex", transform.Name())
}

func TestRegexTransform_Configure(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name:        "valid config",
			config:      `{"source_field": "email", "pattern": "\\d+", "replacement": "X"}`,
			expectError: false,
		},
		{
			name:        "valid config with target field",
			config:      `{"source_field": "email", "target_field": "clean_email", "pattern": "@.*", "replacement": "@domain.com"}`,
			expectError: false,
		},
		{
			name:        "missing source_field",
			config:      `{"pattern": "\\d+", "replacement": "X"}`,
			expectError: true,
		},
		{
			name:        "missing pattern",
			config:      `{"source_field": "email", "replacement": "X"}`,
			expectError: true,
		},
		{
			name:        "invalid regex pattern",
			config:      `{"source_field": "email", "pattern": "[invalid", "replacement": "X"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &RegexTransform{}
			err := transform.Configure([]byte(tt.config))
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRegexTransform_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		input       map[string]interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "replace digits",
			config:   `{"source_field": "phone", "pattern": "\\d", "replacement": "*"}`,
			input:    map[string]interface{}{"phone": "123-456-7890"},
			expected: map[string]interface{}{"phone": "***-***-****"},
		},
		{
			name:     "replace email domain",
			config:   `{"source_field": "email", "pattern": "@.*$", "replacement": "@example.com"}`,
			input:    map[string]interface{}{"email": "user@company.com"},
			expected: map[string]interface{}{"email": "user@example.com"},
		},
		{
			name:     "replace with target field",
			config:   `{"source_field": "text", "target_field": "clean_text", "pattern": "\\s+", "replacement": " "}`,
			input:    map[string]interface{}{"text": "hello   world"},
			expected: map[string]interface{}{"text": "hello   world", "clean_text": "hello world"},
		},
		{
			name:        "non-string field",
			config:      `{"source_field": "number", "pattern": "\\d+", "replacement": "X"}`,
			input:       map[string]interface{}{"number": 123},
			expectError: true,
		},
		{
			name:        "missing field",
			config:      `{"source_field": "missing", "pattern": "\\d+", "replacement": "X"}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &RegexTransform{}
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
