package validate

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestRequiredTransform(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		event     event.Event
		wantError bool
		wantDrop  bool
		checkDLQ  bool
	}{
		{
			name:   "all required fields present",
			config: `{"fields": ["email", "name"], "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com", "name": "John"},
				Headers: map[string]string{},
			},
			wantDrop: false,
		},
		{
			name:   "missing required field with drop",
			config: `{"fields": ["email", "name"], "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
		{
			name:   "missing required field with dlq",
			config: `{"fields": ["email", "name"], "on_error": "dlq"}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
				Headers: map[string]string{},
			},
			wantDrop: false,
			checkDLQ: true,
		},
		{
			name:   "missing required field with fail",
			config: `{"fields": ["email", "name"], "on_error": "fail"}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
				Headers: map[string]string{},
			},
			wantError: true,
		},
		{
			name:   "multiple missing fields",
			config: `{"fields": ["email", "name", "age"], "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewRequiredTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkDLQ {
					ev, ok := result.(event.Event)
					require.True(t, ok, "result should be an event.Event, got %T", result)
					assert.NotEmpty(t, ev.Headers["_dlq_reason"])
					assert.Equal(t, "validate.required", ev.Headers["_dlq_transform"])
				}
			}
		})
	}
}

func TestRequiredTransformNested(t *testing.T) {
	trans := NewRequiredTransform()
	err := trans.Configure(json.RawMessage(`{"fields": ["user.email", "user.name"], "on_error": "drop"}`))
	require.NoError(t, err)

	event := event.Event{
		Payload: map[string]interface{}{
			"user": map[string]interface{}{
				"email": "user@example.com",
				"name":  "John",
			},
		},
		Headers: map[string]string{},
	}

	result, err := trans.Execute(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, event, result)
}

func TestSchemaTransform(t *testing.T) {
	// Create a temporary schema file
	tmpDir := t.TempDir()
	schemaFile := filepath.Join(tmpDir, "test-schema.json")

	schema := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string", "format": "email"},
			"age": {"type": "integer", "minimum": 0}
		},
		"required": ["name", "email"]
	}`

	err := os.WriteFile(schemaFile, []byte(schema), 0644)
	require.NoError(t, err)

	tests := []struct {
		name      string
		config    string
		event     event.Event
		wantError bool
		wantDrop  bool
		checkDLQ  bool
	}{
		{
			name:   "valid event",
			config: `{"schema_file": "` + schemaFile + `", "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"name": "John", "email": "john@example.com"},
				Headers: map[string]string{},
			},
			wantDrop: false,
		},
		{
			name:   "invalid event missing required field",
			config: `{"schema_file": "` + schemaFile + `", "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"name": "John"},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
		{
			name:   "invalid type",
			config: `{"schema_file": "` + schemaFile + `", "on_error": "drop"}`,
			event: event.Event{
				Payload: map[string]interface{}{"name": "John", "email": "john@example.com", "age": "thirty"},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
		{
			name:   "invalid with dlq",
			config: `{"schema_file": "` + schemaFile + `", "on_error": "dlq"}`,
			event: event.Event{
				Payload: map[string]interface{}{"name": "John"},
				Headers: map[string]string{},
			},
			wantDrop: false,
			checkDLQ: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewSchemaTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkDLQ {
					ev, ok := result.(event.Event)
					require.True(t, ok, "result should be event.Event")
					assert.NotEmpty(t, ev.Headers["_dlq_reason"])
					assert.Equal(t, "validate.schema", ev.Headers["_dlq_transform"])
				}
			}
		})
	}
}

func TestSchemaTransformMissingFile(t *testing.T) {
	trans := NewSchemaTransform()
	err := trans.Configure(json.RawMessage(`{"schema_file": "/nonexistent/schema.json", "on_error": "drop"}`))
	assert.Error(t, err)
}

func TestRulesTransform(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		event     event.Event
		wantError bool
		wantDrop  bool
		checkDLQ  bool
	}{
		{
			name: "all rules pass",
			config: `{
				"rules": {
					"has_email": "email != \"\"",
					"valid_age": "age >= 18"
				},
				"on_error": "drop"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com", "age": 25},
				Headers: map[string]string{},
			},
			wantDrop: false,
		},
		{
			name: "rule fails with drop",
			config: `{
				"rules": {
					"has_email": "email != \"\"",
					"valid_age": "age >= 18"
				},
				"on_error": "drop"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com", "age": 15},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
		{
			name: "rule fails with dlq",
			config: `{
				"rules": {
					"has_email": "email != \"\"",
					"valid_age": "age >= 18"
				},
				"on_error": "dlq"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com", "age": 15},
				Headers: map[string]string{},
			},
			wantDrop: false,
			checkDLQ: true,
		},
		{
			name: "multiple rules fail",
			config: `{
				"rules": {
					"has_email": "email != \"\"",
					"valid_age": "age >= 18",
					"verified": "verified == true"
				},
				"on_error": "drop"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "", "age": 15},
				Headers: map[string]string{},
			},
			wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewRulesTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkDLQ {
					ev := result.(event.Event)
					assert.NotEmpty(t, ev.Headers["_dlq_reason"])
					assert.Equal(t, "validate.rules", ev.Headers["_dlq_transform"])
				}
			}
		})
	}
}

func TestRulesTransformWithComplexConditions(t *testing.T) {
	trans := NewRulesTransform()
	config := `{
		"rules": {
			"premium_active": "tier == \"gold\""
		},
		"on_error": "drop"
	}`
	err := trans.Configure(json.RawMessage(config))
	require.NoError(t, err)

	event := event.Event{
		Payload: map[string]interface{}{
			"tier":   "gold",
			"status": "active",
			"score":  75,
		},
		Headers: map[string]string{},
	}

	result, err := trans.Execute(context.Background(), event)
	assert.NoError(t, err)
	assert.Equal(t, event, result)
}

func TestConfigurationErrors(t *testing.T) {
	tests := []struct {
		name   string
		config string
		trans  func() interface {
			Configure(json.RawMessage) error
		}
		wantErr bool
	}{
		{
			name:   "required empty config",
			config: `{}`,
			trans: func() interface {
				Configure(json.RawMessage) error
			} {
				return NewRequiredTransform()
			},
			wantErr: true,
		},
		{
			name:   "required no fields",
			config: `{"fields": []}`,
			trans: func() interface {
				Configure(json.RawMessage) error
			} {
				return NewRequiredTransform()
			},
			wantErr: true,
		},
		{
			name:   "rules empty config",
			config: `{}`,
			trans: func() interface {
				Configure(json.RawMessage) error
			} {
				return NewRulesTransform()
			},
			wantErr: true,
		},
		{
			name:   "rules no rules",
			config: `{"rules": {}}`,
			trans: func() interface {
				Configure(json.RawMessage) error
			} {
				return NewRulesTransform()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := tt.trans()
			err := trans.Configure(json.RawMessage(tt.config))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
