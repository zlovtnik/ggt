package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// SplitConfig configures filter.split for expanding array fields into individual events.
type SplitConfig struct {
	Field     string `json:"field"`
	Target    string `json:"target,omitempty"`
	DropEmpty bool   `json:"drop_empty"`
}

type splitTransform struct {
	cfg SplitConfig
}

func (s *splitTransform) Name() string { return "filter.split" }

func (s *splitTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("split configuration required")
	}
	if err := json.Unmarshal(raw, &s.cfg); err != nil {
		return fmt.Errorf("failed to decode split config: %w", err)
	}
	if s.cfg.Field == "" {
		return fmt.Errorf("field path is required")
	}
	return nil
}

func (s *splitTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	value, exists := ev.GetField(s.cfg.Field)
	if !exists || value == nil {
		if s.cfg.DropEmpty {
			return nil, transform.ErrDrop
		}
		return nil, nil
	}

	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil, fmt.Errorf("field %s is not an array (got %T)", s.cfg.Field, value)
	}

	length := val.Len()
	if length == 0 {
		if s.cfg.DropEmpty {
			return nil, transform.ErrDrop
		}
		return []event.Event{}, nil
	}

	results := make([]event.Event, 0, length)
	targetField := s.cfg.Target
	if targetField == "" {
		targetField = s.cfg.Field
	}

	for i := 0; i < length; i++ {
		item := val.Index(i).Interface()
		newEvent := ev.RemoveField(s.cfg.Field)
		newEvent = newEvent.SetField(targetField, item)
		if newEvent.Headers == nil {
			newEvent.Headers = make(map[string]string)
		}
		newEvent.Headers["_split_index"] = fmt.Sprintf("%d", i)
		results = append(results, newEvent)
	}

	return results, nil
}

// NewSplitTransform constructs filter.split.
func NewSplitTransform() transform.Transform {
	return &splitTransform{}
}
