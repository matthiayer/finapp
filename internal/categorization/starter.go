package categorization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/matthiayer/finapp/internal/domain"
)

type StarterRule struct {
	Name       string
	Category   string
	Combine    string
	Conditions []Condition
}

type StarterCategoryPreview struct {
	Name   string
	Exists bool
}

type StarterRulePreview struct {
	Name          string
	Category      string
	ExistingCount int
}

type StarterPreview struct {
	Categories   []StarterCategoryPreview
	Rules        []StarterRulePreview
	Transactions []StarterTransactionMatch
}

type StarterTransactionMatch struct {
	ID               int64
	BookingDate      string
	Amount           string
	Description      string
	Direction        string
	ExistingCategory string
	ManualCategory   bool
	Options          []string
	RuleNames        []string
}

var starterCategories = []string{
	"Salary", "Benefits", "Interest income", "Refunds", "Other income",
	"Housing", "Utilities", "Groceries", "Restaurants and cafes", "Transport",
	"Vehicle expenses", "Health", "Insurance", "Household", "Clothing",
	"Personal care", "Education", "Communications", "Subscriptions",
	"Entertainment", "Travel", "Taxes", "Bank fees", "Gifts and donations",
	"Other expenses",
}

var starterRules = []StarterRule{
	{
		Name: "Salary description terms", Category: "Salary", Combine: "all",
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: string(domain.DirectionIncoming)},
			{Field: "description", Operator: "contains_any", Values: []string{
				"salary", "salaris", "loon", "wedde", "salaire",
			}},
		},
	},
	{
		Name: "Interest description terms", Category: "Interest income", Combine: "all",
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: string(domain.DirectionIncoming)},
			{Field: "description", Operator: "contains_any", Values: []string{
				"interest", "rente", "interet", "intérêt",
			}},
		},
	},
	{
		Name: "Refund description terms", Category: "Refunds", Combine: "all",
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: string(domain.DirectionIncoming)},
			{Field: "description", Operator: "contains_any", Values: []string{
				"refund", "terugbetaling", "terugstorting", "remboursement",
			}},
		},
	},
	{
		Name: "Bank fee description terms", Category: "Bank fees", Combine: "all",
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: string(domain.DirectionOutgoing)},
			{Field: "description", Operator: "contains_any", Values: []string{
				"bank fee", "bank fees", "bankkosten", "rekeningkosten",
				"frais bancaires", "frais de compte",
			}},
		},
	},
}

func StarterCategories() []string {
	return append([]string(nil), starterCategories...)
}

func StarterRules() []StarterRule {
	result := make([]StarterRule, len(starterRules))
	for i, rule := range starterRules {
		result[i] = rule
		result[i].Conditions = append([]Condition(nil), rule.Conditions...)
		for j := range result[i].Conditions {
			result[i].Conditions[j].Values = append([]string(nil), rule.Conditions[j].Values...)
		}
	}
	return result
}

func PreviewStarterSelection(
	ctx context.Context,
	db *sql.DB,
	categoryNames, ruleNames []string,
) (StarterPreview, error) {
	categories, rules, err := validateStarterSelection(ctx, db, categoryNames, ruleNames)
	if err != nil {
		return StarterPreview{}, err
	}
	preview := StarterPreview{}
	for _, name := range categories {
		var exists int
		err := db.QueryRowContext(ctx,
			"SELECT 1 FROM categories WHERE name = ? COLLATE NOCASE", name,
		).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			return StarterPreview{}, fmt.Errorf("check starter category: %w", err)
		}
		preview.Categories = append(preview.Categories,
			StarterCategoryPreview{Name: name, Exists: err == nil})
	}
	for _, rule := range rules {
		count, err := countExistingMatches(ctx, db, rule)
		if err != nil {
			return StarterPreview{}, err
		}
		preview.Rules = append(preview.Rules, StarterRulePreview{
			Name: rule.Name, Category: rule.Category, ExistingCount: count,
		})
	}
	preview.Transactions, err = existingMatches(ctx, db, rules)
	if err != nil {
		return StarterPreview{}, err
	}
	return preview, nil
}

func ApplyStarterSelection(
	ctx context.Context,
	db *sql.DB,
	categoryNames, ruleNames []string,
	choices map[int64]string,
) error {
	categories, rules, err := validateStarterSelection(ctx, db, categoryNames, ruleNames)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin starter-set transaction: %w", err)
	}
	defer tx.Rollback()

	for _, rule := range starterRules {
		if _, err := tx.ExecContext(ctx,
			"UPDATE categorization_rules SET enabled = 0 WHERE name = ? COLLATE NOCASE", rule.Name,
		); err != nil {
			return fmt.Errorf("disable unselected starter rule %q: %w", rule.Name, err)
		}
	}
	for _, name := range categories {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO categories (name) VALUES (?) ON CONFLICT(name) DO NOTHING", name,
		); err != nil {
			return fmt.Errorf("add starter category: %w", err)
		}
	}
	for _, rule := range rules {
		var categoryID int64
		if err := tx.QueryRowContext(ctx,
			"SELECT id FROM categories WHERE name = ? COLLATE NOCASE", rule.Category,
		).Scan(&categoryID); err != nil {
			return fmt.Errorf("find category for starter rule %q: %w", rule.Name, err)
		}
		conditions, err := json.Marshal(rule.Conditions)
		if err != nil {
			return fmt.Errorf("encode starter rule %q: %w", rule.Name, err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO categorization_rules
				(name, category_id, combine_mode, conditions_json, enabled)
			VALUES (?, ?, ?, ?, 1)
			ON CONFLICT(name) DO UPDATE SET
				category_id = excluded.category_id,
				combine_mode = excluded.combine_mode,
				conditions_json = excluded.conditions_json,
				enabled = 1`,
			rule.Name, categoryID, rule.Combine, string(conditions),
		); err != nil {
			return fmt.Errorf("enable starter rule %q: %w", rule.Name, err)
		}
	}
	matches, err := existingMatches(ctx, tx, rules)
	if err != nil {
		return err
	}
	for _, match := range matches {
		choice := choices[match.ID]
		if choice == "" && !match.ManualCategory && len(match.Options) == 1 {
			choice = match.Options[0]
		}
		if choice == "" {
			return fmt.Errorf("choose a category for existing transaction %d", match.ID)
		}
		valid := false
		for _, option := range match.Options {
			if option == choice {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid category choice for existing transaction %d", match.ID)
		}
		if match.ManualCategory && choice == match.ExistingCategory {
			continue
		}
		var categoryID int64
		if err := tx.QueryRowContext(ctx,
			"SELECT id FROM categories WHERE name = ? COLLATE NOCASE", choice,
		).Scan(&categoryID); err != nil {
			return fmt.Errorf("find chosen category for existing transaction %d: %w", match.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE transactions SET category_id = ?, is_manual_category = 0
			WHERE id = ?`, categoryID, match.ID,
		); err != nil {
			return fmt.Errorf("apply starter rule to existing transaction %d: %w", match.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit starter-set selection: %w", err)
	}
	return nil
}

type queryContext interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func existingMatches(
	ctx context.Context,
	queryer queryContext,
	rules []StarterRule,
) ([]StarterTransactionMatch, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT t.id, t.booking_date, t.amount, t.description, t.direction,
			c.name, t.is_manual_category
		FROM transactions t
		JOIN categories c ON c.id = t.category_id
		WHERE t.pending = 0 AND t.transfer_id IS NULL
			AND t.direction IN ('incoming', 'outgoing')
		ORDER BY t.booking_date, t.id`)
	if err != nil {
		return nil, fmt.Errorf("load existing transactions for starter rules: %w", err)
	}
	defer rows.Close()
	var matches []StarterTransactionMatch
	for rows.Next() {
		var match StarterTransactionMatch
		var manual int
		if err := rows.Scan(&match.ID, &match.BookingDate, &match.Amount, &match.Description,
			&match.Direction, &match.ExistingCategory, &manual); err != nil {
			return nil, fmt.Errorf("scan existing starter rule match: %w", err)
		}
		match.ManualCategory = manual == 1
		optionSet := make(map[string]struct{})
		for _, rule := range rules {
			if !Matches(Rule{
				Enabled: true, Combine: rule.Combine, Conditions: rule.Conditions,
			}, domain.Direction(match.Direction), match.Description) {
				continue
			}
			optionSet[rule.Category] = struct{}{}
			match.RuleNames = append(match.RuleNames, rule.Name)
		}
		if len(match.RuleNames) == 0 {
			continue
		}
		if match.ManualCategory {
			optionSet[match.ExistingCategory] = struct{}{}
		}
		for option := range optionSet {
			match.Options = append(match.Options, option)
		}
		sort.Strings(match.Options)
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing starter rule matches: %w", err)
	}
	return matches, nil
}

func validateStarterSelection(
	ctx context.Context,
	db *sql.DB,
	categoryNames, ruleNames []string,
) ([]string, []StarterRule, error) {
	categories, err := validateNames(categoryNames, starterCategories)
	if err != nil {
		return nil, nil, fmt.Errorf("select starter categories: %w", err)
	}
	selectedRules, err := validateNames(ruleNames, starterRuleNames())
	if err != nil {
		return nil, nil, fmt.Errorf("select starter rules: %w", err)
	}
	selectedCategorySet := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		selectedCategorySet[strings.ToLower(category)] = struct{}{}
	}
	ruleByName := make(map[string]StarterRule, len(starterRules))
	for _, rule := range starterRules {
		ruleByName[rule.Name] = rule
	}
	rules := make([]StarterRule, 0, len(selectedRules))
	for _, name := range selectedRules {
		rule := ruleByName[name]
		_, selected := selectedCategorySet[strings.ToLower(rule.Category)]
		var exists int
		err := db.QueryRowContext(ctx,
			"SELECT 1 FROM categories WHERE name = ? COLLATE NOCASE", rule.Category,
		).Scan(&exists)
		if err != nil && err != sql.ErrNoRows {
			return nil, nil, fmt.Errorf("check category for starter rule: %w", err)
		}
		if err == sql.ErrNoRows && !selected {
			return nil, nil, fmt.Errorf("select category %q or add it before enabling rule %q", rule.Category, rule.Name)
		}
		rules = append(rules, rule)
	}
	return categories, rules, nil
}

func countExistingMatches(ctx context.Context, db *sql.DB, rule StarterRule) (int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT direction, description
		FROM transactions
		WHERE pending = 0 AND transfer_id IS NULL
			AND direction IN ('incoming', 'outgoing')`)
	if err != nil {
		return 0, fmt.Errorf("find existing matches for %q: %w", rule.Name, err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var direction, description string
		if err := rows.Scan(&direction, &description); err != nil {
			return 0, fmt.Errorf("read existing rule match: %w", err)
		}
		if Matches(Rule{
			Enabled: true, Combine: rule.Combine, Conditions: rule.Conditions,
		}, domain.Direction(direction), description) {
			count++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate existing rule matches: %w", err)
	}
	return count, nil
}

func starterRuleNames() []string {
	names := make([]string, len(starterRules))
	for i, rule := range starterRules {
		names[i] = rule.Name
	}
	return names
}

func validateNames(values, allowed []string) ([]string, error) {
	allowedNames := make(map[string]string, len(allowed))
	for _, name := range allowed {
		allowedNames[strings.ToLower(name)] = name
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		name, ok := allowedNames[strings.ToLower(strings.TrimSpace(value))]
		if !ok {
			return nil, errors.New("selection contains an unknown item")
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return nil, errors.New("selection contains a duplicate item")
		}
		seen[key] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}
