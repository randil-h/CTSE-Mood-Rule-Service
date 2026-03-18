-- Drop indexes
DROP INDEX IF EXISTS idx_rules_active_priority_created;
DROP INDEX IF EXISTS idx_rules_version;
DROP INDEX IF EXISTS idx_rules_active;
DROP INDEX IF EXISTS idx_rules_priority;
DROP INDEX IF EXISTS idx_rules_conditions_gin;

-- Drop table
DROP TABLE IF EXISTS rules;
