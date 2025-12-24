package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type caseConfig struct {
	Field string `json:"field"`
	Case  string `json:"case"` // "upper", "lower", "title"
}

type caseTransform struct {
	cfg caseConfig
}

func (c *caseTransform) Name() string { return "data.case" }

func (c *caseTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("config required")
	}
	return json.Unmarshal(raw, &c.cfg)
}

func (c *caseTransform) Execute(_ context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	val, exists := ev.GetField(c.cfg.Field)
	if !exists {
		return ev, nil
	}

	str, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("field %s is not a string", c.cfg.Field)
	}

	var transformed string
	switch c.cfg.Case {
	case "upper":
		transformed = strings.ToUpper(str)
	case "lower":
		transformed = strings.ToLower(str)
	case "title":
		transformed = cases.Title(language.English).String(str)
	default:
		return nil, fmt.Errorf("unsupported case: %s", c.cfg.Case)
	}

	return ev.SetField(c.cfg.Field, transformed), nil
}

func init() {
	transform.Register("data.case", func(raw json.RawMessage) (transform.Transform, error) {
		t := &caseTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
