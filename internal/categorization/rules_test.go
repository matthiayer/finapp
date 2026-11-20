package categorization

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/matthiayer/finapp/internal/storage"
)

func TestStoreAndLoadEnabledRules(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	category, err := db.Exec(`INSERT INTO categories (name) VALUES ('Salary')`)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, err := category.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	rule := Rule{
		Name: " Salary description terms ", CategoryID: categoryID, Category: "Salary",
		Combine: "all", Enabled: true,
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: "incoming"},
			{Field: "description", Operator: "contains_any", Values: []string{"salary", "loon"}},
		},
	}
	id, err := Store(context.Background(), db, rule)
	if err != nil {
		t.Fatal(err)
	}
	rule.Name = "Salary description terms"
	rule.Conditions[1].Values = []string{"wedde"}
	if updatedID, err := Store(context.Background(), db, rule); err != nil || updatedID != id {
		t.Fatalf("Store() update ID = %d, error %v; want %d", updatedID, err, id)
	}

	rules, err := Enabled(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].ID != id || rules[0].Category != "Salary" ||
		rules[0].Conditions[1].Values[0] != "wedde" {
		t.Fatalf("loaded rules = %#v", rules)
	}
	if !Matches(rules[0], "incoming", "Maandelijkse WEDDE") ||
		Matches(rules[0], "outgoing", "Maandelijkse WEDDE") {
		t.Fatal("enabled direction/description conditions did not match as expected")
	}

	rule.Enabled = false
	if _, err := Store(context.Background(), db, rule); err != nil {
		t.Fatal(err)
	}
	rules, err = Enabled(context.Background(), db)
	if err != nil || len(rules) != 0 {
		t.Fatalf("disabled rules = %#v, error %v", rules, err)
	}
}

func TestValidateRejectsUnsupportedConditions(t *testing.T) {
	err := Validate(Rule{
		Name: "Unsupported", CategoryID: 1, Combine: "all",
		Conditions: []Condition{{Field: "payee", Operator: "contains", Value: "sample"}},
	})
	if err == nil {
		t.Fatal("Validate() accepted an unsupported rule condition")
	}
}

func TestMatchesAnyMode(t *testing.T) {
	rule := Rule{
		Enabled: true, Combine: "any",
		Conditions: []Condition{
			{Field: "direction", Operator: "equals", Value: "incoming"},
			{Field: "description", Operator: "contains_any", Values: []string{"salary"}},
		},
	}
	if !Matches(rule, "outgoing", "Salary transfer") ||
		!Matches(rule, "incoming", "Other") ||
		Matches(rule, "outgoing", "Other") {
		t.Fatal("any-mode rule did not match the expected conditions")
	}
}
