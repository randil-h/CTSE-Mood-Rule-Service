package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestDuration tracks the duration of recommendation requests
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mood_rule_request_duration_ms",
			Help:    "Duration of recommendation requests in milliseconds",
			Buckets: []float64{1, 5, 10, 25, 50, 75, 100, 250, 500, 1000},
		},
		[]string{"method", "status"},
	)

	// RuleEvaluationDuration tracks the duration of rule evaluation
	RuleEvaluationDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mood_rule_evaluation_duration_ms",
			Help:    "Duration of rule evaluation in milliseconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 25, 50},
		},
	)

	// CacheHitRatio tracks cache hit ratio
	CacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mood_rule_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	CacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mood_rule_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	// KafkaReloadCount tracks the number of rule reloads from Kafka
	KafkaReloadCount = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "mood_rule_kafka_reload_total",
			Help: "Total number of rule reloads triggered by Kafka",
		},
	)

	// RulesLoaded tracks the number of rules currently loaded
	RulesLoaded = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mood_rule_rules_loaded",
			Help: "Number of rules currently loaded in memory",
		},
	)

	// RuleVersion tracks the current rule version
	RuleVersion = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "mood_rule_version",
			Help: "Current rule version",
		},
	)

	// RulesMatched tracks the number of rules matched per request
	RulesMatched = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "mood_rule_rules_matched",
			Help:    "Number of rules matched per request",
			Buckets: []float64{0, 1, 2, 5, 10, 20, 50, 100},
		},
	)

	// ExternalServiceDuration tracks external service call durations
	ExternalServiceDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mood_rule_external_service_duration_ms",
			Help:    "Duration of external service calls in milliseconds",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		},
		[]string{"service", "method", "status"},
	)

	// CircuitBreakerState tracks circuit breaker states
	CircuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mood_rule_circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"service"},
	)

	// RequestsTotal tracks total number of requests
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mood_rule_requests_total",
			Help: "Total number of requests",
		},
		[]string{"method", "status"},
	)

	// ErrorsTotal tracks total number of errors
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mood_rule_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type"},
	)
)

// RecordCacheHit records a cache hit
func RecordCacheHit() {
	CacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss() {
	CacheMisses.Inc()
}

// GetCacheHitRatio calculates the cache hit ratio
func GetCacheHitRatio() float64 {
	// This is a simplified calculation
	// In production, you might want to use a more sophisticated approach
	// Note: prometheus.Counter is write-only and doesn't expose current values
	// To get cache hit ratio, query Prometheus metrics endpoint
	return 0.0 // Placeholder
}
