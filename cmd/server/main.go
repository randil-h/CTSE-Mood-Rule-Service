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
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/eventbus"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/grpc"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/grpc/clients"
	httphandler "github.com/randil-h/CTSE-Mood-Rule-Service/internal/http"
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

	// Initialize in-memory cache
	memoryCache := cache.NewMemoryCache(10 * time.Minute)
	defer memoryCache.Close()

	logger.Info(ctx, "In-memory cache initialized")

	// Initialize event bus
	bus := eventbus.New(100) // buffer size of 100 events

	logger.Info(ctx, "Event bus initialized")

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

	// Subscribe to rule update events: reload engine and invalidate cache
	bus.Subscribe(eventbus.EventTypeRuleUpdated, func(ctx context.Context, event eventbus.Event) error {
		if err := ruleEngine.Reload(ctx); err != nil {
			return fmt.Errorf("failed to reload rules: %w", err)
		}
		if err := memoryCache.InvalidateByPattern(ctx, "*"); err != nil {
			logger.Error(ctx, "Failed to invalidate cache", zap.Error(err))
		}
		bus.Publish(ctx, eventbus.EventTypeCacheInvalidate, map[string]string{"reason": "rule_update"})
		return nil
	})
	logger.Info(ctx, "Rule update handler registered")

	// Subscribe to cache invalidation events
	bus.Subscribe(eventbus.EventTypeCacheInvalidate, func(ctx context.Context, event eventbus.Event) error {
		logger.Info(ctx, "Cache invalidation event received",
			zap.Any("payload", event.Payload))
		return nil
	})

	// Initialize gRPC server
	grpcServer := grpclib.NewServer(
		grpclib.MaxRecvMsgSize(cfg.Server.MaxPayloadMB*1024*1024),
		grpclib.MaxSendMsgSize(cfg.Server.MaxPayloadMB*1024*1024),
	)

	// Register services
	moodRuleServer := grpc.NewServer(ruleEngine, memoryCache, authClient, productClient)
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

	// Initialize mood handler
	authServiceURL := fmt.Sprintf("http://%s", os.Getenv("AUTH_SERVICE_HTTP_ADDR"))
	if authServiceURL == "http://" {
		authServiceURL = "http://auth:4000" // Default value
	}
	moodHandler := httphandler.NewMoodHandler(authServiceURL, bus, productClient)

	// Start HTTP health check and metrics server
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	httpMux.Handle("/metrics", promhttp.Handler())
	httpMux.HandleFunc("/mood/update", moodHandler.UpdateMood)

	httpMux.HandleFunc("/docs/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, "docs/swagger.json")
	})

	httpMux.HandleFunc("/docs/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
  <title>Mood-Rule-Service API</title>
  <meta charset="utf-8"/>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/docs/swagger.json",
      dom_id: '#swagger-ui',
      presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
      layout: "BaseLayout"
    })
  </script>
</body>
</html>`))
	})

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

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
