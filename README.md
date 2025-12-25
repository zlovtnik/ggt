# Transform Service

This repository hosts the Transform Service described in [devspec.md](./devspec.md). It is a Go-based processor that sits between your Kafka CDC stream and the downstream consumers. The current tree provides the minimum wiring for configuration loading, structured logging, and Prometheus metrics so implementers can hook in the transformations defined in the specification.

## Getting Started

1. Install Go 1.22+ and run the helper script [scripts/build.sh](scripts/build.sh) for make-style commands.
2. Copy the sample configuration at [configs/config.example.yaml](configs/config.example.yaml) to `configs/config.yaml` and adjust values as needed.
3. Run `go test ./...` to exercise the skeletons or `go build ./...` for a dry run.

## Running the Service

1. Build or run the service binary (`go build -o bin/transform ./cmd/transform` and then `bin/transform`, or `go run ./cmd/transform`) and rely on [cmd/transform/main.go](cmd/transform/main.go) to wire metrics, health probes, and the pipeline worker.
2. Point the process at your edited `configs/config.yaml` (a copy of the sample) and update [internal/config/loader.go](internal/config/loader.go) if you prefer a different path; the YAML still drives brokers, consumer groups, and port values such as `service.metrics_port`/`service.health_port`.
3. Once the health server is up on the configured `service.health_port` (default `8080` in the sample), confirm readiness with `curl http://localhost:8080/healthz`.

## Project Layout

- [cmd/transform/main.go](cmd/transform/main.go) — entry point for the long-running process.
- [internal/config/config.go](internal/config/config.go) and [internal/config/loader.go](internal/config/loader.go) — for schemas and the YAML loader.
- [internal/logging/logger.go](internal/logging/logger.go) — zap-based logger.
- [internal/metrics/metrics.go](internal/metrics/metrics.go) — Prometheus observability scaffolding.
- [pkg/event/event.go](pkg/event/event.go) — immutable event helpers that the pipeline components will consume.
- [configs/config.example.yaml](configs/config.example.yaml) — sample YAML files.
- [scripts/build.sh](scripts/build.sh) — reusable helpers for build/test/deploy.

See [devspec.md](./devspec.md) for the detailed architecture and planned transforms.

## Deployment

### Local Development with Docker Compose

1. Start the local Kafka stack:
   ```bash
   docker-compose up -d
   ```

2. Build and run the service:
   ```bash
   make docker-run
   ```

### Kubernetes Deployment

1. Apply the Kubernetes manifests:
   ```bash
   make k8s-deploy
   ```

2. Or use Helm:
   ```bash
   make helm-install
   ```

### Production Deployment

The CI/CD pipeline automatically builds and deploys to Kubernetes when changes are pushed to the `main` or `dev` branches.

- **Development**: Deploys to the `development` namespace on push to `dev` branch
- **Production**: Deploys to the `production` namespace on push to `main` branch

## Configuration

The service is configured via YAML files. See [configs/config.example.yaml](configs/config.example.yaml) for all available options.

Key configuration sections:
- **Kafka**: Broker addresses, consumer group settings
- **Pipelines**: Transform definitions with input/output topics
- **Metrics**: Prometheus configuration
- **Health**: Health check endpoints
- **Logging**: Log level and format

## Monitoring

- **Health Checks**: `GET /healthz` on the health port (default: 8080)
- **Metrics**: Prometheus metrics on `/metrics` (default: 9090)
- **Logs**: Structured JSON logging with configurable levels
