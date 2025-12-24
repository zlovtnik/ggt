package field

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestAddTransform(t *testing.T) {
	tests := []struct {
		name     string
		config   addConfig
		input    event.Event
		expected map[string]interface{}
	}{
		{
			name: "add simple field",
			config: addConfig{
				Field: "newField",
				Value: "newValue",
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"existing": "value",
				},
			},
			expected: map[string]interface{}{
				"existing": "value",
				"newField": "newValue",
			},
		},
		{
			name: "add nested field",
			config: addConfig{
				Field: "user.name",
				Value: "john",
			},
			input: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"age": 30,
					},
				},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"age":  30,
					"name": "john",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transform := &addTransform{cfg: tt.config}
			result, err := transform.Execute(context.Background(), tt.input)
			assert.NoError(t, err)
			assert.IsType(t, event.Event{}, result)
			resultEvent := result.(event.Event)
			assert.Equal(t, tt.expected, resultEvent.Payload)
		})
	}
}
