package postgres

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/zlovtnik/ggt/internal/enrichment"
	"go.uber.org/zap"
)

const maxPreparedStmts = 500

// Row represents a database row that can be scanned
type Row interface {
	Scan(dest ...interface{}) error
	Err() error
}

// errorRow implements Row interface and defers errors to Scan()
type errorRow struct {
	err error
}

func (r *errorRow) Scan(dest ...interface{}) error {
	return r.err
}

func (r *errorRow) Err() error {
	return r.err
}

// rowWrapper wraps *sql.Row to implement the Row interface with Err() method
type rowWrapper struct {
	r   *sql.Row
	err error
}

func (rw *rowWrapper) Scan(dest ...interface{}) error {
	rw.err = rw.r.Scan(dest...)
	return rw.err
}

func (rw *rowWrapper) Err() error {
	return rw.err
}

// Client provides a Postgres connection pool with prepared statement caching.
type Client struct {
	db              *sql.DB
	cache           *enrichment.LRUCache
	stmtCache       map[string]*sql.Stmt
	stmtLRU         *list.List               // tracks access order for LRU eviction
	stmtElements    map[string]*list.Element // maps query to LRU list element
	stmtCacheMutex  sync.RWMutex
	connMaxLifetime time.Duration
	maxOpenConns    int
	maxIdleConns    int
	queryTimeout    time.Duration
	retryCfg        *enrichment.RetryConfig
}

// ClientConfig holds Postgres client configuration.
type ClientConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	CacheTTL        time.Duration
	CacheSize       int
	// QueryTimeout is the default timeout applied to individual queries
	// when a context without deadline is provided. Zero means no timeout.
	QueryTimeout time.Duration
	// RetryCount controls how many attempts are made for transient errors.
	RetryCount int
}

// DefaultClientConfig returns sensible Postgres client defaults.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		CacheTTL:        10 * time.Minute,
		CacheSize:       1000,
		QueryTimeout:    5 * time.Second,
		RetryCount:      3,
	}
}

// NewClient creates a new Postgres client with connection pooling and statement caching.
func NewClient(cfg *ClientConfig) (*Client, error) {
	if cfg == nil {
		cfg = DefaultClientConfig()
	}

	if cfg.URL == "" {
		return nil, fmt.Errorf("postgres: connection URL is required")
	}

	db, err := sql.Open("postgres", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres: open failed: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping failed: %w", err)
	}

	logger := zap.L().With(zap.String("component", "postgres_client"))
	fields := []zap.Field{
		zap.Int("max_open_conns", cfg.MaxOpenConns),
		zap.Int("max_idle_conns", cfg.MaxIdleConns),
		zap.Duration("conn_max_lifetime", cfg.ConnMaxLifetime),
	}
	if parsed, err := url.Parse(cfg.URL); err == nil {
		if host := parsed.Hostname(); host != "" {
			fields = append(fields, zap.String("host", host))
		}
		if port := parsed.Port(); port != "" {
			fields = append(fields, zap.String("port", port))
		}
		if dbName := strings.TrimPrefix(parsed.Path, "/"); dbName != "" {
			fields = append(fields, zap.String("database", dbName))
		}
	} else {
		fields = append(fields, zap.String("dsn", "unparseable"))
	}
	logger.Info("postgres client ready", fields...)

	cache := enrichment.NewLRUCache(cfg.CacheSize, cfg.CacheTTL)

	return &Client{
		db:              db,
		cache:           cache,
		stmtCache:       make(map[string]*sql.Stmt),
		stmtLRU:         list.New(),
		stmtElements:    make(map[string]*list.Element),
		connMaxLifetime: cfg.ConnMaxLifetime,
		maxOpenConns:    cfg.MaxOpenConns,
		maxIdleConns:    cfg.MaxIdleConns,
		queryTimeout:    cfg.QueryTimeout,
		retryCfg:        &enrichment.RetryConfig{MaxRetries: cfg.RetryCount, InitialBackoff: 100 * time.Millisecond, MaxBackoff: 2 * time.Second, BackoffMultiplier: 2.0},
	}, nil
}

// PreparedStmt returns a cached prepared statement or creates and caches a new one.
func (c *Client) PreparedStmt(ctx context.Context, query string) (*sql.Stmt, error) {
	c.stmtCacheMutex.RLock()
	stmt, exists := c.stmtCache[query]
	c.stmtCacheMutex.RUnlock()

	if exists {
		// Move to front of LRU list
		c.stmtCacheMutex.Lock()
		if elem, elemExists := c.stmtElements[query]; elemExists {
			c.stmtLRU.MoveToFront(elem)
		}
		c.stmtCacheMutex.Unlock()
		return stmt, nil
	}

	c.stmtCacheMutex.Lock()
	defer c.stmtCacheMutex.Unlock()

	stmt, exists = c.stmtCache[query]
	if exists {
		if elem, elemExists := c.stmtElements[query]; elemExists {
			c.stmtLRU.MoveToFront(elem)
		}
		return stmt, nil
	}

	// Evict LRU entry if cache is full
	if len(c.stmtCache) >= maxPreparedStmts {
		c.evictLRU()
	}

	stmt, err := c.db.Prepare(query)
	if err != nil {
		return nil, fmt.Errorf("postgres: prepare failed: %w", err)
	}

	c.stmtCache[query] = stmt
	elem := c.stmtLRU.PushFront(query)
	c.stmtElements[query] = elem
	return stmt, nil
}

// ensureCtx returns a context with the client's QueryTimeout applied if the
// provided context does not already have a deadline. If a new context is
// created, a cancel function is returned which must be called by the caller.
func (c *Client) ensureCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if c == nil || c.queryTimeout == 0 {
		return ctx, nil
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, nil
	}
	return context.WithTimeout(ctx, c.queryTimeout)
}

// evictLRU removes the least recently used statement from cache
func (c *Client) evictLRU() {
	if c.stmtLRU.Len() == 0 {
		return
	}

	// Get the least recently used query (back of the list)
	lruElement := c.stmtLRU.Back()
	lruQuery := lruElement.Value.(string)

	// Remove from LRU list
	c.stmtLRU.Remove(lruElement)

	// Remove from elements map
	delete(c.stmtElements, lruQuery)

	// Close and remove from cache
	if stmt, exists := c.stmtCache[lruQuery]; exists {
		_ = stmt.Close()
		delete(c.stmtCache, lruQuery)
	}
}

// removeInvalidStmt removes a prepared statement from cache if the error indicates it's invalid
func (c *Client) removeInvalidStmt(query string, err error) {
	if err == nil {
		return
	}

	// Check if error indicates invalid prepared statement using pq.Error codes
	if pqErr, ok := err.(*pq.Error); ok {
		if pqErr.Code == "26000" || pqErr.Code == "42P14" {
			c.stmtCacheMutex.Lock()
			if stmt, exists := c.stmtCache[query]; exists {
				_ = stmt.Close()
				delete(c.stmtCache, query)
				// Remove from LRU list and elements map
				if elem, elemExists := c.stmtElements[query]; elemExists {
					c.stmtLRU.Remove(elem)
					delete(c.stmtElements, query)
				}
			}
			c.stmtCacheMutex.Unlock()
		}
	}
}

// QueryRow executes a query and returns a single row.
func (c *Client) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	stmt, err := c.PreparedStmt(ctx, query)
	if err != nil {
		return &errorRow{err: err}
	}
	return &rowWrapper{r: stmt.QueryRowContext(ctx, args...)}
}

// Query executes a query and returns the results.
func (c *Client) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	stmt, err := c.PreparedStmt(ctx, query)
	if err != nil {
		return nil, err
	}

	ctx2, cancel := c.ensureCtx(ctx)
	if cancel != nil {
		defer cancel()
	}

	var rows *sql.Rows
	err = enrichment.Do(ctx2, c.retryCfg, func() error {
		var errInner error
		rows, errInner = stmt.QueryContext(ctx2, args...)
		if errInner != nil {
			c.removeInvalidStmt(query, errInner)
			return errInner
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Exec executes a statement and returns the result.
func (c *Client) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	stmt, err := c.PreparedStmt(ctx, query)
	if err != nil {
		return nil, err
	}

	ctx2, cancel := c.ensureCtx(ctx)
	if cancel != nil {
		defer cancel()
	}

	// Use predicate-aware retry: retry only for transient errors (connection/timeouts).
	var result sql.Result
	shouldRetry := func(err error) bool {
		if err == nil {
			return false
		}
		// Do not retry on context cancellation/deadline here; enrichment.DoWithPredicate will handle that.
		// Inspect pq errors and treat constraint/syntax/permission errors as non-retriable.
		if pqErr, ok := err.(*pq.Error); ok {
			switch string(pqErr.Code) {
			case "23505": // unique_violation
				return false
			case "23503": // foreign_key_violation
				return false
			case "42601": // syntax_error
				return false
			case "40001": // serialization_failure / transaction aborted
				return false
			default:
				// unknown pq error: be conservative and do not retry
				return false
			}
		}
		// For generic errors (network/timeouts), allow retry.
		return true
	}

	err = enrichment.DoWithPredicate(ctx2, c.retryCfg, func() error {
		var errInner error
		result, errInner = stmt.ExecContext(ctx2, args...)
		if errInner != nil {
			c.removeInvalidStmt(query, errInner)
			return errInner
		}
		return nil
	}, shouldRetry)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ExecNoRetry executes the statement once and returns the result without any retries.
func (c *Client) ExecNoRetry(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	stmt, err := c.PreparedStmt(ctx, query)
	if err != nil {
		return nil, err
	}
	ctx2, cancel := c.ensureCtx(ctx)
	if cancel != nil {
		defer cancel()
	}
	result, err := stmt.ExecContext(ctx2, args...)
	if err != nil {
		c.removeInvalidStmt(query, err)
		return nil, err
	}
	return result, nil
}

// Close closes the database connection and clears caches.
func (c *Client) Close() error {
	c.stmtCacheMutex.Lock()
	for _, stmt := range c.stmtCache {
		_ = stmt.Close()
	}
	c.stmtCache = make(map[string]*sql.Stmt)
	c.stmtLRU = list.New()
	c.stmtElements = make(map[string]*list.Element)
	c.stmtCacheMutex.Unlock()

	c.cache.Clear()
	return c.db.Close()
}

// CacheStats returns cache statistics.
func (c *Client) CacheStats() map[string]interface{} {
	c.stmtCacheMutex.RLock()
	stmtCacheSize := len(c.stmtCache)
	c.stmtCacheMutex.RUnlock()

	return map[string]interface{}{
		"result_cache_size": c.cache.Size(),
		"stmt_cache_size":   stmtCacheSize,
	}
}

// Health checks the health of the database connection.
func (c *Client) Health(ctx context.Context) error {
	return c.db.PingContext(ctx)
}
