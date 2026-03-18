package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/cache"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/config"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/engine"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/grpc"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/grpc/clients"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/kafka"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/repository"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/metrics"
	pb "github.com/randil-h/CTSE-Mood-Rule-Service/proto/moodrule"
	"go.uber.org/zap"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	ctx := context.Background()

	// Initialize logger
	if err := logger.Init(false); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info(ctx, "Starting Mood-Rule-Service")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal(ctx, "Failed to load configuration", zap.Error(err))
	}

	logger.Info(ctx, "Configuration loaded",
		zap.Int("grpc_port", cfg.Server.GRPCPort),
		zap.Int("health_port", cfg.Server.HealthPort))

	// Initialize database connection
	dbpool, err := initDatabase(ctx, cfg.Database)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize database", zap.Error(err))
	}
	defer dbpool.Close()

	logger.Info(ctx, "Database connection established")

	// Initialize Redis cache
	redisCache, err := cache.NewRedisCache(cfg.Redis)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Redis cache", zap.Error(err))
	}
	defer redisCache.Close()

	logger.Info(ctx, "Redis cache initialized")

	// Initialize repository
	ruleRepo := repository.NewRuleRepository(dbpool)

	// Initialize rule engine
	ruleEngine := engine.NewRuleEngine(ruleRepo)

	// Load rules into memory
	if err := ruleEngine.Load(ctx); err != nil {
		logger.Fatal(ctx, "Failed to load rules", zap.Error(err))
	}

	// Update metrics
	metrics.RulesLoaded.Set(float64(ruleEngine.GetRuleCount()))
	metrics.RuleVersion.Set(float64(ruleEngine.GetVersion()))

	logger.Info(ctx, "Rules loaded into engine",
		zap.Int("count", ruleEngine.GetRuleCount()),
		zap.Int("version", ruleEngine.GetVersion()))

	// Initialize external service clients (optional - service will work with degraded functionality)
	authClient, err := clients.NewAuthClient(cfg.Services.AuthService)
	if err != nil {
		logger.Warn(ctx, "Failed to initialize Auth Service client - authentication will be disabled", zap.Error(err))
		authClient = nil
	} else {
		defer authClient.Close()
		logger.Info(ctx, "Auth Service client initialized")
	}

	productClient, err := clients.NewProductCatalogClient(cfg.Services.ProductCatalog)
	if err != nil {
		logger.Warn(ctx, "Failed to initialize Product Catalog client - product validation will be disabled", zap.Error(err))
		productClient = nil
	} else {
		defer productClient.Close()
		logger.Info(ctx, "Product Catalog client initialized")
	}

	// Initialize Kafka consumer
	kafkaConsumer := kafka.NewConsumer(cfg.Kafka, ruleEngine, redisCache)
	if err := kafkaConsumer.Start(ctx); err != nil {
		logger.Fatal(ctx, "Failed to start Kafka consumer", zap.Error(err))
	}
	defer kafkaConsumer.Close()

	logger.Info(ctx, "Kafka consumer started")

	// Subscribe to cache invalidations
	if err := redisCache.SubscribeToInvalidations(ctx, func(msg string) {
		logger.Info(ctx, "Cache invalidation received", zap.String("message", msg))
	}); err != nil {
		logger.Warn(ctx, "Failed to subscribe to cache invalidations", zap.Error(err))
	}

	// Initialize gRPC server
	grpcServer := grpclib.NewServer(
		grpclib.MaxRecvMsgSize(cfg.Server.MaxPayloadMB*1024*1024),
		grpclib.MaxSendMsgSize(cfg.Server.MaxPayloadMB*1024*1024),
	)

	// Register services
	moodRuleServer := grpc.NewServer(ruleEngine, redisCache, authClient, productClient)
	pb.RegisterMoodRuleServiceServer(grpcServer, moodRuleServer)

	// Register health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// Register reflection for development
	reflection.Register(grpcServer)

	// Start gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GRPCPort))
	if err != nil {
		logger.Fatal(ctx, "Failed to listen", zap.Error(err))
	}

	go func() {
		logger.Info(ctx, "gRPC server listening", zap.Int("port", cfg.Server.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatal(ctx, "Failed to serve", zap.Error(err))
		}
	}()

	// Start HTTP health check and metrics server
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	httpMux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.HealthPort),
		Handler:      httpMux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info(ctx, "HTTP server listening", zap.Int("port", cfg.Server.HealthPort))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(ctx, "HTTP server error", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "Shutting down gracefully...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop gRPC server
	grpcServer.GracefulStop()

	// Stop HTTP server
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error(ctx, "HTTP server shutdown error", zap.Error(err))
	}

	logger.Info(ctx, "Server stopped")
}

// initDatabase initializes the database connection pool
func initDatabase(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s pool_max_conns=%d pool_min_conns=%d",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
		cfg.MaxConns,
		cfg.MinConns,
	)

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
