package validate

import (
	"encoding/json"

	"github.com/zlovtnik/ggt/internal/transform"
)

func init() {
	register := func(name string, ctor func() transform.Transform) {
		transform.Register(name, func(raw json.RawMessage) (transform.Transform, error) {
			t := ctor()
			if err := t.Configure(raw); err != nil {
				return nil, err
			}
			return t, nil
		})
	}

	register("validate.required", func() transform.Transform { return &requiredTransform{} })
	register("validate.schema", func() transform.Transform { return &schemaTransform{} })
	register("validate.rules", func() transform.Transform { return &rulesTransform{} })
}
