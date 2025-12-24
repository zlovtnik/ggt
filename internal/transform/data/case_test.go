package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestCaseTransform_Name(t *testing.T) {
	c := &caseTransform{}
	assert.Equal(t, "data.case", c.Name())
}

func TestCaseTransform_Configure(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  `{"field": "name", "case": "upper"}`,
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
			c := &caseTransform{}
			err := c.Configure([]byte(tt.config))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCaseTransform_Execute(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		config   string
		input    event.Event
		expected string
		wantErr  bool
	}{
		{
			name:     "upper case",
			config:   `{"field": "name", "case": "upper"}`,
			input:    event.Event{Payload: map[string]interface{}{"name": "hello"}},
			expected: "HELLO",
			wantErr:  false,
		},
		{
			name:     "lower case",
			config:   `{"field": "name", "case": "lower"}`,
			input:    event.Event{Payload: map[string]interface{}{"name": "HELLO"}},
			expected: "hello",
			wantErr:  false,
		},
		{
			name:     "title case",
			config:   `{"field": "name", "case": "title"}`,
			input:    event.Event{Payload: map[string]interface{}{"name": "hello world"}},
			expected: "Hello World",
			wantErr:  false,
		},
		{
			name:     "non-string field",
			config:   `{"field": "age", "case": "upper"}`,
			input:    event.Event{Payload: map[string]interface{}{"age": 30}},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "unsupported case",
			config:   `{"field": "name", "case": "invalid"}`,
			input:    event.Event{Payload: map[string]interface{}{"name": "hello"}},
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &caseTransform{}
			err := c.Configure([]byte(tt.config))
			require.NoError(t, err)

			ev := tt.input
			result, err := c.Execute(ctx, ev)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				resultEv, ok := result.(event.Event)
				require.True(t, ok)
				val, exists := resultEv.GetField("name")
				assert.True(t, exists)
				assert.Equal(t, tt.expected, val)
			}
		})
	}
}
