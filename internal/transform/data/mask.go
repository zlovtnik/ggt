package data

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type MaskTransform struct {
	sourceField string
	targetField string
	pattern     *regexp.Regexp
	keepPrefix  int
	maskChar    string
}

type MaskConfig struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field,omitempty"`
	Pattern     string `json:"pattern,omitempty"`     // regex pattern to match what to mask
	KeepPrefix  int    `json:"keep_prefix,omitempty"` // number of characters to keep at start
	MaskChar    string `json:"mask_char,omitempty"`   // character to use for masking (default "*")
}

func (t *MaskTransform) Name() string {
	return "data.mask"
}

func (t *MaskTransform) Configure(configRaw json.RawMessage) error {
	var config MaskConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return err
	}
	if config.SourceField == "" {
		return fmt.Errorf("source_field is required")
	}
	if config.Pattern == "" && config.KeepPrefix == 0 {
		return fmt.Errorf("either pattern or keep_prefix must be specified")
	}
	if config.MaskChar == "" {
		config.MaskChar = "*"
	}
	if utf8.RuneCountInString(config.MaskChar) != 1 {
		return fmt.Errorf("mask_char must be a single character")
	}

	t.sourceField = config.SourceField
	t.targetField = config.TargetField
	if t.targetField == "" {
		t.targetField = t.sourceField
	}
	t.keepPrefix = config.KeepPrefix
	t.maskChar = config.MaskChar

	if config.Pattern != "" {
		pattern, err := regexp.Compile(config.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern: %w", err)
		}
		t.pattern = pattern
	}

	return nil
}

func (t *MaskTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
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

	// Helper to safely keep prefix and mask remainder based on rune count
	maskString := func(s string) string {
		runes := []rune(s)
		if len(runes) <= t.keepPrefix {
			return s
		}
		prefix := string(runes[:t.keepPrefix])
		masked := strings.Repeat(t.maskChar, len(runes)-t.keepPrefix)
		return prefix + masked
	}

	var result string
	if t.pattern != nil {
		// Regex-based masking
		result = t.pattern.ReplaceAllStringFunc(str, func(match string) string {
			return maskString(match)
		})
	} else {
		// Simple prefix-based masking
		result = maskString(str)
	}

	return evt.SetField(t.targetField, result), nil
}

func init() {
	transform.Register("data.mask", func(raw json.RawMessage) (transform.Transform, error) {
		t := &MaskTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
