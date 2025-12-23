package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const envPrefix = "TRANSFORM"

var envOverrides = []struct {
	key   string
	apply func(*Config, string) error
}{
	{key: envPrefix + "_SERVICE_NAME", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		cfg.Service.Name = raw
		return nil
	}},
	{key: envPrefix + "_SERVICE_LOG_LEVEL", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		cfg.Service.LogLevel = raw
		return nil
	}},
	{key: envPrefix + "_SERVICE_METRICS_PORT", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		port, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse metrics port: %w", err)
		}
		cfg.Service.MetricsPort = port
		return nil
	}},
	{key: envPrefix + "_SERVICE_HEALTH_PORT", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		port, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse health port: %w", err)
		}
		cfg.Service.HealthPort = port
		return nil
	}},
	{key: envPrefix + "_SERVICE_SHUTDOWN_TIMEOUT", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("parse shutdown timeout: %w", err)
		}
		cfg.Service.ShutdownTimeout = d
		return nil
	}},
	{key: envPrefix + "_KAFKA_CONSUMER_BROKERS", apply: func(cfg *Config, raw string) error {
		cfg.Kafka.Consumer.Brokers = parseCSV(raw)
		return nil
	}},
	{key: envPrefix + "_KAFKA_CONSUMER_GROUP_ID", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		cfg.Kafka.Consumer.GroupID = raw
		return nil
	}},
	{key: envPrefix + "_KAFKA_CONSUMER_TOPICS", apply: func(cfg *Config, raw string) error {
		cfg.Kafka.Consumer.Topics = parseCSV(raw)
		return nil
	}},
	{key: envPrefix + "_KAFKA_PRODUCER_BROKERS", apply: func(cfg *Config, raw string) error {
		cfg.Kafka.Producer.Brokers = parseCSV(raw)
		return nil
	}},
	{key: envPrefix + "_METRICS_NAMESPACE", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		cfg.Metrics.Namespace = raw
		return nil
	}},
	{key: envPrefix + "_METRICS_SUBSYSTEM", apply: func(cfg *Config, raw string) error {
		if raw == "" {
			return nil
		}
		cfg.Metrics.Subsystem = raw
		return nil
	}},
}

// ApplyEnvOverrides allows config values to be adjusted using environment variables.
func (cfg *Config) ApplyEnvOverrides() error {
	for _, override := range envOverrides {
		if raw, ok := os.LookupEnv(override.key); ok {
			trimmed := strings.TrimSpace(raw)
			if err := override.apply(cfg, trimmed); err != nil {
				return fmt.Errorf("%s: %w", override.key, err)
			}
		}
	}
	return nil
}

func parseCSV(raw string) []string {
	var values []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}
