package aggregate

import (
	"encoding/json"

	"github.com/zlovtnik/ggt/internal/transform"
)

func init() {
	transform.Register("aggregate.window", func(raw json.RawMessage) (transform.Transform, error) {
		t := &WindowTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("aggregate.count", func(raw json.RawMessage) (transform.Transform, error) {
		t := &CountTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
