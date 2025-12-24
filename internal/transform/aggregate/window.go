package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
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

// AggregationState holds running state for an aggregation
type AggregationState struct {
	Count int     `json:"count"`
	Sum   float64 `json:"sum"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// WindowState holds the state for a single window
type WindowState struct {
	StartTime  time.Time
	EndTime    time.Time
	Events     []event.Event
	Aggregates map[string]*AggregationState
}

// WindowTransform implements windowed aggregation
type WindowTransform struct {
	config     WindowConfig
	windows    map[string]*WindowState // keyed by window key
	windowMux  sync.RWMutex
	scheduler  *cron.Cron             // cron scheduler for window expirations (deprecated, use timers)
	entries    map[string]*time.Timer // maps window keys to timers
	entriesMux sync.Mutex
	emitFunc   func(ctx context.Context, events []event.Event) error
	emitMux    sync.RWMutex
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

	// Validate aggregation functions
	validFunctions := map[string]bool{
		"sum":   true,
		"count": true,
		"avg":   true,
		"min":   true,
		"max":   true,
	}
	var invalidFunctions []string
	for i, agg := range w.config.Aggregations {
		normalizedFunc := strings.ToLower(agg.Function)
		if !validFunctions[normalizedFunc] {
			invalidFunctions = append(invalidFunctions, fmt.Sprintf("%q (%s)", agg.Function, agg.As))
		}
		// Normalize the function name to lowercase for consistent processing
		w.config.Aggregations[i].Function = normalizedFunc
	}
	if len(invalidFunctions) > 0 {
		return fmt.Errorf("invalid aggregation function(s): %s; valid functions are: sum, count, avg, min, max", strings.Join(invalidFunctions, ", "))
	}

	w.windows = make(map[string]*WindowState)
	w.entries = make(map[string]*time.Timer)
	w.scheduler = cron.New()
	w.scheduler.Start()

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

	// Only accept string keys for safety
	windowKey, ok := keyValue.(string)
	if !ok {
		return nil, fmt.Errorf("key field %s must be a string, got %T", w.config.KeyField, keyValue)
	}

	w.windowMux.Lock()

	// Get or create window state
	state, exists := w.windows[windowKey]
	if !exists {
		now := time.Now()
		state = &WindowState{
			StartTime:  now,
			EndTime:    now.Add(w.config.MaxDuration),
			Events:     make([]event.Event, 0),
			Aggregates: make(map[string]*AggregationState),
		}
		w.windows[windowKey] = state

		// Set up cron job for window expiration if duration is configured
		if w.config.MaxDuration > 0 {
			w.scheduleWindowTimeout(ctx, windowKey, w.config.MaxDuration)
		}
	}

	// Add event to window
	state.Events = append(state.Events, ev)

	// Update aggregations
	if err := w.updateAggregations(state); err != nil {
		w.windowMux.Unlock()
		return nil, fmt.Errorf("failed to update aggregations: %w", err)
	}

	// Check if window is full (MaxEvents enforcement)
	if w.config.MaxEvents > 0 && len(state.Events) >= w.config.MaxEvents {
		w.windowMux.Unlock()
		w.emitWindowResults(ctx, windowKey)
		return nil, transform.ErrDrop
	}

	w.windowMux.Unlock()

	// For now, we'll return nil and emit results via timers
	// In a more sophisticated implementation, we might emit partial results
	return nil, nil
}

// scheduleWindowTimeout schedules a one-shot timer to emit window results
func (w *WindowTransform) scheduleWindowTimeout(ctx context.Context, windowKey string, duration time.Duration) {
	w.entriesMux.Lock()
	defer w.entriesMux.Unlock()

	// Cancel and remove existing timer if present
	if timer, exists := w.entries[windowKey]; exists {
		timer.Stop()
		delete(w.entries, windowKey)
	}

	// Create a new timer that runs once after the duration
	timer := time.AfterFunc(duration, func() {
		// Check if the context is done before emitting to respect cancellation
		select {
		case <-ctx.Done():
			// Context was cancelled, skip emission
			return
		default:
			// Context is still active, proceed with emission
			w.emitWindowResults(ctx, windowKey)
		}
	})

	w.entries[windowKey] = timer
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
		if aggState, exists := state.Aggregates[agg.As]; exists && aggState.Count > 0 {
			var value interface{}
			switch agg.Function {
			case "count":
				value = aggState.Count
			case "sum":
				value = aggState.Sum
			case "avg":
				value = aggState.Sum / float64(aggState.Count)
			case "min":
				value = aggState.Min
			case "max":
				value = aggState.Max
			}
			resultEvent = resultEvent.SetField(agg.As, value)
		}
	}

	// Clean up window state
	delete(w.windows, windowKey)

	w.entriesMux.Lock()
	if timer, exists := w.entries[windowKey]; exists {
		timer.Stop()
		delete(w.entries, windowKey)
	}
	w.entriesMux.Unlock()

	w.windowMux.Unlock()

	// Emit the result (this would need to be integrated with the pipeline system)
	// For now, we'll need to modify the pipeline to handle aggregation transforms differently
	w.emitMux.RLock()
	emitFunc := w.emitFunc
	w.emitMux.RUnlock()
	if emitFunc != nil {
		emitFunc(ctx, []event.Event{resultEvent})
	}
}

// toFloat64 converts various numeric types to float64
func toFloat64(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", val)
	}
}

// updateAggregations updates the aggregate values for a window incrementally
func (w *WindowTransform) updateAggregations(state *WindowState) error {
	// Get the most recently added event
	if len(state.Events) == 0 {
		return nil
	}
	ev := state.Events[len(state.Events)-1]

	for _, agg := range w.config.Aggregations {
		val, exists := ev.GetField(agg.Field)
		if !exists {
			continue // Skip events without the field
		}

		num, err := toFloat64(val)
		if err != nil {
			continue // Skip non-numeric values
		}

		// Initialize aggregate state if missing
		if state.Aggregates[agg.As] == nil {
			state.Aggregates[agg.As] = &AggregationState{
				Count: 0,
				Sum:   0,
				Min:   num,
				Max:   num,
			}
		}
		aggState := state.Aggregates[agg.As]

		// Update based on function
		switch agg.Function {
		case "count":
			aggState.Count++
		case "sum":
			aggState.Sum += num
			aggState.Count++
		case "avg":
			aggState.Sum += num
			aggState.Count++
		case "min":
			if num < aggState.Min || aggState.Count == 0 {
				aggState.Min = num
			}
			aggState.Count++
		case "max":
			if num > aggState.Max || aggState.Count == 0 {
				aggState.Max = num
			}
			aggState.Count++
		default:
			return fmt.Errorf("unknown aggregation function: %s", agg.Function)
		}
	}
	return nil
}

// ForceEmitAllWindows synchronously emits all current windows (for testing)
func (w *WindowTransform) ForceEmitAllWindows(ctx context.Context) error {
	w.windowMux.Lock()
	defer w.windowMux.Unlock()

	var results []event.Event
	for windowKey, state := range w.windows {
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
			if aggState, exists := state.Aggregates[agg.As]; exists && aggState.Count > 0 {
				var value interface{}
				switch agg.Function {
				case "count":
					value = aggState.Count
				case "sum":
					value = aggState.Sum
				case "avg":
					value = aggState.Sum / float64(aggState.Count)
				case "min":
					value = aggState.Min
				case "max":
					value = aggState.Max
				}
				resultEvent = resultEvent.SetField(agg.As, value)
			}
		}

		results = append(results, resultEvent)
	}

	// Clear all windows
	w.windows = make(map[string]*WindowState)

	// Clear all timers
	w.entriesMux.Lock()
	for windowKey, timer := range w.entries {
		timer.Stop()
		delete(w.entries, windowKey)
	}
	w.entriesMux.Unlock()

	w.emitMux.RLock()
	emitFunc := w.emitFunc
	w.emitMux.RUnlock()
	if emitFunc != nil && len(results) > 0 {
		return emitFunc(ctx, results)
	}

	return nil
}

// SetEmitFunc sets the function to call when windows complete (for testing)
func (w *WindowTransform) SetEmitFunc(fn func(ctx context.Context, events []event.Event) error) {
	w.emitMux.Lock()
	w.emitFunc = fn
	w.emitMux.Unlock()
}

// Stop gracefully shuts down the window transform scheduler and timers
func (w *WindowTransform) Stop() {
	// Stop all timers
	w.entriesMux.Lock()
	for _, timer := range w.entries {
		timer.Stop()
	}
	w.entries = make(map[string]*time.Timer)
	w.entriesMux.Unlock()

	// Stop cron scheduler if used
	if w.scheduler != nil {
		ctx := w.scheduler.Stop()
		<-ctx.Done()
	}
}

// NewWindowTransform creates a new windowed aggregation transform
func NewWindowTransform() transform.Transform {
	return &WindowTransform{}
}
