# Copilot Instructions for ggt

## Architecture Snapshot
- ggt is a Kafka-to-Kafka transform service; [cmd/transform/main.go](cmd/transform/main.go) wires config loading, consumer startup, pipeline execution, metrics, and health endpoints.
- Pipelines are built from config descriptors via [internal/transform/pipeline.go](internal/transform/pipeline.go); the pipeline clones [pkg/event/event.go](pkg/event/event.go) payloads so downstream transforms mutate their own copy and can emit nil or transform.ErrDrop to filter.
- Transforms live under [internal/transform](internal/transform) per category; each package has an init that calls transform.Register so blank imports in main keep registration automatic.
- Kafka IO uses franz-go: [internal/consumer/kafka.go](internal/consumer/kafka.go) handles manual commit-on-success with handler timeouts and rebalance callbacks, while [internal/producer/kafka.go](internal/producer/kafka.go) wraps produce sync with configurable acks.

## Daily Workflows
- Use make build or [scripts/build.sh](scripts/build.sh) to produce bin/ggt; make test and [scripts/test.sh](scripts/test.sh) run go test ./... with -race and coverage.
- Copy [configs/config.example.yaml](configs/config.example.yaml) to configs/config.yaml (or set TRANSFORM_CONFIG_PATH) before running bin/ggt; the loader in [internal/config/loader.go](internal/config/loader.go) auto-selects YAML or JSON and fills defaults.
- Run [scripts/run-benchmarks.sh](scripts/run-benchmarks.sh) for local performance baselines; results land in [benchmarks/results.txt](benchmarks/results.txt) and [BENCHMARK_REPORT.md](BENCHMARK_REPORT.md).
- curl the healthz endpoint on service.health_port and scrape metrics_port for Prometheus once main is running.

## Configuration Conventions
- The schema in [internal/config/config.go](internal/config/config.go) mirrors [configs/config.example.yaml](configs/config.example.yaml); update both together and adjust validation in [internal/config/validate.go](internal/config/validate.go) when adding fields.
- Pipelines config names input_topics to one output_topic and optional dlq_topic; each transform entry carries a type plus a JSON-like config object that Configure receives unchanged.
- Metrics buckets, enrichment pools, and Redis cache settings come from metrics.buckets and enrichment.* blocks; defaults are injected by applyDefaults in loader.go.
- Sensitive values such as Redis password and Postgres URL must remain empty in sample YAML and be provided via external secrets.

## Transform Patterns
- Implement transform.Transform with Name, Configure(json.RawMessage), and Execute(ctx, payload) living alongside peers; see [internal/transform/field/rename.go](internal/transform/field/rename.go) for structure and error handling.
- Register via transform.Register inside init; keep names unique and match the Type string in configuration exactly.
- Execute must accept interface{} but typically assert to event.Event; use event.SetField and event.RemoveField helpers to preserve immutability, and return ErrDrop (from [internal/transform/transform.go](internal/transform/transform.go)) when filters should short-circuit.
- Tests prefer [pkg/event/builder.go](pkg/event/builder.go) for fixture creation and expect transforms to be deterministic; table-driven tests live next to implementation with suffix _test.go.

## Observability and Operations
- Logging goes through zap configured in [internal/logging/logger.go](internal/logging/logger.go); log only metadata, never raw payloads.
- Prometheus counters and histograms originate from [internal/metrics/metrics.go](internal/metrics/metrics.go); use provided vectors to label by transform and pipeline rather than creating new registries.
- Health and metrics servers come from [internal/health/server.go](internal/health/server.go) and metrics.New(); ensure service.metrics_port > 0 to expose /metrics.
- Graceful shutdown relies on [internal/shutdown/coordinator.go](internal/shutdown/coordinator.go) coordinating consumer.Stop, producer.Close, and HTTP servers with context deadlines.

## Integration Touchpoints
- Enrichment adapters under [internal/enrichment](internal/enrichment) implement caching (LRU for Redis, circuit breaker for HTTP, pgx pools for Postgres); reuse those helpers instead of creating ad hoc clients.
- Filtering and routing transforms surface metrics via the counter vecs mentioned above; keep ErrDrop semantics consistent so pipeline-level DLQ handling can react correctly.
- When extending Kafka handling, reuse the abstractions in [internal/consumer/offset_manager.go](internal/consumer/offset_manager.go) and [internal/consumer/processor.go](internal/consumer/processor.go) to keep commit logic centralized.
- Always document new pipeline capabilities in [docs/architecture.md](docs/architecture.md) and [devspec.md](devspec.md) so future agents inherit accurate design context.