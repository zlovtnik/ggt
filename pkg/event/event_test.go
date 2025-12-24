package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventClone(t *testing.T) {
	tests := []struct {
		name     string
		original Event
	}{
		{
			name: "empty event",
			original: Event{
				Payload:   nil,
				Metadata:  Metadata{},
				Timestamp: time.Now(),
				Headers:   nil,
			},
		},
		{
			name: "event with simple payload",
			original: Event{
				Payload: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
				Metadata: Metadata{
					Topic:     "test-topic",
					Partition: 0,
					Offset:    123,
					Key:       []byte("test-key"),
				},
				Timestamp: time.Now(),
				Headers: map[string]string{
					"content-type": "application/json",
				},
			},
		},
		{
			name: "event with nested payload",
			original: Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"name": "Alice",
							"age":  25,
						},
						"preferences": []string{"dark-mode", "notifications"},
					},
					"metadata": map[string]interface{}{
						"source":  "api",
						"version": "1.0",
					},
				},
				Metadata: Metadata{
					Topic:     "complex-topic",
					Partition: 1,
					Offset:    456,
					Key:       []byte("complex-key"),
				},
				Timestamp: time.Now(),
				Headers: map[string]string{
					"user-agent": "test-client",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloned := tt.original.Clone()

			// Clone preserves nil maps as nil (consistent with deepCopyMap behavior)
			assert.Equal(t, tt.original.Payload, cloned.Payload)
			assert.Equal(t, tt.original.Headers, cloned.Headers)

			// Metadata should be properly cloned
			assert.Equal(t, tt.original.Metadata.Topic, cloned.Metadata.Topic)
			assert.Equal(t, tt.original.Metadata.Partition, cloned.Metadata.Partition)
			assert.Equal(t, tt.original.Metadata.Offset, cloned.Metadata.Offset)
			if tt.original.Metadata.Key == nil {
				assert.Nil(t, cloned.Metadata.Key)
			} else {
				assert.Equal(t, tt.original.Metadata.Key, cloned.Metadata.Key)
			}

			assert.Equal(t, tt.original.Timestamp, cloned.Timestamp)

			// Verify immutability - modifying clone doesn't affect original
			// (Nested payload immutability is covered in a dedicated test.)

			// Modify metadata in clone
			cloned.Metadata.Topic = "modified-topic"
			assert.NotEqual(t, tt.original.Metadata.Topic, cloned.Metadata.Topic)

			// Modify headers in clone
			if cloned.Headers != nil {
				cloned.Headers["modified"] = "true"
				_, exists := tt.original.Headers["modified"]
				assert.False(t, exists, "Modifying clone headers should not affect original")
			}
		})
	}
}

func TestEventGetField(t *testing.T) {
	event := Event{
		Payload: map[string]interface{}{
			"name": "John",
			"age":  30,
			"user": map[string]interface{}{
				"profile": map[string]interface{}{
					"firstName": "John",
					"lastName":  "Doe",
					"details": map[string]interface{}{
						"city":    "New York",
						"country": "USA",
					},
				},
				"preferences": map[string]interface{}{
					"theme": "dark",
					"lang":  "en",
				},
			},
			"tags": []string{"golang", "testing"},
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		found    bool
	}{
		{
			name:     "root level field",
			path:     "name",
			expected: "John",
			found:    true,
		},
		{
			name:     "nested field",
			path:     "user.profile.firstName",
			expected: "John",
			found:    true,
		},
		{
			name:     "deeply nested field",
			path:     "user.profile.details.city",
			expected: "New York",
			found:    true,
		},
		{
			name:     "array field",
			path:     "tags",
			expected: []string{"golang", "testing"},
			found:    true,
		},
		{
			name:  "non-existent field",
			path:  "nonexistent",
			found: false,
		},
		{
			name:  "non-existent nested field",
			path:  "user.profile.nonexistent",
			found: false,
		},
		{
			name:  "invalid path - trying to traverse into non-map",
			path:  "name.invalid",
			found: false,
		},
		{
			name:  "empty path",
			path:  "",
			found: false,
		},
		{
			name:  "path with empty segments",
			path:  "user..profile",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, found := event.GetField(tt.path)
			assert.Equal(t, tt.found, found)
			if tt.found {
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

func TestEventSetField(t *testing.T) {
	tests := []struct {
		name     string
		initial  Event
		path     string
		value    interface{}
		expected map[string]interface{}
	}{
		{
			name: "set root level field",
			initial: Event{
				Payload: map[string]interface{}{},
			},
			path:  "name",
			value: "John",
			expected: map[string]interface{}{
				"name": "John",
			},
		},
		{
			name: "set nested field - create intermediate maps",
			initial: Event{
				Payload: map[string]interface{}{},
			},
			path:  "user.profile.name",
			value: "Alice",
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"name": "Alice",
					},
				},
			},
		},
		{
			name: "set deeply nested field",
			initial: Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"name": "John",
						},
					},
				},
			},
			path:  "user.profile.age",
			value: 25,
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"name": "John",
						"age":  25,
					},
				},
			},
		},
		{
			name: "overwrite existing field",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
				},
			},
			path:  "name",
			value: "Jane",
			expected: map[string]interface{}{
				"name": "Jane",
			},
		},
		{
			name: "empty path - should return unchanged",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
				},
			},
			path:  "",
			value: "ignored",
			expected: map[string]interface{}{
				"name": "John",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.initial.SetField(tt.path, tt.value)
			assert.Equal(t, tt.expected, result.Payload)

			// Verify immutability - original should be unchanged
			if tt.path != "" {
				// For non-empty paths we expect SetField to return a distinct
				// event with a cloned payload (immutability preserved).
				assert.NotEqual(t, tt.initial.Payload, result.Payload)
			} else {
				// Empty path is a no-op. Current implementation returns the
				// original payload reference (no deep clone) as an optimization.
				// Verify the no-op preserves the original payload reference by
				// mutating the result and observing the original is affected.
				if result.Payload != nil {
					result.Payload["__test_noop_marker__"] = true
					v, ok := tt.initial.Payload["__test_noop_marker__"]
					assert.True(t, ok)
					assert.Equal(t, true, v)
				}
			}
		})
	}
}

func TestEventClone_NestedPayloadImmutability(t *testing.T) {
	original := Event{
		Payload: map[string]interface{}{
			"user": map[string]interface{}{
				"profile": map[string]interface{}{
					"name": "Alice",
					"age":  25,
				},
			},
		},
	}

	cloned := original.Clone()

	// Mutate nested value in clone
	if nested, ok := cloned.Payload["user"].(map[string]interface{}); ok {
		if profile, ok := nested["profile"].(map[string]interface{}); ok {
			profile["name"] = "Modified"
		}
	}

	// Original should remain unchanged
	userVal, ok := original.Payload["user"].(map[string]interface{})
	require.True(t, ok)
	profileVal, ok := userVal["profile"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Alice", profileVal["name"])
}

func TestEventRemoveField(t *testing.T) {
	tests := []struct {
		name     string
		initial  Event
		path     string
		expected map[string]interface{}
	}{
		{
			name: "remove root level field",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
					"age":  30,
				},
			},
			path: "name",
			expected: map[string]interface{}{
				"age": 30,
			},
		},
		{
			name: "remove nested field",
			initial: Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"name": "John",
							"age":  25,
						},
					},
				},
			},
			path: "user.profile.age",
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"profile": map[string]interface{}{
						"name": "John",
					},
				},
			},
		},
		{
			name: "remove non-existent field - should not panic",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
				},
			},
			path: "nonexistent",
			expected: map[string]interface{}{
				"name": "John",
			},
		},
		{
			name: "remove field with invalid path - should not panic",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
				},
			},
			path: "name.invalid",
			expected: map[string]interface{}{
				"name": "John",
			},
		},
		{
			name: "empty path - should return unchanged",
			initial: Event{
				Payload: map[string]interface{}{
					"name": "John",
				},
			},
			path: "",
			expected: map[string]interface{}{
				"name": "John",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.initial.RemoveField(tt.path)
			assert.Equal(t, tt.expected, result.Payload)

			// For cases where nothing should be removed, verify payload is the same
			if tt.path == "nonexistent" || tt.path == "name.invalid" || tt.path == "" {
				assert.Equal(t, tt.initial.Payload, result.Payload, "Payload should be unchanged when nothing is removed")
			} else {
				// Verify immutability - new event should be returned
				assert.NotEqual(t, tt.initial.Payload, result.Payload)
			}
		})
	}
}

func TestBuilder(t *testing.T) {
	t.Run("NewBuilder", func(t *testing.T) {
		builder := NewBuilder()
		require.NotNil(t, builder)

		event := builder.Build()
		assert.NotNil(t, event.Payload)
		assert.NotNil(t, event.Headers)
		assert.False(t, event.Timestamp.IsZero())
	})

	t.Run("WithPayload", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":  "Test",
			"value": 42,
		}

		event := NewBuilder().WithPayload(payload).Build()
		assert.Equal(t, payload, event.Payload)

		// Verify immutability - modifying original payload doesn't affect event
		payload["new"] = "field"
		assert.NotContains(t, event.Payload, "new")
	})

	t.Run("WithField", func(t *testing.T) {
		tests := []struct {
			name     string
			path     string
			value    interface{}
			expected map[string]interface{}
		}{
			{
				name:  "simple field",
				path:  "name",
				value: "John",
				expected: map[string]interface{}{
					"name": "John",
				},
			},
			{
				name:  "nested field",
				path:  "user.profile.name",
				value: "Alice",
				expected: map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"name": "Alice",
						},
					},
				},
			},
			{
				name:  "multiple nested fields",
				path:  "a.b.c.d",
				value: "deep",
				expected: map[string]interface{}{
					"a": map[string]interface{}{
						"b": map[string]interface{}{
							"c": map[string]interface{}{
								"d": "deep",
							},
						},
					},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				event := NewBuilder().WithField(tt.path, tt.value).Build()
				assert.Equal(t, tt.expected, event.Payload)
			})
		}
	})

	t.Run("WithMetadata", func(t *testing.T) {
		metadata := Metadata{
			Topic:     "test-topic",
			Partition: 1,
			Offset:    123,
			Key:       []byte("test-key"),
		}

		event := NewBuilder().WithMetadata(metadata).Build()
		assert.Equal(t, metadata, event.Metadata)
	})

	t.Run("WithTopic", func(t *testing.T) {
		event := NewBuilder().WithTopic("my-topic").Build()
		assert.Equal(t, "my-topic", event.Metadata.Topic)
	})

	t.Run("WithPartition", func(t *testing.T) {
		event := NewBuilder().WithPartition(5).Build()
		assert.Equal(t, int32(5), event.Metadata.Partition)
	})

	t.Run("WithOffset", func(t *testing.T) {
		event := NewBuilder().WithOffset(999).Build()
		assert.Equal(t, int64(999), event.Metadata.Offset)
	})

	t.Run("WithKey", func(t *testing.T) {
		key := []byte("my-key")
		event := NewBuilder().WithKey(key).Build()
		assert.Equal(t, key, event.Metadata.Key)

		// Verify immutability - modifying original key doesn't affect metadata
		key[0] = 'X'
		assert.NotEqual(t, key[0], event.Metadata.Key[0])
	})

	t.Run("WithHeaders", func(t *testing.T) {
		headers := map[string]string{
			"content-type": "application/json",
			"user-agent":   "test",
		}

		event := NewBuilder().WithHeaders(headers).Build()
		assert.Equal(t, headers, event.Headers)

		// Verify immutability
		headers["new"] = "header"
		assert.NotContains(t, event.Headers, "new")
	})

	t.Run("WithHeader", func(t *testing.T) {
		event := NewBuilder().
			WithHeader("content-type", "application/json").
			WithHeader("version", "1.0").
			Build()

		assert.Equal(t, "application/json", event.Headers["content-type"])
		assert.Equal(t, "1.0", event.Headers["version"])
	})

	t.Run("WithTimestamp", func(t *testing.T) {
		ts := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
		event := NewBuilder().WithTimestamp(ts).Build()
		assert.Equal(t, ts, event.Timestamp)
	})

	t.Run("Builder chaining", func(t *testing.T) {
		ts := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

		event := NewBuilder().
			WithField("user.name", "John").
			WithField("user.age", 30).
			WithTopic("users").
			WithPartition(0).
			WithOffset(123).
			WithHeader("source", "test").
			WithTimestamp(ts).
			Build()

		userVal, ok := event.Payload["user"].(map[string]interface{})
		require.True(t, ok)
		userMap := userVal
		assert.Equal(t, "John", userMap["name"])
		assert.Equal(t, 30, userMap["age"])
		assert.Equal(t, "users", event.Metadata.Topic)
		assert.Equal(t, int32(0), event.Metadata.Partition)
		assert.Equal(t, int64(123), event.Metadata.Offset)
		assert.Equal(t, "test", event.Headers["source"])
		assert.Equal(t, ts, event.Timestamp)
	})
}

// Benchmark tests for performance-critical operations
func BenchmarkEventClone(b *testing.B) {
	event := NewBuilder().
		WithField("user.profile.name", "John").
		WithField("user.profile.age", 30).
		WithField("user.preferences.theme", "dark").
		WithField("metadata.source", "api").
		WithTopic("test").
		WithPartition(0).
		WithOffset(123).
		WithHeader("content-type", "application/json").
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.Clone()
	}
}

func BenchmarkEventGetField(b *testing.B) {
	event := NewBuilder().
		WithField("user.profile.name", "John").
		WithField("user.profile.details.address.city", "New York").
		WithField("user.profile.details.address.country", "USA").
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = event.GetField("user.profile.details.address.city")
	}
}

func BenchmarkEventSetField(b *testing.B) {
	event := NewBuilder().
		WithField("user.profile.name", "John").
		Build()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.SetField("user.profile.age", 30)
	}
}
