-- Create rules table
CREATE TABLE IF NOT EXISTS rules (
    id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    conditions JSONB NOT NULL,
    actions JSONB NOT NULL,
    weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    version INTEGER NOT NULL DEFAULT 1,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create GIN index on conditions for fast JSONB queries
CREATE INDEX IF NOT EXISTS idx_rules_conditions_gin ON rules USING GIN (conditions);

-- Create B-tree index on priority for sorting
CREATE INDEX IF NOT EXISTS idx_rules_priority ON rules (priority DESC);

-- Create partial index on active rules only
CREATE INDEX IF NOT EXISTS idx_rules_active ON rules (active, priority DESC) WHERE active = true;

-- Create index on version for tracking rule versions
CREATE INDEX IF NOT EXISTS idx_rules_version ON rules (version DESC);

-- Create composite index for common query patterns
CREATE INDEX IF NOT EXISTS idx_rules_active_priority_created ON rules (active, priority DESC, created_at ASC) WHERE active = true;

-- Add a comment to the table
COMMENT ON TABLE rules IS 'Stores recommendation rules with conditions and actions';
COMMENT ON COLUMN rules.conditions IS 'JSONB containing rule matching conditions (mood, time_of_day, weather, etc.)';
COMMENT ON COLUMN rules.actions IS 'JSONB containing actions to take when rule matches (tags, categories, price_range, boost)';
COMMENT ON COLUMN rules.priority IS 'Higher priority rules are evaluated first';
COMMENT ON COLUMN rules.weight IS 'Weight multiplier for rule scoring';
COMMENT ON COLUMN rules.version IS 'Version number for cache invalidation';
