package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// AggregateState holds running aggregate values for incremental computation
type AggregateState struct {
	Sum   float64
	Count int
	Min   *float64 // nil until first value
	Max   *float64 // nil until first value
}

// CountWindowState holds the state for a count-based aggregation window
type CountWindowState struct {
	StartTime  time.Time
	EventCount int                        // running count of events processed
	Aggregates map[string]*AggregateState // per-aggregation running state
	LastSeen   time.Time                  // last time this window was accessed
}

// CountConfig represents configuration for count-based aggregation
type CountConfig struct {
	KeyField     string        `json:"key_field"`
	Count        int           `json:"count"` // emit when window reaches this many events
	Aggregations []Aggregation `json:"aggregations"`
	IdleTimeout  time.Duration `json:"idle_timeout"` // optional, default 1h; windows idle longer than this are evicted
}

// CountTransform implements count-based aggregation
type CountTransform struct {
	config    CountConfig
	windows   map[string]*CountWindowState // keyed by window key
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

	// Validate aggregation functions
	validFunctions := map[string]bool{
		"sum":   true,
		"count": true,
		"avg":   true,
		"min":   true,
		"max":   true,
	}
	var invalidFunctions []string
	for i, agg := range c.config.Aggregations {
		normalizedFunc := strings.ToLower(agg.Function)
		if !validFunctions[normalizedFunc] {
			invalidFunctions = append(invalidFunctions, fmt.Sprintf("%q (%s)", agg.Function, agg.As))
		}
		// Normalize the function name to lowercase for consistent processing
		c.config.Aggregations[i].Function = normalizedFunc
	}
	if len(invalidFunctions) > 0 {
		return fmt.Errorf("invalid aggregation function(s): %s; valid functions are: sum, count, avg, min, max", strings.Join(invalidFunctions, ", "))
	}

	c.windows = make(map[string]*CountWindowState)

	if c.config.IdleTimeout == 0 {
		c.config.IdleTimeout = time.Hour
	}

	// Start background eviction to prevent unbounded memory growth
	go c.evictionLoop()

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

	// Acquire write lock for the entire operation to prevent race conditions
	c.windowMux.Lock()
	defer c.windowMux.Unlock()

	// Get or create window state
	state, exists := c.windows[windowKey]
	if !exists {
		state = &CountWindowState{
			StartTime:  ev.Timestamp,
			EventCount: 0,
			Aggregates: make(map[string]*AggregateState),
		}
		// Initialize aggregate states
		for _, agg := range c.config.Aggregations {
			state.Aggregates[agg.As] = &AggregateState{}
		}
		c.windows[windowKey] = state
	}

	state.LastSeen = time.Now()

	// Check if we need to emit after this event
	shouldEmit := state.EventCount+1 >= c.config.Count

	// Increment event count and update aggregations
	state.EventCount++
	c.updateAggregationsIncremental(state, ev)

	// If not emitting, we're done
	if !shouldEmit {
		return nil, nil
	}

	// We need to emit - create result event
	resultEvent := event.Event{
		Payload:   make(map[string]interface{}),
		Timestamp: ev.Timestamp,
	}

	// Add window metadata
	resultEvent = resultEvent.SetField("window_key", windowKey)
	resultEvent = resultEvent.SetField("window_start", state.StartTime)
	resultEvent = resultEvent.SetField("event_count", state.EventCount)

	// Add aggregations
	for _, agg := range c.config.Aggregations {
		if aggState, exists := state.Aggregates[agg.As]; exists {
			var value interface{}
			switch agg.Function {
			case "sum":
				value = aggState.Sum
			case "count":
				value = aggState.Count
			case "avg":
				if aggState.Count > 0 {
					value = aggState.Sum / float64(aggState.Count)
				} else {
					value = 0.0
				}
			case "min":
				if aggState.Min != nil {
					value = *aggState.Min
				} else {
					value = nil
				}
			case "max":
				if aggState.Max != nil {
					value = *aggState.Max
				} else {
					value = nil
				}
			}
			if value != nil {
				resultEvent = resultEvent.SetField(agg.As, value)
			}
		}
	}

	// Clean up window state
	delete(c.windows, windowKey)

	return resultEvent, nil
}

// updateAggregationsIncremental updates running aggregate state for a single event
func (c *CountTransform) updateAggregationsIncremental(state *CountWindowState, ev event.Event) {
	for _, agg := range c.config.Aggregations {
		// Get the field value
		fieldValue, exists := ev.GetField(agg.Field)
		if !exists {
			continue // Skip if field doesn't exist
		}

		// Convert to float64 for aggregation
		var numericValue float64
		switch v := fieldValue.(type) {
		case int:
			numericValue = float64(v)
		case int32:
			numericValue = float64(v)
		case int64:
			numericValue = float64(v)
		case float32:
			numericValue = float64(v)
		case float64:
			numericValue = v
		default:
			// Skip non-numeric values
			continue
		}

		// Get aggregate state (already initialized when window was created)
		aggState := state.Aggregates[agg.As]

		// Update running aggregates
		switch agg.Function {
		case "sum", "avg":
			aggState.Sum += numericValue
			aggState.Count++
		case "count":
			aggState.Count++
		case "min":
			if aggState.Min == nil || numericValue < *aggState.Min {
				minVal := numericValue
				aggState.Min = &minVal
			}
		case "max":
			if aggState.Max == nil || numericValue > *aggState.Max {
				maxVal := numericValue
				aggState.Max = &maxVal
			}
		}
	}
}

// evictionLoop periodically removes idle windows to prevent unbounded memory growth
func (c *CountTransform) evictionLoop() {
	ticker := time.NewTicker(c.config.IdleTimeout / 4) // check every 1/4 of idle timeout
	defer ticker.Stop()

	for range ticker.C {
		c.windowMux.Lock()
		now := time.Now()
		for key, state := range c.windows {
			if now.Sub(state.LastSeen) > c.config.IdleTimeout {
				delete(c.windows, key)
			}
		}
		c.windowMux.Unlock()
	}
}

// NewCountTransform creates a new count-based aggregation transform
func NewCountTransform() transform.Transform {
	return &CountTransform{}
}
