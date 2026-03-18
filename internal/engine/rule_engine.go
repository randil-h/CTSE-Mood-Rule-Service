package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/repository"
	"github.com/randil-h/CTSE-Mood-Rule-Service/pkg/logger"
	"go.uber.org/zap"
)

// RuleEngine manages and evaluates rules in memory
type RuleEngine struct {
	mu          sync.RWMutex
	rules       []*model.Rule
	rulesByMood map[string][]*model.Rule
	version     int
	repo        *repository.RuleRepository
}

// NewRuleEngine creates a new rule engine
func NewRuleEngine(repo *repository.RuleRepository) *RuleEngine {
	return &RuleEngine{
		rules:       make([]*model.Rule, 0, 1000),
		rulesByMood: make(map[string][]*model.Rule),
		repo:        repo,
	}
}

// Load loads all active rules from the database into memory
func (e *RuleEngine) Load(ctx context.Context) error {
	rules, err := e.repo.GetAllActiveRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	version, err := e.repo.GetMaxVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get max version: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.rules = rules
	e.version = version
	e.buildIndex()

	logger.Info(ctx, "Rules loaded into memory",
		zap.Int("count", len(rules)),
		zap.Int("version", version))

	return nil
}

// Reload reloads rules from the database (hot reload)
func (e *RuleEngine) Reload(ctx context.Context) error {
	startTime := time.Now()

	rules, err := e.repo.GetAllActiveRules(ctx)
	if err != nil {
		return fmt.Errorf("failed to reload rules: %w", err)
	}

	version, err := e.repo.GetMaxVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get max version: %w", err)
	}

	e.mu.Lock()
	oldVersion := e.version
	e.rules = rules
	e.version = version
	e.buildIndex()
	e.mu.Unlock()

	duration := time.Since(startTime)
	logger.Info(ctx, "Rules reloaded",
		zap.Int("count", len(rules)),
		zap.Int("old_version", oldVersion),
		zap.Int("new_version", version),
		zap.Duration("duration", duration))

	return nil
}

// buildIndex builds indexes for faster rule lookup (must be called with lock held)
func (e *RuleEngine) buildIndex() {
	e.rulesByMood = make(map[string][]*model.Rule)

	for _, rule := range e.rules {
		if len(rule.Conditions.Mood) > 0 {
			for _, mood := range rule.Conditions.Mood {
				e.rulesByMood[mood] = append(e.rulesByMood[mood], rule)
			}
		} else {
			// Rules without mood condition match all moods
			e.rulesByMood["*"] = append(e.rulesByMood["*"], rule)
		}
	}
}

// Evaluate evaluates rules against the given context and returns matched rules
func (e *RuleEngine) Evaluate(ctx context.Context, matchCtx *model.MatchContext) ([]*model.Rule, EvaluationStats) {
	startTime := time.Now()

	e.mu.RLock()
	defer e.mu.RUnlock()

	var matched []*model.Rule
	evaluated := 0

	// Get rules for specific mood
	candidateRules := e.rulesByMood[matchCtx.Mood]
	// Also include rules that match all moods
	candidateRules = append(candidateRules, e.rulesByMood["*"]...)

	for _, rule := range candidateRules {
		evaluated++
		if rule.Matches(matchCtx) {
			matched = append(matched, rule)
		}
	}

	// Sort by priority and score
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Priority != matched[j].Priority {
			return matched[i].Priority > matched[j].Priority
		}
		return matched[i].CalculateScore() > matched[j].CalculateScore()
	})

	duration := time.Since(startTime)

	stats := EvaluationStats{
		RulesEvaluated: evaluated,
		RulesMatched:   len(matched),
		Duration:       duration,
		Version:        e.version,
	}

	logger.Debug(ctx, "Rules evaluated",
		zap.Int("evaluated", evaluated),
		zap.Int("matched", len(matched)),
		zap.Duration("duration", duration))

	return matched, stats
}

// GetVersion returns the current rule version
func (e *RuleEngine) GetVersion() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.version
}

// GetRuleCount returns the number of loaded rules
func (e *RuleEngine) GetRuleCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.rules)
}

// EvaluationStats contains statistics about rule evaluation
type EvaluationStats struct {
	RulesEvaluated int
	RulesMatched   int
	Duration       time.Duration
	Version        int
}
