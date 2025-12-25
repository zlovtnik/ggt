package filter

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestFilterSplitTransform(t *testing.T) {
	cfg := `{"field":"items","target":"item"}`
	trans := NewSplitTransform()
	require.NoError(t, trans.Configure(json.RawMessage(cfg)))

	ev := event.Event{
		Payload: map[string]interface{}{
			"items": []interface{}{"a", "b", "c"},
		},
		Headers: map[string]string{},
	}

	out, err := trans.Execute(context.Background(), ev)
	require.NoError(t, err)

	results, ok := out.([]event.Event)
	require.True(t, ok)
	require.Len(t, results, 3)

	for idx, res := range results {
		val, exists := res.GetField("item")
		assert.True(t, exists)
		assert.Equal(t, ev.Payload["items"].([]interface{})[idx], val)
		_, stillExists := res.GetField("items")
		assert.False(t, stillExists, "original array should be removed")
		assert.Equal(t, strconv.Itoa(idx), res.Headers["_split_index"])
	}
}

func TestFilterSplitTransformDropEmpty(t *testing.T) {
	cfg := `{"field":"items","drop_empty":true}`
	trans := NewSplitTransform()
	require.NoError(t, trans.Configure(json.RawMessage(cfg)))

	ev := event.Event{Payload: map[string]interface{}{"items": []interface{}{}}, Headers: map[string]string{}}

	out, err := trans.Execute(context.Background(), ev)
	assert.ErrorIs(t, err, transform.ErrDrop)
	assert.Nil(t, out)
}

func TestFilterSplitTransformMissingField(t *testing.T) {
	cfg := `{"field":"items"}`
	trans := NewSplitTransform()
	require.NoError(t, trans.Configure(json.RawMessage(cfg)))

	ev := event.Event{Payload: map[string]interface{}{"other": []interface{}{1, 2, 3}}, Headers: map[string]string{}}

	out, err := trans.Execute(context.Background(), ev)
	assert.NoError(t, err)
	assert.Nil(t, out)
}

func TestFilterSplitTransformNonArray(t *testing.T) {
	cfg := `{"field":"items"}`
	trans := NewSplitTransform()
	require.NoError(t, trans.Configure(json.RawMessage(cfg)))

	ev := event.Event{Payload: map[string]interface{}{"items": "not-array"}, Headers: map[string]string{}}

	out, err := trans.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.Nil(t, out)
}
