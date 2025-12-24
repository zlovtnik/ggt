package split

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestSplitArrayTransform_Name(t *testing.T) {
	trans := NewSplitArrayTransform()
	assert.Equal(t, "split.array", trans.Name())
}

func TestSplitArrayTransform_Configure(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "valid config",
			config:  `{"field": "items"}`,
			wantErr: false,
		},
		{
			name:    "valid config with key",
			config:  `{"field": "items", "key": "element"}`,
			wantErr: false,
		},
		{
			name:       "missing field",
			config:     `{"key": "value"}`,
			wantErr:    true,
			wantErrMsg: "field path is required",
		},
		{
			name:       "empty config",
			config:     ``,
			wantErr:    true,
			wantErrMsg: "split array configuration required",
		},
		{
			name:    "invalid json",
			config:  `{invalid}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewSplitArrayTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSplitArrayTransform_Execute(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		event      event.Event
		wantCount  int
		wantErr    bool
		wantErrMsg string
		validate   func(t *testing.T, results []event.Event)
	}{
		{
			name:      "split array of strings",
			config:    `{"field": "items"}`,
			event:     event.NewBuilder().WithField("items", []interface{}{"a", "b", "c"}).Build(),
			wantCount: 3,
			wantErr:   false,
			validate: func(t *testing.T, results []event.Event) {
				val1, ok := results[0].GetField("value")
				require.True(t, ok)
				assert.Equal(t, "a", val1)
				val2, ok := results[1].GetField("value")
				require.True(t, ok)
				assert.Equal(t, "b", val2)
				val3, ok := results[2].GetField("value")
				require.True(t, ok)
				assert.Equal(t, "c", val3)
			},
		},
		{
			name:      "split array of numbers",
			config:    `{"field": "numbers"}`,
			event:     event.NewBuilder().WithField("numbers", []interface{}{1.0, 2.0, 3.0}).Build(),
			wantCount: 3,
			wantErr:   false,
			validate: func(t *testing.T, results []event.Event) {
				val, ok := results[0].GetField("value")
				require.True(t, ok)
				assert.Equal(t, 1.0, val)
			},
		},
		{
			name:      "split array of objects",
			config:    `{"field": "items", "key": "item"}`,
			event:     event.NewBuilder().WithField("items", []interface{}{map[string]interface{}{"id": "1"}, map[string]interface{}{"id": "2"}}).Build(),
			wantCount: 2,
			wantErr:   false,
			validate: func(t *testing.T, results []event.Event) {
				item, ok := results[0].GetField("item")
				require.True(t, ok)
				assert.Equal(t, map[string]interface{}{"id": "1"}, item)
			},
		},
		{
			name:      "split empty array",
			config:    `{"field": "items"}`,
			event:     event.NewBuilder().WithField("items", []interface{}{}).Build(),
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "split with nil field",
			config:    `{"field": "items"}`,
			event:     event.NewBuilder().WithField("items", nil).Build(),
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:       "field is not array",
			config:     `{"field": "items"}`,
			event:      event.NewBuilder().WithField("items", "not an array").Build(),
			wantCount:  0,
			wantErr:    true,
			wantErrMsg: "is not an array",
		},
		{
			name:       "missing field",
			config:     `{"field": "nonexistent"}`,
			event:      event.NewBuilder().WithField("items", []interface{}{"a", "b"}).Build(),
			wantErr:    true,
			wantErrMsg: "field does not exist",
		},
		{
			name:      "nested array field",
			config:    `{"field": "user.tags"}`,
			event:     event.NewBuilder().WithField("user", map[string]interface{}{"tags": []interface{}{"admin", "user"}}).Build(),
			wantCount: 2,
			wantErr:   false,
			validate: func(t *testing.T, results []event.Event) {
				val, ok := results[0].GetField("value")
				require.True(t, ok)
				assert.Equal(t, "admin", val)
			},
		},
		{
			name:      "split preserves other fields",
			config:    `{"field": "items"}`,
			event:     event.NewBuilder().WithField("items", []interface{}{"a", "b"}).WithField("id", "123").Build(),
			wantCount: 2,
			wantErr:   false,
			validate: func(t *testing.T, results []event.Event) {
				id, ok := results[0].GetField("id")
				require.True(t, ok)
				assert.Equal(t, "123", id)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewSplitArrayTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}
			} else {
				require.NoError(t, err)
				results, ok := result.([]event.Event)
				require.True(t, ok, "result should be []event.Event")
				assert.Equal(t, tt.wantCount, len(results))
				if tt.validate != nil {
					tt.validate(t, results)
				}
			}
		})
	}
}

func TestSplitArrayTransform_InvalidPayload(t *testing.T) {
	trans := NewSplitArrayTransform()
	err := trans.Configure(json.RawMessage(`{"field": "items"}`))
	require.NoError(t, err)

	result, err := trans.Execute(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unexpected payload type")
}

func TestSplitArrayTransform_DefaultKey(t *testing.T) {
	// Test that default key "value" is used when key is not specified
	trans := NewSplitArrayTransform()
	err := trans.Configure(json.RawMessage(`{"field": "items"}`))
	require.NoError(t, err)

	ev := event.NewBuilder().WithField("items", []interface{}{"test"}).Build()
	result, err := trans.Execute(context.Background(), ev)
	require.NoError(t, err)

	results, ok := result.([]event.Event)
	require.True(t, ok, "result should be []event.Event")
	require.Equal(t, 1, len(results))
	val, ok := results[0].GetField("value")
	require.True(t, ok)
	assert.Equal(t, "test", val)
}
