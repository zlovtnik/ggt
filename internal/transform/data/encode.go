package data

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type EncodeTransform struct {
	sourceField string
	targetField string
	operation   string // "encode" or "decode"
	format      string // "base64", "url", "hex"
}

type EncodeConfig struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field,omitempty"`
	Operation   string `json:"operation"` // "encode" or "decode"
	Format      string `json:"format"`    // "base64", "url", "hex"
}

func (t *EncodeTransform) Name() string {
	return "data.encode"
}

func (t *EncodeTransform) Configure(configRaw json.RawMessage) error {
	var config EncodeConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return err
	}
	if config.SourceField == "" {
		return fmt.Errorf("source_field is required")
	}
	if config.Operation == "" {
		return fmt.Errorf("operation is required")
	}
	if config.Operation != "encode" && config.Operation != "decode" {
		return fmt.Errorf("operation must be 'encode' or 'decode', got %s", config.Operation)
	}
	if config.Format == "" {
		return fmt.Errorf("format is required")
	}
	if config.Format != "base64" && config.Format != "url" && config.Format != "hex" {
		return fmt.Errorf("format must be 'base64', 'url', or 'hex', got %s", config.Format)
	}
	t.sourceField = config.SourceField
	t.targetField = config.TargetField
	if t.targetField == "" {
		t.targetField = t.sourceField
	}
	t.operation = config.Operation
	t.format = config.Format
	return nil
}

func (t *EncodeTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
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

	var result string
	switch t.format {
	case "base64":
		if t.operation == "encode" {
			result = base64.StdEncoding.EncodeToString([]byte(str))
		} else {
			data, err := base64.StdEncoding.DecodeString(str)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64: %w", err)
			}
			result = string(data)
		}
	case "url":
		if t.operation == "encode" {
			result = url.QueryEscape(str)
		} else {
			decoded, err := url.QueryUnescape(str)
			if err != nil {
				return nil, fmt.Errorf("failed to decode URL: %w", err)
			}
			result = decoded
		}
	case "hex":
		if t.operation == "encode" {
			result = hex.EncodeToString([]byte(str))
		} else {
			data, err := hex.DecodeString(str)
			if err != nil {
				return nil, fmt.Errorf("failed to decode hex: %w", err)
			}
			result = string(data)
		}
	}

	return evt.SetField(t.targetField, result), nil
}

func init() {
	transform.Register("data.encode", func(raw json.RawMessage) (transform.Transform, error) {
		t := &EncodeTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
