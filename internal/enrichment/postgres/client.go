package postgres

import (
	"container/list"
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/zlovtnik/ggt/internal/enrichment"
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
}

// ClientConfig holds Postgres client configuration.
type ClientConfig struct {
	URL             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	CacheTTL        time.Duration
	CacheSize       int
}

// DefaultClientConfig returns sensible Postgres client defaults.
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		CacheTTL:        10 * time.Minute,
		CacheSize:       1000,
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

	rows, err := stmt.QueryContext(ctx, args...)
	if err != nil {
		c.removeInvalidStmt(query, err)
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

	result, err := stmt.ExecContext(ctx, args...)
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
