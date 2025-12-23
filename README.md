# Transform Service

This repository hosts the Transform Service described in [devspec.md](./devspec.md). It is a Go-based processor that sits between your Kafka CDC stream and the downstream consumers. The current tree provides the minimum wiring for configuration loading, structured logging, and Prometheus metrics so implementers can hook in the transformations defined in the specification.

## Getting Started

1. Install Go 1.22+ and run the helper script [scripts/build.sh](scripts/build.sh#L1-L4) for make-style commands.
2. Copy the sample configuration at [configs/config.example.yaml](configs/config.example.yaml#L1-L117) to `configs/config.yaml` and adjust values as needed.
3. Run `go test ./...` to exercise the skeletons or `go build ./...` for a dry run.

## Running the Service

1. Build or run the service binary (`go build -o bin/transform ./cmd/transform` and then `bin/transform`, or `go run ./cmd/transform`) and rely on [cmd/transform/main.go](cmd/transform/main.go#L1-L145) to wire metrics, health probes, and the pipeline worker.
2. Point the process at your edited `configs/config.yaml` (a copy of the sample) and update [internal/config/loader.go](internal/config/loader.go#L1-L34) if you prefer a different path; the YAML still drives brokers, consumer groups, and port values such as `service.metrics_port`/`service.health_port`.
3. Once the health server is up on the configured `service.health_port` (default `8080` in the sample), confirm readiness with `curl http://localhost:8080/healthz`.

## Project Layout

- `[cmd/transform/main.go](cmd/transform/main.go#L1-L145)` entry point for the long-running process.
- `[internal/config/config.go](internal/config/config.go#L1-L103)` and `[internal/config/loader.go](internal/config/loader.go#L1-L34)` for schemas and the YAML loader.
- `[internal/logging/logger.go](internal/logging/logger.go#L1-L26)` zap-based logger.
- `[internal/metrics/metrics.go](internal/metrics/metrics.go#L1-L87)` Prometheus observability scaffolding.
- `[pkg/event/event.go](pkg/event/event.go#L1-L139)` immutable event helpers that the pipeline components will consume.
- `[configs/config.example.yaml](configs/config.example.yaml#L1-L117)` sample YAML files.
- `[scripts/build.sh](scripts/build.sh#L1-L4)` reusable helpers for build/test/deploy.

See [devspec.md](./devspec.md) for the detailed architecture and planned transforms.
