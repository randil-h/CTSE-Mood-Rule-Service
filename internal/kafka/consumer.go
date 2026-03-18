package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/cache"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/config"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/engine"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// RuleUpdateEvent represents a rule update event from Kafka
type RuleUpdateEvent struct {
	EventType string    `json:"event_type"` // "created", "updated", "deleted"
	RuleID    string    `json:"rule_id"`
	Timestamp time.Time `json:"timestamp"`
}

// Consumer handles Kafka message consumption
type Consumer struct {
	reader *kafka.Reader
	engine *engine.RuleEngine
	cache  *cache.RedisCache
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg config.KafkaConfig, eng *engine.RuleEngine, cache *cache.RedisCache) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		Topic:          cfg.Topic,
		GroupID:        cfg.GroupID,
		StartOffset:    cfg.StartOffset,
		CommitInterval: cfg.CommitInterval,
		MinBytes:       1,
		MaxBytes:       10e6, // 10MB
		MaxWait:        1 * time.Second,
	})

	return &Consumer{
		reader: reader,
		engine: eng,
		cache:  cache,
	}
}

// Start starts consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	logger.Info(ctx, "Starting Kafka consumer",
		zap.String("topic", c.reader.Config().Topic),
		zap.String("group_id", c.reader.Config().GroupID))

	go c.consume(ctx)

	return nil
}

// consume processes messages from Kafka
func (c *Consumer) consume(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "Stopping Kafka consumer")
			return
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if err == context.Canceled {
					return
				}
				logger.Error(ctx, "Failed to fetch message", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if err := c.processMessage(ctx, msg); err != nil {
				logger.Error(ctx, "Failed to process message",
					zap.Error(err),
					zap.String("topic", msg.Topic),
					zap.Int("partition", msg.Partition),
					zap.Int64("offset", msg.Offset))
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				logger.Error(ctx, "Failed to commit message", zap.Error(err))
			}
		}
	}
}

// processMessage processes a single Kafka message
func (c *Consumer) processMessage(ctx context.Context, msg kafka.Message) error {
	startTime := time.Now()

	var event RuleUpdateEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	logger.Info(ctx, "Received rule update event",
		zap.String("event_type", event.EventType),
		zap.String("rule_id", event.RuleID))

	// Reload rules from database
	if err := c.engine.Reload(ctx); err != nil {
		return fmt.Errorf("failed to reload rules: %w", err)
	}

	// Invalidate cache
	if err := c.cache.InvalidateByPattern(ctx, "*"); err != nil {
		logger.Error(ctx, "Failed to invalidate cache", zap.Error(err))
		// Continue even if cache invalidation fails
	}

	// Publish cache invalidation event
	invalidationMsg := fmt.Sprintf("rule_update:%s:%s", event.EventType, event.RuleID)
	if err := c.cache.PublishInvalidation(ctx, invalidationMsg); err != nil {
		logger.Error(ctx, "Failed to publish invalidation", zap.Error(err))
		// Continue even if publish fails
	}

	duration := time.Since(startTime)
	logger.Info(ctx, "Rule update processed",
		zap.String("event_type", event.EventType),
		zap.String("rule_id", event.RuleID),
		zap.Duration("duration", duration))

	return nil
}

// Close closes the Kafka consumer
func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("failed to close Kafka reader: %w", err)
	}
	return nil
}
