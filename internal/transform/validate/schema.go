package validate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// SchemaConfig represents the configuration for validate.schema transform
type SchemaConfig struct {
	SchemaFile string `json:"schema_file"`
	OnError    string `json:"on_error"` // "drop", "dlq", "fail"
}

// schemaTransform implements validate.schema using JSON Schema validation
type schemaTransform struct {
	cfg    SchemaConfig
	schema *gojsonschema.Schema
}

func (s *schemaTransform) Name() string { return "validate.schema" }

func (s *schemaTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("schema validation configuration required")
	}
	if err := json.Unmarshal(raw, &s.cfg); err != nil {
		return err
	}
	if s.cfg.SchemaFile == "" {
		return fmt.Errorf("schema_file is required")
	}
	if s.cfg.OnError == "" {
		s.cfg.OnError = "drop" // default behavior
	}
	if s.cfg.OnError != "drop" && s.cfg.OnError != "dlq" && s.cfg.OnError != "fail" {
		return fmt.Errorf("on_error must be 'drop', 'dlq', or 'fail'")
	}

	// Load and compile the JSON schema
	schemaData, err := os.ReadFile(s.cfg.SchemaFile)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	var schemaJSON interface{}
	if err := json.Unmarshal(schemaData, &schemaJSON); err != nil {
		return fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	schema, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(schemaJSON))
	if err != nil {
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	s.schema = schema
	return nil
}

func (s *schemaTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Validate the event payload against the schema
	documentLoader := gojsonschema.NewGoLoader(ev.Payload)
	result, err := s.schema.Validate(documentLoader)
	if err != nil {
		return nil, fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var sb strings.Builder
		for _, err := range result.Errors() {
			sb.WriteString(err.String())
			sb.WriteString("; ")
		}
		errMessages := sb.String()

		switch s.cfg.OnError {
		case "drop":
			return nil, transform.ErrDrop
		case "dlq":
			if ev.Headers == nil {
				ev.Headers = make(map[string]string)
			}
			ev.Headers["_dlq_reason"] = errMessages
			ev.Headers["_dlq_transform"] = "validate.schema"
			return ev, nil
		case "fail":
			return nil, fmt.Errorf("schema validation failed: %s", errMessages)
		default:
			return nil, fmt.Errorf("invalid on_error value: %s", s.cfg.OnError)
		}
	}

	return ev, nil
}

// NewSchemaTransform creates a new JSON schema validator transform
func NewSchemaTransform() transform.Transform {
	return &schemaTransform{}
}
