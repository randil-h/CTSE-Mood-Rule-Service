package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the service
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	Services ServicesConfig
	Engine   EngineConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	GRPCPort     int
	HealthPort   int
	TLSEnabled   bool
	TLSCertFile  string
	TLSKeyFile   string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	MaxPayloadMB int
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxConns        int
	MinConns        int
	MaxConnLifetime time.Duration
	MaxConnIdleTime time.Duration
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	CacheTTL     time.Duration
}

// KafkaConfig holds Kafka configuration
type KafkaConfig struct {
	Brokers        []string
	Topic          string
	GroupID        string
	StartOffset    int64
	CommitInterval time.Duration
}

// ServicesConfig holds external service configurations
type ServicesConfig struct {
	AuthService    ServiceEndpoint
	ProductCatalog ServiceEndpoint
}

// ServiceEndpoint represents a gRPC service endpoint
type ServiceEndpoint struct {
	Address        string
	Timeout        time.Duration
	MaxRetries     int
	RetryBackoff   time.Duration
	CircuitBreaker CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	MaxRequests  uint32
	Interval     time.Duration
	Timeout      time.Duration
	FailureRatio float64
}

// EngineConfig holds rule engine configuration
type EngineConfig struct {
	InitialCapacity int
	ReloadInterval  time.Duration
	MaxConcurrency  int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			GRPCPort:     getEnvAsInt("GRPC_PORT", 50051),
			HealthPort:   getEnvAsInt("HEALTH_PORT", 8080),
			TLSEnabled:   getEnvAsBool("TLS_ENABLED", false),
			TLSCertFile:  getEnv("TLS_CERT_FILE", ""),
			TLSKeyFile:   getEnv("TLS_KEY_FILE", ""),
			ReadTimeout:  getEnvAsDuration("READ_TIMEOUT", 30*time.Second),
			WriteTimeout: getEnvAsDuration("WRITE_TIMEOUT", 30*time.Second),
			MaxPayloadMB: getEnvAsInt("MAX_PAYLOAD_MB", 4),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Database:        getEnv("DB_NAME", "mood_rules"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxConns:        getEnvAsInt("DB_MAX_CONNS", 25),
			MinConns:        getEnvAsInt("DB_MIN_CONNS", 5),
			MaxConnLifetime: getEnvAsDuration("DB_MAX_CONN_LIFETIME", 1*time.Hour),
			MaxConnIdleTime: getEnvAsDuration("DB_MAX_CONN_IDLE_TIME", 30*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("REDIS_ADDR", "localhost:6379"),
			Password:     getEnv("REDIS_PASSWORD", ""),
			DB:           getEnvAsInt("REDIS_DB", 0),
			PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 100),
			MinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 10),
			CacheTTL:     getEnvAsDuration("REDIS_CACHE_TTL", 10*time.Minute),
		},
		Kafka: KafkaConfig{
			Brokers:        getEnvAsSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
			Topic:          getEnv("KAFKA_TOPIC", "rule-updates"),
			GroupID:        getEnv("KAFKA_GROUP_ID", "mood-rule-service"),
			StartOffset:    int64(getEnvAsInt("KAFKA_START_OFFSET", -1)),
			CommitInterval: getEnvAsDuration("KAFKA_COMMIT_INTERVAL", 1*time.Second),
		},
		Services: ServicesConfig{
			AuthService: ServiceEndpoint{
				Address:      getEnv("AUTH_SERVICE_ADDR", "localhost:50052"),
				Timeout:      getEnvAsDuration("AUTH_SERVICE_TIMEOUT", 5*time.Second),
				MaxRetries:   getEnvAsInt("AUTH_SERVICE_MAX_RETRIES", 3),
				RetryBackoff: getEnvAsDuration("AUTH_SERVICE_RETRY_BACKOFF", 100*time.Millisecond),
				CircuitBreaker: CircuitBreakerConfig{
					MaxRequests:  uint32(getEnvAsInt("AUTH_CB_MAX_REQUESTS", 5)),
					Interval:     getEnvAsDuration("AUTH_CB_INTERVAL", 10*time.Second),
					Timeout:      getEnvAsDuration("AUTH_CB_TIMEOUT", 60*time.Second),
					FailureRatio: getEnvAsFloat("AUTH_CB_FAILURE_RATIO", 0.6),
				},
			},
			ProductCatalog: ServiceEndpoint{
				Address:      getEnv("PRODUCT_CATALOG_ADDR", "localhost:50053"),
				Timeout:      getEnvAsDuration("PRODUCT_CATALOG_TIMEOUT", 5*time.Second),
				MaxRetries:   getEnvAsInt("PRODUCT_CATALOG_MAX_RETRIES", 3),
				RetryBackoff: getEnvAsDuration("PRODUCT_CATALOG_RETRY_BACKOFF", 100*time.Millisecond),
				CircuitBreaker: CircuitBreakerConfig{
					MaxRequests:  uint32(getEnvAsInt("PRODUCT_CB_MAX_REQUESTS", 5)),
					Interval:     getEnvAsDuration("PRODUCT_CB_INTERVAL", 10*time.Second),
					Timeout:      getEnvAsDuration("PRODUCT_CB_TIMEOUT", 60*time.Second),
					FailureRatio: getEnvAsFloat("PRODUCT_CB_FAILURE_RATIO", 0.6),
				},
			},
		},
		Engine: EngineConfig{
			InitialCapacity: getEnvAsInt("ENGINE_INITIAL_CAPACITY", 1000),
			ReloadInterval:  getEnvAsDuration("ENGINE_RELOAD_INTERVAL", 5*time.Minute),
			MaxConcurrency:  getEnvAsInt("ENGINE_MAX_CONCURRENCY", 10000),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.GRPCPort <= 0 || c.Server.GRPCPort > 65535 {
		return fmt.Errorf("invalid GRPC port: %d", c.Server.GRPCPort)
	}
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis address is required")
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka brokers are required")
	}
	return nil
}

// Helper functions to read environment variables

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsFloat(key string, defaultValue float64) float64 {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsSlice(key string, defaultValue []string) []string {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	// Simple split by comma - for production, consider a more robust parser
	result := []string{}
	for i := 0; i < len(valueStr); {
		end := i
		for end < len(valueStr) && valueStr[end] != ',' {
			end++
		}
		if end > i {
			result = append(result, valueStr[i:end])
		}
		i = end + 1
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}
