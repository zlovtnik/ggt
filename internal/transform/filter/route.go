package filter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

const (
	// maxTopicNameLength is the maximum allowed length for Kafka topic names
	maxTopicNameLength = 249
)

// validateTopicCharacters checks for invalid characters in topic names
func validateTopicCharacters(name string) error {
	for _, char := range name {
		if char == '\x00' || char == '\n' || char == '\r' {
			return fmt.Errorf("topic name contains invalid character")
		}
	}
	return nil
}

// RouteConfig represents the configuration for filter.route transform
type RouteConfig struct {
	Conditions []ConditionRoute `json:"conditions"` // ordered list of condition -> topic mappings
	Default    string           `json:"default"`    // fallback topic if no conditions match
}

// ConditionRoute represents a single condition-to-topic routing rule
type ConditionRoute struct {
	Condition string `json:"condition"` // condition expression
	Topic     string `json:"topic"`     // target topic name
}

// routeTransform implements filter.route for dynamic topic routing
type routeTransform struct {
	cfg RouteConfig
}

func (r *routeTransform) Name() string { return "filter.route" }

func (r *routeTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("route conditions required")
	}
	if err := json.Unmarshal(raw, &r.cfg); err != nil {
		return err
	}
	if len(r.cfg.Conditions) == 0 && r.cfg.Default == "" {
		return fmt.Errorf("at least one condition or a default topic is required")
	}

	// Validate default topic if specified
	if r.cfg.Default != "" {
		if len(r.cfg.Default) > maxTopicNameLength {
			return fmt.Errorf("default topic name too long (max %d characters)", maxTopicNameLength)
		}
		if r.cfg.Default[0] == '.' || r.cfg.Default[len(r.cfg.Default)-1] == '.' {
			return fmt.Errorf("default topic name cannot start or end with a dot")
		}
		if err := validateTopicCharacters(r.cfg.Default); err != nil {
			return fmt.Errorf("default topic: %w", err)
		}
	}

	// Validate all condition expressions
	for i, entry := range r.cfg.Conditions {
		if entry.Condition == "" {
			return fmt.Errorf("condition %d: expression cannot be empty", i)
		}
		if entry.Topic == "" {
			return fmt.Errorf("condition %d: topic cannot be empty", i)
		}
		// Validate topic name (basic Kafka topic name validation)
		if len(entry.Topic) > maxTopicNameLength {
			return fmt.Errorf("condition %d: topic name too long (max %d characters)", i, maxTopicNameLength)
		}
		if entry.Topic[0] == '.' || entry.Topic[len(entry.Topic)-1] == '.' {
			return fmt.Errorf("condition %d: topic name cannot start or end with a dot", i)
		}
		if err := validateTopicCharacters(entry.Topic); err != nil {
			return fmt.Errorf("condition %d: %w", i, err)
		}
		// Validate the condition expression by parsing it
		if _, err := ParseCondition(entry.Condition); err != nil {
			return fmt.Errorf("condition %d: invalid expression '%s': %w", i, entry.Condition, err)
		}
	}

	return nil
}

func (r *routeTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Initialize Headers if nil to prevent panic on assignment
	if ev.Headers == nil {
		ev.Headers = make(map[string]string)
	}

	// Evaluate conditions in order to find matching route
	for _, entry := range r.cfg.Conditions {
		cond, err := ParseCondition(entry.Condition)
		if err != nil {
			return nil, fmt.Errorf("condition parsing failed: %w", err)
		}
		match, err := cond.Evaluate(ev)
		if err != nil {
			return nil, fmt.Errorf("condition evaluation failed: %w", err)
		}
		if match {
			ev.Headers["_route_target"] = entry.Topic
			return ev, nil
		}
	}

	// No conditions matched, use default if available
	if r.cfg.Default != "" {
		ev.Headers["_route_target"] = r.cfg.Default
		return ev, nil
	}

	// No matching route and no default
	return nil, fmt.Errorf("no matching route found and no default topic specified")
}

// NewRouteTransform creates a new route filter transform
func NewRouteTransform() transform.Transform {
	return &routeTransform{}
}
