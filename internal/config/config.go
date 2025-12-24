package config

import (
	"time"
)

// ServiceConfig captures the top-level service parameters.
type ServiceConfig struct {
	Name            string        `yaml:"name" json:"name"`
	LogLevel        string        `yaml:"log_level" json:"log_level"`
	MetricsPort     int           `yaml:"metrics_port" json:"metrics_port"`
	HealthPort      int           `yaml:"health_port" json:"health_port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" json:"shutdown_timeout"`
}

type ConsumerConfig struct {
	Brokers          []string `yaml:"brokers" json:"brokers"`
	GroupID          string   `yaml:"group_id" json:"group_id"`
	Topics           []string `yaml:"topics" json:"topics"`
	SessionTimeout   string   `yaml:"session_timeout" json:"session_timeout"`
	MaxPollInterval  string   `yaml:"max_poll_interval" json:"max_poll_interval"`
	AutoOffsetReset  string   `yaml:"auto_offset_reset" json:"auto_offset_reset"`
	EnableAutoCommit bool     `yaml:"enable_auto_commit" json:"enable_auto_commit"`
	FetchMaxRecords  int      `yaml:"fetch_max_records" json:"fetch_max_records"`
	PollIntervalMs   int      `yaml:"poll_interval_ms" json:"poll_interval_ms"`
}

// ParsedSessionTimeout returns the parsed session timeout duration.
func (c ConsumerConfig) ParsedSessionTimeout() (time.Duration, error) {
	return time.ParseDuration(c.SessionTimeout)
}

// ParsedMaxPollInterval returns the parsed max poll interval duration.
func (c ConsumerConfig) ParsedMaxPollInterval() (time.Duration, error) {
	return time.ParseDuration(c.MaxPollInterval)
}

type ProducerConfig struct {
	Brokers     []string `yaml:"brokers" json:"brokers"`
	Compression string   `yaml:"compression" json:"compression"`
	Idempotent  bool     `yaml:"idempotent" json:"idempotent"`
	MaxInFlight int      `yaml:"max_in_flight" json:"max_in_flight"`
	BatchSize   int      `yaml:"batch_size" json:"batch_size"`
	LingerMs    int      `yaml:"linger_ms" json:"linger_ms"`
	Acks        string   `yaml:"acks" json:"acks"`
}

type KafkaConfig struct {
	Consumer ConsumerConfig `yaml:"consumer" json:"consumer"`
	Producer ProducerConfig `yaml:"producer" json:"producer"`
}

type TransformDescriptor struct {
	Type   string      `yaml:"type" json:"type"`
	Config interface{} `yaml:"config" json:"config"`
}

type PipelineConfig struct {
	Name        string                `yaml:"name" json:"name"`
	InputTopics []string              `yaml:"input_topics" json:"input_topics"`
	OutputTopic string                `yaml:"output_topic" json:"output_topic"`
	DLQTopic    string                `yaml:"dlq_topic" json:"dlq_topic"`
	Transforms  []TransformDescriptor `yaml:"transforms" json:"transforms"`
}

type TransformConfig struct {
	Pipelines []PipelineConfig `yaml:"pipelines" json:"pipelines"`
}

// RedisEnrichment describes how a transform can talk to Redis for lookups.
// Password, if present, is sensitive and must be provisioned via environment variables, secret managers,
// or encrypted configuration; it should never be committed in plaintext or logged.
type RedisEnrichment struct {
	Addr string `yaml:"addr" json:"addr"`
	// Password is sensitive and should only be stored in secure secrets or encrypted stores.
	Password   string        `yaml:"password,omitempty" json:"password,omitempty"`
	DB         int           `yaml:"db" json:"db"`
	MaxRetries int           `yaml:"max_retries" json:"max_retries"`
	PoolSize   int           `yaml:"pool_size" json:"pool_size"`
	CacheTTL   time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	CacheSize  int           `yaml:"cache_size" json:"cache_size"`
}

type HTTPEnrichment struct {
	DefaultTimeout  string `yaml:"default_timeout" json:"default_timeout"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	MaxConnsPerHost int    `yaml:"max_conns_per_host" json:"max_conns_per_host"`
	MaxResponseSize int64  `yaml:"max_response_size" json:"max_response_size"`
}

// PostgresEnrichment captures the PostgreSQL connection details used by enrichment transforms.
// URL includes credentials and other sensitive data, so it must come from secure sources and never be checked into
// version control or logged in clear text.
type PostgresEnrichment struct {
	// URL contains the full connection string and should be supplied via secrets or encrypted configuration.
	URL             string        `yaml:"url,omitempty" json:"url,omitempty"`
	MaxOpenConns    int           `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	QueryTimeout    time.Duration `yaml:"query_timeout" json:"query_timeout"`
}

type EnrichmentConfig struct {
	Redis    map[string]RedisEnrichment `yaml:"redis" json:"redis"`
	HTTP     HTTPEnrichment             `yaml:"http" json:"http"`
	Postgres PostgresEnrichment         `yaml:"postgres" json:"postgres"`
}

type MetricsConfig struct {
	Namespace string               `yaml:"namespace" json:"namespace"`
	Subsystem string               `yaml:"subsystem" json:"subsystem"`
	Buckets   map[string][]float64 `yaml:"buckets" json:"buckets"`
}

type Config struct {
	Service    ServiceConfig    `yaml:"service" json:"service"`
	Kafka      KafkaConfig      `yaml:"kafka" json:"kafka"`
	Transforms TransformConfig  `yaml:"transforms" json:"transforms"`
	Enrichment EnrichmentConfig `yaml:"enrichment" json:"enrichment"`
	Metrics    MetricsConfig    `yaml:"metrics" json:"metrics"`
}
