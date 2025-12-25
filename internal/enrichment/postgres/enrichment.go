package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// enrichConfig holds Postgres enrichment configuration.
type enrichConfig struct {
	Query        string   `json:"query"`
	Params       []string `json:"params"`
	TargetField  string   `json:"target_field"`
	OutputFields []string `json:"output_fields"`
	DropOnError  bool     `json:"drop_on_error"`
}

// enrichTransform is the Postgres enrichment transform.
type enrichTransform struct {
	config       *enrichConfig
	client       *Client
	queryTimeout time.Duration
}

// Name returns the transform name.
func (e *enrichTransform) Name() string {
	return "enrich.postgres"
}

// Configure validates and configures the transform from raw JSON.
func (e *enrichTransform) Configure(raw json.RawMessage) error {
	cfg := &enrichConfig{}

	if err := json.Unmarshal(raw, cfg); err != nil {
		return fmt.Errorf("enrich.postgres configure: %w", err)
	}

	if cfg.Query == "" {
		return fmt.Errorf("enrich.postgres configure: query required")
	}

	if cfg.TargetField == "" {
		return fmt.Errorf("enrich.postgres configure: target_field required")
	}

	e.config = cfg
	return nil
}

// Execute performs the Postgres enrichment.
func (e *enrichTransform) Execute(ctx context.Context, payload interface{}) (interface{}, error) {
	if e.config == nil {
		return nil, fmt.Errorf("enrich.postgres: not configured")
	}

	ev, ok := payload.(event.Event)
	if !ok {
		return nil, fmt.Errorf("enrich.postgres: payload is not an event")
	}

	args := make([]interface{}, 0, len(e.config.Params))
	for _, paramName := range e.config.Params {
		val, ok := ev.GetField(paramName)
		if !ok {
			if e.config.DropOnError {
				return nil, transform.ErrDrop
			}
			return nil, fmt.Errorf("enrich.postgres: missing parameter %s", paramName)
		}
		args = append(args, val)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, e.queryTimeout)
	defer cancel()

	var result interface{}

	if len(e.config.OutputFields) > 1 {
		rows, err := e.client.Query(ctxWithTimeout, e.config.Query, args...)
		if err != nil {
			if e.config.DropOnError {
				return nil, transform.ErrDrop
			}
			return nil, fmt.Errorf("enrich.postgres execute: %w", err)
		}
		defer rows.Close()

		results := make([]map[string]interface{}, 0)
		cols, err := rows.Columns()
		if err != nil {
			if e.config.DropOnError {
				return nil, transform.ErrDrop
			}
			return nil, fmt.Errorf("enrich.postgres: get columns failed: %w", err)
		}

		// Build a set of desired output fields for filtering
		outputSet := make(map[string]bool, len(e.config.OutputFields))
		for _, f := range e.config.OutputFields {
			outputSet[f] = true
		}

		for rows.Next() {
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range cols {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				if e.config.DropOnError {
					return nil, transform.ErrDrop
				}
				return nil, fmt.Errorf("enrich.postgres: scan failed: %w", err)
			}

			row := make(map[string]interface{})
			for i, col := range cols {
				if outputSet[col] {
					row[col] = values[i]
				}
			}
			results = append(results, row)
		}

		if err := rows.Err(); err != nil {
			if e.config.DropOnError {
				return nil, transform.ErrDrop
			}
			return nil, fmt.Errorf("enrich.postgres: rows iteration failed: %w", err)
		}

		result = results
	} else {
		row := e.client.QueryRow(ctxWithTimeout, e.config.Query, args...)
		// database/sql.QueryRow never returns nil; the returned Row will report the error on Scan

		var val interface{}
		if err := row.Scan(&val); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				result = nil
			} else {
				if e.config.DropOnError {
					return nil, transform.ErrDrop
				}
				return nil, fmt.Errorf("enrich.postgres: scan failed: %w", err)
			}
		} else {
			result = val
		}
	}

	return ev.SetField(e.config.TargetField, result), nil
}

// EnrichTransformFactory returns a factory function for creating Postgres enrichment transforms.
func EnrichTransformFactory(client *Client, queryTimeout time.Duration) func(json.RawMessage) (transform.Transform, error) {
	if queryTimeout <= 0 {
		queryTimeout = 10 * time.Second // Default to 10 seconds
	}
	return func(raw json.RawMessage) (transform.Transform, error) {
		t := &enrichTransform{client: client, queryTimeout: queryTimeout}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	}
}

// RegisterPostgresEnrichment registers the Postgres enrichment transform with the registry.
func RegisterPostgresEnrichment(cfg *config.EnrichmentConfig) error {
	pgCfg := &ClientConfig{
		URL:             cfg.Postgres.URL,
		MaxOpenConns:    cfg.Postgres.MaxOpenConns,
		MaxIdleConns:    cfg.Postgres.MaxIdleConns,
		ConnMaxLifetime: cfg.Postgres.ConnMaxLifetime,
		CacheTTL:        10 * time.Minute,
		CacheSize:       1000,
		RetryCount:      cfg.Postgres.RetryCount,
	}

	client, err := NewClient(pgCfg)
	if err != nil {
		return fmt.Errorf("enrich.postgres: create client failed: %w", err)
	}

	queryTimeout := cfg.Postgres.QueryTimeout
	if queryTimeout <= 0 {
		queryTimeout = 10 * time.Second // Default to 10 seconds
	}

	factory := EnrichTransformFactory(client, queryTimeout)
	transform.Register("enrich.postgres", factory)
	return nil
}
