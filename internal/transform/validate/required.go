package validate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// RequiredConfig represents the configuration for validate.required transform
type RequiredConfig struct {
	Fields  []string `json:"fields"`
	OnError string   `json:"on_error"` // "drop", "dlq", "fail"
}

// requiredTransform implements validate.required
type requiredTransform struct {
	cfg RequiredConfig
}

func (r *requiredTransform) Name() string { return "validate.required" }

func (r *requiredTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("required fields configuration required")
	}
	if err := json.Unmarshal(raw, &r.cfg); err != nil {
		return err
	}
	if len(r.cfg.Fields) == 0 {
		return fmt.Errorf("at least one required field must be specified")
	}
	if r.cfg.OnError == "" {
		r.cfg.OnError = "drop" // default behavior
	}
	if r.cfg.OnError != "drop" && r.cfg.OnError != "dlq" && r.cfg.OnError != "fail" {
		return fmt.Errorf("on_error must be 'drop', 'dlq', or 'fail'")
	}
	return nil
}

func (r *requiredTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	var missingFields []string
	for _, field := range r.cfg.Fields {
		_, exists := ev.GetField(field)
		if !exists {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		errMsg := fmt.Sprintf("required fields missing: %v", missingFields)

		switch r.cfg.OnError {
		case "drop":
			return nil, transform.ErrDrop
		case "dlq":
			// Mark for DLQ
			if ev.Headers == nil {
				ev.Headers = make(map[string]string)
			}
			ev.Headers["_dlq_reason"] = errMsg
			ev.Headers["_dlq_transform"] = "validate.required"
			return ev, nil
		case "fail":
			return nil, fmt.Errorf("%s", errMsg)
		default:
			return nil, fmt.Errorf("invalid on_error value: %s", r.cfg.OnError)
		}
	}

	return ev, nil
}

// NewRequiredTransform creates a new required field validator transform
func NewRequiredTransform() transform.Transform {
	return &requiredTransform{}
}
