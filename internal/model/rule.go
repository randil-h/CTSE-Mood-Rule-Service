package model

import (
	"time"

	"github.com/google/uuid"
)

// Rule represents a recommendation rule
type Rule struct {
	ID         uuid.UUID       `json:"id"`
	Name       string          `json:"name"`
	Priority   int             `json:"priority"`
	Conditions RuleConditions  `json:"conditions"`
	Actions    RuleActions     `json:"actions"`
	Weight     float64         `json:"weight"`
	Version    int             `json:"version"`
	Active     bool            `json:"active"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// RuleConditions defines the conditions for a rule to match
type RuleConditions struct {
	Mood         []string          `json:"mood,omitempty"`
	TimeOfDay    []string          `json:"time_of_day,omitempty"`
	Weather      []string          `json:"weather,omitempty"`
	Occasion     []string          `json:"occasion,omitempty"`
	Preferences  map[string]string `json:"preferences,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Logic        string            `json:"logic"` // "AND" or "OR"
}

// RuleActions defines what actions to take when a rule matches
type RuleActions struct {
	Tags         []string          `json:"tags"`
	Categories   []string          `json:"categories,omitempty"`
	PriceRange   *PriceRange       `json:"price_range,omitempty"`
	Boost        float64           `json:"boost,omitempty"`
	Filters      map[string]string `json:"filters,omitempty"`
}

// PriceRange defines a price range filter
type PriceRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// MatchContext contains the context to match against rules
type MatchContext struct {
	Mood                string
	TimeOfDay           string
	Weather             string
	Occasion            string
	Preferences         map[string]string
	PurchaseHistoryTags []string
}

// Matches checks if the rule matches the given context
func (r *Rule) Matches(ctx *MatchContext) bool {
	if !r.Active {
		return false
	}

	conditions := r.Conditions
	matchCount := 0
	totalConditions := 0

	// Check mood
	if len(conditions.Mood) > 0 {
		totalConditions++
		if r.matchesSlice(ctx.Mood, conditions.Mood) {
			matchCount++
		}
	}

	// Check time of day
	if len(conditions.TimeOfDay) > 0 {
		totalConditions++
		if r.matchesSlice(ctx.TimeOfDay, conditions.TimeOfDay) {
			matchCount++
		}
	}

	// Check weather
	if len(conditions.Weather) > 0 {
		totalConditions++
		if r.matchesSlice(ctx.Weather, conditions.Weather) {
			matchCount++
		}
	}

	// Check occasion
	if len(conditions.Occasion) > 0 {
		totalConditions++
		if r.matchesSlice(ctx.Occasion, conditions.Occasion) {
			matchCount++
		}
	}

	// Check preferences
	if len(conditions.Preferences) > 0 {
		totalConditions++
		if r.matchesPreferences(ctx.Preferences, conditions.Preferences) {
			matchCount++
		}
	}

	// Check tags from purchase history
	if len(conditions.Tags) > 0 {
		totalConditions++
		if r.matchesTags(ctx.PurchaseHistoryTags, conditions.Tags) {
			matchCount++
		}
	}

	// Apply logic (AND/OR)
	if conditions.Logic == "OR" {
		return matchCount > 0
	}
	// Default to AND logic
	return totalConditions > 0 && matchCount == totalConditions
}

func (r *Rule) matchesSlice(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func (r *Rule) matchesPreferences(userPrefs, rulePrefs map[string]string) bool {
	for key, ruleValue := range rulePrefs {
		userValue, exists := userPrefs[key]
		if !exists || userValue != ruleValue {
			return false
		}
	}
	return true
}

func (r *Rule) matchesTags(userTags, ruleTags []string) bool {
	tagMap := make(map[string]bool, len(userTags))
	for _, tag := range userTags {
		tagMap[tag] = true
	}

	for _, ruleTag := range ruleTags {
		if tagMap[ruleTag] {
			return true
		}
	}
	return false
}

// CalculateScore calculates the final score for this rule
func (r *Rule) CalculateScore() float64 {
	baseScore := float64(r.Priority) * r.Weight
	if r.Actions.Boost > 0 {
		baseScore *= (1.0 + r.Actions.Boost)
	}
	return baseScore
}
