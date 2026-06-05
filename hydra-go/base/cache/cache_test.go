package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"hydra-gitops.org/hydra/hydra-go/base/log"
)

// testKey implements CacheKey for testing
type testKey string

func (k testKey) String() string {
	return string(k)
}

func TestNewCache(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	if c == nil {
		t.Fatal("NewCache returned nil")
	}
	if c.name != "test-cache" {
		t.Errorf("expected name 'test-cache', got '%s'", c.name)
	}
	if !c.quiet {
		t.Error("expected quiet to be true")
	}
}

func TestGetOrLoad_CacheMiss(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	loadCount := 0
	value, err := c.GetOrLoad("key1", func() (string, error) {
		loadCount++
		return "value1", nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}
	if loadCount != 1 {
		t.Errorf("expected loader to be called once, got %d", loadCount)
	}
}

func TestGetOrLoad_CacheHit(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	loadCount := 0
	loader := func() (string, error) {
		loadCount++
		return "value1", nil
	}

	// First call - cache miss
	_, _ = c.GetOrLoad("key1", loader)

	// Second call - cache hit
	value, err := c.GetOrLoad("key1", loader)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}
	if loadCount != 1 {
		t.Errorf("expected loader to be called once, got %d", loadCount)
	}
}

func TestGetOrLoad_WithError(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	expectedErr := errors.New("load error")
	value, err := c.GetOrLoad("key1", func() (string, error) {
		return "", expectedErr
	})

	if err != expectedErr {
		t.Errorf("expected error '%v', got '%v'", expectedErr, err)
	}
	if value != "" {
		t.Errorf("expected empty value, got '%s'", value)
	}

	// Error should also be cached
	_, err = c.GetOrLoad("key1", func() (string, error) {
		return "should not be called", nil
	})

	if err != expectedErr {
		t.Errorf("expected cached error '%v', got '%v'", expectedErr, err)
	}
}

func TestGetOrLoad_OnStoreCallback(t *testing.T) {
	var storedKey testKey
	var storedValue string
	var callCount int

	onStore := func(c *Cache[testKey, string], k testKey, v string, err error) {
		storedKey = k
		storedValue = v
		callCount++
	}

	c := NewCache[testKey, string](log.Default(), "test-cache", true, onStore)

	_, _ = c.GetOrLoad("key1", func() (string, error) {
		return "value1", nil
	})

	if storedKey != "key1" {
		t.Errorf("expected key 'key1', got '%s'", storedKey)
	}
	if storedValue != "value1" {
		t.Errorf("expected value 'value1', got '%s'", storedValue)
	}
	if callCount != 1 {
		t.Errorf("expected onStore to be called once, got %d", callCount)
	}
}

func TestStoreIfAbsent(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	// Store first value
	c.StoreIfAbsent("key1", "value1", nil)

	// Try to store different value for same key
	c.StoreIfAbsent("key1", "value2", nil)

	// Should return first value
	value, err := c.GetOrLoad("key1", func() (string, error) {
		return "should not be called", nil
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}
}

func TestGetOrLoad_Concurrent(t *testing.T) {
	c := NewCache[testKey, int](log.Default(), "test-cache", true, nil)

	var loadCount atomic.Int32
	loader := func() (int, error) {
		loadCount.Add(1)
		return 42, nil
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			value, err := c.GetOrLoad("key1", loader)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if value != 42 {
				t.Errorf("expected 42, got %d", value)
			}
		})
	}
	wg.Wait()

	// Due to race conditions, loader might be called more than once,
	// but the final cached value should be consistent
	value, _ := c.GetOrLoad("key1", func() (int, error) {
		return 0, nil
	})
	if value != 42 {
		t.Errorf("expected cached value 42, got %d", value)
	}
}

func TestCache_MultipleKeys(t *testing.T) {
	c := NewCache[testKey, string](log.Default(), "test-cache", true, nil)

	_, _ = c.GetOrLoad("key1", func() (string, error) { return "value1", nil })
	_, _ = c.GetOrLoad("key2", func() (string, error) { return "value2", nil })
	_, _ = c.GetOrLoad("key3", func() (string, error) { return "value3", nil })

	tests := []struct {
		key      testKey
		expected string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	for _, tt := range tests {
		value, _ := c.GetOrLoad(tt.key, func() (string, error) {
			return "should not be called", nil
		})
		if value != tt.expected {
			t.Errorf("key %s: expected '%s', got '%s'", tt.key, tt.expected, value)
		}
	}
}
