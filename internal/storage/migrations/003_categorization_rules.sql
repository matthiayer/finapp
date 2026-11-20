-- +goose Up
CREATE TABLE categorization_rules (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
    combine_mode TEXT NOT NULL CHECK (combine_mode IN ('all', 'any')),
    conditions_json TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
) STRICT;

CREATE INDEX categorization_rules_enabled_idx
    ON categorization_rules (enabled, category_id);

-- +goose Down
DROP INDEX categorization_rules_enabled_idx;
DROP TABLE categorization_rules;
