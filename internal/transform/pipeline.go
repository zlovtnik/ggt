package transform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/pkg/event"
)

// Pipeline executes an ordered list of transforms.
type Pipeline struct {
	id         string
	transforms []Transform
}

// NewPipeline creates a pipeline from descriptors using the global registry.
func NewPipeline(id string, descriptors []config.TransformDescriptor) (*Pipeline, error) {
	// Validate inputs to avoid confusing runtime behavior.
	if id == "" {
		return nil, fmt.Errorf("id must be non-empty")
	}
	if descriptors == nil || len(descriptors) == 0 {
		return nil, fmt.Errorf("descriptors must be provided")
	}

	t := &Pipeline{id: id}
	for i, d := range descriptors {
		cfgBytes, err := json.Marshal(d.Config)
		if err != nil {
			return nil, fmt.Errorf("descriptor[%d]: failed to marshal config: %w", i, err)
		}
		tr, err := Create(d.Type, cfgBytes)
		if err != nil {
			return nil, fmt.Errorf("descriptor[%d]: %w", i, err)
		}
		t.transforms = append(t.transforms, tr)
	}
	return t, nil
}

// ID returns the pipeline identifier.
func (p *Pipeline) ID() string { return p.id }

// Execute runs the pipeline synchronously. If a transform returns ErrDrop,
// the pipeline halts and returns ErrDrop. The incoming event is cloned if it's
// an event.Event so transforms can mutate safely.
func (p *Pipeline) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	// clone event.Event values to preserve immutability
	if ev, ok := e.(event.Event); ok {
		e = ev.Clone()
	}

	// mark inflight (metrics hook could go here)
	// removed unused start variable and placeholder defer to eliminate dead code

	for idx, tr := range p.transforms {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// context used to attach transform index for downstream operations
		childCtx := context.WithValue(ctx, ctxKeyTransformIndex{}, idx)

		out, err := tr.Execute(childCtx, e)
		if err != nil {
			if errors.Is(err, ErrDrop) {
				return nil, ErrDrop
			}
			return nil, fmt.Errorf("transform %s failed: %w", tr.Name(), err)
		}
		e = out
	}

	return e, nil
}

// ctxKeyTransformIndex is a context key for transform index.
type ctxKeyTransformIndex struct{}
