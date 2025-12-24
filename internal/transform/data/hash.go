package data

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
	"golang.org/x/crypto/bcrypt"
)

type HashTransform struct {
	sourceField string
	targetField string
	algorithm   string
}

type HashConfig struct {
	SourceField string `json:"source_field"`
	TargetField string `json:"target_field,omitempty"`
	Algorithm   string `json:"algorithm"` // "sha256", "bcrypt"
}

func (t *HashTransform) Name() string {
	return "data.hash"
}

func (t *HashTransform) Configure(configRaw json.RawMessage) error {
	var config HashConfig
	if err := json.Unmarshal(configRaw, &config); err != nil {
		return err
	}
	if config.SourceField == "" {
		return fmt.Errorf("source_field is required")
	}
	if config.Algorithm == "" {
		return fmt.Errorf("algorithm is required")
	}
	if config.Algorithm != "sha256" && config.Algorithm != "bcrypt" {
		return fmt.Errorf("algorithm must be 'sha256' or 'bcrypt', got %s", config.Algorithm)
	}
	t.sourceField = config.SourceField
	t.targetField = config.TargetField
	if t.targetField == "" {
		t.targetField = t.sourceField
	}
	t.algorithm = config.Algorithm
	return nil
}

func (t *HashTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
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
	switch t.algorithm {
	case "sha256":
		hash := sha256.Sum256([]byte(str))
		result = fmt.Sprintf("%x", hash)
	case "bcrypt":
		if len(str) > 72 {
			return nil, fmt.Errorf("bcrypt input exceeds 72 bytes (got %d bytes); longer inputs are truncated", len(str))
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(str), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to generate bcrypt hash: %w", err)
		}
		result = string(hash)
	}

	return evt.SetField(t.targetField, result), nil
}

func init() {
	transform.Register("data.hash", func(raw json.RawMessage) (transform.Transform, error) {
		t := &HashTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
