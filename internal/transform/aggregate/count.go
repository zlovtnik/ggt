package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// CountConfig represents configuration for count-based aggregation
type CountConfig struct {
	KeyField     string        `json:"key_field"`
	Count        int           `json:"count"` // emit when window reaches this many events
	Aggregations []Aggregation `json:"aggregations"`
}

// CountTransform implements count-based aggregation
type CountTransform struct {
	config    CountConfig
	windows   map[string]*WindowState // keyed by window key
	windowMux sync.RWMutex
}

func (c *CountTransform) Name() string { return "aggregate.count" }

func (c *CountTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("count aggregation configuration required")
	}
	if err := json.Unmarshal(raw, &c.config); err != nil {
		return err
	}

	if c.config.KeyField == "" {
		return fmt.Errorf("key_field is required")
	}
	if c.config.Count <= 0 {
		return fmt.Errorf("count must be positive")
	}
	if len(c.config.Aggregations) == 0 {
		return fmt.Errorf("at least one aggregation is required")
	}

	c.windows = make(map[string]*WindowState)
	return nil
}

// Execute processes an event and returns aggregated results when count threshold is reached
func (c *CountTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Extract window key
	keyValue, exists := ev.GetField(c.config.KeyField)
	if !exists {
		return nil, fmt.Errorf("key field %s not found in event", c.config.KeyField)
	}
	windowKey := fmt.Sprintf("%v", keyValue)

	c.windowMux.Lock()
	defer c.windowMux.Unlock()

	// Get or create window state
	state, exists := c.windows[windowKey]
	if !exists {
		state = &WindowState{
			StartTime:  ev.Timestamp,
			Events:     make([]event.Event, 0),
			Aggregates: make(map[string]interface{}),
		}
		c.windows[windowKey] = state
	}

	// Add event to window
	state.Events = append(state.Events, ev)

	// Update aggregations
	if err := c.updateAggregations(state); err != nil {
		return nil, fmt.Errorf("failed to update aggregations: %w", err)
	}

	// Check if we should emit results
	if len(state.Events) >= c.config.Count {
		// Create result event
		resultEvent := event.Event{
			Payload:   make(map[string]interface{}),
			Timestamp: ev.Timestamp,
		}

		// Add window metadata
		resultEvent = resultEvent.SetField("window_key", windowKey)
		resultEvent = resultEvent.SetField("window_start", state.StartTime)
		resultEvent = resultEvent.SetField("event_count", len(state.Events))

		// Add aggregations
		for _, agg := range c.config.Aggregations {
			if value, exists := state.Aggregates[agg.As]; exists {
				resultEvent = resultEvent.SetField(agg.As, value)
			}
		}

		// Clean up window state - start fresh for next window
		delete(c.windows, windowKey)

		// Return the aggregated result
		return resultEvent, nil
	}

	// Not enough events yet, drop this event
	return nil, transform.ErrDrop
}

// updateAggregations updates the aggregate values for a window
func (c *CountTransform) updateAggregations(state *WindowState) error {
	for _, agg := range c.config.Aggregations {
		values := make([]float64, 0, len(state.Events))

		// Extract numeric values from the specified field
		for _, ev := range state.Events {
			if val, exists := ev.GetField(agg.Field); exists {
				if num, ok := val.(float64); ok {
					values = append(values, num)
				} else if num, ok := val.(int); ok {
					values = append(values, float64(num))
				}
			}
		}

		if len(values) == 0 {
			continue
		}

		switch agg.Function {
		case "sum":
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			state.Aggregates[agg.As] = sum
		case "count":
			state.Aggregates[agg.As] = len(values)
		case "avg":
			sum := 0.0
			for _, v := range values {
				sum += v
			}
			state.Aggregates[agg.As] = sum / float64(len(values))
		case "min":
			min := values[0]
			for _, v := range values[1:] {
				if v < min {
					min = v
				}
			}
			state.Aggregates[agg.As] = min
		case "max":
			max := values[0]
			for _, v := range values[1:] {
				if v > max {
					max = v
				}
			}
			state.Aggregates[agg.As] = max
		default:
			return fmt.Errorf("unknown aggregation function: %s", agg.Function)
		}
	}
	return nil
}

// NewCountTransform creates a new count-based aggregation transform
func NewCountTransform() transform.Transform {
	return &CountTransform{}
}
