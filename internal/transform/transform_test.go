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
