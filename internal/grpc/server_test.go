package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/engine"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
	pb "github.com/randil-h/CTSE-Mood-Rule-Service/proto/moodrule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRuleEngine is a mock for testing
type MockRuleEngine struct {
	mock.Mock
}

func (m *MockRuleEngine) Evaluate(ctx context.Context, matchCtx *model.MatchContext) ([]*model.Rule, engine.EvaluationStats) {
	args := m.Called(ctx, matchCtx)
	return args.Get(0).([]*model.Rule), args.Get(1).(engine.EvaluationStats)
}

func (m *MockRuleEngine) GetVersion() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockRuleEngine) GetRuleCount() int {
	args := m.Called()
	return args.Int(0)
}

// MockCache is a mock for testing
type MockCache struct {
	mock.Mock
}

func (m *MockCache) Get(ctx context.Context, key interface{}) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCache) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockAuthClient is a mock for testing
type MockAuthClient struct {
	mock.Mock
}

func (m *MockAuthClient) GetUserMood(ctx context.Context, userID string, traceID string) (string, error) {
	args := m.Called(ctx, userID, traceID)
	return args.String(0), args.Error(1)
}

// MockProductClient is a mock for testing
type MockProductClient struct {
	mock.Mock
}

func (m *MockProductClient) GetProductsByFilters(
	ctx context.Context,
	tags []string,
	categories []string,
	minPrice float64,
	maxPrice float64,
	limit int32,
	traceID string,
) ([]*model.Product, error) {
	args := m.Called(ctx, tags, categories, minPrice, maxPrice, limit, traceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Product), args.Error(1)
}

func TestRecommendValidation(t *testing.T) {
	mockEngine := new(MockRuleEngine)
	mockCache := new(MockCache)
	mockAuthClient := new(MockAuthClient)
	mockProductClient := new(MockProductClient)

	server := NewServer(mockEngine, mockCache, mockAuthClient, mockProductClient)

	tests := []struct {
		name    string
		req     *pb.UserContext
		wantErr bool
	}{
		{
			name: "Valid request",
			req: &pb.UserContext{
				UserId: "user123",
				Mood:   "happy",
			},
			wantErr: false,
		},
		{
			name: "Missing user ID",
			req: &pb.UserContext{
				Mood: "happy",
			},
			wantErr: true,
		},
		{
			name: "Valid request without mood (will fetch from auth)",
			req: &pb.UserContext{
				UserId: "user123",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := server.validateRequest(tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	mockEngine := new(MockRuleEngine)
	mockCache := new(MockCache)
	mockAuthClient := new(MockAuthClient)
	mockProductClient := new(MockProductClient)

	mockCache.On("Ping", mock.Anything).Return(nil)
	mockEngine.On("GetRuleCount").Return(10)

	server := NewServer(mockEngine, mockCache, mockAuthClient, mockProductClient)

	resp, err := server.HealthCheck(context.Background(), &pb.HealthCheckRequest{})

	assert.NoError(t, err)
	assert.Equal(t, "healthy", resp.Status)
	assert.Contains(t, resp.Dependencies, "redis")
	assert.Contains(t, resp.Dependencies, "rule_engine")
}

func TestAggregateFilters(t *testing.T) {
	server := &Server{}

	rules := []*model.Rule{
		{
			ID: uuid.New(),
			Actions: model.RuleActions{
				Tags:       []string{"tag1", "tag2"},
				Categories: []string{"cat1"},
				PriceRange: &model.PriceRange{Min: 10, Max: 100},
			},
		},
		{
			ID: uuid.New(),
			Actions: model.RuleActions{
				Tags:       []string{"tag2", "tag3"},
				Categories: []string{"cat2"},
				PriceRange: &model.PriceRange{Min: 20, Max: 80},
			},
		},
	}

	tags, categories, minPrice, maxPrice := server.aggregateFilters(rules)

	assert.Contains(t, tags, "tag1")
	assert.Contains(t, tags, "tag2")
	assert.Contains(t, tags, "tag3")
	assert.Contains(t, categories, "cat1")
	assert.Contains(t, categories, "cat2")
	assert.Equal(t, 20.0, minPrice)
	assert.Equal(t, 80.0, maxPrice)
}

func TestScoreProducts(t *testing.T) {
	server := &Server{}

	rules := []*model.Rule{
		{
			ID:       uuid.New(),
			Name:     "Rule1",
			Priority: 100,
			Weight:   1.5,
			Actions: model.RuleActions{
				Tags: []string{"tag1"},
			},
		},
	}

	products := []*model.Product{
		{
			ProductID: "prod1",
			Name:      "Product 1",
			Tags:      []string{"tag1", "tag2"},
			Price:     50.0,
		},
		{
			ProductID: "prod2",
			Name:      "Product 2",
			Tags:      []string{"tag3"},
			Price:     30.0,
		},
	}

	scored := server.scoreProducts(products, rules)

	assert.Equal(t, 2, len(scored))
	// Product 1 should have higher score than Product 2
	assert.True(t, scored[0].Score >= scored[1].Score)
}
