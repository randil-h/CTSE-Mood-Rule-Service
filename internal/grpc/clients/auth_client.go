package clients

import (
	"context"
	"fmt"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/config"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/metrics"
	pb "github.com/randil-h/CTSE-Mood-Rule-Service/proto/auth"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AuthClient wraps the Auth Service gRPC client with circuit breaker
type AuthClient struct {
	client         pb.AuthServiceClient
	conn           *grpc.ClientConn
	circuitBreaker *gobreaker.CircuitBreaker
	timeout        time.Duration
	maxRetries     int
	retryBackoff   time.Duration
}

// NewAuthClient creates a new Auth Service client
func NewAuthClient(cfg config.ServiceEndpoint) (*AuthClient, error) {
	// Setup gRPC connection
	conn, err := grpc.Dial(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Auth Service: %w", err)
	}

	client := pb.NewAuthServiceClient(conn)

	// Setup circuit breaker
	cbSettings := gobreaker.Settings{
		Name:        "AuthService",
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

	return &AuthClient{
		client:         client,
		conn:           conn,
		circuitBreaker: cb,
		timeout:        cfg.Timeout,
		maxRetries:     cfg.MaxRetries,
		retryBackoff:   cfg.RetryBackoff,
	}, nil
}

// GetUserMood retrieves the user's mood from Auth Service
func (c *AuthClient) GetUserMood(ctx context.Context, userID string, traceID string) (string, error) {
	startTime := time.Now()
	var mood string
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * c.retryBackoff
			time.Sleep(backoff)
			logger.Debug(ctx, "Retrying GetUserMood",
				zap.Int("attempt", attempt),
				zap.Duration("backoff", backoff))
		}

		result, err := c.circuitBreaker.Execute(func() (interface{}, error) {
			reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
			defer cancel()

			// TODO: Implement GetUserInfo RPC in Auth Service
			// For now, returning empty mood as this RPC method is not yet implemented
			_ = reqCtx
			_ = userID
			return "", fmt.Errorf("GetUserInfo RPC not implemented in Auth Service")

			// resp, err := c.client.GetUserInfo(reqCtx, &pb.GetUserInfoRequest{
			// 	UserId: userID,
			// })
			//
			// if err != nil {
			// 	return "", err
			// }
			//
			// return resp.Mood, nil
		})

		if err == nil {
			mood = result.(string)
			duration := time.Since(startTime).Milliseconds()
			metrics.ExternalServiceDuration.WithLabelValues("auth", "GetUserMood", "success").Observe(float64(duration))
			return mood, nil
		}

		lastErr = err

		if err == gobreaker.ErrOpenState {
			logger.Warn(ctx, "Circuit breaker open for Auth Service")
			break
		}
	}

	duration := time.Since(startTime).Milliseconds()
	metrics.ExternalServiceDuration.WithLabelValues("auth", "GetUserMood", "error").Observe(float64(duration))
	metrics.ErrorsTotal.WithLabelValues("auth_service").Inc()

	return "", fmt.Errorf("failed to get user mood after %d attempts: %w", c.maxRetries+1, lastErr)
}

// Close closes the gRPC connection
func (c *AuthClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
