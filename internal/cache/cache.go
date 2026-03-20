package cache

import "context"

// Cache defines the interface for cache operations
type Cache interface {
	Get(ctx context.Context, key CacheKey) ([]byte, error)
	Set(ctx context.Context, key CacheKey, value interface{}) error
	Delete(ctx context.Context, key CacheKey) error
	InvalidateByPattern(ctx context.Context, pattern string) error
	GetStats(ctx context.Context) (map[string]string, error)
	Close() error
	Ping(ctx context.Context) error
}
