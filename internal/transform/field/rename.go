package field

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

type renameConfig struct {
	Mappings map[string]string `json:"mappings"`
}

type renameTransform struct {
	cfg renameConfig
}

func (r *renameTransform) Name() string { return "field.rename" }
func (r *renameTransform) Configure(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &r.cfg)
}

func (r *renameTransform) Execute(_ctx context.Context, e interface{}) (interface{}, error) {
	ev, ok := e.(event.Event)
	if !ok {
		return nil, fmt.Errorf("unexpected payload type")
	}

	out := ev
	// Check for conflicts before applying any renames
	// First, collect sources per target
	targetToSources := make(map[string][]string)
	for from, to := range r.cfg.Mappings {
		targetToSources[to] = append(targetToSources[to], from)
	}
	// Then, check for conflicts
	for target, sources := range targetToSources {
		var existingSources []string
		for _, source := range sources {
			if _, exists := out.GetField(source); exists {
				existingSources = append(existingSources, source)
			}
		}
		if len(existingSources) > 1 {
			return nil, fmt.Errorf("field.rename: multiple source fields %v map to the same target %q, would cause overwrite", existingSources, target)
		}
		// Also check if target already exists and any source exists
		if _, targetExists := out.GetField(target); targetExists && len(existingSources) > 0 {
			return nil, fmt.Errorf("field.rename: target field %q already exists and source field(s) %v would overwrite it", target, existingSources)
		}
	}

	// Apply renames in a deterministic order
	keys := make([]string, 0, len(r.cfg.Mappings))
	for k := range r.cfg.Mappings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, from := range keys {
		to := r.cfg.Mappings[from]
		if v, ok := out.GetField(from); ok {
			out = out.SetField(to, v)
			out = out.RemoveField(from)
		}
	}
	return out, nil
}

func init() {
	transform.Register("field.rename", func(raw json.RawMessage) (transform.Transform, error) {
		t := &renameTransform{}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	})
}
