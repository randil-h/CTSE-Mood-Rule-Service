package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/config"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// CacheKeyPrefix is the prefix for all cache keys
	CacheKeyPrefix = "recommendation:"
	// InvalidationChannel is the pub/sub channel for cache invalidation
	InvalidationChannel = "cache:invalidation"
)

// RedisCache handles Redis caching operations
type RedisCache struct {
	client *redis.Client
	ttl    time.Duration
	pubsub *redis.PubSub
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(cfg config.RedisConfig) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		ttl:    cfg.CacheTTL,
	}, nil
}

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

// Get retrieves a value from cache
func (c *RedisCache) Get(ctx context.Context, key CacheKey) ([]byte, error) {
	result, err := c.client.Get(ctx, key.String()).Bytes()
	if err == redis.Nil {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}
	return result, nil
}

// Set stores a value in cache with TTL
func (c *RedisCache) Set(ctx context.Context, key CacheKey, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	err = c.client.Set(ctx, key.String(), data, c.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %w", err)
	}

	return nil
}

// Delete removes a value from cache
func (c *RedisCache) Delete(ctx context.Context, key CacheKey) error {
	err := c.client.Del(ctx, key.String()).Err()
	if err != nil {
		return fmt.Errorf("failed to delete from cache: %w", err)
	}
	return nil
}

// InvalidateByPattern invalidates all keys matching a pattern
func (c *RedisCache) InvalidateByPattern(ctx context.Context, pattern string) error {
	iter := c.client.Scan(ctx, 0, CacheKeyPrefix+pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan keys: %w", err)
	}

	if len(keys) > 0 {
		if err := c.client.Del(ctx, keys...).Err(); err != nil {
			return fmt.Errorf("failed to delete keys: %w", err)
		}
		logger.Info(ctx, "Invalidated cache keys", zap.Int("count", len(keys)))
	}

	return nil
}

// PublishInvalidation publishes a cache invalidation message
func (c *RedisCache) PublishInvalidation(ctx context.Context, message string) error {
	err := c.client.Publish(ctx, InvalidationChannel, message).Err()
	if err != nil {
		return fmt.Errorf("failed to publish invalidation: %w", err)
	}
	return nil
}

// SubscribeToInvalidations subscribes to cache invalidation messages
func (c *RedisCache) SubscribeToInvalidations(ctx context.Context, handler func(string)) error {
	c.pubsub = c.client.Subscribe(ctx, InvalidationChannel)

	// Wait for confirmation
	_, err := c.pubsub.Receive(ctx)
	if err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Start listening in a goroutine
	go func() {
		ch := c.pubsub.Channel()
		for {
			select {
			case msg := <-ch:
				if msg != nil {
					handler(msg.Payload)
				}
			case <-ctx.Done():
				logger.Info(ctx, "Stopping invalidation subscription")
				return
			}
		}
	}()

	logger.Info(ctx, "Subscribed to cache invalidations")
	return nil
}

// GetStats returns cache statistics
func (c *RedisCache) GetStats(ctx context.Context) (map[string]string, error) {
	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis stats: %w", err)
	}

	// Parse basic stats
	stats := map[string]string{
		"status": "connected",
		"info":   info,
	}

	return stats, nil
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	if c.pubsub != nil {
		if err := c.pubsub.Close(); err != nil {
			return fmt.Errorf("failed to close pubsub: %w", err)
		}
	}
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close Redis client: %w", err)
	}
	return nil
}

// Ping checks if Redis is reachable
func (c *RedisCache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
