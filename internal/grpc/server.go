package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/cache"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/engine"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/metrics"
	pb "github.com/randil-h/CTSE-Mood-Rule-Service/proto/moodrule"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RuleEvaluator defines the interface for rule evaluation
type RuleEvaluator interface {
	Evaluate(ctx context.Context, matchCtx *model.MatchContext) ([]*model.Rule, engine.EvaluationStats)
	GetVersion() int
	GetRuleCount() int
}

// AuthClient defines the interface for authentication service
type AuthClient interface {
	GetUserMood(ctx context.Context, userID string, traceID string) (string, error)
}

// ProductClient defines the interface for product catalog service
type ProductClient interface {
	GetProductsByFilters(
		ctx context.Context,
		tags []string,
		categories []string,
		minPrice float64,
		maxPrice float64,
		limit int32,
		traceID string,
	) ([]*model.Product, error)
}

// Server implements the MoodRuleService gRPC server
type Server struct {
	pb.UnimplementedMoodRuleServiceServer
	engine        RuleEvaluator
	cache         cache.Cache
	authClient    AuthClient
	productClient ProductClient
}

// NewServer creates a new gRPC server
func NewServer(
	eng RuleEvaluator,
	c cache.Cache,
	authClient AuthClient,
	productClient ProductClient,
) *Server {
	return &Server{
		engine:        eng,
		cache:         c,
		authClient:    authClient,
		productClient: productClient,
	}
}

// Recommend provides product recommendations based on user context
func (s *Server) Recommend(ctx context.Context, req *pb.UserContext) (*pb.ProductRecommendations, error) {
	startTime := time.Now()

	// Generate trace ID if not provided
	traceID := req.TraceId
	if traceID == "" {
		traceID = uuid.New().String()
	}
	ctx = logger.WithTraceID(ctx, traceID)

	logger.Info(ctx, "Received recommendation request",
		zap.String("user_id", req.UserId))

	// Validate input
	if err := s.validateRequest(req); err != nil {
		metrics.RequestsTotal.WithLabelValues("Recommend", "invalid_input").Inc()
		metrics.ErrorsTotal.WithLabelValues("validation").Inc()
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: %v", err)
	}

	// Always fetch mood from Auth Service
	if s.authClient == nil {
		logger.Error(ctx, "Auth Service client not available")
		metrics.RequestsTotal.WithLabelValues("Recommend", "auth_unavailable").Inc()
		return nil, status.Errorf(codes.FailedPrecondition, "Auth Service is required to fetch user mood")
	}

	mood, err := s.authClient.GetUserMood(ctx, req.UserId, traceID)
	if err != nil {
		logger.Error(ctx, "Failed to fetch user mood from Auth Service", zap.Error(err))
		metrics.RequestsTotal.WithLabelValues("Recommend", "auth_error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to fetch user mood: %v", err)
	}

	logger.Info(ctx, "Fetched mood from Auth Service",
		zap.String("user_id", req.UserId),
		zap.String("mood", mood))

	// Check cache first
	cacheKey := s.generateCacheKey(mood, req.TimeOfDay, req.Weather, req.UserId)
	recommendations, fromCache, err := s.checkCache(ctx, cacheKey)
	if err != nil {
		logger.Warn(ctx, "Cache check failed", zap.Error(err))
	}

	if fromCache && recommendations != nil {
		metrics.RecordCacheHit()
		duration := time.Since(startTime).Milliseconds()
		metrics.RequestDuration.WithLabelValues("Recommend", "success").Observe(float64(duration))
		metrics.RequestsTotal.WithLabelValues("Recommend", "success").Inc()

		logger.Info(ctx, "Returning cached recommendations",
			zap.Int("count", len(recommendations.Recommendations)),
			zap.Int64("duration_ms", duration))

		return recommendations, nil
	}

	metrics.RecordCacheMiss()

	// Build match context
	matchCtx := &model.MatchContext{
		Mood:                mood,
		TimeOfDay:           req.TimeOfDay,
		Weather:             req.Weather,
		Occasion:            req.Occasion,
		Preferences:         req.UserPreferences,
		PurchaseHistoryTags: req.PurchaseHistoryTags,
	}

	// Evaluate rules
	evalStart := time.Now()
	matchedRules, evalStats := s.engine.Evaluate(ctx, matchCtx)
	evalDuration := time.Since(evalStart)
	metrics.RuleEvaluationDuration.Observe(float64(evalDuration.Milliseconds()))
	metrics.RulesMatched.Observe(float64(evalStats.RulesMatched))

	logger.Debug(ctx, "Rules evaluated",
		zap.Int("matched", evalStats.RulesMatched),
		zap.Int("evaluated", evalStats.RulesEvaluated),
		zap.Duration("duration", evalDuration))

	if len(matchedRules) == 0 {
		logger.Info(ctx, "No rules matched")
		emptyResp := &pb.ProductRecommendations{
			Recommendations: []*pb.ProductRecommendation{},
			TraceId:         traceID,
			Metadata: &pb.RecommendationMetadata{
				RulesEvaluated:   int32(evalStats.RulesEvaluated),
				RulesMatched:     int32(evalStats.RulesMatched),
				FromCache:        false,
				EvaluationTimeMs: evalDuration.Milliseconds(),
				RuleVersion:      fmt.Sprintf("%d", evalStats.Version),
			},
		}

		duration := time.Since(startTime).Milliseconds()
		metrics.RequestDuration.WithLabelValues("Recommend", "no_matches").Observe(float64(duration))
		metrics.RequestsTotal.WithLabelValues("Recommend", "no_matches").Inc()

		return emptyResp, nil
	}

	// Aggregate filters from matched rules
	tags, categories, minPrice, maxPrice := s.aggregateFilters(matchedRules)

	// Fetch products from Product Catalog
	if s.productClient == nil {
		logger.Warn(ctx, "Product Catalog Service not available")
		metrics.RequestsTotal.WithLabelValues("Recommend", "product_unavailable").Inc()
		return nil, status.Errorf(codes.Unavailable, "Product Catalog Service is currently unavailable")
	}

	products, err := s.productClient.GetProductsByFilters(
		ctx,
		tags,
		categories,
		minPrice,
		maxPrice,
		50, // limit
		traceID,
	)

	if err != nil {
		logger.Error(ctx, "Failed to fetch products", zap.Error(err))
		metrics.RequestsTotal.WithLabelValues("Recommend", "product_error").Inc()
		return nil, status.Errorf(codes.Internal, "failed to fetch products: %v", err)
	}

	// Score and rank products
	rankedProducts := s.scoreProducts(products, matchedRules)

	// Build response
	recommendations = &pb.ProductRecommendations{
		Recommendations: rankedProducts,
		TraceId:         traceID,
		Metadata: &pb.RecommendationMetadata{
			RulesEvaluated:   int32(evalStats.RulesEvaluated),
			RulesMatched:     int32(evalStats.RulesMatched),
			FromCache:        false,
			EvaluationTimeMs: evalDuration.Milliseconds(),
			RuleVersion:      fmt.Sprintf("%d", evalStats.Version),
		},
	}

	// Cache the result
	if err := s.cache.Set(ctx, cacheKey, recommendations); err != nil {
		logger.Warn(ctx, "Failed to cache recommendations", zap.Error(err))
	}

	duration := time.Since(startTime).Milliseconds()
	metrics.RequestDuration.WithLabelValues("Recommend", "success").Observe(float64(duration))
	metrics.RequestsTotal.WithLabelValues("Recommend", "success").Inc()

	logger.Info(ctx, "Recommendation request completed",
		zap.Int("product_count", len(rankedProducts)),
		zap.Int64("total_duration_ms", duration),
		zap.Int64("eval_duration_ms", evalDuration.Milliseconds()))

	return recommendations, nil
}

// HealthCheck performs a health check
func (s *Server) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	deps := make(map[string]string)

	// Check Redis
	if err := s.cache.Ping(ctx); err != nil {
		deps["redis"] = "unhealthy: " + err.Error()
	} else {
		deps["redis"] = "healthy"
	}

	// Check rule engine
	ruleCount := s.engine.GetRuleCount()
	deps["rule_engine"] = fmt.Sprintf("healthy (%d rules loaded)", ruleCount)

	// Check external services
	if s.authClient == nil {
		deps["auth_service"] = "unavailable (degraded mode)"
	} else {
		deps["auth_service"] = "healthy"
	}

	if s.productClient == nil {
		deps["product_catalog"] = "unavailable (degraded mode)"
	} else {
		deps["product_catalog"] = "healthy"
	}

	return &pb.HealthCheckResponse{
		Status:       "healthy",
		Dependencies: deps,
	}, nil
}

// validateRequest validates the incoming request
func (s *Server) validateRequest(req *pb.UserContext) error {
	if req.UserId == "" {
		return fmt.Errorf("user_id is required")
	}
	// Mood can be empty - we'll fetch it from Auth Service
	return nil
}

// generateCacheKey creates a cache key from request parameters
func (s *Server) generateCacheKey(mood, timeOfDay, weather, userSegment string) cache.CacheKey {
	return cache.CacheKey{
		Mood:        mood,
		TimeOfDay:   timeOfDay,
		Weather:     weather,
		Segment:     userSegment,
		RuleVersion: s.engine.GetVersion(),
	}
}

// checkCache attempts to retrieve recommendations from cache
func (s *Server) checkCache(ctx context.Context, key cache.CacheKey) (*pb.ProductRecommendations, bool, error) {
	data, err := s.cache.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	if data == nil {
		return nil, false, nil
	}

	var recommendations pb.ProductRecommendations
	if err := json.Unmarshal(data, &recommendations); err != nil {
		return nil, false, err
	}

	recommendations.Metadata.FromCache = true
	return &recommendations, true, nil
}

// aggregateFilters combines filters from all matched rules
func (s *Server) aggregateFilters(rules []*model.Rule) ([]string, []string, float64, float64) {
	tagMap := make(map[string]bool)
	categoryMap := make(map[string]bool)
	minPrice := 0.0
	maxPrice := 1000000.0

	for _, rule := range rules {
		for _, tag := range rule.Actions.Tags {
			tagMap[tag] = true
		}
		for _, cat := range rule.Actions.Categories {
			categoryMap[cat] = true
		}
		if rule.Actions.PriceRange != nil {
			if rule.Actions.PriceRange.Min > minPrice {
				minPrice = rule.Actions.PriceRange.Min
			}
			if rule.Actions.PriceRange.Max > 0 && rule.Actions.PriceRange.Max < maxPrice {
				maxPrice = rule.Actions.PriceRange.Max
			}
		}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}

	return tags, categories, minPrice, maxPrice
}

// scoreProducts scores and ranks products based on matched rules
func (s *Server) scoreProducts(products []*model.Product, rules []*model.Rule) []*pb.ProductRecommendation {
	type scoredProduct struct {
		product      *model.Product
		score        float64
		matchedRules []string
	}

	scored := make([]scoredProduct, 0, len(products))

	for _, product := range products {
		score := 0.0
		var matchedRuleNames []string

		for _, rule := range rules {
			// Check if product matches rule criteria
			matches := false
			for _, tag := range rule.Actions.Tags {
				for _, pTag := range product.Tags {
					if tag == pTag {
						matches = true
						break
					}
				}
				if matches {
					break
				}
			}

			if matches {
				score += rule.CalculateScore()
				matchedRuleNames = append(matchedRuleNames, rule.Name)
			}
		}

		scored = append(scored, scoredProduct{
			product:      product,
			score:        score,
			matchedRules: matchedRuleNames,
		})
	}

	// Sort by score descending
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// Convert to protobuf
	recommendations := make([]*pb.ProductRecommendation, 0, len(scored))
	for _, sp := range scored {
		recommendations = append(recommendations, &pb.ProductRecommendation{
			ProductId:    sp.product.ProductID,
			Name:         sp.product.Name,
			Description:  sp.product.Description,
			Price:        sp.product.Price,
			Category:     sp.product.Category,
			Tags:         sp.product.Tags,
			Score:        sp.score,
			ImageUrl:     sp.product.ImageURL,
			MatchedRules: sp.matchedRules,
		})
	}

	return recommendations
}
