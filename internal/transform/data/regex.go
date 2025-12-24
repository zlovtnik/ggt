package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type RegexTransform struct {
	sourceField string
	targetField string
	pattern     *regexp.Regexp
	replacement string
}

type RegexConfig struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field,omitempty"`
	Pattern     string `json:"pattern"`
	Replacement string `json:"replacement"`
}

func (t *RegexTransform) Name() string {
	return "data.regex"
}

func (t *RegexTransform) Configure(configRaw json.RawMessage) error {
	var config RegexConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return err
	}
	if config.SourceField == "" {
		return fmt.Errorf("source_field is required")
	}
	if config.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	pattern, err := regexp.Compile(config.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	t.sourceField = config.SourceField
	t.targetField = config.TargetField
	if t.targetField == "" {
		t.targetField = t.sourceField
	}
	t.pattern = pattern
	t.replacement = config.Replacement
	return nil
}

func (t *RegexTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	evt, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("expected event.Event, got %T", e)
	}

	val, exists := evt.GetField(t.sourceField)
	if !exists {
		return nil, fmt.Errorf("field %s does not exist", t.sourceField)
	}

	str, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("field %s must be a string, got %T", t.sourceField, val)
	}

	result := t.pattern.ReplaceAllString(str, t.replacement)
	return evt.SetField(t.targetField, result), nil
}

func init() {
	transform.Register("data.regex", func(raw json.RawMessage) (transform.Transform, error) {
		t := &RegexTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
