# Makefile for Transform Service

.PHONY: all build test clean fmt vet tidy run coverage help

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

# Deploy (placeholder)
deploy:
	./scripts/deploy.sh

# Show help
help:
	@echo "Available targets:"
	@echo "  all       - Build the project (default)"
	@echo "  build     - Build the binary"
	@echo "  test      - Run tests"
	@echo "  clean     - Clean build artifacts"
	@echo "  fmt       - Format code"
	@echo "  vet       - Vet code"
	@echo "  tidy      - Tidy dependencies"
	@echo "  run       - Run the service"
	@echo "  coverage  - Generate coverage report"
	@echo "  deploy    - Deploy (placeholder)"
	@echo "  help      - Show this help"