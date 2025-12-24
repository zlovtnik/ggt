package split

import (
	"encoding/json"

	"github.com/zlovtnik/ggt/internal/transform"
)

func init() {
	transform.Register("split.array", func(raw json.RawMessage) (transform.Transform, error) {
		t := &splitArrayTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})

	transform.Register("split.regex", func(raw json.RawMessage) (transform.Transform, error) {
		t := &splitRegexTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
