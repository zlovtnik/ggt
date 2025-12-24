#!/bin/bash
# Benchmark runner script for split/aggregate transforms
# Usage: ./scripts/run-benchmarks.sh [options]
# Options:
#   -t, --time         Duration for each benchmark (default: 1s)
#   -c, --count        Number of benchmark iterations (default: auto)
#   -o, --output       Output file for results (default: benchmarks/results.txt)
#   -p, --profile      Generate CPU/memory profiles (pprof)
#   -d, --detailed     Show detailed statistics (-v flag)
#   --split-only       Run only split.array benchmarks
#   --aggregate-only   Run only aggregate benchmarks
#   --all             Run all benchmarks
#   -h, --help        Show this help message
#
# To compare benchmark results: benchstat <file1> <file2>

set -e
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Defaults
BENCH_TIME="1s"
BENCH_COUNT=""
OUTPUT_FILE="benchmarks/results.txt"
PROFILE=false
DETAILED=false
RUN_SPLIT=true
RUN_AGGREGATE=true

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--time)
            BENCH_TIME="$2"
            shift 2
            ;;
        -c|--count)
            BENCH_COUNT="$2"
            shift 2
            ;;
        -o|--output)
            OUTPUT_FILE="$2"
            shift 2
            ;;
        -p|--profile)
            PROFILE=true
            shift
            ;;
        -d|--detailed)
            DETAILED=true
            shift
            ;;
        --split-only)
            RUN_AGGREGATE=false
            shift
            ;;
        --aggregate-only)
            RUN_SPLIT=false
            shift
            ;;
        --all)
            RUN_SPLIT=true
            RUN_AGGREGATE=true
            shift
            ;;
        -h|--help)
            grep "^#" "$0" | head -20
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Create output directory
mkdir -p "$(dirname "$OUTPUT_FILE")"

# Build benchmark flags
BENCH_FLAGS="-benchmem"
if [ -n "$BENCH_COUNT" ]; then
    BENCH_FLAGS="$BENCH_FLAGS -benchtime=${BENCH_COUNT}x"
else
    BENCH_FLAGS="$BENCH_FLAGS -benchtime=$BENCH_TIME"
fi

if [ "$DETAILED" = true ]; then
    BENCH_FLAGS="$BENCH_FLAGS -v"
fi

PROFILE_FLAGS=""
if [ "$PROFILE" = true ]; then
    PROFILE_FLAGS="-cpuprofile=benchmarks/cpu.prof -memprofile=benchmarks/mem.prof"
fi

echo "Starting benchmarks..."
echo "========================" > "$OUTPUT_FILE"
echo "Benchmark Results - $(date)" >> "$OUTPUT_FILE"
echo "========================" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Run split.array benchmarks
if [ "$RUN_SPLIT" = true ]; then
    echo "Running split.array benchmarks..."
    echo "" >> "$OUTPUT_FILE"
    echo "## split.array Benchmarks" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    
    cd "$PROJECT_ROOT"
    PROFILE_FLAGS=""
    if [ "$PROFILE" = true ]; then
        PROFILE_FLAGS="-cpuprofile=benchmarks/split_cpu.prof -memprofile=benchmarks/split_mem.prof"
    fi
    go test -run=^$ -bench='^BenchmarkSplitArray' $BENCH_FLAGS $PROFILE_FLAGS ./internal/transform/split 2>&1 | tee -a "$OUTPUT_FILE"
    
    echo "" >> "$OUTPUT_FILE"
fi

# Run aggregate.count benchmarks
if [ "$RUN_AGGREGATE" = true ]; then
    echo "Running aggregate.count benchmarks..."
    echo "" >> "$OUTPUT_FILE"
    echo "## aggregate.count Benchmarks" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    
    cd "$PROJECT_ROOT"
    PROFILE_FLAGS=""
    if [ "$PROFILE" = true ]; then
        PROFILE_FLAGS="-cpuprofile=benchmarks/aggregate_count_cpu.prof -memprofile=benchmarks/aggregate_count_mem.prof"
    fi
    go test -run=^$ -bench='^BenchmarkAggregateCount' $BENCH_FLAGS $PROFILE_FLAGS ./internal/transform/aggregate 2>&1 | tee -a "$OUTPUT_FILE"
    
    echo "" >> "$OUTPUT_FILE"
    
    # Run aggregate.window benchmarks
    echo "Running aggregate.window benchmarks..."
    echo "" >> "$OUTPUT_FILE"
    echo "## aggregate.window Benchmarks" >> "$OUTPUT_FILE"
    echo "" >> "$OUTPUT_FILE"
    
    cd "$PROJECT_ROOT"
    PROFILE_FLAGS=""
    if [ "$PROFILE" = true ]; then
        PROFILE_FLAGS="-cpuprofile=benchmarks/aggregate_window_cpu.prof -memprofile=benchmarks/aggregate_window_mem.prof"
    fi
    go test -run=^$ -bench='^BenchmarkAggregateWindow' $BENCH_FLAGS $PROFILE_FLAGS ./internal/transform/aggregate 2>&1 | tee -a "$OUTPUT_FILE"
    
    echo "" >> "$OUTPUT_FILE"
fi

echo ""
echo "Benchmarks completed!"
echo "Results saved to: $OUTPUT_FILE"
echo ""
echo "To compare results with previous runs, use:"
echo "  benchstat $OUTPUT_FILE <previous_results.txt>"

# If profiles were generated, show analysis
if [ "$PROFILE" = true ]; then
    echo ""
    echo "Generated profiles:"
    
    if [ "$RUN_SPLIT" = true ] && [ -f "benchmarks/split_cpu.prof" ] && [ -f "benchmarks/split_mem.prof" ]; then
        echo "  Split array CPU profile: benchmarks/split_cpu.prof"
        echo "  Split array memory profile: benchmarks/split_mem.prof"
    fi
    
    if [ "$RUN_AGGREGATE" = true ] && [ -f "benchmarks/aggregate_count_cpu.prof" ] && [ -f "benchmarks/aggregate_count_mem.prof" ]; then
        echo "  Aggregate count CPU profile: benchmarks/aggregate_count_cpu.prof"
        echo "  Aggregate count memory profile: benchmarks/aggregate_count_mem.prof"
    fi
    
    if [ "$RUN_AGGREGATE" = true ] && [ -f "benchmarks/aggregate_window_cpu.prof" ] && [ -f "benchmarks/aggregate_window_mem.prof" ]; then
        echo "  Aggregate window CPU profile: benchmarks/aggregate_window_cpu.prof"
        echo "  Aggregate window memory profile: benchmarks/aggregate_window_mem.prof"
    fi
    
    echo ""
    echo "View profiles with:"
    echo "  go tool pprof <profile_file>"
fi
