package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type splitConfig struct {
	Field     string `json:"field"`
	Separator string `json:"separator"`
	Target    string `json:"target,omitempty"` // if empty, replace the field
}

type splitTransform struct {
	cfg splitConfig
}

func (s *splitTransform) Name() string { return "data.split" }

func (s *splitTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("config required")
	}
	if err := json.Unmarshal(raw, &s.cfg); err != nil {
		return err
	}
	if s.cfg.Field == "" {
		return fmt.Errorf("field is required")
	}
	if s.cfg.Separator == "" {
		return fmt.Errorf("separator is required")
	}
	return nil
}

func (s *splitTransform) Execute(_ context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	val, exists := ev.GetField(s.cfg.Field)
	if !exists {
		return ev, nil
	}

	str, ok := val.(string)
	if !ok {
		return nil, fmt.Errorf("field %s is not a string", s.cfg.Field)
	}

	parts := strings.Split(str, s.cfg.Separator)
	target := s.cfg.Target
	if target == "" {
		target = s.cfg.Field
	}

	return ev.SetField(target, parts), nil
}

func init() {
	transform.Register("data.split", func(raw json.RawMessage) (transform.Transform, error) {
		t := &splitTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
