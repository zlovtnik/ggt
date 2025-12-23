package config

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestConfigValidateSucceeds(t *testing.T) {
	cfg := baseConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}

func TestConfigValidateFails(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*Config)
		expectPart string
	}{
		{
			name: "missing service name",
			mutate: func(c *Config) {
				c.Service.Name = ""
			},
			expectPart: "service.name",
		},
		{
			name: "invalid log level",
			mutate: func(c *Config) {
				c.Service.LogLevel = "verbose"
			},
			expectPart: "log_level",
		},
		{
			name: "zero shutdown timeout",
			mutate: func(c *Config) {
				c.Service.ShutdownTimeout = 0
			},
			expectPart: "shutdown_timeout",
		},
		{
			name: "no producer brokers",
			mutate: func(c *Config) {
				c.Kafka.Producer.Brokers = nil
			},
			expectPart: "producer.brokers",
		},
		{
			name: "invalid session timeout",
			mutate: func(c *Config) {
				c.Kafka.Consumer.SessionTimeout = "invalid"
			},
			expectPart: "session_timeout",
		},
		{
			name: "missing pipeline output topic",
			mutate: func(c *Config) {
				c.Transforms.Pipelines[0].OutputTopic = ""
			},
			expectPart: "output_topic",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected error for %s", tt.name)
			}
			if tt.expectPart != "" && !strings.Contains(err.Error(), tt.expectPart) {
				t.Fatalf("expected error mentioning %q, got %v", tt.expectPart, err)
			}
		})
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv(envPrefix+"_SERVICE_LOG_LEVEL", "debug")
	t.Setenv(envPrefix+"_KAFKA_CONSUMER_TOPICS", "foo,bar")

	cfg := baseConfig()
	if err := cfg.ApplyEnvOverrides(); err != nil {
		t.Fatalf("apply overrides: %v", err)
	}
	if cfg.Service.LogLevel != "debug" {
		t.Fatalf("expected log level override, got %s", cfg.Service.LogLevel)
	}
	if len(cfg.Kafka.Consumer.Topics) != 2 || cfg.Kafka.Consumer.Topics[0] != "foo" || cfg.Kafka.Consumer.Topics[1] != "bar" {
		t.Fatalf("expected comma separated topics, got %v", cfg.Kafka.Consumer.Topics)
	}
}

func TestApplyEnvOverridesErrors(t *testing.T) {
	t.Setenv(envPrefix+"_SERVICE_METRICS_PORT", "not-a-port")
	cfg := baseConfig()
	if err := cfg.ApplyEnvOverrides(); err == nil {
		t.Fatalf("expected error when overriding metrics port")
	}
}

func baseConfig() Config {
	return Config{
		Service: ServiceConfig{
			Name:            "transform-service",
			LogLevel:        "info",
			MetricsPort:     9090,
			HealthPort:      8080,
			ShutdownTimeout: 30 * time.Second,
		},
		Kafka: KafkaConfig{
			Consumer: ConsumerConfig{
				Brokers:         []string{"localhost:9092"},
				GroupID:         "transform-service",
				Topics:          []string{"raw.events"},
				SessionTimeout:  "45s",
				MaxPollInterval: "5m",
			},
			Producer: ProducerConfig{
				Brokers:     []string{"localhost:9092"},
				Compression: "snappy",
				Idempotent:  true,
				MaxInFlight: 5,
				BatchSize:   16384,
				LingerMs:    10,
				Acks:        "all",
			},
		},
		Transforms: TransformConfig{
			Pipelines: []PipelineConfig{
				{
					Name:        "sample",
					InputTopics: []string{"raw.sample"},
					OutputTopic: "enriched.sample",
					Transforms: []TransformDescriptor{
						{
							Type:   "field.rename",
							Config: json.RawMessage(`{"mappings": {"foo": "bar"}}`),
						},
					},
				},
			},
		},
		Enrichment: EnrichmentConfig{
			Redis: map[string]RedisEnrichment{
				"default": {
					Addr:       "localhost:6379",
					DB:         0,
					MaxRetries: 1,
					PoolSize:   1,
				},
			},
			HTTP: HTTPEnrichment{DefaultTimeout: "1s"},
			Postgres: PostgresEnrichment{
				URL:             "postgres://user:pass@localhost:5432/db",
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: 5 * time.Minute,
			},
		},
		Metrics: MetricsConfig{
			Namespace: "transform_service",
			Subsystem: "pipeline",
			Buckets:   map[string][]float64{"latency": {0.001, 0.01, 0.1}},
		},
	}
}
