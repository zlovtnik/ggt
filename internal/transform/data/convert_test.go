package data

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestConvertTransform_Name(t *testing.T) {
	c := &convertTransform{}
	assert.Equal(t, "data.convert", c.Name())
}

func TestConvertTransform_Configure(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  `{"field": "test", "to": "int"}`,
			wantErr: false,
		},
		{
			name:    "empty config",
			config:  ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &convertTransform{}
			err := c.Configure([]byte(tt.config))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertTransform_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		config   string
		input    event.Event
		expected interface{}
		wantErr  bool
	}{
		{
			name:     "string to int",
			config:   `{"field": "value", "to": "int"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": "42"}},
			expected: int64(42),
			wantErr:  false,
		},
		{
			name:     "int to string",
			config:   `{"field": "value", "to": "string"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": 42}},
			expected: "42",
			wantErr:  false,
		},
		{
			name:     "float to int",
			config:   `{"field": "value", "to": "int"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": 3.14}},
			expected: int64(3),
			wantErr:  false,
		},
		{
			name:     "bool to string",
			config:   `{"field": "value", "to": "string"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": true}},
			expected: "true",
			wantErr:  false,
		},
		{
			name:     "string to bool",
			config:   `{"field": "value", "to": "bool"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": "true"}},
			expected: true,
			wantErr:  false,
		},
		{
			name:     "timestamp string to time",
			config:   `{"field": "value", "to": "timestamp"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": "2023-01-01T00:00:00Z"}},
			expected: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			wantErr:  false,
		},
		{
			name:     "time to string",
			config:   `{"field": "value", "to": "string"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)}},
			expected: "2023-01-01T00:00:00Z",
			wantErr:  false,
		},
		{
			name:     "invalid conversion",
			config:   `{"field": "value", "to": "int"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": "not_a_number"}},
			expected: nil,
			wantErr:  true,
		},
		{
			name:     "field not present",
			config:   `{"field": "missing", "to": "int"}`,
			input:    event.Event{Payload: map[string]interface{}{"value": 42}},
			expected: 42, // no change to existing field
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &convertTransform{}
			err := c.Configure([]byte(tt.config))
			require.NoError(t, err)

			ev := tt.input
			result, err := c.Execute(ctx, ev)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expected != nil {
					resultEv, ok := result.(event.Event)
					require.True(t, ok)
					val, exists := resultEv.GetField("value")
					assert.True(t, exists)
					assert.Equal(t, tt.expected, val)
				}
			}
		})
	}
}
