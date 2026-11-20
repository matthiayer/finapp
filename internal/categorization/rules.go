package categorization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/matthiayer/finapp/internal/domain"
)

type Rule struct {
	ID         int64
	Name       string
	CategoryID int64
	Category   string
	Combine    string
	Conditions []Condition
	Enabled    bool
}

type Condition struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Value    string   `json:"value,omitempty"`
	Values   []string `json:"values,omitempty"`
}

func Validate(rule Rule) error {
	if strings.TrimSpace(rule.Name) == "" {
		return errors.New("rule name is required")
	}
	if rule.CategoryID < 1 {
		return errors.New("rule category is required")
	}
	if rule.Combine != "all" && rule.Combine != "any" {
		return errors.New("rule combine mode must be all or any")
	}
	if len(rule.Conditions) == 0 {
		return errors.New("rule must have at least one condition")
	}
	for _, condition := range rule.Conditions {
		switch {
		case condition.Field == "direction" && condition.Operator == "equals":
			if condition.Value != string(domain.DirectionIncoming) &&
				condition.Value != string(domain.DirectionOutgoing) {
				return errors.New("direction rule value must be incoming or outgoing")
			}
		case condition.Field == "description" && condition.Operator == "contains_any":
			if len(condition.Values) == 0 {
				return errors.New("description rule must include at least one phrase")
			}
			for _, value := range condition.Values {
				if strings.TrimSpace(value) == "" {
					return errors.New("description rule phrases must not be empty")
				}
			}
		default:
			return fmt.Errorf("unsupported rule condition %q with operator %q", condition.Field, condition.Operator)
		}
	}
	return nil
}

func Store(ctx context.Context, db *sql.DB, rule Rule) (int64, error) {
	if err := Validate(rule); err != nil {
		return 0, err
	}
	conditions, err := json.Marshal(rule.Conditions)
	if err != nil {
		return 0, fmt.Errorf("encode rule conditions: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin categorization rule transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO categorization_rules
			(name, category_id, combine_mode, conditions_json, enabled)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			category_id = excluded.category_id,
			combine_mode = excluded.combine_mode,
			conditions_json = excluded.conditions_json,
			enabled = excluded.enabled`,
		strings.TrimSpace(rule.Name), rule.CategoryID, rule.Combine, string(conditions), boolInt(rule.Enabled))
	if err != nil {
		return 0, fmt.Errorf("save categorization rule: %w", err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM categorization_rules WHERE name = ?", strings.TrimSpace(rule.Name),
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("find saved categorization rule: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit categorization rule: %w", err)
	}
	return id, nil
}

func Enabled(ctx context.Context, db *sql.DB) ([]Rule, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT r.id, r.name, r.category_id, c.name, r.combine_mode,
			r.conditions_json, r.enabled
		FROM categorization_rules r
		JOIN categories c ON c.id = r.category_id
		WHERE r.enabled = 1
		ORDER BY r.id`)
	if err != nil {
		return nil, fmt.Errorf("load enabled categorization rules: %w", err)
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var rule Rule
		var conditionsJSON string
		var enabled int
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.CategoryID, &rule.Category,
			&rule.Combine, &conditionsJSON, &enabled); err != nil {
			return nil, fmt.Errorf("scan categorization rule: %w", err)
		}
		if err := json.Unmarshal([]byte(conditionsJSON), &rule.Conditions); err != nil {
			return nil, fmt.Errorf("decode categorization rule %d: %w", rule.ID, err)
		}
		rule.Enabled = enabled == 1
		if err := Validate(rule); err != nil {
			return nil, fmt.Errorf("validate stored categorization rule %d: %w", rule.ID, err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categorization rules: %w", err)
	}
	return rules, nil
}

func Matches(rule Rule, direction domain.Direction, description string) bool {
	if !rule.Enabled || len(rule.Conditions) == 0 {
		return false
	}
	matchedCount := 0
	for _, condition := range rule.Conditions {
		matched := false
		switch {
		case condition.Field == "direction" && condition.Operator == "equals":
			matched = string(direction) == condition.Value
		case condition.Field == "description" && condition.Operator == "contains_any":
			foldedDescription := strings.ToLower(description)
			for _, phrase := range condition.Values {
				if strings.Contains(foldedDescription, strings.ToLower(phrase)) {
					matched = true
					break
				}
			}
		}
		if matched {
			matchedCount++
		}
	}
	if rule.Combine == "all" {
		return matchedCount == len(rule.Conditions)
	}
	return matchedCount > 0
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
