# Apache JMeter Load Tests

This folder contains a reusable Apache JMeter test plan for exercising the transform service’s HTTP surfaces while the Kafka pipelines are running locally.

## Prerequisites

- Apache JMeter 5.6 or newer installed (`brew install jmeter` on macOS).
- The transform service running locally (for example `make run`).
- Kafka and any upstream dependencies running so the service remains healthy during the test.

## Files

- `transform-service.jmx` – JMeter plan that
  - probes `/healthz` on the configured health port to confirm readiness;
  - polls `/metrics` to generate HTTP load and verify Prometheus endpoint stability.

## Running the Plan Headlessly

```bash
jmeter -n \
  -t loadtests/jmeter/transform-service.jmx \
  -Jprotocol=http \
  -Jhost=localhost \
  -Jport=8085 \
  -Jthreads=10 \
  -JdurationSeconds=60 \
  -l loadtests/jmeter/results.jtl
```
- `protocol`, `host`, and `port` point the samplers at the target service. The default `8085` mirrors `service.health_port` in `configs/config.yaml`; override `-Jport` if your instance serves `/healthz` on a different port (for example via `TRANSFORM_SERVICE_HEALTH_PORT`).
- `threads` configures concurrent users.
- `durationSeconds` sets how long the thread group runs before stopping.
- `results.jtl` captures response metrics for later analysis (optional).

You can open the same plan in the JMeter GUI for exploratory testing:

```bash
jmeter -t loadtests/jmeter/transform-service.jmx
```

## Extending the Plan

- Add HTTP samplers that publish test events to any supporting admin endpoints you expose.
- Install the [JMeter Kafka plugin](https://github.com/streamnative/kafka-jmeter) if you want to generate Kafka traffic directly from JMeter threads; you can then append Kafka Producer and Consumer samplers to the existing thread group.
- Adjust timers, assertions, and listeners to match your SLOs before running in CI/CD.
