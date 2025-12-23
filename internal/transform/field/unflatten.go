package field

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type unflattenConfig struct{}

type unflattenTransform struct{ cfg unflattenConfig }

func (u *unflattenTransform) Name() string { return "field.unflatten" }
func (u *unflattenTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &u.cfg)
}

func (u *unflattenTransform) Execute(_ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	unflattened, err := unflattenMap(ev.Payload)
	if err != nil {
		return nil, fmt.Errorf("field.unflatten: %w", err)
	}
	newEv := ev
	newEv.Payload = unflattened
	return newEv, nil
}

func unflattenMap(m map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for k, v := range m {
		parts := strings.Split(k, ".")
		if err := setNested(result, parts, v); err != nil {
			return nil, fmt.Errorf("unflatten conflict at key %q: %w", k, err)
		}
	}
	return result, nil
}

func setNested(m map[string]interface{}, parts []string, value interface{}) error {
	if len(parts) == 1 {
		key := parts[0]
		if existing, exists := m[key]; exists {
			if _, isMap := existing.(map[string]interface{}); isMap {
				return fmt.Errorf("cannot set scalar value at %q, key already contains a nested map", key)
			}
			// Overwrite scalar with scalar is allowed, but since we're unflattening, this shouldn't happen
		}
		m[key] = value
		return nil
	}
	key := parts[0]
	if existing, exists := m[key]; exists {
		if _, isMap := existing.(map[string]interface{}); !isMap {
			return fmt.Errorf("cannot create nested structure at %q, key already contains a scalar value: %T", key, existing)
		}
	} else {
		m[key] = make(map[string]interface{})
	}
	nested := m[key].(map[string]interface{})
	return setNested(nested, parts[1:], value)
}

func init() {
	transform.Register("field.unflatten", func(raw json.RawMessage) (transform.Transform, error) {
		t := &unflattenTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
