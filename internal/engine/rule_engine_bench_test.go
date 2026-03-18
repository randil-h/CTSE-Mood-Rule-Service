package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
)

// BenchmarkRuleEvaluation benchmarks rule evaluation performance
func BenchmarkRuleEvaluation(b *testing.B) {
	ctx := context.Background()

	// Create 100 rules
	rules := make([]*model.Rule, 100)
	for i := 0; i < 100; i++ {
		rules[i] = &model.Rule{
			ID:       uuid.New(),
			Name:     "Rule 01",
			Priority: i,
			Conditions: model.RuleConditions{
				Mood:      []string{"happy", "sad", "excited"},
				TimeOfDay: []string{"morning", "afternoon", "evening"},
				Logic:     "OR",
			},
			Actions: model.RuleActions{
				Tags:       []string{"tag1", "tag2", "tag3"},
				Categories: []string{"cat1", "cat2"},
			},
			Weight:  1.5,
			Version: 1,
			Active:  true,
		}
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	engine := NewRuleEngine(mockRepo)
	engine.Load(ctx)

	matchCtx := &model.MatchContext{
		Mood:      "happy",
		TimeOfDay: "morning",
		Weather:   "sunny",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(ctx, matchCtx)
	}
}

// BenchmarkRuleEvaluation1000Rules benchmarks with 1000 rules
func BenchmarkRuleEvaluation1000Rules(b *testing.B) {
	ctx := context.Background()

	// Create 1000 rules
	rules := make([]*model.Rule, 1000)
	for i := 0; i < 1000; i++ {
		rules[i] = &model.Rule{
			ID:       uuid.New(),
			Name:     "Test Rule",
			Priority: i,
			Conditions: model.RuleConditions{
				Mood:      []string{"happy", "sad", "excited"},
				TimeOfDay: []string{"morning", "afternoon", "evening"},
				Weather:   []string{"sunny", "rainy", "cloudy"},
				Logic:     "OR",
			},
			Actions: model.RuleActions{
				Tags:       []string{"tag1", "tag2", "tag3"},
				Categories: []string{"cat1", "cat2"},
			},
			Weight:  1.5,
			Version: 1,
			Active:  true,
		}
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	engine := NewRuleEngine(mockRepo)
	engine.Load(ctx)

	matchCtx := &model.MatchContext{
		Mood:      "happy",
		TimeOfDay: "morning",
		Weather:   "sunny",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(ctx, matchCtx)
	}
}

// BenchmarkRuleMatching benchmarks individual rule matching
func BenchmarkRuleMatching(b *testing.B) {
	rule := &model.Rule{
		ID:       uuid.New(),
		Name:     "Test Rule",
		Priority: 100,
		Conditions: model.RuleConditions{
			Mood:      []string{"happy", "excited"},
			TimeOfDay: []string{"morning", "afternoon"},
			Weather:   []string{"sunny", "clear"},
			Logic:     "AND",
		},
		Active: true,
	}

	matchCtx := &model.MatchContext{
		Mood:      "happy",
		TimeOfDay: "morning",
		Weather:   "sunny",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rule.Matches(matchCtx)
	}
}

// BenchmarkRuleLoad benchmarks rule loading
func BenchmarkRuleLoad(b *testing.B) {
	ctx := context.Background()

	rules := make([]*model.Rule, 1000)
	for i := 0; i < 1000; i++ {
		rules[i] = &model.Rule{
			ID:       uuid.New(),
			Name:     "Test Rule",
			Priority: i,
			Conditions: model.RuleConditions{
				Mood:  []string{"happy"},
				Logic: "AND",
			},
			Actions: model.RuleActions{
				Tags: []string{"tag1"},
			},
			Weight:  1.0,
			Version: 1,
			Active:  true,
		}
	}

	mockRepo := &MockRuleRepository{
		rules:      rules,
		maxVersion: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine := NewRuleEngine(mockRepo)
		engine.Load(ctx)
	}
}

// BenchmarkConcurrentEvaluation benchmarks concurrent rule evaluation
func BenchmarkConcurrentEvaluation(b *testing.B) {
	ctx := context.Background()

	rules := make([]*model.Rule, 500)
	for i := 0; i < 500; i++ {
		rules[i] = &model.Rule{
			ID:       uuid.New(),
			Name:     "Test Rule",
			Priority: i,
			Conditions: model.RuleConditions{
				Mood:  []string{"happy", "sad", "excited"},
				Logic: "OR",
			},
			Actions: model.RuleActions{
				Tags: []string{"tag1", "tag2"},
			},
			Weight:  1.5,
			Version: 1,
			Active:  true,
		}
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

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			engine.Evaluate(ctx, matchCtx)
		}
	})
}
