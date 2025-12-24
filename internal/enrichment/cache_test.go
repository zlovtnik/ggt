package enrichment

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_SetGet(t *testing.T) {
	cache := NewLRUCache(2, 1*time.Minute)
	cache.Set("key1", "value1")
	val := cache.Get("key1")
	assert.Equal(t, "value1", val)
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(10, 50*time.Millisecond)
	cache.Set("key1", "value1")
	time.Sleep(100 * time.Millisecond)
	val := cache.Get("key1")
	assert.Nil(t, val)
}

func TestLRUCache_LRUEviction(t *testing.T) {
	cache := NewLRUCache(2, 1*time.Minute)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	val1 := cache.Get("key1")
	assert.Nil(t, val1)

	val2 := cache.Get("key2")
	assert.Equal(t, "value2", val2)

	val3 := cache.Get("key3")
	assert.Equal(t, "value3", val3)
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Minute)
	cache.Set("key1", "value1")
	cache.Delete("key1")
	val := cache.Get("key1")
	assert.Nil(t, val)
}

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Minute)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestLRUCache_UpdateValue(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Minute)
	cache.Set("key1", "value1")
	cache.Set("key1", "value2")
	val := cache.Get("key1")
	assert.Equal(t, "value2", val)
}

func TestLRUCache_MoveToFront(t *testing.T) {
	cache := NewLRUCache(2, 1*time.Minute)
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Get("key1")

	cache.Set("key3", "value3")

	val1 := cache.Get("key1")
	assert.Equal(t, "value1", val1)

	val2 := cache.Get("key2")
	assert.Nil(t, val2)
}

func TestLRUCache_Size(t *testing.T) {
	cache := NewLRUCache(10, 1*time.Minute)
	assert.Equal(t, 0, cache.Size())
	cache.Set("key1", "value1")
	assert.Equal(t, 1, cache.Size())
	cache.Set("key2", "value2")
	assert.Equal(t, 2, cache.Size())
}

func TestLRUCache_DefaultConfig(t *testing.T) {
	cache := NewLRUCache(0, 0)
	assert.Equal(t, 1000, cache.maxSize)
	assert.Equal(t, 5*time.Minute, cache.ttl)
}
