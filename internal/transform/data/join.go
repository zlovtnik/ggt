package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type JoinTransform struct {
	sourceField string
	targetField string
	separator   string
}

type JoinConfig struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field,omitempty"`
	Separator   string `json:"separator"`
}

func (t *JoinTransform) Name() string {
	return "data.join"
}

func (t *JoinTransform) Configure(configRaw json.RawMessage) error {
	var config JoinConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return err
	}
	if config.SourceField == "" {
		return fmt.Errorf("source_field is required")
	}
	if config.Separator == "" {
		config.Separator = ","
	}
	t.sourceField = config.SourceField
	t.targetField = config.TargetField
	if t.targetField == "" {
		t.targetField = t.sourceField
	}
	t.separator = config.Separator
	return nil
}

func (t *JoinTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	evt, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("expected event.Event, got %T", e)
	}

	val, exists := evt.GetField(t.sourceField)
	if !exists {
		return nil, fmt.Errorf("field %s does not exist", t.sourceField)
	}

	var strSlice []string
	switch v := val.(type) {
	case []string:
		strSlice = v
	case []interface{}:
		for _, item := range v {
			strSlice = append(strSlice, fmt.Sprintf("%v", item))
		}
	default:
		return nil, fmt.Errorf("field %s must be an array, got %T", t.sourceField, v)
	}

	joined := strings.Join(strSlice, t.separator)
	return evt.SetField(t.targetField, joined), nil
}

func init() {
	transform.Register("data.join", func(raw json.RawMessage) (transform.Transform, error) {
		t := &JoinTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
