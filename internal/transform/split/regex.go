package split

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// RegexConfig represents configuration for split.regex transform
type RegexConfig struct {
	Field   string `json:"field"`   // Path to string field to split
	Pattern string `json:"pattern"` // Regex pattern to split on
	Key     string `json:"key"`     // Optional key to store split element (defaults to "value")
}

// splitRegexTransform implements split.regex for expanding one message into many based on regex
type splitRegexTransform struct {
	cfg   RegexConfig
	regex *regexp.Regexp
}

func (s *splitRegexTransform) Name() string { return "split.regex" }

func (s *splitRegexTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("split regex configuration required")
	}
	if err := json.Unmarshal(raw, &s.cfg); err != nil {
		return err
	}
	if s.cfg.Field == "" {
		return fmt.Errorf("field path is required")
	}
	if s.cfg.Pattern == "" {
		return fmt.Errorf("pattern is required")
	}
	if s.cfg.Key == "" {
		s.cfg.Key = "value" // Default key
	}

	// Compile regex
	regex, err := regexp.Compile(s.cfg.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	s.regex = regex

	return nil
}

// Execute returns a slice of events - one per regex match
func (s *splitRegexTransform) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	// Get the string field from the event
	fieldVal, exists := ev.GetField(s.cfg.Field)
	if !exists {
		return nil, fmt.Errorf("failed to get field %s: field does not exist", s.cfg.Field)
	}

	// Handle nil value - return empty slice (no output messages)
	if fieldVal == nil {
		return []event.Event{}, nil
	}

	// Convert to string
	var str string
	switch v := fieldVal.(type) {
	case string:
		str = v
	default:
		return nil, fmt.Errorf("field %s is not a string (got %T)", s.cfg.Field, v)
	}

	// Find all matches
	matches := s.regex.FindAllString(str, -1)
	if len(matches) == 0 {
		return []event.Event{}, nil
	}

	// Create one event per match
	results := make([]event.Event, 0, len(matches))
	for _, match := range matches {
		// Clone the event for each match
		newEv := ev.Clone()

		// Set the key in the event
		newEv = newEv.SetField(s.cfg.Key, match)

		// Remove the original field to avoid duplication
		newEv = newEv.RemoveField(s.cfg.Field)

		results = append(results, newEv)
	}

	return results, nil
}

// NewSplitRegexTransform creates a new split.regex transform
func NewSplitRegexTransform() transform.Transform {
	return &splitRegexTransform{}
}
