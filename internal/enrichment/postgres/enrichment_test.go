package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	transformPkg "github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestPostgresEnrichment_Configure(t *testing.T) {
	cfg := &enrichConfig{
		Query:       "SELECT * FROM users WHERE id = $1",
		Params:      []string{"user_id"},
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM users WHERE id = $1", transform.config.Query)
	assert.Equal(t, []string{"user_id"}, transform.config.Params)
	assert.Equal(t, "user_data", transform.config.TargetField)
}

func TestPostgresEnrichment_Configure_MissingQuery(t *testing.T) {
	cfg := &enrichConfig{
		TargetField: "user_data",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestPostgresEnrichment_Configure_MissingTargetField(t *testing.T) {
	cfg := &enrichConfig{
		Query: "SELECT * FROM users WHERE id = $1",
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	assert.Error(t, err)
}

func TestPostgresEnrichment_Name(t *testing.T) {
	transform := &enrichTransform{}
	assert.Equal(t, "enrich.postgres", transform.Name())
}

func TestPostgresEnrichment_Execute_NotConfigured(t *testing.T) {
	transform := &enrichTransform{}

	ev := event.Event{
		Payload: map[string]interface{}{},
	}

	result, err := transform.Execute(context.Background(), ev)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestPostgresEnrichment_Execute_InvalidPayload(t *testing.T) {
	cfg := &enrichConfig{
		Query:       "SELECT * FROM users WHERE id = $1",
		Params:      []string{"user_id"},
		TargetField: "user_data",
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

func TestPostgresEnrichment_Execute_MissingParam(t *testing.T) {
	cfg := &enrichConfig{
		Query:       "SELECT * FROM users WHERE id = $1",
		Params:      []string{"user_id"},
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
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestPostgresEnrichment_Execute_DropOnError(t *testing.T) {
	cfg := &enrichConfig{
		Query:       "SELECT * FROM users WHERE id = $1",
		Params:      []string{"user_id"},
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
	assert.ErrorIs(t, err, transformPkg.ErrDrop)
	assert.Nil(t, result)
}

func TestPostgresEnrichment_MultipleOutputFields(t *testing.T) {
	cfg := &enrichConfig{
		Query:        "SELECT id, name, email FROM users WHERE id = $1",
		Params:       []string{"user_id"},
		TargetField:  "user_data",
		OutputFields: []string{"id", "name", "email"},
	}

	cfgBytes, err := json.Marshal(cfg)
	require.NoError(t, err)

	transform := &enrichTransform{}
	err = transform.Configure(cfgBytes)
	require.NoError(t, err)

	assert.Equal(t, []string{"id", "name", "email"}, transform.config.OutputFields)
}
