package field

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type extractConfig struct {
	Source    string `json:"source"`
	Target    string `json:"target"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

type extractTransform struct{ cfg extractConfig }

func (e *extractTransform) Name() string { return "field.extract" }
func (e *extractTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("configuration required: source and target must be specified")
	}
	if err := json.Unmarshal(raw, &e.cfg); err != nil {
		return err
	}
	if e.cfg.Source == "" {
		return fmt.Errorf("source field must be specified")
	}
	if e.cfg.Target == "" {
		return fmt.Errorf("target field must be specified")
	}
	if e.cfg.Source == e.cfg.Target {
		return fmt.Errorf("source and target must be different: both are %q", e.cfg.Source)
	}
	return nil
}

func (e *extractTransform) Execute(_ctx context.Context, ev interface{}) (interface{}, error) {
	event, ok := ev.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	value, exists := event.GetField(e.cfg.Source)
	if !exists {
		return event, nil // source field not present, no-op
	}

	if !e.cfg.Overwrite {
		if _, exists := event.GetField(e.cfg.Target); exists {
			return nil, fmt.Errorf("target field %q already exists and overwrite is not enabled", e.cfg.Target)
		}
	}

	result := event.SetField(e.cfg.Target, value)
	result = result.RemoveField(e.cfg.Source)
	return result, nil
}

func init() {
	transform.Register("field.extract", func(raw json.RawMessage) (transform.Transform, error) {
		t := &extractTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
