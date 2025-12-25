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
# ggt — Kafka Transform Service

A small, fast Kafka-to-Kafka transform service for stream processing and CDC pipelines. ggt reads events from Kafka, runs configurable transform pipelines, and produces results to output topics — with metrics, health checks, and configurable retries/DLQ handling.

Why ggt?
- Lightweight, production-minded transform scaffolding in Go.
- Pluggable transforms registered via packages under `internal/transform`.
- Built-in metrics (Prometheus), structured logging (zap), and graceful shutdown.

## Quick start
1. Copy the example config:

   cp configs/config.example.yaml configs/config.yaml

2. Build the binary:

   make build

3. Run locally (uses your `configs/config.yaml`):

   bin/ggt

4. Check health and metrics:

   curl http://localhost:8080/healthz
   curl http://localhost:9090/metrics

## Configuration
- Primary config: [configs/config.example.yaml](configs/config.example.yaml)
- Important keys: `service.metrics_port`, `service.health_port`, Kafka brokers, pipeline definitions (input_topics/output_topic/dlq_topic).
- Environment overrides: set `TRANSFORM_CONFIG_PATH` to point to a custom config file.

## Development
- Run tests:

  make test

- Build locally:

  make build

- Run a single transform unit test or bench from its package under `internal/transform`.

## Docker & Kubernetes
- Local dev with Docker Compose (starts Kafka):

  docker-compose up -d

- Build a docker image and run with compose or your registry. See `Dockerfile` and `Makefile` targets.
- Kubernetes: manifests are in `k8s/` and a Helm chart in `helm/ggt`.

## Project layout (high level)
- `cmd/transform` — main program wiring config, consumer, producer, metrics and health.
- `internal/transform` — transform registration and pipeline builder.
- `internal/consumer`, `internal/producer` — Kafka IO wrappers.
- `internal/config` — config schema and loader.
- `pkg/event` — immutable event helpers used across transforms.

## Contributing
- Implement transforms under `internal/transform/*` and register them via `transform.Register` in `init()`.
- Keep changes focused; run `make test` and `go vet` before pushing.
- Update `docs/architecture.md` and `devspec.md` when adding major features.

## Useful commands
- Build: `make build`
- Test: `make test` or `go test ./...`
- Run locally: `bin/ggt`

## Where to look next
- [devspec.md](devspec.md) — architecture and transform design.
- [configs/config.example.yaml](configs/config.example.yaml) — all configuration options.

## License
- Licensed under the MIT License. See [LICENSE](LICENSE).

If you want, I can add badges, a quick diagram, or a short example pipeline config next.
