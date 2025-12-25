package filter

import (
	"encoding/json"

	"github.com/zlovtnik/ggt/internal/transform"
)

func init() {
	transform.Register("filter.condition", func(raw json.RawMessage) (transform.Transform, error) {
		t := &conditionTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("filter.route", func(raw json.RawMessage) (transform.Transform, error) {
		t := &routeTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("filter.split", func(raw json.RawMessage) (transform.Transform, error) {
		t := &splitTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
