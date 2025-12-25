package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zlovtnik/ggt/internal/condition"
)

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Service.Name) == "" {
		return errors.New("service.name is required")
	}
	if !validLogLevel(cfg.Service.LogLevel) {
		return fmt.Errorf("service.log_level must be one of debug, info, warn, error, dpanic, panic, fatal")
	}
	if cfg.Service.ShutdownTimeout <= 0 {
		return errors.New("service.shutdown_timeout must be greater than zero")
	}
	if cfg.Service.MetricsPort < 0 || cfg.Service.MetricsPort > 65535 {
		return errors.New("service.metrics_port must be zero (disabled) or in range 1-65535")
	}
	if cfg.Service.HealthPort < 0 || cfg.Service.HealthPort > 65535 {
		return errors.New("service.health_port must be zero (disabled) or in range 1-65535")
	}
	if len(cfg.Kafka.Consumer.Brokers) == 0 {
		return errors.New("kafka.consumer.brokers must contain at least one broker")
	}
	if len(cfg.Kafka.Producer.Brokers) == 0 {
		return errors.New("kafka.producer.brokers must contain at least one broker")
	}
	if len(cfg.Kafka.Consumer.Topics) == 0 {
		return errors.New("kafka.consumer.topics must contain at least one topic")
	}
	if err := parseDurationField(cfg.Kafka.Consumer.SessionTimeout, "kafka.consumer.session_timeout"); err != nil {
		return err
	}
	if err := parseDurationField(cfg.Kafka.Consumer.MaxPollInterval, "kafka.consumer.max_poll_interval"); err != nil {
		return err
	}
	if len(cfg.Transforms.Pipelines) == 0 {
		return errors.New("transforms.pipelines must declare at least one pipeline")
	}
	for idx, pipeline := range cfg.Transforms.Pipelines {
		if strings.TrimSpace(pipeline.Name) == "" {
			return fmt.Errorf("transforms.pipelines[%d].name is required", idx)
		}
		if len(pipeline.InputTopics) == 0 {
			return fmt.Errorf("transforms.pipelines[%d].input_topics must contain at least one topic", idx)
		}
		if strings.TrimSpace(pipeline.OutputTopic) == "" {
			return fmt.Errorf("transforms.pipelines[%d].output_topic is required", idx)
		}
		if err := validatePipelineTransforms(pipeline.Transforms, fmt.Sprintf("transforms.pipelines[%d]", idx)); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.Metrics.Namespace) == "" {
		return errors.New("metrics.namespace is required")
	}
	if strings.TrimSpace(cfg.Metrics.Subsystem) == "" {
		return errors.New("metrics.subsystem is required")
	}
	return nil
}

func validatePipelineTransforms(descriptors []TransformDescriptor, scope string) error {
	for i, descriptor := range descriptors {
		if strings.TrimSpace(descriptor.Type) == "" {
			return fmt.Errorf("%s.transforms[%d].type is required", scope, i)
		}
		if strings.TrimSpace(descriptor.Condition) != "" {
			if _, err := condition.Parse(descriptor.Condition); err != nil {
				return fmt.Errorf("%s.transforms[%d].condition is invalid: %w", scope, i, err)
			}
		}
		for bIdx, branch := range descriptor.Branches {
			if strings.TrimSpace(branch.Condition) == "" {
				return fmt.Errorf("%s.transforms[%d].branches[%d].condition is required", scope, i, bIdx)
			}
			if _, err := condition.Parse(branch.Condition); err != nil {
				return fmt.Errorf("%s.transforms[%d].branches[%d].condition is invalid: %w", scope, i, bIdx, err)
			}
			if len(branch.Transforms) == 0 {
				return fmt.Errorf("%s.transforms[%d].branches[%d].transforms must contain at least one transform", scope, i, bIdx)
			}
			if err := validatePipelineTransforms(branch.Transforms, fmt.Sprintf("%s.transforms[%d].branches[%d]", scope, i, bIdx)); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseDurationField(raw, name string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if _, err := time.ParseDuration(trimmed); err != nil {
		return fmt.Errorf("%s is invalid: %w", name, err)
	}
	return nil
}

func validLogLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "error", "dpanic", "panic", "fatal":
		return true
	default:
		return false
	}
}
