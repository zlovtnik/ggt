package split

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/zlovtnik/ggt/pkg/event"
)

// Benchmark configurations for split.array transform
const (
	smallArraySize  = 10
	mediumArraySize = 100
	largeArraySize  = 1000
)

// BenchmarkSplitArraySmall benchmarks split.array with small arrays (10 elements)
func BenchmarkSplitArraySmall(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	// Create an event with a small array
	items := make([]interface{}, smallArraySize)
	for i := 0; i < smallArraySize; i++ {
		items[i] = map[string]interface{}{
			"id":   i,
			"name": fmt.Sprintf("item_%d", i),
		}
	}

	evt := event.NewBuilder().
		WithField("items", items).
		WithField("user_id", "user123").
		WithField("timestamp", 1234567890).
		Build()

	ctx := context.Background()

	// Validate result once before benchmarking
	result, err := transform.Execute(ctx, evt)
	if err != nil {
		b.Fatalf("execute failed: %v", err)
	}
	if events, ok := result.([]event.Event); !ok || len(events) != smallArraySize {
		b.Fatalf("unexpected result: %v", result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transform.Execute(ctx, evt); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSplitArrayMedium benchmarks split.array with medium arrays (100 elements)
func BenchmarkSplitArrayMedium(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	// Create an event with a medium array
	items := make([]interface{}, mediumArraySize)
	for i := 0; i < mediumArraySize; i++ {
		items[i] = map[string]interface{}{
			"id":    i,
			"name":  fmt.Sprintf("item_%d", i),
			"price": float64(i) * 10.5,
		}
	}

	evt := event.NewBuilder().
		WithField("items", items).
		WithField("user_id", "user123").
		WithField("order_id", "order456").
		Build()

	ctx := context.Background()

	// Validate result once before benchmarking
	result, err := transform.Execute(ctx, evt)
	if err != nil {
		b.Fatalf("execute failed: %v", err)
	}
	if events, ok := result.([]event.Event); !ok || len(events) != mediumArraySize {
		b.Fatalf("unexpected result: %v", result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transform.Execute(ctx, evt); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSplitArrayLarge benchmarks split.array with large arrays (1000 elements)
func BenchmarkSplitArrayLarge(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	// Create an event with a large array
	items := make([]interface{}, largeArraySize)
	for i := 0; i < largeArraySize; i++ {
		items[i] = map[string]interface{}{
			"id":       i,
			"name":     fmt.Sprintf("item_%d", i),
			"price":    float64(i) * 10.5,
			"category": fmt.Sprintf("cat_%d", i%10),
		}
	}

	evt := event.NewBuilder().
		WithField("items", items).
		WithField("user_id", "user123").
		WithField("order_id", "order456").
		WithField("timestamp", 1234567890).
		Build()

	ctx := context.Background()

	// Validate result once before benchmarking
	result, err := transform.Execute(ctx, evt)
	if err != nil {
		b.Fatalf("execute failed: %v", err)
	}
	if events, ok := result.([]event.Event); !ok || len(events) != largeArraySize {
		b.Fatalf("unexpected result: %v", result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transform.Execute(ctx, evt); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSplitArrayNestedPayload benchmarks split.array with deeply nested payloads
func BenchmarkSplitArrayNestedPayload(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "data.items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	// Create an event with nested structure containing array
	items := make([]interface{}, mediumArraySize)
	for i := 0; i < mediumArraySize; i++ {
		items[i] = map[string]interface{}{
			"id":   i,
			"name": fmt.Sprintf("item_%d", i),
			"meta": map[string]interface{}{"created": 123456, "updated": 789012},
			"tags": []string{"tag1", "tag2", "tag3"},
		}
	}

	evt := event.NewBuilder().
		WithField("data", map[string]interface{}{
			"items":    items,
			"total":    mediumArraySize,
			"metadata": map[string]interface{}{"version": "1.0"},
		}).
		WithField("user_id", "user123").
		Build()

	ctx := context.Background()

	// Validate result once before benchmarking
	result, err := transform.Execute(ctx, evt)
	if err != nil {
		b.Fatalf("execute failed: %v", err)
	}
	if events, ok := result.([]event.Event); !ok || len(events) != mediumArraySize {
		b.Fatalf("unexpected result: %v", result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transform.Execute(ctx, evt); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSplitArrayStringElements benchmarks split.array with string array elements
func BenchmarkSplitArrayStringElements(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "tags",
		Key:   "tag",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	// Create an event with string array
	tags := make([]interface{}, mediumArraySize)
	for i := 0; i < mediumArraySize; i++ {
		tags[i] = fmt.Sprintf("tag_%d", i)
	}

	evt := event.NewBuilder().
		WithField("tags", tags).
		WithField("product_id", "prod123").
		Build()

	ctx := context.Background()

	// Validate result once before benchmarking
	result, err := transform.Execute(ctx, evt)
	if err != nil {
		b.Fatalf("execute failed: %v", err)
	}
	if events, ok := result.([]event.Event); !ok || len(events) != mediumArraySize {
		b.Fatalf("unexpected result: %v", result)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		transform.Execute(ctx, evt)
	}
}

// BenchmarkSplitArrayMemoryAllocation measures memory allocations during split
func BenchmarkSplitArrayMemoryAllocation(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	items := make([]interface{}, largeArraySize)
	for i := 0; i < largeArraySize; i++ {
		items[i] = map[string]interface{}{
			"id": i,
		}
	}

	evt := event.NewBuilder().
		WithField("items", items).
		Build()

	ctx := context.Background()

	// Run once to stabilize
	if _, err := transform.Execute(ctx, evt); err != nil {
		b.Fatalf("stabilization execute failed: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := transform.Execute(ctx, evt); err != nil {
			b.Fatalf("Execute failed: %v", err)
		}
	}
}

// BenchmarkSplitArrayParallel benchmarks split.array in parallel execution
func BenchmarkSplitArrayParallel(b *testing.B) {
	transform := &splitArrayTransform{}
	config := ArrayConfig{
		Field: "items",
		Key:   "item",
	}

	rawConfig, err := json.Marshal(config)
	if err != nil {
		b.Fatalf("failed to marshal config: %v", err)
	}
	if err := transform.Configure(rawConfig); err != nil {
		b.Fatalf("failed to configure transform: %v", err)
	}

	items := make([]interface{}, mediumArraySize)
	for i := 0; i < mediumArraySize; i++ {
		items[i] = map[string]interface{}{
			"id":   i,
			"name": fmt.Sprintf("item_%d", i),
		}
	}

	evt := event.NewBuilder().
		WithField("items", items).
		Build()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result, err := transform.Execute(ctx, evt)
			if err != nil {
				b.Errorf("execute failed: %v", err)
				return
			}
			b.StopTimer()
			if events, ok := result.([]event.Event); !ok || len(events) != mediumArraySize {
				b.Errorf("unexpected result: %v", result)
				return
			}
			b.StartTimer()
		}
	})
}
