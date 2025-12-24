# Benchmarking Guide: Split & Aggregate Transforms

This guide explains how to run, interpret, and use the benchmarks for split.array and aggregate transforms.

## Overview

The benchmark suite provides comprehensive performance testing for:

- **split.array**: Array element expansion transforms
- **aggregate.count**: Count-based windowed aggregation
- **aggregate.window**: Time/event-based windowed aggregation

## Quick Start

### Run All Benchmarks

```bash
./scripts/run-benchmarks.sh
```

This will run all benchmarks with a 1-second timeout per benchmark and save results to `benchmarks/results.txt`.

### Run Specific Benchmarks

```bash
# Run only split.array benchmarks
./scripts/run-benchmarks.sh --split-only

# Run only aggregate benchmarks
./scripts/run-benchmarks.sh --aggregate-only

# Run with 5 second timeout per benchmark
./scripts/run-benchmarks.sh --time 5s

# Generate CPU and memory profiles
./scripts/run-benchmarks.sh --profile

# Show detailed statistics
./scripts/run-benchmarks.sh --detailed
```

### Direct Go Benchmark Commands

```bash
# Run all split.array benchmarks
go test -bench=. -benchmem ./internal/transform/split

# Run split.array benchmarks with memory allocation tracking
go test -bench=BenchmarkSplitArray -benchmem -benchstat ./internal/transform/split

# Run aggregate benchmarks with CPU profiling
go test -bench=BenchmarkAggregate -cpuprofile=cpu.prof ./internal/transform/aggregate

# Run with specified iteration count
go test -bench=. -benchtime=100x ./internal/transform/split

# Run with race detector (slower but catches concurrency issues)
go test -race -bench=. ./internal/transform/aggregate
```

## Benchmark Suite Details

### split.array Benchmarks

**BenchmarkSplitArraySmall**
- Array size: 10 elements
- Tests basic functionality with minimal overhead
- Use to establish baseline performance

**BenchmarkSplitArrayMedium**
- Array size: 100 elements
- Tests typical real-world usage
- Most representative of production workloads

**BenchmarkSplitArrayLarge**
- Array size: 1,000 elements
- Tests performance with large payloads
- Identifies memory allocation bottlenecks

**BenchmarkSplitArrayNestedPayload**
- Complex nested structures (maps within arrays)
- Tests field path resolution performance
- Reflects real-world schema complexity

**BenchmarkSplitArrayStringElements**
- Array of simple strings
- Lowest memory overhead scenario
- Good for comparison baseline

**BenchmarkSplitArrayMemoryAllocation**
- Measures allocation count and size
- Reports allocations per operation
- Useful for garbage collection analysis

**BenchmarkSplitArrayParallel**
- Concurrent execution on multiple CPUs
- Tests thread-safety and contention
- Shows real-world multi-core performance

### aggregate.count Benchmarks

**BenchmarkAggregateCountSmallWindow**
- Window size: 10 events
- Minimal state tracking overhead
- Frequent window emissions

**BenchmarkAggregateCountMediumWindow**
- Window size: 100 events
- 5 aggregation functions (sum, avg, min, max, count)
- Realistic production configuration

**BenchmarkAggregateCountLargeWindow**
- Window size: 1,000 events
- Extended state accumulation
- Tests memory and CPU scaling

**BenchmarkAggregateCountMultipleKeys**
- 10 different partition keys
- Concurrent window state management
- Tests hash map contention

**BenchmarkAggregateCountSingleAggregation**
- Minimal aggregation (single sum)
- Establishes baseline overhead
- Useful for per-function cost analysis

**BenchmarkAggregateCountMultipleAggregations**
- 7 different aggregation functions
- Tests function dispatch overhead
- Shows impact of aggregation complexity

**BenchmarkAggregateCountMemoryAllocation**
- Reports allocations per event
- Tracks state memory overhead
- Good for GC pause analysis

**BenchmarkAggregateCountParallel**
- Concurrent event processing
- Tests lock contention on window state
- Shows multi-core scaling

### aggregate.window Benchmarks

**BenchmarkAggregateWindowTumblingSmall/Medium/Large**
- Event-based windows (max_events only)
- No time-based flushing
- Sizes: 10, 100, 1000 events

**BenchmarkAggregateWindowDurationBased**
- Time-based windows (max_duration only)
- Event frequency: 100 events per second
- Duration: 1 second

**BenchmarkAggregateWindowHybridEventAndDuration**
- Both event and time limits (50 events OR 500ms)
- Tests dual-trigger logic
- Most complex window type

**BenchmarkAggregateWindowMultiplePartitions**
- 10 concurrent partition keys
- Tests window state map scaling
- Realistic high-cardinality scenario

**BenchmarkAggregateWindowSingleAggregation**
- Minimal aggregation overhead
- Baseline window processing cost

**BenchmarkAggregateWindowMultipleAggregations**
- 7 aggregation functions per window
- High computation overhead

**BenchmarkAggregateWindowMemoryAllocation**
- Allocation count and size tracking
- Window state memory overhead

**BenchmarkAggregateWindowParallel**
- Concurrent window processing
- Thread-safety verification
- Multi-core scaling analysis

## Interpreting Results

### Basic Metrics

Each benchmark line shows:
```
BenchmarkName-8    1000000    1000 ns/op    500 B/op    10 allocs/op
```

- `BenchmarkName-8`: Test name and GOMAXPROCS (8 CPUs)
- `1000000`: Number of iterations run
- `1000 ns/op`: Nanoseconds per operation (lower is better)
- `500 B/op`: Bytes allocated per operation (lower is better)
- `10 allocs/op`: Number of allocations per operation (lower is better)

### Performance Goals

Based on devspec.md requirements:

**Throughput Target**: 10,000+ messages/second per instance
- Maximum acceptable latency per event: 100 microseconds (100,000 ns)
- For split.array: Each event generates N child events, so latency should be ~100µs/element

**Latency Target**: p99 < 100ms for transform execution
- Individual transform execution: < 10µs typical (0.01ms)

**Memory Target**: < 512MB per instance under normal load
- Per-event overhead should be < 50KB
- Window state should be efficiently bounded

### Performance Regression Detection

Compare benchmark results over time:
```bash
# Save baseline
go test -bench=. -benchmem ./internal/transform/split > baseline.txt

# After code changes
go test -bench=. -benchmem ./internal/transform/split > current.txt

# Compare (requires benchstat tool)
benchstat baseline.txt current.txt
```

Install benchstat:
```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

## Optimization Strategies

### For split.array

1. **Reduce allocations**: Preallocate output slice capacity
2. **Cache field resolution**: Store resolved field paths
3. **Minimize cloning**: Share immutable data where possible
4. **Batch processing**: Process multiple events together

### For aggregate.count/window

1. **Lock contention**: Use sharded maps for high-cardinality partitions
2. **Memory pools**: Reuse aggregation state objects
3. **Lazy aggregation**: Delay computation until window flushes
4. **Efficient data structures**: Use optimized hash maps for state tracking

## Profiling

### CPU Profiling

```bash
# Generate profile
go test -bench=BenchmarkSplitArrayLarge -cpuprofile=cpu.prof ./internal/transform/split

# Analyze
go tool pprof cpu.prof

# In pprof:
# (pprof) top      # Show top functions by CPU time
# (pprof) list SplitArray  # Show source code with annotations
# (pprof) web      # Generate graph visualization (requires graphviz)
```

### Memory Profiling

```bash
# Generate profile
go test -bench=BenchmarkSplitArrayMemoryAllocation -memprofile=mem.prof ./internal/transform/split

# Analyze allocations
go tool pprof -alloc_space mem.prof  # Total allocated memory
go tool pprof -alloc_objects mem.prof  # Number of allocations

# Analyze live allocations
go tool pprof -inuse_space mem.prof  # Currently allocated
go tool pprof -inuse_objects mem.prof  # Current object count
```

### Race Detection

```bash
# Run benchmarks with race detector (slower)
go test -race -bench=. ./internal/transform/aggregate

# Or run tests
go test -race ./internal/transform/aggregate
```

## Continuous Performance Monitoring

### GitHub Actions Integration

Create `.github/workflows/benchmarks.yml`:

```yaml
name: Benchmarks
on: [push, pull_request]
jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make test-bench
      - uses: benchmark-action/github-action-benchmark@v1
```

### Local Trend Tracking

```bash
#!/bin/bash
# Track benchmark results over time

DATE=$(date +%Y%m%d_%H%M%S)
go test -bench=. -benchmem ./internal/transform/split > benchmarks/${DATE}_split.txt
go test -bench=. -benchmem ./internal/transform/aggregate > benchmarks/${DATE}_aggregate.txt

# Compare with previous
if [ -f benchmarks/LATEST_split.txt ]; then
    benchstat benchmarks/LATEST_split.txt benchmarks/${DATE}_split.txt
fi

# Update latest
ln -sf ${DATE}_split.txt benchmarks/LATEST_split.txt
ln -sf ${DATE}_aggregate.txt benchmarks/LATEST_aggregate.txt
```

## Makefile Integration

Add to Makefile:

```makefile
.PHONY: bench bench-split bench-aggregate bench-all bench-profile

bench:
	@./scripts/run-benchmarks.sh

bench-split:
	@./scripts/run-benchmarks.sh --split-only

bench-aggregate:
	@./scripts/run-benchmarks.sh --aggregate-only

bench-all:
	@./scripts/run-benchmarks.sh --time 10s --detailed

bench-profile:
	@./scripts/run-benchmarks.sh --profile
```

Then run with: `make bench`, `make bench-profile`, etc.

## Best Practices

1. **Run on dedicated hardware**: Background processes affect results
2. **Warm up JIT**: Run a few iterations before timing
3. **Use long benchmarks**: Minimum 1-2 seconds per benchmark
4. **Run multiple times**: Results vary slightly between runs
5. **Measure realistic loads**: Match production event sizes and structures
6. **Track trends**: Compare results over time to detect regressions
7. **Profile bottlenecks**: Use pprof to identify hot functions
8. **Test edge cases**: Include pathological scenarios (empty arrays, huge windows)

## Troubleshooting

### Benchmark Takes Too Long

```bash
# Reduce benchmark time
./scripts/run-benchmarks.sh --time 100ms

# Run specific benchmark
go test -bench=BenchmarkSplitArraySmall -benchtime=100ms ./internal/transform/split
```

### High Allocation Count

Check for:
- Unnecessary map allocations in hot paths
- String concatenations in loops
- Repeated field path resolution

### Inconsistent Results

Causes:
- CPU frequency scaling (disable with `cpupower`)
- Background processes (close other applications)
- NUMA effects on multi-socket systems

Solutions:
```bash
# Pin to specific CPU cores
taskset -c 0-3 go test -bench=. ./internal/transform/split

# Set CPU governor to performance
sudo cpupower frequency-set -g performance
```

## Next Steps

1. Run baseline benchmarks: `./scripts/run-benchmarks.sh`
2. Identify bottlenecks: `./scripts/run-benchmarks.sh --profile`
3. Analyze hot paths: `go tool pprof benchmarks/cpu.prof`
4. Implement optimizations
5. Validate improvements with new benchmarks
6. Track results in version control or CI system

## References

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Go Benchmarking Guide](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [pprof Documentation](https://github.com/google/pprof/tree/master/doc)
- [benchstat Tool](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat)
