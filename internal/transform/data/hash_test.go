package data

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
	"golang.org/x/crypto/bcrypt"
)

func TestHashTransform_Name(t *testing.T) {
	transform := &HashTransform{}
	assert.Equal(t, "data.hash", transform.Name())
}

func TestHashTransform_Configure(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name:        "valid sha256",
			config:      `{"source_field": "password", "algorithm": "sha256"}`,
			expectError: false,
		},
		{
			name:        "valid bcrypt",
			config:      `{"source_field": "password", "algorithm": "bcrypt"}`,
			expectError: false,
		},
		{
			name:        "missing source_field",
			config:      `{"algorithm": "sha256"}`,
			expectError: true,
		},
		{
			name:        "missing algorithm",
			config:      `{"source_field": "password"}`,
			expectError: true,
		},
		{
			name:        "invalid algorithm",
			config:      `{"source_field": "password", "algorithm": "invalid"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &HashTransform{}
			err := transform.Configure([]byte(tt.config))
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHashTransform_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		input       map[string]interface{}
		validate    func(t *testing.T, result map[string]interface{})
		expectError bool
	}{
		{
			name:   "sha256 hash",
			config: `{"source_field": "text", "algorithm": "sha256"}`,
			input:  map[string]interface{}{"text": "hello world"},
			validate: func(t *testing.T, result map[string]interface{}) {
				hash, ok := result["text"].(string)
				require.True(t, ok)
				assert.Len(t, hash, 64) // SHA256 produces 64 hex characters
				assert.Equal(t, "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9", hash)
			},
		},
		{
			name:   "bcrypt hash",
			config: `{"source_field": "password", "algorithm": "bcrypt"}`,
			input:  map[string]interface{}{"password": "secret"},
			validate: func(t *testing.T, result map[string]interface{}) {
				hash, ok := result["password"].(string)
				require.True(t, ok)
				// Bcrypt hash should start with $2a$ or $2b$
				assert.True(t, strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$"))
				// Verify it can be used to check the original password
				err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret"))
				assert.NoError(t, err)
			},
		},
		{
			name:   "hash with target field",
			config: `{"source_field": "input", "target_field": "output", "algorithm": "sha256"}`,
			input:  map[string]interface{}{"input": "data"},
			validate: func(t *testing.T, result map[string]interface{}) {
				assert.Equal(t, "data", result["input"])
				hash, ok := result["output"].(string)
				require.True(t, ok)
				assert.Len(t, hash, 64)
			},
		},
		{
			name:        "non-string field",
			config:      `{"source_field": "number", "algorithm": "sha256"}`,
			input:       map[string]interface{}{"number": 123},
			expectError: true,
		},
		{
			name:        "missing field",
			config:      `{"source_field": "missing", "algorithm": "sha256"}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &HashTransform{}
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
				resultMap := make(map[string]interface{})
				for key, val := range resultEvt.Payload {
					resultMap[key] = val
				}
				tt.validate(t, resultMap)
			}
		})
	}
}
