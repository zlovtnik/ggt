package field

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type removeConfig struct {
	Fields []string `json:"fields"`
}

type removeTransform struct{ cfg removeConfig }

func (r *removeTransform) Name() string { return "field.remove" }
func (r *removeTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &r.cfg)
}

func (r *removeTransform) Execute(_ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	out := ev
	for _, f := range r.cfg.Fields {
		out = out.RemoveField(f)
	}
	return out, nil
}

func init() {
	transform.Register("field.remove", func(raw json.RawMessage) (transform.Transform, error) {
		t := &removeTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
