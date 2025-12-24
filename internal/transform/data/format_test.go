package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestFormatTransform_Name(t *testing.T) {
	f := &formatTransform{}
	assert.Equal(t, "data.format", f.Name())
}

func TestFormatTransform_Configure(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  `{"field": "result", "format": "Hello %s", "args": ["name"]}`,
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
			f := &formatTransform{}
			err := f.Configure([]byte(tt.config))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatTransform_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		config        string
		input         event.Event
		expected      string
		expectedField string
		wantErr       bool
	}{
		{
			name:          "simple format",
			config:        `{"field": "greeting", "format": "Hello %s", "args": ["name"]}`,
			input:         event.Event{Payload: map[string]interface{}{"name": "World"}},
			expected:      "Hello World",
			expectedField: "greeting",
			wantErr:       false,
		},
		{
			name:          "multiple args",
			config:        `{"field": "result", "format": "%s is %d years old", "args": ["name", "age"]}`,
			input:         event.Event{Payload: map[string]interface{}{"name": "Alice", "age": 30}},
			expected:      "Alice is 30 years old",
			expectedField: "result",
			wantErr:       false,
		},
		{
			name:          "missing field",
			config:        `{"field": "result", "format": "Hello %s", "args": ["missing"]}`,
			input:         event.Event{Payload: map[string]interface{}{"name": "World"}},
			expected:      "",
			expectedField: "result",
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &formatTransform{}
			err := f.Configure([]byte(tt.config))
			require.NoError(t, err)

			ev := tt.input
			result, err := f.Execute(ctx, ev)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				resultEv, ok := result.(event.Event)
				require.True(t, ok)
				val, exists := resultEv.GetField(tt.expectedField)
				assert.True(t, exists)
				assert.Equal(t, tt.expected, val)
			}
		})
	}
}
