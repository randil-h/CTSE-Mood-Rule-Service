package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
	"github.com/stretchr/testify/assert"
)

// MockRuleRepository is a mock implementation for testing
type MockRuleRepository struct {
	rules      []*model.Rule
	maxVersion int
}

func (m *MockRuleRepository) GetAllActiveRules(ctx context.Context) ([]*model.Rule, error) {
	return m.rules, nil
}

func (m *MockRuleRepository) GetMaxVersion(ctx context.Context) (int, error) {
	return m.maxVersion, nil
}

func TestRuleEngineLoad(t *testing.T) {
	ctx := context.Background()

	rules := []*model.Rule{
		{
			ID:       uuid.New(),
			Name:     "Happy Morning",
			Priority: 100,
			Conditions: model.RuleConditions{
				Mood:      []string{"happy"},
				TimeOfDay: []string{"morning"},
				Logic:     "AND",
			},
			Actions: model.RuleActions{
				Tags: []string{"energizing", "fresh"},
			},
			Weight:  1.5,
			Version: 1,
			Active:  true,
		},
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	engine := NewRuleEngine(mockRepo)
	err := engine.Load(ctx)

	assert.NoError(t, err)
	assert.Equal(t, 1, engine.GetRuleCount())
	assert.Equal(t, 1, engine.GetVersion())
}

func TestRuleEngineEvaluate(t *testing.T) {
	ctx := context.Background()

	rules := []*model.Rule{
		{
			ID:       uuid.New(),
			Name:     "Happy Morning",
			Priority: 100,
			Conditions: model.RuleConditions{
				Mood:      []string{"happy"},
				TimeOfDay: []string{"morning"},
				Logic:     "AND",
			},
			Actions: model.RuleActions{
				Tags: []string{"energizing", "fresh"},
			},
			Weight:  1.5,
			Version: 1,
			Active:  true,
		},
		{
			ID:       uuid.New(),
			Name:     "Sad Comfort",
			Priority: 90,
			Conditions: model.RuleConditions{
				Mood:  []string{"sad"},
				Logic: "AND",
			},
			Actions: model.RuleActions{
				Tags: []string{"comfort", "cozy"},
			},
			Weight:  1.8,
			Version: 1,
			Active:  true,
		},
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	engine := NewRuleEngine(mockRepo)
	engine.Load(ctx)

	// Test case 1: Match happy morning
	matchCtx := &model.MatchContext{
		Mood:      "happy",
		TimeOfDay: "morning",
	}

	matched, stats := engine.Evaluate(ctx, matchCtx)

	assert.Equal(t, 1, stats.RulesMatched)
	assert.Equal(t, 1, len(matched))
	assert.Equal(t, "Happy Morning", matched[0].Name)

	// Test case 2: Match sad mood
	matchCtx2 := &model.MatchContext{
		Mood: "sad",
	}

	matched2, stats2 := engine.Evaluate(ctx, matchCtx2)

	assert.Equal(t, 1, stats2.RulesMatched)
	assert.Equal(t, 1, len(matched2))
	assert.Equal(t, "Sad Comfort", matched2[0].Name)

	// Test case 3: No match
	matchCtx3 := &model.MatchContext{
		Mood: "angry",
	}

	matched3, stats3 := engine.Evaluate(ctx, matchCtx3)

	assert.Equal(t, 0, stats3.RulesMatched)
	assert.Equal(t, 0, len(matched3))
}

func TestRuleMatching(t *testing.T) {
	tests := []struct {
		name      string
		rule      *model.Rule
		context   *model.MatchContext
		shouldMatch bool
	}{
		{
			name: "AND logic - all conditions match",
			rule: &model.Rule{
				Conditions: model.RuleConditions{
					Mood:      []string{"happy"},
					TimeOfDay: []string{"morning"},
					Logic:     "AND",
				},
				Active: true,
			},
			context: &model.MatchContext{
				Mood:      "happy",
				TimeOfDay: "morning",
			},
			shouldMatch: true,
		},
		{
			name: "AND logic - partial match",
			rule: &model.Rule{
				Conditions: model.RuleConditions{
					Mood:      []string{"happy"},
					TimeOfDay: []string{"morning"},
					Logic:     "AND",
				},
				Active: true,
			},
			context: &model.MatchContext{
				Mood:      "happy",
				TimeOfDay: "evening",
			},
			shouldMatch: false,
		},
		{
			name: "OR logic - one condition matches",
			rule: &model.Rule{
				Conditions: model.RuleConditions{
					Mood:  []string{"happy", "excited"},
					Logic: "OR",
				},
				Active: true,
			},
			context: &model.MatchContext{
				Mood: "happy",
			},
			shouldMatch: true,
		},
		{
			name: "OR logic - no conditions match",
			rule: &model.Rule{
				Conditions: model.RuleConditions{
					Mood:  []string{"happy", "excited"},
					Logic: "OR",
				},
				Active: true,
			},
			context: &model.MatchContext{
				Mood: "sad",
			},
			shouldMatch: false,
		},
		{
			name: "Inactive rule",
			rule: &model.Rule{
				Conditions: model.RuleConditions{
					Mood: []string{"happy"},
				},
				Active: false,
			},
			context: &model.MatchContext{
				Mood: "happy",
			},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.rule.Matches(tt.context)
			assert.Equal(t, tt.shouldMatch, result)
		})
	}
}

func TestRulePriorityOrdering(t *testing.T) {
	ctx := context.Background()

	rules := []*model.Rule{
		{
			ID:       uuid.New(),
			Name:     "Low Priority",
			Priority: 50,
			Conditions: model.RuleConditions{
				Mood:  []string{"happy"},
				Logic: "AND",
			},
			Active: true,
		},
		{
			ID:       uuid.New(),
			Name:     "High Priority",
			Priority: 100,
			Conditions: model.RuleConditions{
				Mood:  []string{"happy"},
				Logic: "AND",
			},
			Active: true,
		},
		{
			ID:       uuid.New(),
			Name:     "Medium Priority",
			Priority: 75,
			Conditions: model.RuleConditions{
				Mood:  []string{"happy"},
				Logic: "AND",
			},
			Active: true,
		},
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	engine := NewRuleEngine(mockRepo)
	engine.Load(ctx)

	matchCtx := &model.MatchContext{
		Mood: "happy",
	}

	matched, _ := engine.Evaluate(ctx, matchCtx)

	assert.Equal(t, 3, len(matched))
	assert.Equal(t, "High Priority", matched[0].Name)
	assert.Equal(t, "Medium Priority", matched[1].Name)
	assert.Equal(t, "Low Priority", matched[2].Name)
}
