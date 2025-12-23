package logging

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger creates a structured zap logger at the requested level.
func NewLogger(level string) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	trimmed := strings.TrimSpace(level)
	lowered := strings.ToLower(trimmed)
	if lowered == "" {
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
		return config.Build()
	}
	switch lowered {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		return nil, fmt.Errorf("invalid log level %q: must be one of debug, info, warn, error", level)
	}

	return config.Build()
}
