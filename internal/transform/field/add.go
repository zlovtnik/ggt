package field

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type addConfig struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

type addTransform struct{ cfg addConfig }

func (a *addTransform) Name() string { return "field.add" }
func (a *addTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &a.cfg); err != nil {
		return err
	}
	if a.cfg.Field == "" {
		return fmt.Errorf("field name is required")
	}
	return nil
}

func (a *addTransform) Execute(_ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	return ev.SetField(a.cfg.Field, a.cfg.Value), nil
}

func init() {
	transform.Register("field.add", func(raw json.RawMessage) (transform.Transform, error) {
		t := &addTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
