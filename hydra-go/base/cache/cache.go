package cache

import (
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"
)

// LogId for Cache struct
var logIdCache = log.BaseCacheCache()

type CacheKey interface {
	comparable
	String() string
}

// cacheEntry holds a cached chart with optional error
type cacheEntry[V any] struct {
	value V
	err   error
}

// Cache caches loaded charts based on path and mode
type Cache[K CacheKey, V any] struct {
	l       log.Logger
	name    string
	onStore func(*Cache[K, V], K, V, error)
	quiet   bool
	mu      sync.RWMutex
	cache   map[K]cacheEntry[V]
}

// NewCache creates a new cache
func NewCache[K CacheKey, V any](l log.Logger, name string, quiet bool, onStore func(*Cache[K, V], K, V, error)) *Cache[K, V] {
	return &Cache[K, V]{
		l:       l,
		name:    name,
		onStore: onStore,
		quiet:   quiet,
		mu:      sync.RWMutex{},
		cache:   map[K]cacheEntry[V]{},
	}
}

func (c *Cache[K, V]) StoreIfAbsent(key K, value V, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, found := c.cache[key]; !found {
		c.l.DebugLog(logIdCache, "storing cache entry for cache '{name}' with key '{key}'",
			log.String("name", c.name), log.String("key", key.String()))
		c.cache[key] = cacheEntry[V]{value: value, err: err}
		if c.onStore != nil {
			c.onStore(c, key, value, err)
		}
	}
}

// GetOrLoad retrieves a cache entry from the cache or loads it using the provided loader function
func (c *Cache[K, V]) GetOrLoad(key K, loader func() (V, error)) (V, error) {
	// Try to get from cache first
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if !c.quiet {
		t := "cache hit"
		if !ok {
			t = "cache miss"
		}

		c.l.DebugLog(logIdCache, "{type} for cache '{name}' with key '{key}'",
			log.String("type", t), log.String("name", c.name), log.String("key", key.String()))
	}

	if ok {
		return entry.value, entry.err
	}

	// Load if not cached
	value, err := loader()
	entry = cacheEntry[V]{value: value, err: err}

	// Store in cache
	c.mu.Lock()
	c.cache[key] = entry
	c.mu.Unlock()

	if c.onStore != nil {
		c.onStore(c, key, value, err)
	}

	return value, err
}
