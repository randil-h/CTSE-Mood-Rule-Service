package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/randil-h/CTSE-Mood-Rule-Service/internal/model"
)

// RuleRepository handles database operations for rules
type RuleRepository struct {
	pool *pgxpool.Pool
}

// NewRuleRepository creates a new rule repository
func NewRuleRepository(pool *pgxpool.Pool) *RuleRepository {
	return &RuleRepository{
		pool: pool,
	}
}

// GetAllActiveRules retrieves all active rules ordered by priority
func (r *RuleRepository) GetAllActiveRules(ctx context.Context) ([]*model.Rule, error) {
	query := `
		SELECT id, name, priority, conditions, actions, weight, version, active, created_at, updated_at
		FROM rules
		WHERE active = true
		ORDER BY priority DESC, created_at ASC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query rules: %w", err)
	}
	defer rows.Close()

	var rules []*model.Rule
	for rows.Next() {
		rule, err := r.scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rule: %w", err)
		}
		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rules: %w", err)
	}

	return rules, nil
}

// GetRuleByID retrieves a rule by its ID
func (r *RuleRepository) GetRuleByID(ctx context.Context, id uuid.UUID) (*model.Rule, error) {
	query := `
		SELECT id, name, priority, conditions, actions, weight, version, active, created_at, updated_at
		FROM rules
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	rule, err := r.scanRule(row)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get rule: %w", err)
	}

	return rule, nil
}

// CreateRule creates a new rule
func (r *RuleRepository) CreateRule(ctx context.Context, rule *model.Rule) error {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return fmt.Errorf("failed to marshal conditions: %w", err)
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return fmt.Errorf("failed to marshal actions: %w", err)
	}

	if rule.ID == uuid.Nil {
		rule.ID = uuid.New()
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	query := `
		INSERT INTO rules (id, name, priority, conditions, actions, weight, version, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.pool.Exec(ctx, query,
		rule.ID,
		rule.Name,
		rule.Priority,
		conditionsJSON,
		actionsJSON,
		rule.Weight,
		rule.Version,
		rule.Active,
		rule.CreatedAt,
		rule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create rule: %w", err)
	}

	return nil
}

// UpdateRule updates an existing rule
func (r *RuleRepository) UpdateRule(ctx context.Context, rule *model.Rule) error {
	conditionsJSON, err := json.Marshal(rule.Conditions)
	if err != nil {
		return fmt.Errorf("failed to marshal conditions: %w", err)
	}

	actionsJSON, err := json.Marshal(rule.Actions)
	if err != nil {
		return fmt.Errorf("failed to marshal actions: %w", err)
	}

	rule.UpdatedAt = time.Now()

	query := `
		UPDATE rules
		SET name = $2, priority = $3, conditions = $4, actions = $5, weight = $6,
		    version = $7, active = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query,
		rule.ID,
		rule.Name,
		rule.Priority,
		conditionsJSON,
		actionsJSON,
		rule.Weight,
		rule.Version,
		rule.Active,
		rule.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	return nil
}

// DeleteRule deletes a rule by ID
func (r *RuleRepository) DeleteRule(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM rules WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("rule not found: %s", id)
	}

	return nil
}

// GetMaxVersion returns the maximum version number of all rules
func (r *RuleRepository) GetMaxVersion(ctx context.Context) (int, error) {
	query := `SELECT COALESCE(MAX(version), 0) FROM rules WHERE active = true`

	var maxVersion int
	err := r.pool.QueryRow(ctx, query).Scan(&maxVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to get max version: %w", err)
	}

	return maxVersion, nil
}

// scanRule is a helper interface for scanning rows
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanRule scans a rule from a database row
func (r *RuleRepository) scanRule(row scanner) (*model.Rule, error) {
	var rule model.Rule
	var conditionsJSON, actionsJSON []byte

	err := row.Scan(
		&rule.ID,
		&rule.Name,
		&rule.Priority,
		&conditionsJSON,
		&actionsJSON,
		&rule.Weight,
		&rule.Version,
		&rule.Active,
		&rule.CreatedAt,
		&rule.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(conditionsJSON, &rule.Conditions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal conditions: %w", err)
	}

	if err := json.Unmarshal(actionsJSON, &rule.Actions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actions: %w", err)
	}

	return &rule, nil
}

// Close closes the database connection pool
func (r *RuleRepository) Close() {
	r.pool.Close()
}
