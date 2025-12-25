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

   curl <http://localhost:8080/healthz>
   curl <http://localhost:9090/metrics>

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

**Example: Enable `all_transforms_demo`**

- Copy the example configuration and run the service using the demo pipeline included in `configs/config.example.yaml`:

   ```bash
   cp configs/config.example.yaml configs/config.yaml
   make build
   bin/ggt
   ```

- The demo pipeline listens on `raw.all` and emits to `processed.all`. To send a quick test message use the included helper script (or your favorite Kafka producer):

   ```bash
   echo '{"id":"test-1","total_amount":123.45,"name":"Jane Doe","email":"jane@example.com"}' | \
      scripts/send-test-messages.sh --topic raw.all
   ```

- The schema used by `validate.schema` for the demo is `schemas/all.json`. Adjust `configs/config.yaml` to enable, disable, or modify the pipeline as needed.
