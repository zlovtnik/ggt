package redis

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestEnrichment_Configure(t *testing.T) {
	cfg := &enrichConfig{
		KeyField:    "user_id",
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)
	assert.Equal(t, "user_id", transform.cfg.KeyField)
	assert.Equal(t, "user_data", transform.cfg.TargetField)
}

func TestEnrichment_Configure_MissingKeyField(t *testing.T) {
	cfg := &enrichConfig{
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestEnrichment_Configure_MissingTargetField(t *testing.T) {
	cfg := &enrichConfig{
		KeyField: "user_id",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestEnrichment_Name(t *testing.T) {
	transform := &enrichTransform{}
	assert.Equal(t, "enrich.redis", transform.Name())
}

func TestEnrichment_Execute_NoKeyField(t *testing.T) {
	cfg := &enrichConfig{
		KeyField:    "user_id",
		TargetField: "user_data",
		DropOnError: false,
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	require.NoError(t, err)
	assert.Equal(t, ev, result)
}

func TestEnrichment_Execute_DropOnError(t *testing.T) {
	cfg := &enrichConfig{
		KeyField:    "user_id",
		TargetField: "user_data",
		DropOnError: true,
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEnrichment_Execute_NotConfigured(t *testing.T) {
	transform := &enrichTransform{}

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestEnrichment_Execute_InvalidPayload(t *testing.T) {
	cfg := &enrichConfig{
		KeyField:    "user_id",
		TargetField: "user_data",
		DropOnError: false,
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	result, err := transform.Execute(context.Background(), "not an event")
	assert.Error(t, err)
	assert.Nil(t, result)
}
