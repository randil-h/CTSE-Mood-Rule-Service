package eventbus

import (
	"context"
	"fmt"
	"sync"

	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"go.uber.org/zap"
)

// EventType represents the type of event
type EventType string

const (
	// EventTypeMoodChanged is fired when a user's mood changes
	EventTypeMoodChanged EventType = "mood_changed"
	// EventTypeRuleUpdated is fired when a rule is created, updated, or deleted
	EventTypeRuleUpdated EventType = "rule_updated"
	// EventTypeCacheInvalidate is fired when cache needs to be invalidated
	EventTypeCacheInvalidate EventType = "cache_invalidate"
)

// Event represents an event in the system
type Event struct {
	Type    EventType
	Payload interface{}
}

// Handler is a function that handles an event
type Handler func(ctx context.Context, event Event) error

// EventBus is an in-memory event bus for pub/sub within the same process
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]Handler
	bufferSize  int
}

// New creates a new EventBus instance
func New(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 100 // default buffer size
	}
	return &EventBus{
		subscribers: make(map[EventType][]Handler),
		bufferSize:  bufferSize,
	}
}

// Subscribe registers a handler for a specific event type
// Multiple handlers can subscribe to the same event type
func (eb *EventBus) Subscribe(eventType EventType, handler Handler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if eb.subscribers[eventType] == nil {
		eb.subscribers[eventType] = make([]Handler, 0)
	}
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)

	logger.Info(context.Background(), "Event handler subscribed",
		zap.String("event_type", string(eventType)),
		zap.Int("total_subscribers", len(eb.subscribers[eventType])))
}

// Publish publishes an event to all subscribers asynchronously
// Events are handled in goroutines to avoid blocking the publisher
func (eb *EventBus) Publish(ctx context.Context, eventType EventType, payload interface{}) {
	eb.mu.RLock()
	handlers := eb.subscribers[eventType]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		logger.Debug(ctx, "No subscribers for event type",
			zap.String("event_type", string(eventType)))
		return
	}

	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	logger.Debug(ctx, "Publishing event",
		zap.String("event_type", string(eventType)),
		zap.Int("subscribers", len(handlers)))

	// Publish to all subscribers asynchronously
	for _, handler := range handlers {
		// Capture handler in loop
		h := handler
		go func() {
			if err := h(ctx, event); err != nil {
				logger.Error(ctx, "Error handling event",
					zap.String("event_type", string(eventType)),
					zap.Error(err))
			}
		}()
	}
}

// PublishSync publishes an event to all subscribers synchronously
// Use this when you need to wait for all handlers to complete
func (eb *EventBus) PublishSync(ctx context.Context, eventType EventType, payload interface{}) error {
	eb.mu.RLock()
	handlers := eb.subscribers[eventType]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		logger.Debug(ctx, "No subscribers for event type",
			zap.String("event_type", string(eventType)))
		return nil
	}

	event := Event{
		Type:    eventType,
		Payload: payload,
	}

	logger.Debug(ctx, "Publishing event synchronously",
		zap.String("event_type", string(eventType)),
		zap.Int("subscribers", len(handlers)))

	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		h := handler
		go func() {
			defer wg.Done()
			if err := h(ctx, event); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Collect all errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("event handling errors: %v", errors)
	}

	return nil
}

// Unsubscribe removes all handlers for a specific event type
func (eb *EventBus) Unsubscribe(eventType EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.subscribers, eventType)

	logger.Info(context.Background(), "All handlers unsubscribed",
		zap.String("event_type", string(eventType)))
}

// SubscriberCount returns the number of subscribers for an event type
func (eb *EventBus) SubscriberCount(eventType EventType) int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return len(eb.subscribers[eventType])
}

// Clear removes all subscribers
func (eb *EventBus) Clear() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers = make(map[EventType][]Handler)

	logger.Info(context.Background(), "Event bus cleared")
}
