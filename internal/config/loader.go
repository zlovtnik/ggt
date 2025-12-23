package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigPath       = "configs/config.example.yaml"
	ConfigPathEnvKey        = "TRANSFORM_CONFIG_PATH"
	defaultServiceName      = "transform-service"
	defaultLogLevel         = "info"
	defaultShutdownDuration = 30 * time.Second
	defaultMetricsNamespace = "transform_service"
	defaultMetricsSubsystem = "pipeline"
)

// LoadConfig reads the YAML or JSON file pointed to by path or the
// TRANSFORM_CONFIG_PATH environment variable and populates Config.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		path = os.Getenv(ConfigPathEnvKey)
		if path == "" {
			path = DefaultConfigPath
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := unmarshalConfig(path, data, &cfg); err != nil {
		return Config{}, err
	}

	applyDefaults(&cfg)
	return cfg, nil
}

func unmarshalConfig(path string, data []byte, cfg *Config) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("unmarshal json config: %w", err)
		}
	case ".yaml", ".yml":
		fallthrough
	default:
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("unmarshal yaml config: %w", err)
		}
	}
	return nil
}

func applyDefaults(cfg *Config) {
	if cfg.Service.Name == "" {
		cfg.Service.Name = defaultServiceName
	}
	if cfg.Service.LogLevel == "" {
		cfg.Service.LogLevel = defaultLogLevel
	}
	if cfg.Service.ShutdownTimeout == 0 {
		cfg.Service.ShutdownTimeout = defaultShutdownDuration
	}
	if cfg.Metrics.Namespace == "" {
		cfg.Metrics.Namespace = defaultMetricsNamespace
	}
	if cfg.Metrics.Subsystem == "" {
		cfg.Metrics.Subsystem = defaultMetricsSubsystem
	}
	if cfg.Metrics.Buckets == nil {
		cfg.Metrics.Buckets = map[string][]float64{}
	}
}
