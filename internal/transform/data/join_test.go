package data

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestJoinTransform_Name(t *testing.T) {
	transform := &JoinTransform{}
	assert.Equal(t, "data.join", transform.Name())
}

func TestJoinTransform_Configure(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		expected    JoinTransform
	}{
		{
			name:        "valid config",
			config:      `{"source_field": "tags", "separator": ";"}`,
			expectError: false,
			expected: JoinTransform{
				sourceField: "tags",
				targetField: "tags",
				separator:   ";",
			},
		},
		{
			name:        "valid config with target field",
			config:      `{"source_field": "tags", "target_field": "joined_tags", "separator": ","}`,
			expectError: false,
			expected: JoinTransform{
				sourceField: "tags",
				targetField: "joined_tags",
				separator:   ",",
			},
		},
		{
			name:        "missing source_field",
			config:      `{"separator": ","}`,
			expectError: true,
		},
		{
			name:        "default separator",
			config:      `{"source_field": "tags"}`,
			expectError: false,
			expected: JoinTransform{
				sourceField: "tags",
				targetField: "tags",
				separator:   ",",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &JoinTransform{}
			err := transform.Configure([]byte(tt.config))
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected.sourceField, transform.sourceField)
				assert.Equal(t, tt.expected.targetField, transform.targetField)
				assert.Equal(t, tt.expected.separator, transform.separator)
			}
		})
	}
}

func TestJoinTransform_Execute(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		input       map[string]interface{}
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:     "join string array",
			config:   `{"source_field": "tags", "separator": ";"}`,
			input:    map[string]interface{}{"tags": []string{"a", "b", "c"}},
			expected: map[string]interface{}{"tags": "a;b;c"},
		},
		{
			name:     "join interface array",
			config:   `{"source_field": "numbers", "separator": "-"}`,
			input:    map[string]interface{}{"numbers": []interface{}{1, 2, 3}},
			expected: map[string]interface{}{"numbers": "1-2-3"},
		},
		{
			name:     "join with target field",
			config:   `{"source_field": "tags", "target_field": "joined", "separator": ","}`,
			input:    map[string]interface{}{"tags": []string{"x", "y", "z"}},
			expected: map[string]interface{}{"tags": []string{"x", "y", "z"}, "joined": "x,y,z"},
		},
		{
			name:        "non-array field",
			config:      `{"source_field": "name"}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
		{
			name:        "missing field",
			config:      `{"source_field": "missing"}`,
			input:       map[string]interface{}{"name": "test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &JoinTransform{}
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
