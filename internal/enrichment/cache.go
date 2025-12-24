package enrichment

import (
	"container/list"
	"sync"
	"time"
)

// CacheEntry represents a single cached value with expiration time.
type CacheEntry struct {
	Value   interface{}
	Expiry  time.Time
	Element *list.Element // for LRU tracking
}

// LRUCache provides a simple thread-safe LRU cache with TTL support.
type LRUCache struct {
	mu      sync.RWMutex
	maxSize int
	data    map[string]*CacheEntry
	lruList *list.List
	ttl     time.Duration
}

// NewLRUCache creates a new LRU cache with the specified max size and TTL.
func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &LRUCache{
		maxSize: maxSize,
		data:    make(map[string]*CacheEntry),
		lruList: list.New(),
		ttl:     ttl,
	}
}

// Get retrieves a value from the cache, returning nil if not found or expired.
func (c *LRUCache) Get(key string) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.data[key]
	if !ok {
		return nil
	}

	// Check if expired
	if time.Now().After(entry.Expiry) {
		c.removeEntry(key, entry)
		return nil
	}

	// Move to front (most recently used)
	if entry.Element != nil {
		c.lruList.MoveToFront(entry.Element)
	}

	return entry.Value
}

// Set stores a value in the cache with the configured TTL.
func (c *LRUCache) Set(key string, value interface{}) {
	c.SetWithTTL(key, value, c.ttl)
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *LRUCache) SetWithTTL(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiry := time.Now().Add(ttl)

	// If key already exists, update it
	if entry, ok := c.data[key]; ok {
		entry.Value = value
		entry.Expiry = expiry
		if entry.Element != nil {
			c.lruList.MoveToFront(entry.Element)
		}
		return
	}

	// If we're at max size, evict the least recently used item
	if len(c.data) >= c.maxSize {
		c.evictLRU()
	}

	// Create new entry
	elem := c.lruList.PushFront(key)
	c.data[key] = &CacheEntry{
		Value:   value,
		Expiry:  expiry,
		Element: elem,
	}
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.data[key]; ok {
		c.removeEntry(key, entry)
	}
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]*CacheEntry)
	c.lruList = list.New()
}

// Size returns the current number of items in the cache.
func (c *LRUCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

// evictLRU removes the least recently used item (assumes lock is held).
func (c *LRUCache) evictLRU() {
	elem := c.lruList.Back()
	if elem != nil {
		key := elem.Value.(string)
		c.removeEntry(key, c.data[key])
	}
}

// removeEntry removes an entry from both the map and LRU list (assumes lock is held).
func (c *LRUCache) removeEntry(key string, entry *CacheEntry) {
	delete(c.data, key)
	if entry.Element != nil {
		c.lruList.Remove(entry.Element)
	}
}
