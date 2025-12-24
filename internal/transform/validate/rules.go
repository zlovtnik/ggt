package validate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/internal/transform/filter"
	"github.com/zlovtnik/ggt/pkg/event"
)

// RulesConfig represents the configuration for validate.rules transform
type RulesConfig struct {
	Rules   map[string]string `json:"rules"`    // rule_name -> condition expression
	OnError string            `json:"on_error"` // "drop", "dlq", "fail"
}

// rulesTransform implements validate.rules with custom rule expressions
type rulesTransform struct {
	cfg   RulesConfig
	rules map[string]filter.Condition
}

func (r *rulesTransform) Name() string { return "validate.rules" }

func (r *rulesTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("rules validation configuration required")
	}
	if err := json.Unmarshal(raw, &r.cfg); err != nil {
		return err
	}
	if len(r.cfg.Rules) == 0 {
		return fmt.Errorf("at least one rule must be specified")
	}
	if r.cfg.OnError == "" {
		r.cfg.OnError = "drop" // default behavior
	}
	if r.cfg.OnError != "drop" && r.cfg.OnError != "dlq" && r.cfg.OnError != "fail" {
		return fmt.Errorf("on_error must be 'drop', 'dlq', or 'fail'")
	}

	// Parse and validate all rule expressions
	r.rules = make(map[string]filter.Condition)
	for ruleName, expr := range r.cfg.Rules {
		cond, err := filter.ParseCondition(expr)
		if err != nil {
			return fmt.Errorf("invalid rule '%s': %w", ruleName, err)
		}
		r.rules[ruleName] = cond
	}

	return nil
}

func (r *rulesTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	var failedRules []string
	for ruleName, condition := range r.rules {
		match, err := condition.Evaluate(ev)
		if err != nil {
			return nil, fmt.Errorf("rule evaluation failed for '%s': %w", ruleName, err)
		}
		if !match {
			failedRules = append(failedRules, ruleName)
		}
	}

	if len(failedRules) > 0 {
		errMsg := fmt.Sprintf("validation rules failed: %v", failedRules)

		switch r.cfg.OnError {
		case "drop":
			return nil, transform.ErrDrop
		case "dlq":
			if ev.Headers == nil {
				ev.Headers = make(map[string]string)
			}
			ev.Headers["_dlq_reason"] = errMsg
			ev.Headers["_dlq_transform"] = "validate.rules"
			return ev, nil
		case "fail":
			return nil, fmt.Errorf("%s", errMsg)
		}
	}

	return ev, nil
}

// NewRulesTransform creates a new custom rules validator transform
func NewRulesTransform() transform.Transform {
	return &rulesTransform{}
}
