package http

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/zlovtnik/ggt/internal/config"
	"github.com/zlovtnik/ggt/internal/transform"
	"github.com/zlovtnik/ggt/pkg/event"
)

// tplVarRe matches {{.fieldname}} placeholders in URL templates
var tplVarRe = regexp.MustCompile(`\{\{\.\w+\}\}`)

// enrichConfig holds HTTP enrichment configuration.
type enrichConfig struct {
	URL         string        `json:"url"`
	Method      string        `json:"method,omitempty"`
	Body        string        `json:"body,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	TargetField string        `json:"target_field"`
	CacheTTL    time.Duration `json:"cache_ttl,omitempty"`
	DropOnError bool          `json:"drop_on_error"`
}

// enrichTransform is the HTTP enrichment transform.
type enrichTransform struct {
	config *enrichConfig
	client *Client
}

// Name returns the transform name.
func (e *enrichTransform) Name() string {
	return "enrich.http"
}

// Configure validates and configures the transform from raw JSON.
func (e *enrichTransform) Configure(raw json.RawMessage) error {
	cfg := &enrichConfig{
		Method:   "GET",
		Timeout:  10 * time.Second,
		CacheTTL: 5 * time.Minute,
	}

	if err := json.Unmarshal(raw, cfg); err != nil {
		return fmt.Errorf("enrich.http configure: %w", err)
	}

	if cfg.URL == "" {
		return fmt.Errorf("enrich.http configure: url required")
	}

	if cfg.TargetField == "" {
		return fmt.Errorf("enrich.http configure: target_field required")
	}

	if cfg.Method != "GET" && cfg.Method != "POST" {
		return fmt.Errorf("enrich.http configure: method must be GET or POST, got: %s", cfg.Method)
	}

	e.config = cfg
	return nil
}

// Execute performs the HTTP enrichment.
func (e *enrichTransform) Execute(ctx context.Context, payload interface{}) (interface{}, error) {
	if e.config == nil {
		return nil, fmt.Errorf("enrich.http: not configured")
	}

	ev, ok := payload.(event.Event)
	if !ok {
		return nil, fmt.Errorf("enrich.http: payload is not an event")
	}

	url, err := interpolateTemplate(e.config.URL, ev)
	if err != nil {
		if e.config.DropOnError {
			return nil, transform.ErrDrop
		}
		return nil, fmt.Errorf("enrich.http: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	var body string

	switch e.config.Method {
	case "POST":
		var requestBody string
		if e.config.Body != "" {
			var err error
			requestBody, err = interpolateTemplate(e.config.Body, ev)
			if err != nil {
				if e.config.DropOnError {
					return nil, transform.ErrDrop
				}
				return nil, fmt.Errorf("enrich.http: body interpolation failed: %w", err)
			}
		}
		body, err = e.client.Post(ctxWithTimeout, url, "application/json", []byte(requestBody))
	default:
		body, err = e.client.GetJSON(ctxWithTimeout, url)
	}

	if err != nil {
		if e.config.DropOnError {
			return nil, transform.ErrDrop
		}
		return nil, fmt.Errorf("enrich.http execute: %w", err)
	}

	var result interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		if e.config.DropOnError {
			return nil, transform.ErrDrop
		}
		return nil, fmt.Errorf("enrich.http: json parse failed: %w", err)
	}

	return ev.SetField(e.config.TargetField, result), nil
}

// interpolateTemplate replaces {{.fieldname}} placeholders with event field values.
// Returns an error if any placeholder field is not found in the event.
func interpolateTemplate(template string, ev event.Event) (string, error) {
	var missingFields []string

	result := tplVarRe.ReplaceAllStringFunc(template, func(match string) string {
		// Extract field name from {{.fieldname}} pattern
		fieldName := match[3 : len(match)-2]
		val, ok := ev.GetField(fieldName)
		if !ok {
			missingFields = append(missingFields, fieldName)
			return match // Return unchanged placeholder to indicate error
		}
		return fmt.Sprintf("%v", val)
	})

	if len(missingFields) > 0 {
		return "", fmt.Errorf("template interpolation failed: placeholder fields not found in event: %v", missingFields)
	}

	return result, nil
}

// EnrichTransformFactory returns a factory function for creating HTTP enrichment transforms.
func EnrichTransformFactory(client *Client) func(json.RawMessage) (transform.Transform, error) {
	return func(raw json.RawMessage) (transform.Transform, error) {
		t := &enrichTransform{client: client}
		if err := t.Configure(raw); err != nil {
			return nil, err
		}
		return t, nil
	}
}

// RegisterHTTPEnrichment registers the HTTP enrichment transform with the registry.
func RegisterHTTPEnrichment(cfg *config.EnrichmentConfig) error {
	defaultTimeout := 10 * time.Second
	if cfg.HTTP.DefaultTimeout != "" {
		d, err := time.ParseDuration(cfg.HTTP.DefaultTimeout)
		if err != nil {
			return fmt.Errorf("invalid default_timeout duration: %w", err)
		}
		defaultTimeout = d
	}

	httpCfg := &ClientConfig{
		DefaultTimeout:   defaultTimeout,
		MaxIdleConns:     cfg.HTTP.MaxIdleConns,
		MaxConnsPerHost:  cfg.HTTP.MaxConnsPerHost,
		CacheTTL:         5 * time.Minute,
		CacheSize:        1000,
		MaxRetries:       3,
		CircuitOpenTime:  30 * time.Second,
		FailureThreshold: 5,
		SuccessThreshold: 2,
		MaxResponseSize:  cfg.HTTP.MaxResponseSize,
	}

	client := NewClient(httpCfg)
	factory := EnrichTransformFactory(client)

	transform.Register("enrich.http", factory)
	return nil
}
