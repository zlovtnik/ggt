package field

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestExtractTransform(t *testing.T) {
	tests := []struct {
		name     string
		config   extractConfig
		input    event.Event
		expected map[string]interface{}
		wantErr  bool
	}{
		{
			name: "extract nested field",
			config: extractConfig{
				Source: "user.name",
				Target: "username",
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "john",
						"age":  30,
					},
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"age": 30,
				},
				"username": "john",
			},
			wantErr: false,
		},
		{
			name: "extract non-existent field",
			config: extractConfig{
				Source: "user.email",
				Target: "email",
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "john",
					},
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name": "john",
				},
			},
			wantErr: false,
		},
		{
			name: "extract to existing target without overwrite",
			config: extractConfig{
				Source:    "user.name",
				Target:    "existing",
				Overwrite: false,
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "john",
					},
					"existing": "old_value",
				},
			},
			expected: nil,
			wantErr:  true,
		},
		{
			name: "extract to existing target with overwrite",
			config: extractConfig{
				Source:    "user.name",
				Target:    "existing",
				Overwrite: true,
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "john",
					},
					"existing": "old_value",
				},
			},
			expected: map[string]interface{}{
				"user":     map[string]interface{}{},
				"existing": "john",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &extractTransform{cfg: tt.config}
			result, err := transform.Execute(context.Background(), tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				resultEvent, ok := result.(event.Event)
				assert.True(t, ok, "result should be of type event.Event")
				assert.Equal(t, tt.expected, resultEvent.Payload)
			}
		})
	}
}
