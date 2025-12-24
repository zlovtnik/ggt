package data

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type formatConfig struct {
	Field  string   `json:"field"`
	Format string   `json:"format"`
	Args   []string `json:"args"` // field names to use as args
}

type formatTransform struct {
	cfg formatConfig
}

func (f *formatTransform) Name() string { return "data.format" }

func (f *formatTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("config required")
	}
	if err := json.Unmarshal(raw, &f.cfg); err != nil {
		return err
	}
	if f.cfg.Field == "" {
		return fmt.Errorf("field name is required")
	}
	if f.cfg.Format == "" {
		return fmt.Errorf("format string is required")
	}
	if len(f.cfg.Args) == 0 {
		return fmt.Errorf("at least one argument field is required")
	}
	return nil
}

func (f *formatTransform) Execute(_ context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	args := make([]interface{}, len(f.cfg.Args))
	for i, field := range f.cfg.Args {
		val, exists := ev.GetField(field)
		if !exists {
			return nil, fmt.Errorf("field %s not found", field)
		}
		args[i] = val
	}

	formatted := fmt.Sprintf(f.cfg.Format, args...)

	return ev.SetField(f.cfg.Field, formatted), nil
}

func init() {
	transform.Register("data.format", func(raw json.RawMessage) (transform.Transform, error) {
		t := &formatTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
