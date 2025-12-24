package transform

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zlovtnik/ggt/pkg/event"
)

func TestPipeline_HappyPath(t *testing.T) {
	p := &Pipeline{
		id: "test-p",
		transforms: []Transform{
			NewFunc("set-foo", func(ctx context.Context, e interface{}) (interface{}, error) {
				ev, ok := e.(event.Event)
				if !ok {
					return nil, fmt.Errorf("expected event.Event, got %T", e)
				}
				return ev.SetField("foo", "bar"), nil
			}),
			NewFunc("set-n", func(ctx context.Context, e interface{}) (interface{}, error) {
				ev, ok := e.(event.Event)
				if !ok {
					return nil, fmt.Errorf("expected event.Event, got %T", e)
				}
				return ev.SetField("n", 42), nil
			}),
		},
	}

	in := event.Event{Payload: map[string]interface{}{}}
	out, err := p.Execute(context.Background(), in)
	require.NoError(t, err)

	ev, ok := out.(event.Event)
	require.True(t, ok, "expected event.Event")

	v, ok := ev.GetField("foo")
	require.True(t, ok)
	require.Equal(t, "bar", v)

	n, ok := ev.GetField("n")
	require.True(t, ok)
	require.Equal(t, 42, n)
}

func TestPipeline_Drop(t *testing.T) {
	p := &Pipeline{
		id: "drop-p",
		transforms: []Transform{
			NewFunc("dropper", func(ctx context.Context, e interface{}) (interface{}, error) {
				return nil, ErrDrop
			}),
		},
	}

	in := event.Event{Payload: map[string]interface{}{"x": 1}}
	_, err := p.Execute(context.Background(), in)
	require.True(t, errors.Is(err, ErrDrop))
}

func TestPipeline_DropWithNil(t *testing.T) {
	p := &Pipeline{
		id: "drop-nil-p",
		transforms: []Transform{
			NewFunc("dropper-nil", func(ctx context.Context, e interface{}) (interface{}, error) {
				return nil, nil // Returning nil also drops
			}),
		},
	}

	in := event.Event{Payload: map[string]interface{}{"x": 1}}
	_, err := p.Execute(context.Background(), in)
	require.True(t, errors.Is(err, ErrDrop))
}

func TestPipeline_ErrorPropagation(t *testing.T) {
	p := &Pipeline{
		id: "err-p",
		transforms: []Transform{
			NewFunc("boom", func(ctx context.Context, e interface{}) (interface{}, error) {
				return nil, fmt.Errorf("boom")
			}),
		},
	}

	in := event.Event{Payload: map[string]interface{}{}}
	_, err := p.Execute(context.Background(), in)
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestPipeline_MultipleEvents(t *testing.T) {
	p := &Pipeline{
		id: "multi-p",
		transforms: []Transform{
			NewFunc("splitter", func(ctx context.Context, e interface{}) (interface{}, error) {
				ev, ok := e.(event.Event)
				if !ok {
					return nil, fmt.Errorf("expected event.Event, got %T", e)
				}
				// Return multiple events
				event1 := ev.SetField("part", 1)
				event2 := ev.SetField("part", 2)
				return []event.Event{event1, event2}, nil
			}),
		},
	}

	in := event.Event{Payload: map[string]interface{}{"original": true}}
	out, err := p.Execute(context.Background(), in)
	require.NoError(t, err)

	events, ok := out.([]event.Event)
	require.True(t, ok, "expected []event.Event")
	require.Len(t, events, 2)

	// Check first event
	v1, ok := events[0].GetField("part")
	require.True(t, ok)
	require.Equal(t, 1, v1)
	orig1, ok := events[0].GetField("original")
	require.True(t, ok)
	require.Equal(t, true, orig1)

	// Check second event
	v2, ok := events[1].GetField("part")
	require.True(t, ok)
	require.Equal(t, 2, v2)
	orig2, ok := events[1].GetField("original")
	require.True(t, ok)
	require.Equal(t, true, orig2)
}
