package transform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrDrop is returned by a Transform to indicate the event should be filtered
// or dropped. Callers should use errors.Is(err, transform.ErrDrop) to detect it.
var ErrDrop = errors.New("transform: drop event")

// Transform is the interface implemented by all transforms.
// Implementations should be pure functions operating on an event payload.
type Transform interface {
	// Name returns the transform name used for logging/metrics.
	Name() string
	// Configure receives the raw config for the transform.
	Configure(cfg json.RawMessage) error
	// Execute performs the transform. It receives a context and an event.
	// It returns the transformed event(s) or an error.
	// A returned ErrDrop indicates the event should be filtered/dropped.
	// Returns can be: event.Event (single), []event.Event (multiple), or nil (dropped).
	Execute(ctx context.Context, e interface{}) (interface{}, error)
}

// TransformFuncAdapter adapts a simple function into a Transform.
type TransformFuncAdapter struct {
	name string
	fn   func(ctx context.Context, e interface{}) (interface{}, error)
}

func (t *TransformFuncAdapter) Name() string                      { return t.name }
func (t *TransformFuncAdapter) Configure(_ json.RawMessage) error { return nil }
func (t *TransformFuncAdapter) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	return t.fn(ctx, e)
}

// NewFunc creates a Transform from a name and function.
func NewFunc(name string, fn func(ctx context.Context, e interface{}) (interface{}, error)) Transform {
	if fn == nil {
		panic("transform: fn cannot be nil")
	}
	if name == "" {
		panic("transform: name cannot be empty")
	}
	return &TransformFuncAdapter{name: name, fn: fn}
}

// TransformT is a generic transform interface that provides compile-time
// typing for transform payloads. This is provided alongside the legacy
// non-generic `Transform` interface to allow incremental migration.
type TransformT[T any] interface {
	Name() string
	Configure(cfg json.RawMessage) error
	Execute(ctx context.Context, e T) (T, error)
}

// GenericToInterfaceAdapter adapts a generic TransformT[T] to the legacy
// Transform interface by performing runtime type assertions. Use this when
// registering generic transforms in places that expect the old interface.
type GenericToInterfaceAdapter[T any] struct {
	inner TransformT[T]
}

func (g *GenericToInterfaceAdapter[T]) Name() string { return g.inner.Name() }
func (g *GenericToInterfaceAdapter[T]) Configure(cfg json.RawMessage) error {
	return g.inner.Configure(cfg)
}
func (g *GenericToInterfaceAdapter[T]) Execute(ctx context.Context, e interface{}) (interface{}, error) {
	// Attempt to cast incoming payload to T
	te, ok := e.(T)
	if !ok {
		return nil, fmt.Errorf("transform: unexpected payload type for %s", g.inner.Name())
	}
	out, err := g.inner.Execute(ctx, te)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NewGenericFunc creates a generic TransformT[T] from a name and function,
// and returns it wrapped as a legacy Transform (so callers that expect the
// old interface can continue to operate). If you want the raw generic
// interface, create your own implementation of TransformT[T].
func NewGenericFunc[T any](name string, fn func(ctx context.Context, e T) (T, error)) Transform {
	if fn == nil {
		panic("transform: fn cannot be nil")
	}
	if name == "" {
		panic("transform: name cannot be empty")
	}
	// create a small TransformT[T] implementation
	t := &genericFuncImpl[T]{name: name, fn: fn}
	return &GenericToInterfaceAdapter[T]{inner: t}
}

type genericFuncImpl[T any] struct {
	name string
	fn   func(ctx context.Context, e T) (T, error)
}

func (g *genericFuncImpl[T]) Name() string                                { return g.name }
func (g *genericFuncImpl[T]) Configure(_ json.RawMessage) error           { return nil }
func (g *genericFuncImpl[T]) Execute(ctx context.Context, e T) (T, error) { return g.fn(ctx, e) }
