package filter

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/internal/condition"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestConditionTransform(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		event     event.Event
		wantError bool
		wantDrop  bool
	}{
		{
			name:   "simple equality",
			config: `{"expression": "status == \"active\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "active"},
			},
			wantDrop: false,
		},
		{
			name:   "equality mismatch",
			config: `{"expression": "status == \"active\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "inactive"},
			},
			wantDrop: true,
		},
		{
			name:   "not equal operator",
			config: `{"expression": "status != \"deleted\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "active"},
			},
			wantDrop: false,
		},
		{
			name:   "numeric comparison gt",
			config: `{"expression": "count > 10"}`,
			event: event.Event{
				Payload: map[string]interface{}{"count": 15},
			},
			wantDrop: false,
		},
		{
			name:   "numeric comparison lt",
			config: `{"expression": "count < 5"}`,
			event: event.Event{
				Payload: map[string]interface{}{"count": 15},
			},
			wantDrop: true,
		},
		{
			name:   "regex match",
			config: `{"expression": "email =~ \".*@example.com$\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
			},
			wantDrop: false,
		},
		{
			name:   "regex not match",
			config: `{"expression": "email !~ \".*@example.com$\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"email": "user@example.com"},
			},
			wantDrop: true,
		},
		{
			name:   "string contains",
			config: `{"expression": "description contains \"important\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"description": "This is an important message"},
			},
			wantDrop: false,
		},
		{
			name:   "string startswith",
			config: `{"expression": "code startswith \"ERR\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"code": "ERR_001"},
			},
			wantDrop: false,
		},
		{
			name:   "string endswith",
			config: `{"expression": "filename endswith \".log\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"filename": "debug.log"},
			},
			wantDrop: false,
		},
		{
			name:   "in operator",
			config: `{"expression": "env in \"dev,staging,prod\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"env": "staging"},
			},
			wantDrop: false,
		},
		{
			name:   "in operator not match",
			config: `{"expression": "env in \"dev,staging\""}`,
			event: event.Event{
				Payload: map[string]interface{}{"env": "prod"},
			},
			wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewConditionTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.event, result)
			}
		})
	}
}

func TestLogicalConditions(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		event    event.Event
		wantDrop bool
	}{
		{
			name:   "AND condition both true",
			config: `{"expression": "(status == \"active\") AND (count > 5)"}`,
			event: event.Event{
				Payload: map[string]interface{}{
					"status": "active",
					"count":  10,
				},
			},
			wantDrop: false,
		},
		{
			name:   "AND condition one false",
			config: `{"expression": "(status == \"active\") AND (count > 5)"}`,
			event: event.Event{
				Payload: map[string]interface{}{
					"status": "inactive",
					"count":  10,
				},
			},
			wantDrop: true,
		},
		{
			name:   "OR condition first true",
			config: `{"expression": "(status == \"active\") OR (status == \"pending\")"}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "active"},
			},
			wantDrop: false,
		},
		{
			name:   "OR condition second true",
			config: `{"expression": "(status == \"active\") OR (status == \"pending\")"}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "pending"},
			},
			wantDrop: false,
		},
		{
			name:   "OR condition both false",
			config: `{"expression": "(status == \"active\") OR (status == \"pending\")"}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "archived"},
			},
			wantDrop: true,
		},
		{
			name:   "NOT condition true",
			config: `{"expression": "NOT (status == \"deleted\")"}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "active"},
			},
			wantDrop: false,
		},
		{
			name:   "NOT condition false",
			config: `{"expression": "NOT (status == \"active\")"}`,
			event: event.Event{
				Payload: map[string]interface{}{"status": "active"},
			},
			wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewConditionTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			_, err = trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRouteTransform(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		event      event.Event
		wantError  bool
		wantTarget string
	}{
		{
			name: "route to topic based on condition - premium topic",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "premium-topic"},
					{"condition": "tier == \"silver\"", "topic": "standard-topic"}
				],
				"default": "other-topic"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"tier": "gold"},
				Headers: make(map[string]string),
			},
			wantTarget: "premium-topic",
		},
		{
			name: "route to topic based on condition - standard topic",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "premium-topic"},
					{"condition": "tier == \"silver\"", "topic": "standard-topic"}
				],
				"default": "other-topic"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"tier": "silver"},
				Headers: make(map[string]string),
			},
			wantTarget: "standard-topic",
		},
		{
			name: "route to default when no condition matches",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "premium-topic"},
					{"condition": "tier == \"silver\"", "topic": "standard-topic"}
				],
				"default": "other-topic"
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"tier": "bronze"},
				Headers: make(map[string]string),
			},
			wantTarget: "other-topic",
		},
		{
			name: "error when no match and no default",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "premium-topic"}
				]
			}`,
			event: event.Event{
				Payload: map[string]interface{}{"tier": "bronze"},
				Headers: make(map[string]string),
			},
			wantError: true,
		},
	}

	// Test topic validation
	validationTests := []struct {
		name       string
		config     string
		wantErrMsg string
	}{
		{
			name: "invalid topic name - starts with dot",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": ".invalid"}
				]
			}`,
			wantErrMsg: "topic name cannot start or end with a dot",
		},
		{
			name: "invalid topic name - ends with dot",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "invalid."}
				]
			}`,
			wantErrMsg: "topic name cannot start or end with a dot",
		},
		{
			name: "invalid topic name - too long",
			config: `{
				"conditions": [
					{"condition": "tier == \"gold\"", "topic": "very_long_topic_name_that_exceeds_the_maximum_allowed_length_of_two_hundred_and_forty_nine_characters_and_should_fail_validation_1234567890_abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyz_abcdefghijklmnopqrstuvwxyzxxx"}
				]
			}`,
			wantErrMsg: "topic name too long",
		},
		{
			name: "invalid default topic - starts with dot",
			config: `{
				"conditions": [],
				"default": ".invalid"
			}`,
			wantErrMsg: "default topic name cannot start or end with a dot",
		},
	}

	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewRouteTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewRouteTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			result, err := trans.Execute(context.Background(), tt.event)

			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				ev, ok := result.(event.Event)
				require.True(t, ok, "result should be event.Event")
				assert.Equal(t, tt.wantTarget, ev.Headers["_route_target"])
			}
		})
	}
}

func TestConditionParser(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"simple eq", "field == \"value\"", false},
		{"simple ne", "field != \"value\"", false},
		{"numeric lt", "count < 10", false},
		{"numeric lte", "count <= 10", false},
		{"numeric gt", "count > 10", false},
		{"numeric gte", "count >= 10", false},
		{"regex match", "email =~ \".*@example.com\"", false},
		{"regex not match", "email !~ \".*@example.com\"", false},
		{"contains", "text contains \"keyword\"", false},
		{"startswith", "text startswith \"prefix\"", false},
		{"endswith", "text endswith \"suffix\"", false},
		{"in operator", "env in \"dev,prod\"", false},
		{"AND operator", "(a == \"x\") AND (b == \"y\")", false},
		{"OR operator", "(a == \"x\") OR (b == \"y\")", false},
		{"NOT operator", "NOT (a == \"x\")", false},
		{"invalid operator", "field ~~ \"value\"", true},
		{"empty expression", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := condition.Parse(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConditionParserIgnoresOperatorsInQuotedValues(t *testing.T) {
	cond, err := condition.Parse(`message contains "a == b"`)
	require.NoError(t, err)

	ev := event.Event{Payload: map[string]interface{}{"message": "value a == b here"}}
	match, err := cond.Evaluate(ev)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestConditionParserInOperatorQuotedValues(t *testing.T) {
	cond, err := condition.Parse(`value in "\"a\" , 'b', c"`)
	require.NoError(t, err)

	assert.True(t, evaluateCondition(t, cond, event.Event{Payload: map[string]interface{}{"value": "a"}}))
	assert.True(t, evaluateCondition(t, cond, event.Event{Payload: map[string]interface{}{"value": "b"}}))
	assert.True(t, evaluateCondition(t, cond, event.Event{Payload: map[string]interface{}{"value": "c"}}))
	assert.False(t, evaluateCondition(t, cond, event.Event{Payload: map[string]interface{}{"value": "d"}}))
}

func evaluateCondition(t *testing.T, cond condition.Condition, ev event.Event) bool {
	t.Helper()
	match, err := cond.Evaluate(ev)
	require.NoError(t, err)
	return match
}

func TestNestedFieldConditions(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		event    event.Event
		wantDrop bool
	}{
		{
			name:   "nested field equality",
			config: `{"expression": "user.status == \"active\""}`,
			event: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"status": "active",
					},
				},
			},
			wantDrop: false,
		},
		{
			name:   "deeply nested field",
			config: `{"expression": "user.profile.email contains \"@example.com\""}`,
			event: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"profile": map[string]interface{}{
							"email": "user@example.com",
						},
					},
				},
			},
			wantDrop: false,
		},
		{
			name:   "nonexistent nested field",
			config: `{"expression": "user.status == \"active\""}`,
			event: event.Event{
				Payload: map[string]interface{}{
					"user": map[string]interface{}{
						"name": "John",
					},
				},
			},
			wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trans := NewConditionTransform()
			err := trans.Configure(json.RawMessage(tt.config))
			require.NoError(t, err)

			_, err = trans.Execute(context.Background(), tt.event)

			if tt.wantDrop {
				assert.ErrorIs(t, err, transform.ErrDrop)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
