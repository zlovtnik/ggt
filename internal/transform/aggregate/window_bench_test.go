package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/zlovtnik/ggt/pkg/event"
)

func BenchmarkAggregateWindowTumblingSmall(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   10,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	w.Configure(raw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0] // Clear results
		w.SetEmitFunc(emitFunc)

		for j := 0; j < 10; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				Build()

			w.Execute(context.Background(), evt)
		}

		// Force emit all windows
		w.ForceEmitAllWindows(context.Background())
		w.Stop()
	}
}

func BenchmarkAggregateWindowTumblingMedium(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   100,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, _ := json.Marshal(config)

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	w.Configure(raw)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0] // Clear results
		w.SetEmitFunc(emitFunc)

		for j := 0; j < 100; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				Build()

			w.Execute(context.Background(), evt)
		}

		// Force emit all windows
		w.ForceEmitAllWindows(context.Background())
		w.Stop()
	}
}

func BenchmarkAggregateWindowTumblingLarge(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   1000,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	if err := w.Configure(raw); err != nil {
		b.Fatalf("failed to configure window: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0] // Clear results
		w.SetEmitFunc(emitFunc)

		for j := 0; j < 1000; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				Build()

			w.Execute(context.Background(), evt)
		}

		// Force emit all windows
		w.ForceEmitAllWindows(context.Background())
		w.Stop()
	}
}

func BenchmarkAggregateWindowDurationBased(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   0,
		MaxDuration: 100 * time.Millisecond,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	if err := w.Configure(raw); err != nil {
		b.Fatalf("failed to configure window: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0]
		w.SetEmitFunc(emitFunc)

		for j := 0; j < 50; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				Build()

			w.Execute(context.Background(), evt)
		}

		w.Stop()
	}
}

func BenchmarkAggregateWindowHybridEventDuration(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   50,
		MaxDuration: 100 * time.Millisecond,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
			{Field: "amount", Function: "avg", As: "average"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	w := &WindowTransform{}
	if err := w.Configure(raw); err != nil {
		b.Fatalf("failed to configure window: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Set a no-op emit function to ensure windows are emitted during benchmark
		w.SetEmitFunc(func(ctx context.Context, events []event.Event) error {
			return nil // Discard emitted events
		})

		for j := 0; j < 50; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				Build()

			w.Execute(context.Background(), evt)
		}

		w.Stop()
	}
}

func BenchmarkAggregateWindowMultiplePartitions(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   100,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	if err := w.Configure(raw); err != nil {
		b.Fatalf("failed to configure window: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0]
		w.SetEmitFunc(emitFunc)

		for partition := 0; partition < 5; partition++ {
			for j := 0; j < 20; j++ {
				evt := event.NewBuilder().
					WithField("user_id", fmt.Sprintf("user%d", partition)).
					WithField("amount", float64(j)).
					Build()

				w.Execute(context.Background(), evt)
			}
		}

		w.Stop()
	}
}

func BenchmarkAggregateWindowSingleAggregation(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   50,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	var results []event.Event
	emitFunc := func(ctx context.Context, events []event.Event) error {
		results = append(results, events...)
		return nil
	}

	w := &WindowTransform{}
	if err := w.Configure(raw); err != nil {
		b.Fatalf("failed to configure window: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results = results[:0]
		w.SetEmitFunc(emitFunc)

		for j := 0; j < 100; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user1").
				WithField("amount", float64(j)).
				WithField("payload", map[string]interface{}{}).
				Build()

			w.Execute(context.Background(), evt)
		}

		w.Stop()
	}
}

func BenchmarkAggregateWindowMemoryAllocation(b *testing.B) {
	config := WindowConfig{
		KeyField:    "user_id",
		MaxEvents:   100,
		MaxDuration: 0,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}
	raw, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	b.RunParallel(func(pb *testing.PB) {
		var results []event.Event
		emitFunc := func(ctx context.Context, events []event.Event) error {
			results = append(results, events...)
			return nil
		}

		w := &WindowTransform{}
		if err := w.Configure(raw); err != nil {
			b.Fatalf("failed to configure window: %v", err)
		}

		for pb.Next() {
			results = results[:0]
			w.SetEmitFunc(emitFunc)

			for j := 0; j < 50; j++ {
				evt := event.NewBuilder().
					WithField("user_id", "user1").
					WithField("amount", float64(j)).
					Build()

				w.Execute(context.Background(), evt)
			}

			w.Stop()
		}
	})
}
