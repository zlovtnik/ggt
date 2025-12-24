# Copilot Instructions for ggt

## Grasp the high-level service
- This repo implements the Transform Service described in [docs/architecture.md](docs/architecture.md) and wired out in [devspec.md](devspec.md); start there to understand the Kafka-to-Kafka flow, observability requirements, and the intended transform categories (field, data, enrichment, filter, validate).
- The service consumes CDC events from Kafka, applies configurable transformation pipelines in-memory, and produces enriched events back to Kafka. It uses immutable `pkg/event.Event` structures to ensure thread safety and idempotency.
- Core components: Kafka consumer/producer ([internal/consumer/](internal/consumer/), [internal/producer/](internal/producer/)), composable transform pipelines ([internal/transform/](internal/transform/)), enrichment sources (HTTP with circuit breaker, Postgres, Redis with TTL caching), and shared infrastructure (logging, metrics, health, config).

## Local dev workflows
- Use `make build` or `./scripts/build.sh` to compile the binary to `bin/ggt`; `make test` or `./scripts/test.sh` runs `go test -race -coverprofile=coverage.out ./...` for unit tests.
- Run the service with `./bin/ggt` after copying `configs/config.example.yaml` to `configs/config.yaml` and adjusting Kafka brokers/topics.
- Health checks: `curl http://localhost:8080/healthz` (example port; see `configs/config.example.yaml` for default); metrics at `http://localhost:9090/metrics` (only exposed when `service.metrics_port > 0`; see `configs/config.example.yaml` for default).
- There is no deployment pipeline yet; `scripts/deploy.sh` prints a notice. Expect manual containerization/deployment plans in future `deployments/` directories.

## Configuration, secrets, and conventions
- The YAML schema in `configs/config.example.yaml` mirrors `internal/config/config.go`. Key sections: `service` (ports, timeouts), `kafka` (consumer/producer settings), `transforms.pipelines` (per-pipeline input/output topics, DLQ, and transform list).
- Pipelines are defined as arrays of transforms with `type` (e.g., `field.rename`) and `config` (JSON object). Example:

```yaml
transforms:
  pipelines:
    - transforms:
        - type: "field.rename"
          config:
            mappings:
              user_id: id
```
- Sensitive data (Redis passwords, Postgres URLs) are injected at deploy time; config fields use `omitempty` and optional validation.
- Currently implemented transforms: field (add, extract, flatten, remove, rename, unflatten), data (case, convert, encode, format, hash, join, mask, regex, split), filter (condition, route), validate (required, rules, schema), enrichment (lookup via HTTP, Postgres, Redis).

## Observability and runtime behavior
- Logs from `internal/logging/logger.go` use zap with ISO 8601 timestamps; levels: info, warn, error, debug. Structured fields for service events.
- Metrics from `internal/metrics/metrics.go`: `transform_processed_total`, `transform_dropped_total`, `transform_dlq_total`, histograms like `transform_duration_seconds` per transform type. Exposed at `/metrics` when `service.metrics_port > 0`.
- Health server at `/healthz` returns 200 if alive; configured via `service.health_port` (see `configs/config.example.yaml` for default).
- Graceful shutdown: All components (consumer, producer, servers) share `context` with `signal.NotifyContext`; use `shutdown.NewCoordinator` for consistent stopping.

## What to edit first when adding functionality
- For new transforms: Implement `Transform` interface (`Name()`, `Configure()`, `Execute()`) in appropriate subpackage (e.g., `internal/transform/field/`), register in `init()` via `transform.Register()` (defined in `internal/transform/registry.go`). Each concrete transform subpackage (e.g., `internal/transform/data/`, `internal/transform/field/`, etc.) must call `transform.Register()` from an `init()` function with a unique name (e.g., `func init() { transform.Register("field.rename", func(raw json.RawMessage) (transform.Transform, error) { ... }) }`). Blank imports in `cmd/transform/main.go` trigger registration: `_ "github.com/zlovtnik/ggt/internal/transform/data"`, `_ "github.com/zlovtnik/ggt/internal/transform/field"`, `_ "github.com/zlovtnik/ggt/internal/transform/filter"`, `_ "github.com/zlovtnik/ggt/internal/transform/validate"`. Follow package path conventions like `internal/transform/{category}/` for unambiguous registration.
- For enrichment: Add to `internal/enrichment/` (e.g., `http/client.go` with circuit breaker pattern); implement caching for Redis/Postgres lookups.
- Pipeline wiring: Update `cmd/transform/main.go` `startPipelineWorker` if needed; transforms are auto-registered via blank imports in `cmd/transform/main.go` (see above).
- Always add unit tests (e.g., `*_test.go`) before integration; use `event.Builder` for test events.
- When adding config fields: Update `configs/config.example.yaml` first, then `internal/config/config.go` structs and validation.

## Signals for reviewers
- Config changes: Update `configs/config.example.yaml` first, sync `internal/config/config.go` tags and validation.
- New transforms/packages: Document in `devspec.md` and `docs/architecture.md` for design intent.
- Metrics/logging changes: Reference specific counter/histogram names from `internal/metrics/metrics.go` for dashboard compatibility.
- Code patterns: Prefer functional core (pure transforms) over imperative shell; use immutable events; follow existing error handling (wrap with context).

Please review these notes and let me know if any section feels unclear or needs more detail so I can iterate.