package redis

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Get_Set(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	err = client.Set(ctx, "test_key", "test_value", 1*time.Minute)
	require.NoError(t, err)

	val, err := client.Get(ctx, "test_key")
	require.NoError(t, err)
	assert.Equal(t, "test_value", val)

	defer client.Delete(ctx, "test_key")
}

func TestClient_GetJSON_SetJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	testData := map[string]interface{}{"name": "test", "value": 123}
	err = client.SetJSON(ctx, "test_json_key", testData, 1*time.Minute)
	require.NoError(t, err)

	var result map[string]interface{}
	err = client.GetJSON(ctx, "test_json_key", &result)
	require.NoError(t, err)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, float64(123), result["value"])

	defer client.Delete(ctx, "test_json_key")
}

func TestClient_Caching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	err = client.Set(ctx, "cache_key", "value1", 1*time.Minute)
	require.NoError(t, err)

	val1, err := client.Get(ctx, "cache_key")
	require.NoError(t, err)
	assert.Equal(t, "value1", val1)

	stats := client.CacheStats()
	size, ok := stats["size"]
	require.True(t, ok, "stats should contain 'size' key")
	assert.Greater(t, size, 0)

	defer client.Delete(ctx, "cache_key")
}

func TestClient_HGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	err = client.HSet(ctx, "test_hash", "field1", "value1")
	require.NoError(t, err)

	val, err := client.HGet(ctx, "test_hash", "field1")
	require.NoError(t, err)
	assert.Equal(t, "value1", val)

	defer client.Delete(ctx, "test_hash")
}

func TestClient_Exists(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	err = client.Set(ctx, "exists_key", "value", 1*time.Minute)
	require.NoError(t, err)

	exists, err := client.Exists(ctx, "exists_key")
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	exists, err = client.Exists(ctx, "nonexistent_key")
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)

	defer client.Delete(ctx, "exists_key")
}

func TestClient_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &ClientConfig{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Skipf("Redis not available: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	err = client.Set(ctx, "delete_key", "value", 1*time.Minute)
	require.NoError(t, err)

	err = client.Delete(ctx, "delete_key")
	require.NoError(t, err)

	val, err := client.Get(ctx, "delete_key")
	assert.NoError(t, err)
	assert.Empty(t, val)
}
