# Copilot Instructions for ggt

## Grasp the high-level service
- This repo implements the Transform Service described in [docs/architecture.md](docs/architecture.md) and wired out in [devspec.md](devspec.md); start there to understand the Kafka-to-Kafka flow, observability requirements, and the intended transform categories (field, data, enrichment, filter, validate).
- The service is intentionally minimal today: [cmd/transform/main.go](cmd/transform/main.go) starts the logger, metrics, and health servers, stubs a `pipelineWorker`, and relies on configuration to describe the actual Kafka input/output pairs.
- Future work should keep the Immutable Event → composable pipeline story from the spec (see the pipeline diagram and `TransformFunc`/`Pipeline` descriptions in `devspec.md`). Every stage should operate on an `pkg/event.Event` clone so metadata/offset information stays consistent while allowing pure transforms to mutate payloads.

## Local dev workflows
- `scripts/build.sh` simply runs `go build ./...`; use it to confirm the module compiles before opening PRs, or run `go test ./...` via `scripts/test.sh` for the current unit-test baseline.
- There is no deployment pipeline yet; `scripts/deploy.sh` prints a notice. Expect manual containerization/deployment plans to live in future directories like `deployments/`.
- The canonical config path is `configs/config.example.yaml`; `internal/config/loader.go` will default to that file and only use `configs/config.yaml` if you create one for your environment by copying the example.

## Configuration, secrets, and conventions
- The YAML schema inside `configs/config.example.yaml` mirrors `internal/config/config.go`. Keep runtime knobs here: service ports, Kafka brokers/consumer groups/topics, transforms pipelines, enrichment targets, and custom metric buckets.
- Sensitive data (Redis passwords, Postgres URLs) should be injected at deploy time and not committed. The config loader keeps password fields optional and the validator is expected to gate versioned secrets; key fields are documented with `omitempty` tags.
- Pipelines describe their transforms in the same YAML block; use the `type`/`config` pair to look up future `internal/transform/*` implementations. When adding a new transform, register it in the planned registry interface (`devspec.md` section 5) and keep its config unmarshaled into `TransformDescriptor`. Currently, field transforms like `field.rename`, `field.remove`, `field.add`, `field.flatten` are implemented in `internal/transform/field/`.

## Observability and runtime behavior
- Logs originate from `internal/logging/logger.go`, which wraps zap with `info`, `warn`, `error`, `debug` levels. Engineers expect structured-time stamping in ISO 8601 for every event during startup/shutdown.
- Metrics come from `internal/metrics/metrics.go` and are registered under the `transform_*` family (counters for processed/dropped/dlq, histograms per transform). The metrics server that exposes `/metrics` is wired in `cmd/transform/main.go` and gated on `service.metrics_port` being positive.
- Health probes share the same pattern: `service.health_port` controls `startHealthServer`, which only responds to `/healthz` and returns 200 if the server is alive.

## What to edit first when adding functionality
- Implementation conventions currently focus on creating reusable, immutable helpers: `pkg/event/event.go` provides `Clone`, `GetField`, `SetField`, and `RemoveField` helpers that pipelines must use instead of mutating maps directly. Transforms implement the `Transform` interface (`Name()`, `Configure()`, `Execute()`) or use `NewFunc` for simple functions; register them in `internal/transform/registry.go`.
- The `pipelineWorker` in `cmd/transform/main.go` is a placeholder; real work should flow through Kafka consumer/producer packages (implemented in `internal/consumer/` and `internal/producer/` but not yet wired into the worker) once the pipeline logic expands. For now start by adding unit tests around pure functions before wiring consumer loops.
- Keep shutdown consistent: the worker, metrics, and health servers share the `context` with `signal.NotifyContext`, so any new goroutines should obey the same cancellation pattern and call `Stop` with a `shutdownCtx` derived from `cfg.Service.ShutdownTimeout`.

## Signals for reviewers
- Pull in config changes by updating `configs/config.example.yaml` first and ensuring `internal/config/config.go` stays in sync with the YAML tags.
- Because much of the service is scaffolded, document any new packages or transforms in `devspec.md` and `docs/architecture.md` so reviewers can reconcile the design intent with the code changes.
- If a change touches metrics or logging, include the relevant counter/histogram names from `internal/metrics/metrics.go` to keep Prometheus dashboards stable.

Please review these notes and let me know if any section feels unclear or needs more detail so I can iterate.