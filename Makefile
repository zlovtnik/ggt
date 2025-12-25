# Makefile for Transform Service

.PHONY: all build test clean fmt vet tidy run coverage bench bench-split bench-aggregate bench-profile docker-build docker-run docker-push k8s-deploy k8s-undeploy helm-install helm-upgrade helm-uninstall deploy help

# Docker configuration
IMAGE_NAME := ggt
IMAGE_TAG := latest
REGISTRY := ghcr.io/zlovtnik

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

# Docker targets
docker-build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

docker-run: docker-build
	docker run --rm -p 9095:9095 -p 8085:8085 $(IMAGE_NAME):$(IMAGE_TAG)

docker-push: docker-build
	docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
# Kubernetes targets
k8s-deploy:
	kubectl apply -f k8s/

k8s-undeploy:
	kubectl delete -f k8s/

# Helm targets
helm-install:
	helm install ggt ./helm/ggt

helm-upgrade:
	helm upgrade ggt ./helm/ggt

helm-uninstall:
	helm uninstall ggt

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
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-run     - Run Docker container"
	@echo "  docker-push    - Push Docker image to registry"
	@echo "  k8s-deploy     - Deploy to Kubernetes"
	@echo "  k8s-undeploy   - Remove from Kubernetes"
	@echo "  helm-install   - Install Helm chart"
	@echo "  helm-upgrade   - Upgrade Helm chart"
	@echo "  helm-uninstall - Uninstall Helm chart"
	@echo "  deploy         - Deploy (placeholder)"
	@echo "  help           - Show this help"