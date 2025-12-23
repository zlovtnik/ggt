package field

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestFlattenTransform(t *testing.T) {
	tests := []struct {
		name     string
		input    event.Event
		expected map[string]interface{}
	}{
		{
			name: "flatten nested object",
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "john",
						"age":  30,
					},
					"item": "book",
				},
			},
			expected: map[string]interface{}{
				"user.name": "john",
				"user.age":  30,
				"item":      "book",
			},
		},
		{
			name: "flatten deeply nested",
			input: event.Event{
				Payload: map[string]interface{}{
					"a": map[string]interface{}{
						"b": map[string]interface{}{
							"c": "value",
						},
					},
				},
			},
			expected: map[string]interface{}{
				"a.b.c": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &flattenTransform{}
			result, err := transform.Execute(context.Background(), tt.input)
			assert.NoError(t, err)
			require.IsType(t, event.Event{}, result)
			resultEvent := result.(event.Event)
			assert.Equal(t, tt.expected, resultEvent.Payload)
		})
	}
}
