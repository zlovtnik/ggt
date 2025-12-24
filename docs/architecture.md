# Architecture Overview

The Transform Service sits between Kafka and your data warehouse: a Kafka consumer ingests CDC changes, a configurable transformation pipeline runs in-memory, and the final payloads are produced back to Kafka. Observability, graceful shutdown, and config management live in shared packages so the runtime behavior stays consistent. [devspec.md](./devspec.md) contains the detailed specification of pipelines, transforms, and metrics.

## Core Components and Responsibilities
- **Entrypoint & lifecycle**: [cmd/transform/main.go](cmd/transform/main.go) loads the YAML configuration, constructs the logger, and spins up metrics/health servers plus the worker that keeps the consumer loop alive.
- **Configuration**: [internal/config/loader.go](internal/config/loader.go) centralizes the default config path, YAML deserialization, and fallback timeouts so every component agrees on brokers, topics, and timeouts.
- **Event model**: [pkg/event/event.go](pkg/event/event.go) provides immutable event helpers that each transform stage will consume and mutate safely while preserving metadata for offsets and keys.

## Design Patterns and Decision Drivers
Processing follows a stream-oriented model where the canonical Kafka offsets stay authoritative and every record passes through a linear series of transforms. Idempotency is achieved by letting stages operate on fully cloned `pkg/event` payloads so retries do not corrupt shared state, and schema evolution is handled by explicit validation/mapping steps configured per pipeline.

## Deployment and Scaling Considerations
Horizontal scaling happens at the consumer level: each instance joins the Kafka consumer group configured via the sample YAML, so increased partitions automatically split work. Tune `service.metrics_port` and `service.health_port` and other runtime knobs inside [configs/config.example.yaml](configs/config.example.yaml) before you deploy so metrics stay readable and the health server listens on the right interface.

## Error Handling and Failure Modes
Pipeline stages declare their DLQ topics, retries, and filtering behavior in the same sample configuration, which keeps Kafka errors predictable. Expect retries to follow the Kafka client backoff settings, long-running transforms to signal backpressure via bounded queues, and the main goroutine to trigger graceful shutdown through the same context that the metrics/health servers share.

## Shared Package Dependencies
[internal/logging/logger.go](internal/logging/logger.go) and [internal/metrics/metrics.go](internal/metrics/metrics.go) meaningfully instrument every component with consistent structured logs and Prometheus counters that `cmd/transform/main.go` wires into the ready-and-alive probes.
