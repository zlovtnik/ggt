package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type convertConfig struct {
	Field  string `json:"field"`
	To     string `json:"to"`
	Format string `json:"format,omitempty"` // for timestamps
}

type convertTransform struct {
	cfg convertConfig
}

func (c *convertTransform) Name() string { return "data.convert" }

func (c *convertTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("config required")
	}
	return json.Unmarshal(raw, &c.cfg)
}

func (c *convertTransform) Execute(_ context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	val, exists := ev.GetField(c.cfg.Field)
	if !exists {
		return ev, nil // field not present, no change
	}

	converted, err := c.convertValue(val)
	if err != nil {
		return nil, fmt.Errorf("failed to convert field %s: %w", c.cfg.Field, err)
	}

	return ev.SetField(c.cfg.Field, converted), nil
}

func (c *convertTransform) convertValue(val interface{}) (interface{}, error) {
	switch c.cfg.To {
	case "string":
		return c.toString(val)
	case "int":
		return c.toInt(val)
	case "float":
		return c.toFloat(val)
	case "bool":
		return c.toBool(val)
	case "timestamp":
		return c.toTimestamp(val)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", c.cfg.To)
	}
}

func (c *convertTransform) toString(val interface{}) (string, error) {
	switch v := val.(type) {
	case string:
		return v, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil
	case float32, float64:
		return fmt.Sprintf("%g", v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case time.Time:
		if c.cfg.Format != "" {
			return v.Format(c.cfg.Format), nil
		}
		return v.Format(time.RFC3339), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

func (c *convertTransform) toInt(val interface{}) (int64, error) {
	switch v := val.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		if v > 1<<63-1 {
			return 0, fmt.Errorf("uint value %d overflows int64", v)
		}
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		if v > 1<<63-1 {
			return 0, fmt.Errorf("uint64 value %d overflows int64", v)
		}
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", v)
	}
}

func (c *convertTransform) toFloat(val interface{}) (float64, error) {
	switch v := val.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	case bool:
		if v {
			return 1.0, nil
		}
		return 0.0, nil
	default:
		return 0.0, fmt.Errorf("cannot convert %T to float", v)
	}
}

func (c *convertTransform) toBool(val interface{}) (bool, error) {
	switch v := val.(type) {
	case bool:
		return v, nil
	case int:
		return v != 0, nil
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case float32:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case string:
		return strconv.ParseBool(strings.ToLower(v))
	default:
		return false, fmt.Errorf("cannot convert %T to bool", v)
	}
}

func (c *convertTransform) toTimestamp(val interface{}) (time.Time, error) {
	switch v := val.(type) {
	case time.Time:
		return v, nil
	case string:
		if c.cfg.Format != "" {
			return time.Parse(c.cfg.Format, v)
		}
		// Try common formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02",
			time.Kitchen,
			time.Stamp,
		}
		for _, f := range formats {
			if t, err := time.Parse(f, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse timestamp: %s", v)
	case int64:
		// Assume unix timestamp
		return time.Unix(v, 0), nil
	case float64:
		// Assume unix timestamp with fractional seconds
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		return time.Unix(sec, nsec), nil
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to timestamp", v)
	}
}

func init() {
	transform.Register("data.convert", func(raw json.RawMessage) (transform.Transform, error) {
		t := &convertTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
