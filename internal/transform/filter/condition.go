package filter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zlovtnik/ggt/internal/condition"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// ConditionConfig represents the configuration for filter.condition transform
type ConditionConfig struct {
	Expression string `json:"expression"`
}

// conditionTransform implements filter.condition
type conditionTransform struct {
	cfg       ConditionConfig
	condition condition.Condition
}

func (c *conditionTransform) Name() string { return "filter.condition" }

func (c *conditionTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("condition expression required")
	}
	if err := json.Unmarshal(raw, &c.cfg); err != nil {
		return err
	}
	if c.cfg.Expression == "" {
		return fmt.Errorf("expression cannot be empty")
	}
	// Parse and validate the condition expression
	cond, err := condition.Parse(c.cfg.Expression)
	if err != nil {
		return fmt.Errorf("invalid condition expression: %w", err)
	}
	c.condition = cond
	return nil
}

func (c *conditionTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	match, err := c.condition.Evaluate(ev)
	if err != nil {
		return nil, fmt.Errorf("condition evaluation failed: %w", err)
	}

	if !match {
		return nil, transform.ErrDrop
	}

	return ev, nil
}

// NewConditionTransform creates a new condition filter transform
func NewConditionTransform() transform.Transform {
	return &conditionTransform{}
}
