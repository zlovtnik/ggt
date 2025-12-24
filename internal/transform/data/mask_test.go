package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestMaskTransform_Name(t *testing.T) {
	transform := &MaskTransform{}
	assert.Equal(t, "data.mask", transform.Name())
}

func TestMaskTransform_Configure(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name:        "valid keep_prefix",
			config:      `{"source_field": "ssn", "keep_prefix": 4}`,
			expectError: false,
		},
		{
			name:        "valid pattern",
			config:      `{"source_field": "email", "pattern": "@.*"}`,
			expectError: false,
		},
		{
			name:        "valid with target field and mask char",
			config:      `{"source_field": "phone", "target_field": "masked_phone", "keep_prefix": 3, "mask_char": "X"}`,
			expectError: false,
		},
		{
			name:        "missing source_field",
			config:      `{"keep_prefix": 4}`,
			expectError: true,
		},
		{
			name:        "neither pattern nor keep_prefix",
			config:      `{"source_field": "data"}`,
			expectError: true,
		},
		{
			name:        "both pattern and keep_prefix",
			config:      `{"source_field": "data", "pattern": ".*", "keep_prefix": 2}`,
			expectError: false,
		},
		{
			name:        "invalid mask_char length",
			config:      `{"source_field": "data", "keep_prefix": 2, "mask_char": "XX"}`,
			expectError: true,
		},
		{
			name:        "invalid regex pattern",
			config:      `{"source_field": "data", "pattern": "[invalid"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &MaskTransform{}
			err := transform.Configure([]byte(tt.config))
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMaskTransform_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		input       map[string]interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "keep prefix masking",
			config:   `{"source_field": "ssn", "keep_prefix": 4}`,
			input:    map[string]interface{}{"ssn": "123-45-6789"},
			expected: map[string]interface{}{"ssn": "123-*******"},
		},
		{
			name:     "keep prefix with custom mask char",
			config:   `{"source_field": "card", "keep_prefix": 4, "mask_char": "X"}`,
			input:    map[string]interface{}{"card": "1234567890123456"},
			expected: map[string]interface{}{"card": "1234XXXXXXXXXXXX"},
		},
		{
			name:     "regex pattern masking",
			config:   `{"source_field": "email", "pattern": "@.*"}`,
			input:    map[string]interface{}{"email": "user@company.com"},
			expected: map[string]interface{}{"email": "user************"},
		},
		{
			name:     "regex with keep prefix",
			config:   `{"source_field": "phone", "pattern": "\\d{4}$", "keep_prefix": 2}`,
			input:    map[string]interface{}{"phone": "555-123-4567"},
			expected: map[string]interface{}{"phone": "555-123-45**"},
		},
		{
			name:     "mask with target field",
			config:   `{"source_field": "secret", "target_field": "masked_secret", "keep_prefix": 2}`,
			input:    map[string]interface{}{"secret": "password"},
			expected: map[string]interface{}{"secret": "password", "masked_secret": "pa******"},
		},
		{
			name:     "keep prefix longer than string",
			config:   `{"source_field": "short", "keep_prefix": 10}`,
			input:    map[string]interface{}{"short": "abc"},
			expected: map[string]interface{}{"short": "abc"},
		},
		{
			name:     "regex no matches",
			config:   `{"source_field": "text", "pattern": "\\d+"}`,
			input:    map[string]interface{}{"text": "hello world"},
			expected: map[string]interface{}{"text": "hello world"},
		},
		{
			name:        "non-string field",
			config:      `{"source_field": "number", "keep_prefix": 2}`,
			input:       map[string]interface{}{"number": 123},
			expectError: true,
		},
		{
			name:        "missing field",
			config:      `{"source_field": "missing", "keep_prefix": 2}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &MaskTransform{}
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
