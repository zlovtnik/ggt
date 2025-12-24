# Makefile for Transform Service

.PHONY: all build test clean fmt vet tidy run coverage bench bench-split bench-aggregate bench-profile deploy help

# Default target
all: build

# Build the binary
build:
	./scripts/build.sh

# Run tests
test:
	./scripts/test.sh

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out
	rm -f coverage.html

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy

# Run the service
run:
	go run ./cmd/transform

# Generate coverage report
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
bench:
	@./scripts/run-benchmarks.sh

# Run only split.array benchmarks
bench-split:
	@./scripts/run-benchmarks.sh --split-only

# Run only aggregate benchmarks
bench-aggregate:
	@./scripts/run-benchmarks.sh --aggregate-only

# Run benchmarks with CPU/memory profiling
bench-profile:
	@./scripts/run-benchmarks.sh --profile

# Deploy (placeholder)
deploy:
	./scripts/deploy.sh

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Build the project (default)"
	@echo "  build          - Build the binary"
	@echo "  test           - Run tests"
	@echo "  coverage       - Generate coverage report"
	@echo "  bench          - Run all benchmarks"
	@echo "  bench-split    - Run split.array benchmarks"
	@echo "  bench-aggregate - Run aggregate benchmarks"
	@echo "  bench-profile  - Run benchmarks with profiling"
	@echo "  clean          - Clean build artifacts"
	@echo "  fmt            - Format code"
	@echo "  vet            - Vet code"
	@echo "  tidy           - Tidy dependencies"
	@echo "  run            - Run the service"
	@echo "  deploy         - Deploy (placeholder)"
	@echo "  help           - Show this help"