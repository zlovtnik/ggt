package split

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// ArrayConfig represents configuration for split.array transform
type ArrayConfig struct {
	Field string `json:"field"` // Path to array field to split on
	Key   string `json:"key"`   // Optional key to store array element (defaults to "value")
}

// splitArrayTransform implements split.array for expanding one message into many
type splitArrayTransform struct {
	cfg ArrayConfig
}

func (s *splitArrayTransform) Name() string { return "split.array" }

func (s *splitArrayTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("split array configuration required")
	}
	if err := json.Unmarshal(raw, &s.cfg); err != nil {
		return err
	}
	if s.cfg.Field == "" {
		return fmt.Errorf("field path is required")
	}
	if s.cfg.Key == "" {
		s.cfg.Key = "value" // Default key
	}
	return nil
}

// Execute returns a slice of events - one per array element
func (s *splitArrayTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Get the array field from the event
	arrayVal, exists := ev.GetField(s.cfg.Field)
	if !exists {
		return nil, fmt.Errorf("failed to get field %s: field does not exist", s.cfg.Field)
	}

	// Handle nil value - return empty slice (no output messages)
	if arrayVal == nil {
		return []event.Event{}, nil
	}

	// Convert to slice
	var items []interface{}
	switch v := arrayVal.(type) {
	case []interface{}:
		items = v
	default:
		return nil, fmt.Errorf("field %s is not an array (got %T)", s.cfg.Field, v)
	}

	// Create one event per array element
	results := make([]event.Event, 0, len(items))
	for _, item := range items {
		// Clone the event for each array element
		newEv := ev.Clone()

		// Set the key in the event
		newEv = newEv.SetField(s.cfg.Key, item)

		// Remove the original array field to avoid duplication
		newEv = newEv.RemoveField(s.cfg.Field)

		results = append(results, newEv)
	}

	return results, nil
}

// NewSplitArrayTransform creates a new split.array transform
func NewSplitArrayTransform() transform.Transform {
	return &splitArrayTransform{}
}
