package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// WindowType defines the type of window
type WindowType string

const (
	WindowTypeTumbling WindowType = "tumbling"
	WindowTypeSliding  WindowType = "sliding"
	WindowTypeSession  WindowType = "session"
)

// WindowConfig represents configuration for windowed aggregation
type WindowConfig struct {
	KeyField     string        `json:"key_field"`
	MaxEvents    int           `json:"max_events"`   // emit when window reaches this many events
	MaxDuration  time.Duration `json:"max_duration"` // emit when window reaches this age
	Aggregations []Aggregation `json:"aggregations"`
}

// Aggregation defines an aggregation operation
type Aggregation struct {
	Field    string `json:"field"`
	Function string `json:"function"` // sum, count, avg, min, max
	As       string `json:"as"`       // output field name
}

// WindowState holds the state for a single window
type WindowState struct {
	StartTime  time.Time
	EndTime    time.Time
	Events     []event.Event
	Aggregates map[string]interface{}
}

// WindowTransform implements windowed aggregation
type WindowTransform struct {
	config    WindowConfig
	windows   map[string]*WindowState // keyed by window key
	windowMux sync.RWMutex
	timers    map[string]*time.Timer // keyed by window key
	timerMux  sync.Mutex
	emitFunc  func(ctx context.Context, events []event.Event) error
}

func (w *WindowTransform) Name() string { return "aggregate.window" }

func (w *WindowTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("window aggregation configuration required")
	}
	if err := json.Unmarshal(raw, &w.config); err != nil {
		return err
	}

	if w.config.KeyField == "" {
		return fmt.Errorf("key_field is required")
	}
	if w.config.MaxEvents <= 0 && w.config.MaxDuration <= 0 {
		return fmt.Errorf("either max_events or max_duration must be positive")
	}
	if len(w.config.Aggregations) == 0 {
		return fmt.Errorf("at least one aggregation is required")
	}

	w.windows = make(map[string]*WindowState)
	w.timers = make(map[string]*time.Timer)

	return nil
}

// Execute processes an event and returns aggregated results when windows complete
func (w *WindowTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Extract window key
	keyValue, exists := ev.GetField(w.config.KeyField)
	if !exists {
		return nil, fmt.Errorf("key field %s not found in event", w.config.KeyField)
	}
	windowKey := fmt.Sprintf("%v", keyValue)

	w.windowMux.Lock()
	defer w.windowMux.Unlock()

	// Get or create window state
	state, exists := w.windows[windowKey]
	if !exists {
		now := time.Now()
		state = &WindowState{
			StartTime:  now,
			EndTime:    now.Add(w.config.MaxDuration),
			Events:     make([]event.Event, 0),
			Aggregates: make(map[string]interface{}),
		}
		w.windows[windowKey] = state

		// Set up timer for window expiration if duration is configured
		if w.config.MaxDuration > 0 {
			w.scheduleWindowTimeout(ctx, windowKey, w.config.MaxDuration)
		}
	}

	// Add event to window
	state.Events = append(state.Events, ev)

	// Update aggregations
	if err := w.updateAggregations(state); err != nil {
		return nil, fmt.Errorf("failed to update aggregations: %w", err)
	}

	// For now, we'll return nil and emit results via timers
	// In a more sophisticated implementation, we might emit partial results
	return nil, transform.ErrDrop
}

// scheduleWindowTimeout sets up a timer to emit window results
func (w *WindowTransform) scheduleWindowTimeout(ctx context.Context, windowKey string, duration time.Duration) {
	w.timerMux.Lock()
	defer w.timerMux.Unlock()

	if timer, exists := w.timers[windowKey]; exists {
		timer.Stop()
	}

	w.timers[windowKey] = time.AfterFunc(duration, func() {
		w.emitWindowResults(ctx, windowKey)
	})
}

// emitWindowResults emits the aggregated results for a completed window
func (w *WindowTransform) emitWindowResults(ctx context.Context, windowKey string) {
	w.windowMux.Lock()
	state, exists := w.windows[windowKey]
	if !exists {
		w.windowMux.Unlock()
		return
	}

	// Create result event
	resultEvent := event.Event{
		Payload:   make(map[string]interface{}),
		Timestamp: time.Now(),
	}

	// Add window metadata
	resultEvent = resultEvent.SetField("window_key", windowKey)
	resultEvent = resultEvent.SetField("window_start", state.StartTime)
	resultEvent = resultEvent.SetField("window_end", state.EndTime)
	resultEvent = resultEvent.SetField("event_count", len(state.Events))

	// Add aggregations
	for _, agg := range w.config.Aggregations {
		if value, exists := state.Aggregates[agg.As]; exists {
			resultEvent = resultEvent.SetField(agg.As, value)
		}
	}

	// Clean up window state
	delete(w.windows, windowKey)

	w.timerMux.Lock()
	if timer, exists := w.timers[windowKey]; exists {
		timer.Stop()
		delete(w.timers, windowKey)
	}
	w.timerMux.Unlock()

	w.windowMux.Unlock()

	// Emit the result (this would need to be integrated with the pipeline system)
	// For now, we'll need to modify the pipeline to handle aggregation transforms differently
	if w.emitFunc != nil {
		w.emitFunc(ctx, []event.Event{resultEvent})
	}
}

// updateAggregations updates the aggregate values for a window
func (w *WindowTransform) updateAggregations(state *WindowState) error {
	for _, agg := range w.config.Aggregations {
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

// NewWindowTransform creates a new windowed aggregation transform
func NewWindowTransform() transform.Transform {
	return &WindowTransform{}
}
