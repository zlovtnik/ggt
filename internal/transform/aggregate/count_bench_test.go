package aggregate

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zlovtnik/ggt/pkg/event"
)

// BenchmarkAggregateCountSmallWindow benchmarks aggregate.count with small window size (10)
func BenchmarkAggregateCountSmallWindow(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    10,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total_amount"},
			{Field: "amount", Function: "avg", As: "avg_amount"},
			{Field: "id", Function: "count", As: "event_count"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 10)
	for j := 0; j < 10; j++ {
		events[j] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("id", fmt.Sprintf("evt_%d", j)).
			WithField("amount", float64((j+1)*10)).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 10; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountMediumWindow benchmarks aggregate.count with medium window size (100)
func BenchmarkAggregateCountMediumWindow(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    100,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total_amount"},
			{Field: "amount", Function: "avg", As: "avg_amount"},
			{Field: "amount", Function: "min", As: "min_amount"},
			{Field: "amount", Function: "max", As: "max_amount"},
			{Field: "id", Function: "count", As: "event_count"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 100)
	for j := 0; j < 100; j++ {
		events[j] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("id", fmt.Sprintf("evt_%d", j)).
			WithField("amount", float64((j+1)*10)).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 100; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountLargeWindow benchmarks aggregate.count with large window size (1000)
func BenchmarkAggregateCountLargeWindow(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    1000,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total_amount"},
			{Field: "amount", Function: "avg", As: "avg_amount"},
			{Field: "amount", Function: "min", As: "min_amount"},
			{Field: "amount", Function: "max", As: "max_amount"},
			{Field: "id", Function: "count", As: "event_count"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 1000)
	for j := 0; j < 1000; j++ {
		events[j] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("id", fmt.Sprintf("evt_%d", j)).
			WithField("amount", float64((j+1)*10)).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 1000; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountMultipleKeys benchmarks aggregate.count with multiple partition keys
func BenchmarkAggregateCountMultipleKeys(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    50,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total_amount"},
			{Field: "amount", Function: "avg", As: "avg_amount"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()
	numKeys := 10

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 50*numKeys)
	for j := 0; j < 50*numKeys; j++ {
		userID := fmt.Sprintf("user_%d", j%numKeys)
		events[j] = event.NewBuilder().
			WithField("user_id", userID).
			WithField("id", fmt.Sprintf("evt_%d", j)).
			WithField("amount", float64((j+1)*10)).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 50*numKeys; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountSingleAggregation benchmarks with single aggregation function
func BenchmarkAggregateCountSingleAggregation(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    100,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 100)
	for j := 0; j < 100; j++ {
		events[j] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("amount", float64((j+1)*10)).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 100; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountMultipleAggregations benchmarks with many aggregation functions
func BenchmarkAggregateCountMultipleAggregations(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    50,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total_amount"},
			{Field: "amount", Function: "avg", As: "avg_amount"},
			{Field: "amount", Function: "min", As: "min_amount"},
			{Field: "amount", Function: "max", As: "max_amount"},
			{Field: "id", Function: "count", As: "event_count"},
			{Field: "quantity", Function: "sum", As: "total_qty"},
			{Field: "price", Function: "avg", As: "avg_price"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time
	events := make([]event.Event, 50)
	for j := 0; j < 50; j++ {
		events[j] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("id", fmt.Sprintf("evt_%d", j)).
			WithField("amount", float64((j+1)*10)).
			WithField("quantity", j+1).
			WithField("price", float64(j+1)*2.5).
			Build()
	}

	// Pre-configure transform to avoid measuring setup time
	baseTransform := &CountTransform{}
	if err := baseTransform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		*transform = *baseTransform // Copy configured transform
		// Create fresh state for each iteration
		transform.windows = make(map[string]*CountWindowState)

		for j := 0; j < 50; j++ {
			if _, err := transform.Execute(ctx, events[j]); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountMemoryAllocation measures memory usage
func BenchmarkAggregateCountMemoryAllocation(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    100,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
			{Field: "amount", Function: "avg", As: "avg"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Use fixed event count for memory benchmark to avoid OOM with large b.N
	const eventsPerIteration = 100

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform := &CountTransform{}
		if err := transform.Configure(rawConfig); err != nil {
			b.Fatalf("failed to configure: %v", err)
		}

		for j := 0; j < eventsPerIteration; j++ {
			evt := event.NewBuilder().
				WithField("user_id", "user123").
				WithField("amount", float64(j*10)).
				Build()

			if _, err := transform.Execute(ctx, evt); err != nil {
				b.Fatalf("Execute failed: %v", err)
			}
		}
	}
}

// BenchmarkAggregateCountParallel benchmarks count aggregation in parallel
func BenchmarkAggregateCountParallel(b *testing.B) {
	config := CountConfig{
		KeyField: "user_id",
		Count:    50,
		Aggregations: []Aggregation{
			{Field: "amount", Function: "sum", As: "total"},
		},
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}

	ctx := context.Background()

	// Pre-create events to avoid measuring allocation time in parallel
	events := make([]event.Event, 1000)
	for i := range events {
		events[i] = event.NewBuilder().
			WithField("user_id", "user123").
			WithField("amount", float64((i%50)*10)).
			Build()
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		transform := &CountTransform{}
		if err := transform.Configure(rawConfig); err != nil {
			b.Fatalf("failed to configure: %v", err)
		}

		idx := 0
		for pb.Next() {
			evt := events[idx%len(events)]
			if _, err := transform.Execute(ctx, evt); err != nil {
				b.Errorf("Execute failed: %v", err)
				return
			}
			idx++
		}
	})
}
