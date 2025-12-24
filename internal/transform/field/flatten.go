package field

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type flattenConfig struct{}

type flattenTransform struct{ cfg flattenConfig }

func (f *flattenTransform) Name() string { return "field.flatten" }
func (f *flattenTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &f.cfg)
}

func (f *flattenTransform) Execute(_ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected event type")
	}

	payload := ev.Payload
	flattened := flattenMap("", payload)
	cloned := ev.Clone()
	cloned.Payload = flattened
	return cloned, nil
}

func flattenMap(prefix string, m map[string]interface{}) map[string]interface{} {
	return flattenMapWithDepth(prefix, m, 0, 100) // max depth of 100
}

func flattenMapWithDepth(prefix string, m map[string]interface{}, depth, maxDepth int) map[string]interface{} {
	if depth > maxDepth {
		// Return the map as-is when max depth is reached
		return map[string]interface{}{prefix: m}
	}
	result := make(map[string]interface{})
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := v.(map[string]interface{}); ok {
			nestedFlat := flattenMapWithDepth(key, nested, depth+1, maxDepth)
			for nk, nv := range nestedFlat {
				result[nk] = nv
			}
		} else {
			result[key] = v
		}
	}
	return result
}

func init() {
	transform.Register("field.flatten", func(raw json.RawMessage) (transform.Transform, error) {
		t := &flattenTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
