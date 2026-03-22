package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/config"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/metrics"
	pb "github.com/randil-h/CTSE-Mood-Rule-Service/proto/productcatalog"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProductCatalogClient wraps the Product Catalog gRPC client with circuit breaker
type ProductCatalogClient struct {
	client         pb.ProductCatalogServiceClient
	conn           *grpc.ClientConn
	circuitBreaker *gobreaker.CircuitBreaker
	timeout        time.Duration
	maxRetries     int
	retryBackoff   time.Duration
}

// NewProductCatalogClient creates a new Product Catalog client
func NewProductCatalogClient(cfg config.ServiceEndpoint) (*ProductCatalogClient, error) {
	// Setup gRPC connection
	conn, err := grpc.Dial(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Product Catalog Service: %w", err)
	}

	client := pb.NewProductCatalogServiceClient(conn)

	cbSettings := gobreaker.Settings{
		Name:        "ProductCatalog",
		MaxRequests: cfg.CircuitBreaker.MaxRequests,
		Interval:    cfg.CircuitBreaker.Interval,
		Timeout:     cfg.CircuitBreaker.Timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= cfg.CircuitBreaker.FailureRatio
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			logger.Info(context.Background(), "Circuit breaker state changed",
				zap.String("service", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()))
			metrics.CircuitBreakerState.WithLabelValues(name).Set(float64(to))
		},
	}

	cb := gobreaker.NewCircuitBreaker(cbSettings)

	return &ProductCatalogClient{
		client:         client,
		conn:           conn,
		circuitBreaker: cb,
		timeout:        cfg.Timeout,
		maxRetries:     cfg.MaxRetries,
		retryBackoff:   cfg.RetryBackoff,
	}, nil
}

// GetProductsByFilters retrieves products matching the given filters
func (c *ProductCatalogClient) GetProductsByFilters(
	ctx context.Context,
	tags []string,
	categories []string,
	minPrice float64,
	maxPrice float64,
	limit int32,
	traceID string,
) ([]*model.Product, error) {
	startTime := time.Now()
	var products []*model.Product
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * c.retryBackoff
			time.Sleep(backoff)
			logger.Debug(ctx, "Retrying GetProductsByFilters",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
		}

		result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
			reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			resp, err := c.client.GetProductsByFilters(reqCtx, &pb.ProductFilterRequest{
				Tags:       tags,
				Categories: categories,
				MinPrice:   minPrice,
				MaxPrice:   maxPrice,
				Limit:      limit,
				TraceId:    traceID,
			})

			if err != nil {
				return nil, err
			}

			// Convert protobuf products to internal model
			products := make([]*model.Product, len(resp.Products))
			for i, p := range resp.Products {
				products[i] = &model.Product{
					ProductID:   p.ProductId,
					Name:        p.Name,
					Description: p.Description,
					Price:       p.Price,
					Category:    p.Category,
					Tags:        p.Tags,
					ImageURL:    p.ImageUrl,
					InStock:     p.InStock,
				}
			}

			return products, nil
		})

		if err == nil {
			products = result.([]*model.Product)
			duration := time.Since(startTime).Milliseconds()
			metrics.ExternalServiceDuration.WithLabelValues("product_catalog", "GetProductsByFilters", "success").Observe(float64(duration))
			return products, nil
		}

		lastErr = err

		if err == gobreaker.ErrOpenState {
			logger.Warn(ctx, "Circuit breaker open for Product Catalog Service")
			break
		}
	}

	duration := time.Since(startTime).Milliseconds()
	metrics.ExternalServiceDuration.WithLabelValues("product_catalog", "GetProductsByFilters", "error").Observe(float64(duration))
	metrics.ErrorsTotal.WithLabelValues("product_catalog_service").Inc()

	return nil, fmt.Errorf("failed to get products after %d attempts: %w", c.maxRetries+1, lastErr)
}

// NotifyMoodUpdate sends a mood update notification to the Product Catalog Service
func (c *ProductCatalogClient) NotifyMoodUpdate(
	ctx context.Context,
	userID string,
	mood string,
	previousMood string,
	sessionID string,
	traceID string,
) (*pb.MoodUpdateAcknowledgment, error) {
	startTime := time.Now()
	var ack *pb.MoodUpdateAcknowledgment
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * c.retryBackoff
			time.Sleep(backoff)
			logger.Debug(ctx, "Retrying NotifyMoodUpdate",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
		}

		result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
			reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			resp, err := c.client.NotifyMoodUpdate(reqCtx, &pb.MoodUpdateNotification{
				UserId:       userID,
				Mood:         mood,
				PreviousMood: previousMood,
				SessionId:    sessionID,
				Timestamp:    time.Now().Unix(),
				Metadata: &pb.MoodMetadata{
					Source:       "mood_rule_service",
					ResponseTime: int32(time.Since(startTime).Milliseconds()),
					DeviceType:   "server",
				},
			})

			if err != nil {
				return nil, err
			}

			return resp, nil
		})

		if err == nil {
			ack = result.(*pb.MoodUpdateAcknowledgment)
			duration := time.Since(startTime).Milliseconds()
			metrics.ExternalServiceDuration.WithLabelValues("product_catalog", "NotifyMoodUpdate", "success").Observe(float64(duration))

			logger.Info(ctx, "Successfully notified Product Catalog Service of mood update",
				zap.String("user_id", userID),
				zap.String("mood", mood),
				zap.String("correlation_id", ack.CorrelationId),
				zap.Int32("recommendations_generated", ack.RecommendationsGenerated))

			return ack, nil
		}

		lastErr = err

		if err == gobreaker.ErrOpenState {
			logger.Warn(ctx, "Circuit breaker open for Product Catalog Service")
			break
		}
	}

	duration := time.Since(startTime).Milliseconds()
	metrics.ExternalServiceDuration.WithLabelValues("product_catalog", "NotifyMoodUpdate", "error").Observe(float64(duration))
	metrics.ErrorsTotal.WithLabelValues("product_catalog_service").Inc()

	return nil, fmt.Errorf("failed to notify mood update after %d attempts: %w", c.maxRetries+1, lastErr)
}

// Close closes the gRPC connection
func (c *ProductCatalogClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
