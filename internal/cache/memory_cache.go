package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"go.uber.org/zap"
)

const (
	// CacheKeyPrefix is the prefix for all cache keys
	CacheKeyPrefix = "recommendation:"
)

// CacheKey generates a cache key from context parameters
type CacheKey struct {
	Mood        string
	TimeOfDay   string
	Weather     string
	Segment     string
	RuleVersion int
}

// String returns the string representation of the cache key
func (k CacheKey) String() string {
	return fmt.Sprintf("%s%s:%s:%s:%s:v%d",
		CacheKeyPrefix, k.Mood, k.TimeOfDay, k.Weather, k.Segment, k.RuleVersion)
}

// cacheEntry represents an entry in the cache with TTL
type cacheEntry struct {
	value      []byte
	expiration time.Time
}

// MemoryCache is an in-memory cache with TTL support
type MemoryCache struct {
	mu      sync.RWMutex
	store   map[string]cacheEntry
	ttl     time.Duration
	stopCh  chan struct{}
	stopped bool
}

// NewMemoryCache creates a new in-memory cache instance
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute // default TTL
	}

	cache := &MemoryCache{
		store:  make(map[string]cacheEntry),
		ttl:    ttl,
		stopCh: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go cache.cleanupExpired()

	logger.Info(context.Background(), "In-memory cache initialized",
		zap.Duration("ttl", ttl))

	return cache
}

// Get retrieves a value from cache
func (c *MemoryCache) Get(ctx context.Context, key CacheKey) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.store[key.String()]
	if !exists {
		return nil, nil // Cache miss
	}

	// Check if expired
	if time.Now().After(entry.expiration) {
		return nil, nil // Expired, treat as cache miss
	}

	return entry.value, nil
}

// Set stores a value in cache with TTL
func (c *MemoryCache) Set(ctx context.Context, key CacheKey, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key.String()] = cacheEntry{
		value:      data,
		expiration: time.Now().Add(c.ttl),
	}

	return nil
}

// Delete removes a value from cache
func (c *MemoryCache) Delete(ctx context.Context, key CacheKey) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.store, key.String())
	return nil
}

// InvalidateByPattern invalidates all keys matching a pattern
// For in-memory cache, we'll do a simple prefix match
func (c *MemoryCache) InvalidateByPattern(ctx context.Context, pattern string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// If pattern is "*", clear everything
	if pattern == "*" {
		count := len(c.store)
		c.store = make(map[string]cacheEntry)
		logger.Info(ctx, "Invalidated all cache keys", zap.Int("count", count))
		return nil
	}

	// Otherwise, match prefix
	prefix := CacheKeyPrefix + pattern
	keysToDelete := make([]string, 0)

	for key := range c.store {
		if matchesPattern(key, prefix) {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(c.store, key)
	}

	if len(keysToDelete) > 0 {
		logger.Info(ctx, "Invalidated cache keys", zap.Int("count", len(keysToDelete)))
	}

	return nil
}

// matchesPattern checks if a key matches a pattern (simple prefix matching)
func matchesPattern(key, pattern string) bool {
	if pattern == "" {
		return true
	}
	if len(key) < len(pattern) {
		return false
	}
	return key[:len(pattern)] == pattern
}

// cleanupExpired periodically removes expired entries
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopCh:
			return
		}
	}
}

// removeExpired removes all expired entries from the cache
func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, entry := range c.store {
		if now.After(entry.expiration) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		delete(c.store, key)
	}

	if len(expiredKeys) > 0 {
		logger.Debug(context.Background(), "Removed expired cache entries",
			zap.Int("count", len(expiredKeys)))
	}
}

// GetStats returns cache statistics
func (c *MemoryCache) GetStats(ctx context.Context) (map[string]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := map[string]string{
		"status":     "connected",
		"type":       "in-memory",
		"total_keys": fmt.Sprintf("%d", len(c.store)),
		"ttl":        c.ttl.String(),
	}

	return stats, nil
}

// Close closes the cache and stops background cleanup
func (c *MemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
		logger.Info(context.Background(), "In-memory cache closed")
	}

	return nil
}

// Ping checks if cache is operational (always true for in-memory)
func (c *MemoryCache) Ping(ctx context.Context) error {
	return nil
}

// Size returns the number of entries in the cache
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.store)
}

// Clear removes all entries from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store = make(map[string]cacheEntry)
	logger.Info(context.Background(), "Cache cleared")
}
