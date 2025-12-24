package validate

import (
	"encoding/json"

	"github.com/zlovtnik/ggt/internal/transform"
)

func init() {
	transform.Register("validate.required", func(raw json.RawMessage) (transform.Transform, error) {
		t := &requiredTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("validate.schema", func(raw json.RawMessage) (transform.Transform, error) {
		t := &schemaTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("validate.rules", func(raw json.RawMessage) (transform.Transform, error) {
		t := &rulesTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
