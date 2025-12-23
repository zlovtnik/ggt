Transform Service Development Specification
Executive Summary
This specification outlines the development of a standalone Transform Service in Go that will sit between your Rust CDC ingestion pipeline and PostgreSQL. The service will consume raw CDC events from Kafka, apply configurable transformations, and produce enriched events back to Kafka for downstream consumption.

1. Architecture Overview
1.1 High-Level Design
┌─────────────────────────────────────────────────────────────────┐
│                    Transform Service (Go)                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────┐    ┌───────────────┐    ┌─────────────────┐  │
│  │   Consumer   │───▶│  Transform    │───▶│    Producer     │  │
│  │   (Kafka)    │    │   Pipeline    │    │    (Kafka)      │  │
│  └──────────────┘    └───────────────┘    └─────────────────┘  │
│         │                    │                      │            │
│         │                    │                      │            │
│         ▼                    ▼                      ▼            │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Shared Infrastructure                        │  │
│  │  • Metrics (Prometheus)  • Health Checks                 │  │
│  │  • Config Management     • Graceful Shutdown             │  │
│  │  • Logging (structured)  • Error Handling                │  │
│  └──────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
1.2 Design Principles

Functional Core, Imperative Shell: Pure transformation functions wrapped in I/O handling
Immutability: Transform functions receive and return immutable data structures
Composability: Transformations are composable functions that can be chained
Type Safety: Leverage Go's type system for compile-time guarantees
Observability: Comprehensive metrics, logging, and tracing
Testability: Pure functions are easily unit-testable; integration tests use testcontainers


2. Core Components
2.1 Transform Pipeline Architecture
go// Core domain types (functional/immutable)
type Event struct {
    Payload   map[string]interface{}
    Metadata  EventMetadata
    Timestamp time.Time
}

type EventMetadata struct {
    Topic     string
    Partition int32
    Offset    int64
    Key       string
}

// Transform function signature
type TransformFunc func(Event) (Event, error)

// Composable pipeline
type Pipeline struct {
    stages []TransformFunc
}

func (p Pipeline) Execute(event Event) (Event, error) {
    result := event
    for _, stage := range p.stages {
        transformed, err := stage(result)
        if err != nil {
            return Event{}, err
        }
        result = transformed
    }
    return result, nil
}
```

### 2.2 Transform Function Categories

#### 2.2.1 Field Transformations
- **Rename**: Change field names
- **Remove**: Delete fields
- **Add**: Insert computed or static fields
- **Flatten**: Flatten nested structures
- **Unflatten**: Convert flat structures to nested

#### 2.2.2 Data Transformations
- **Type Conversion**: String ↔ Number ↔ Boolean
- **Format**: Date formatting, string manipulation
- **Encode/Decode**: Base64, URL encoding
- **Hash**: SHA256, MD5 for sensitive data
- **Mask**: PII masking (credit cards, SSN)

#### 2.2.3 Enrichment
- **Lookup**: External service calls (Redis, HTTP APIs, Postgres)
- **Join**: Combine with reference data
- **Geolocation**: IP → Location
- **Time**: Timezone conversions, time parsing

#### 2.2.4 Filtering & Routing
- **Filter**: Conditional message dropping
- **Route**: Dynamic topic routing based on content
- **Split**: One message → Multiple messages
- **Aggregate**: Multiple messages → One (windowed)

#### 2.2.5 Validation
- **Schema Validation**: JSON Schema, custom validators
- **Business Rules**: Custom validation logic
- **Required Fields**: Ensure presence of critical fields

---

## 3. Project Structure
```
transform-service/
├── cmd/
│   ├── transform/
│   │   └── main.go                 # Service entry point
│   └── tools/
│       ├── validate-config/        # Config validation tool
│       └── test-transform/         # Transform testing CLI
├── internal/
│   ├── config/
│   │   ├── config.go               # Configuration structs
│   │   ├── loader.go               # Load from file/env
│   │   └── validator.go            # Config validation
│   ├── consumer/
│   │   ├── kafka.go                # Kafka consumer wrapper
│   │   ├── processor.go            # Message processing loop
│   │   └── offset_manager.go      # Offset commit coordination
│   ├── producer/
│   │   ├── kafka.go                # Kafka producer wrapper
│   │   └── batch.go                # Batching logic
│   ├── transform/
│   │   ├── pipeline.go             # Pipeline orchestration
│   │   ├── registry.go             # Transform function registry
│   │   ├── context.go              # Transform execution context
│   │   ├── field/
│   │   │   ├── rename.go           # Field rename transforms
│   │   │   ├── remove.go           # Field removal
│   │   │   ├── add.go              # Field addition
│   │   │   ├── flatten.go          # Structure flattening
│   │   │   └── extract.go          # Nested field extraction
│   │   ├── data/
│   │   │   ├── convert.go          # Type conversions
│   │   │   ├── format.go           # Formatting (dates, strings)
│   │   │   ├── encode.go           # Encoding/decoding
│   │   │   ├── hash.go             # Hashing functions
│   │   │   └── mask.go             # PII masking
│   │   ├── enrich/
│   │   │   ├── lookup.go           # External lookups
│   │   │   ├── redis.go            # Redis enrichment
│   │   │   ├── http.go             # HTTP API enrichment
│   │   │   ├── postgres.go         # Postgres lookup
│   │   │   └── cache.go            # Lookup caching
│   │   ├── filter/
│   │   │   ├── condition.go        # Conditional filtering
│   │   │   ├── route.go            # Dynamic routing
│   │   │   └── split.go            # Message splitting
│   │   └── validate/
│   │       ├── schema.go           # JSON Schema validation
│   │       ├── rules.go            # Business rules
│   │       └── required.go         # Required fields check
│   ├── metrics/
│   │   ├── prometheus.go           # Prometheus metrics
│   │   └── recorder.go             # Metrics recording
│   ├── health/
│   │   ├── checker.go              # Health check logic
│   │   └── http.go                 # HTTP health endpoint
│   ├── shutdown/
│   │   └── coordinator.go          # Graceful shutdown
│   └── logging/
│       └── logger.go               # Structured logging setup
├── pkg/
│   ├── event/
│   │   ├── event.go                # Core event types (exported)
│   │   ├── metadata.go             # Metadata types
│   │   └── builder.go              # Event builder (tests)
│   └── errors/
│       ├── errors.go               # Custom error types
│       └── codes.go                # Error codes
├── test/
│   ├── integration/
│   │   ├── kafka_test.go           # Kafka integration tests
│   │   ├── postgres_test.go        # Postgres lookup tests
│   │   └── e2e_test.go             # End-to-end tests
│   ├── fixtures/
│   │   ├── events.json             # Test event fixtures
│   │   └── configs/                # Test configurations
│   └── testutil/
│       ├── kafka.go                # Kafka test helpers
│       ├── postgres.go             # Postgres test helpers
│       └── assertions.go           # Custom assertions
├── configs/
│   ├── config.example.yaml         # Example configuration
│   ├── transforms/
│   │   ├── user_transforms.yaml    # Example user transforms
│   │   └── order_transforms.yaml   # Example order transforms
│   └── prometheus.yaml             # Prometheus scrape config
├── scripts/
│   ├── build.sh                    # Build script
│   ├── test.sh                     # Test runner
│   └── deploy.sh                   # Deployment script
├── deployments/
│   ├── docker/
│   │   └── Dockerfile              # Production Dockerfile
│   └── kubernetes/
│       ├── deployment.yaml         # K8s deployment
│       ├── service.yaml            # K8s service
│       └── configmap.yaml          # K8s config
├── docs/
│   ├── architecture.md             # Architecture docs
│   ├── transforms.md               # Transform reference
│   ├── configuration.md            # Config reference
│   └── operations.md               # Ops runbook
├── go.mod
├── go.sum
├── Makefile                        # Common tasks
└── README.md

4. Configuration Schema
4.1 Main Configuration (YAML)
yamlservice:
  name: transform-service
  log_level: info
  metrics_port: 9090
  health_port: 8080
  shutdown_timeout: 30s

kafka:
  consumer:
    brokers:
      - localhost:9092
    group_id: transform-service
    topics:
      - raw.events
    session_timeout: 45s
    max_poll_interval: 5m
    auto_offset_reset: earliest
    enable_auto_commit: false
  
  producer:
    brokers:
      - localhost:9092
    compression: snappy
    idempotent: true
    max_in_flight: 5
    batch_size: 16384
    linger_ms: 10
    acks: all

transforms:
  pipelines:
    - name: user_enrichment
      input_topics:
        - raw.users
      output_topic: enriched.users
      dlq_topic: dlq.users
      transforms:
        - type: field.rename
          config:
            mappings:
              user_id: id
              user_name: name
        
        - type: enrich.lookup
          config:
            source: redis
            key_field: id
            cache_ttl: 5m
            redis:
              addr: localhost:6379
              db: 0
            target_field: user_metadata
        
        - type: data.mask
          config:
            fields:
              - email
              - phone
            mask_type: partial
            preserve_length: true
        
        - type: validate.schema
          config:
            schema_file: schemas/user.json
            on_error: dlq

    - name: order_processing
      input_topics:
        - raw.orders
      output_topic: enriched.orders
      dlq_topic: dlq.orders
      transforms:
        - type: field.flatten
          config:
            field: shipping_address
            prefix: shipping_
        
        - type: data.convert
          config:
            fields:
              - name: total_amount
                from: string
                to: float64
              - name: order_date
                from: string
                to: timestamp
                format: "2006-01-02T15:04:05Z"
        
        - type: enrich.http
          config:
            url: "http://inventory-service/api/v1/products/{{.product_id}}"
            method: GET
            timeout: 500ms
            cache_ttl: 10m
            target_field: product_info
        
        - type: filter.condition
          config:
            condition: "payload.status == 'completed'"
            on_false: drop

enrichment:
  redis:
    default:
      addr: localhost:6379
      password: ""
      db: 0
      max_retries: 3
      pool_size: 10
  
  http:
    default_timeout: 1s
    max_idle_conns: 100
    max_conns_per_host: 10
  
  postgres:
    url: "postgres://user:pass@localhost:5432/db"
    max_open_conns: 10
    max_idle_conns: 5
    conn_max_lifetime: 1h

metrics:
  namespace: transform_service
  subsystem: pipeline
  buckets:
    latency: [0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10]
    size: [100, 500, 1000, 5000, 10000, 50000, 100000]

5. Key Interfaces and Types
5.1 Transform Function Interface
gopackage transform

import (
    "context"
    "github.com/yourusername/transform-service/pkg/event"
)

// Transform represents a single transformation stage
type Transform interface {
    // Name returns the transform identifier
    Name() string
    
    // Apply executes the transformation
    Apply(ctx context.Context, evt event.Event) (event.Event, error)
    
    // Validate checks if the transform configuration is valid
    Validate() error
}

// Factory creates transforms from configuration
type Factory interface {
    Create(config map[string]interface{}) (Transform, error)
}

// Registry manages available transforms
type Registry interface {
    Register(name string, factory Factory)
    Get(name string) (Factory, error)
    List() []string
}
5.2 Event Structure
gopackage event

import (
    "time"
)

// Event represents a single message to be transformed
type Event struct {
    // Payload is the message data (immutable)
    Payload map[string]interface{}
    
    // Metadata contains Kafka metadata
    Metadata Metadata
    
    // Timestamp is when the event was received
    Timestamp time.Time
    
    // Headers are Kafka message headers
    Headers map[string]string
}

// Metadata contains Kafka-specific metadata
type Metadata struct {
    Topic     string
    Partition int32
    Offset    int64
    Key       []byte
}

// Clone creates a deep copy of the event
func (e Event) Clone() Event {
    // Deep copy implementation
}

// GetField retrieves a nested field value
func (e Event) GetField(path string) (interface{}, bool) {
    // Path like "user.address.city"
}

// SetField sets a nested field value (returns new Event)
func (e Event) SetField(path string, value interface{}) Event {
    // Returns modified copy
}
5.3 Pipeline Execution
gopackage transform

import (
    "context"
    "github.com/yourusername/transform-service/pkg/event"
)

// Pipeline executes a series of transforms
type Pipeline struct {
    name       string
    transforms []Transform
    metrics    MetricsRecorder
}

// Execute runs all transforms in sequence
func (p *Pipeline) Execute(ctx context.Context, evt event.Event) (event.Event, error) {
    result := evt
    
    for i, transform := range p.transforms {
        // Record metrics
        start := time.Now()
        
        transformed, err := transform.Apply(ctx, result)
        if err != nil {
            p.metrics.RecordError(transform.Name(), err)
            return event.Event{}, &PipelineError{
                Stage:   i,
                Name:    transform.Name(),
                Wrapped: err,
            }
        }
        
        p.metrics.RecordDuration(transform.Name(), time.Since(start))
        result = transformed
    }
    
    return result, nil
}

// ExecuteAsync runs transforms concurrently where possible
func (p *Pipeline) ExecuteAsync(ctx context.Context, evt event.Event) (event.Event, error) {
    // Implementation for independent transforms
}

6. Example Transform Implementations
6.1 Field Rename Transform
gopackage field

import (
    "context"
    "github.com/yourusername/transform-service/pkg/event"
)

type RenameConfig struct {
    Mappings map[string]string `yaml:"mappings"`
}

type RenameTransform struct {
    config RenameConfig
}

func (t *RenameTransform) Name() string {
    return "field.rename"
}

func (t *RenameTransform) Apply(ctx context.Context, evt event.Event) (event.Event, error) {
    result := evt.Clone()
    
    for oldName, newName := range t.config.Mappings {
        if value, ok := result.GetField(oldName); ok {
            result = result.SetField(newName, value)
            result = result.RemoveField(oldName)
        }
    }
    
    return result, nil
}

func (t *RenameTransform) Validate() error {
    if len(t.config.Mappings) == 0 {
        return errors.New("mappings cannot be empty")
    }
    return nil
}
6.2 Redis Enrichment Transform
gopackage enrich

import (
    "context"
    "encoding/json"
    "github.com/go-redis/redis/v8"
    "github.com/yourusername/transform-service/pkg/event"
)

type RedisLookupConfig struct {
    Addr        string        `yaml:"addr"`
    DB          int           `yaml:"db"`
    KeyField    string        `yaml:"key_field"`
    TargetField string        `yaml:"target_field"`
    CacheTTL    time.Duration `yaml:"cache_ttl"`
}

type RedisLookupTransform struct {
    config RedisLookupConfig
    client *redis.Client
    cache  *cache.LRU
}

func (t *RedisLookupTransform) Name() string {
    return "enrich.redis"
}

func (t *RedisLookupTransform) Apply(ctx context.Context, evt event.Event) (event.Event, error) {
    // Extract key from event
    keyValue, ok := evt.GetField(t.config.KeyField)
    if !ok {
        return evt, nil // Key not found, skip enrichment
    }
    
    key := fmt.Sprintf("%v", keyValue)
    
    // Check cache first
    if cached, found := t.cache.Get(key); found {
        return evt.SetField(t.config.TargetField, cached), nil
    }
    
    // Lookup from Redis
    result, err := t.client.Get(ctx, key).Result()
    if err == redis.Nil {
        return evt, nil // Key not found in Redis, skip
    }
    if err != nil {
        return event.Event{}, fmt.Errorf("redis lookup failed: %w", err)
    }
    
    // Parse result
    var enrichmentData map[string]interface{}
    if err := json.Unmarshal([]byte(result), &enrichmentData); err != nil {
        return event.Event{}, fmt.Errorf("failed to parse enrichment data: %w", err)
    }
    
    // Cache result
    t.cache.Add(key, enrichmentData)
    
    // Add to event
    return evt.SetField(t.config.TargetField, enrichmentData), nil
}

func (t *RedisLookupTransform) Validate() error {
    if t.config.KeyField == "" {
        return errors.New("key_field is required")
    }
    if t.config.TargetField == "" {
        return errors.New("target_field is required")
    }
    return nil
}
6.3 JSON Schema Validation Transform
gopackage validate

import (
    "context"
    "github.com/xeipuuv/gojsonschema"
    "github.com/yourusername/transform-service/pkg/event"
)

type SchemaConfig struct {
    SchemaFile string `yaml:"schema_file"`
    OnError    string `yaml:"on_error"` // "drop", "dlq", "fail"
}

type SchemaTransform struct {
    config SchemaConfig
    schema *gojsonschema.Schema
}

func (t *SchemaTransform) Name() string {
    return "validate.schema"
}

func (t *SchemaTransform) Apply(ctx context.Context, evt event.Event) (event.Event, error) {
    documentLoader := gojsonschema.NewGoLoader(evt.Payload)
    
    result, err := t.schema.Validate(documentLoader)
    if err != nil {
        return event.Event{}, fmt.Errorf("validation error: %w", err)
    }
    
    if !result.Valid() {
        validationErr := &ValidationError{
            Errors: result.Errors(),
        }
        
        switch t.config.OnError {
        case "drop":
            return event.Event{}, ErrDropMessage
        case "dlq":
            return event.Event{}, &DLQError{Wrapped: validationErr}
        case "fail":
            return event.Event{}, validationErr
        default:
            return event.Event{}, validationErr
        }
    }
    
    return evt, nil
}

func (t *SchemaTransform) Validate() error {
    if t.config.SchemaFile == "" {
        return errors.New("schema_file is required")
    }
    
    // Load and validate schema
    schemaLoader := gojsonschema.NewReferenceLoader("file://" + t.config.SchemaFile)
    schema, err := gojsonschema.NewSchema(schemaLoader)
    if err != nil {
        return fmt.Errorf("invalid schema: %w", err)
    }
    
    t.schema = schema
    return nil
}

7. Metrics and Observability
7.1 Prometheus Metrics
gopackage metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
    // Message metrics
    MessagesProcessed prometheus.Counter
    MessagesDropped   prometheus.Counter
    MessagesSentToDLQ prometheus.Counter
    
    // Transform metrics
    TransformDuration *prometheus.HistogramVec
    TransformErrors   *prometheus.CounterVec
    
    // Pipeline metrics
    PipelineDuration prometheus.Histogram
    PipelineThroughput prometheus.Gauge
    
    // Kafka metrics
    KafkaConsumerLag prometheus.Gauge
    KafkaProducerBatchSize prometheus.Histogram
    
    // Enrichment metrics
    EnrichmentCacheHits   *prometheus.CounterVec
    EnrichmentCacheMisses *prometheus.CounterVec
    EnrichmentLatency     *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
    return &Metrics{
        MessagesProcessed: promauto.NewCounter(prometheus.CounterOpts{
            Name: "transform_messages_processed_total",
            Help: "Total number of messages processed",
        }),
        
        TransformDuration: promauto.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: "transform_duration_seconds",
                Help: "Duration of individual transforms",
                Buckets: prometheus.DefBuckets,
            },
            []string{"transform_name", "pipeline"},
        ),
        
        // ... other metrics
    }
}
7.2 Structured Logging
gopackage logging

import (
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func NewLogger(level string) (*zap.Logger, error) {
    config := zap.NewProductionConfig()
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    
    switch level {
    case "debug":
        config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
    case "info":
        config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    case "warn":
        config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
    case "error":
        config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
    default:
        config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    }
    
    return config.Build()
}

// Logging helpers
func LogTransformStart(logger *zap.Logger, pipeline, transform string, evt event.Event) {
    logger.Debug("transform_start",
        zap.String("pipeline", pipeline),
        zap.String("transform", transform),
        zap.String("topic", evt.Metadata.Topic),
        zap.Int32("partition", evt.Metadata.Partition),
        zap.Int64("offset", evt.Metadata.Offset),
    )
}

8. Testing Strategy
8.1 Unit Tests (Pure Functions)
gopackage field_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/yourusername/transform-service/internal/transform/field"
    "github.com/yourusername/transform-service/pkg/event"
)

func TestRenameTransform(t *testing.T) {
    tests := []struct {
        name     string
        config   field.RenameConfig
        input    event.Event
        expected event.Event
        wantErr  bool
    }{
        {
            name: "simple rename",
            config: field.RenameConfig{
                Mappings: map[string]string{
                    "old_name": "new_name",
                },
            },
            input: event.Event{
                Payload: map[string]interface{}{
                    "old_name": "value",
                    "other": "data",
                },
            },
            expected: event.Event{
                Payload: map[string]interface{}{
                    "new_name": "value",
                    "other": "data",
                },
            },
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            transform := &field.RenameTransform{Config: tt.config}
            result, err := transform.Apply(context.Background(), tt.input)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tt.expected.Payload, result.Payload)
        })
    }
}
8.2 Integration Tests (Testcontainers)
gopackage integration_test

import (
    "context"
    "testing"
    "time"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/kafka"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestKafkaIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Start Kafka container
    kafkaContainer, err := kafka.RunContainer(ctx,
        kafka.WithClusterID("test-cluster"),
        testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer kafkaContainer.Terminate(ctx)
    
    brokers, err := kafkaContainer.Brokers(ctx)
    if err != nil {
        t.Fatal(err)
    }
    
    // Test transform service with real Kafka
    // ... test implementation
}

func TestRedisEnrichment(t *testing.T) {
    ctx := context.Background()
    
    // Start Redis container
    redisContainer, err := redis.RunContainer(ctx,
        testcontainers.WithImage("redis:7-alpine"),
    )
    if err != nil {
        t.Fatal(err)
    }
    defer redisContainer.Terminate(ctx)
    
    endpoint, err := redisContainer.Endpoint(ctx, "")
    if err != nil {
        t.Fatal(err)
    }
    
    // Test Redis enrichment transform
    // ... test implementation
}

9. Task List
Phase 1: Project Setup & Core Infrastructure (Week 1)
1.1 Project Initialization

 Initialize Go module (go mod init)
 Set up directory structure
 Create Makefile with common tasks
 Set up CI/CD pipeline (GitHub Actions / GitLab CI)
 Configure golangci-lint
 Set up pre-commit hooks

1.2 Configuration Management

 Implement config structs with validation
 Create YAML/JSON config loader
 Add environment variable override support
 Write config validation tests
 Create example configuration files

1.3 Logging and Metrics

 Set up structured logging (zap)
 Initialize Prometheus metrics
 Create metrics HTTP server
 Implement health check endpoints
 Add graceful shutdown coordinator


Phase 2: Kafka Integration (Week 1-2)
2.1 Kafka Consumer

 Implement Kafka consumer wrapper (Sarama / franz-go)
 Add offset management
 Implement consumer group rebalancing
 Add backpressure handling
 Write consumer integration tests

2.2 Kafka Producer

 Implement Kafka producer wrapper
 Add batching and compression
 Implement retry logic with backoff
 Add producer metrics
 Write producer integration tests

2.3 Message Processing Loop

 Create message processor
 Implement error handling and DLQ
 Add graceful shutdown support
 Implement offset commit strategy
 Write end-to-end tests


Phase 3: Transform Core (Week 2-3)
3.1 Event Model

 Define Event and Metadata structs
 Implement Event.Clone() for immutability
 Implement GetField/SetField with nested paths
 Add Event builder for tests
 Write comprehensive unit tests

3.2 Transform Framework

 Define Transform interface
 Implement Pipeline execution
 Create transform Registry
 Add Factory pattern for transform creation
 Implement context propagation

3.3 Transform Metrics

 Add per-transform duration metrics
 Add error counters
 Implement pipeline-level metrics
 Add throughput gauges


Phase 4: Field Transforms (Week 3)
4.1 Basic Field Operations

 Implement field.rename
 Implement field.remove
 Implement field.add
 Write unit tests for each