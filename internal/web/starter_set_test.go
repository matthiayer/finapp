package web

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matthiayer/finapp/internal/categorization"
	"github.com/matthiayer/finapp/internal/importflow"
	"github.com/matthiayer/finapp/internal/storage"
)

func TestStarterSetSelectionPreviewAndApply(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	imports := importflow.NewService(db)
	preview, err := imports.Preview(context.Background(), []importflow.Upload{{
		Name: "triodos.csv",
		Data: []byte(`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","salary payment","112,50"`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imports.Confirm(context.Background(), preview.Token, nil, nil); err != nil {
		t.Fatal(err)
	}

	handler, err := NewHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	routes := handler.Routes()
	setup := httptest.NewRecorder()
	routes.ServeHTTP(setup, httptest.NewRequest(http.MethodGet, "/settings/starter-set", nil))
	if setup.Code != http.StatusOK || !strings.Contains(setup.Body.String(), "Starter categories and rules") {
		t.Fatalf("starter selection page = %d %q", setup.Code, setup.Body.String())
	}

	form := url.Values{
		"category": {"Salary"},
		"rule":     {"Salary description terms"},
	}
	request := httptest.NewRequest(http.MethodPost, "/settings/starter-set/preview", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	review := httptest.NewRecorder()
	routes.ServeHTTP(review, request)
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), "Salary description terms") ||
		!strings.Contains(review.Body.String(), "1</td>") || !strings.Contains(review.Body.String(), "add category") {
		t.Fatalf("starter selection review = %d %q", review.Code, review.Body.String())
	}

	form.Set("confirm", "yes")
	request = httptest.NewRequest(http.MethodPost, "/settings/starter-set/apply", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applied := httptest.NewRecorder()
	routes.ServeHTTP(applied, request)
	if applied.Code != http.StatusSeeOther || applied.Header().Get("Location") != "/settings/starter-set?applied=1" {
		t.Fatalf("apply response = %d, location %q, body %q",
			applied.Code, applied.Header().Get("Location"), applied.Body.String())
	}

	var categoryID int64
	if err := db.QueryRow("SELECT id FROM categories WHERE name = 'Salary'").Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	rules, err := categorization.Enabled(context.Background(), db)
	if err != nil || len(rules) != 1 || rules[0].CategoryID != categoryID {
		t.Fatalf("enabled rules = %#v, error %v", rules, err)
	}
	var transactionCategory int64
	if err := db.QueryRow("SELECT category_id FROM transactions").Scan(&transactionCategory); err != nil {
		t.Fatal(err)
	}
	if transactionCategory != categoryID {
		t.Fatalf("starter rule setup left existing transaction in category %d, want %d", transactionCategory, categoryID)
	}

	disableForm := url.Values{"category": {"Salary"}, "confirm": {"yes"}}
	request = httptest.NewRequest(http.MethodPost, "/settings/starter-set/apply", strings.NewReader(disableForm.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	disabled := httptest.NewRecorder()
	routes.ServeHTTP(disabled, request)
	if disabled.Code != http.StatusSeeOther {
		t.Fatalf("apply without selected rules = %d, body %q", disabled.Code, disabled.Body.String())
	}
	rules, err = categorization.Enabled(context.Background(), db)
	if err != nil || len(rules) != 0 {
		t.Fatalf("rules after deselection = %#v, error %v", rules, err)
	}
}

func TestStarterSetRequiresChoicesForConflictsAndManualCategories(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("INSERT INTO categories (name) VALUES ('Groceries')"); err != nil {
		t.Fatal(err)
	}
	imports := importflow.NewService(db)
	preview, err := imports.Preview(context.Background(), []importflow.Upload{{
		Name: "triodos.csv",
		Data: []byte(
			`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","salary interest payment","112,50"` + "\n" +
				`"13-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","salary payment","125,00"`,
		),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := imports.Confirm(context.Background(), preview.Token, nil, nil); err != nil {
		t.Fatal(err)
	}
	var groceriesID int64
	if err := db.QueryRow("SELECT id FROM categories WHERE name = 'Groceries'").Scan(&groceriesID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE transactions SET category_id = ?, is_manual_category = 1 WHERE description = 'salary payment'", groceriesID); err != nil {
		t.Fatal(err)
	}

	categories := []string{"Salary", "Interest income"}
	rules := []string{"Salary description terms", "Interest description terms"}
	review, err := categorization.PreviewStarterSelection(context.Background(), db, categories, rules)
	if err != nil || len(review.Transactions) != 2 {
		t.Fatalf("starter preview = %#v, error %v", review, err)
	}
	if len(review.Transactions[0].Options) != 2 || !review.Transactions[1].ManualCategory {
		t.Fatalf("starter preview did not expose conflict/manual choices: %#v", review.Transactions)
	}
	if err := categorization.ApplyStarterSelection(context.Background(), db, categories, rules, nil); err == nil {
		t.Fatal("starter selection applied without resolving conflicts and manual categories")
	}
	var uncommittedCategories int
	if err := db.QueryRow("SELECT COUNT(*) FROM categories WHERE name IN ('Salary', 'Interest income')").Scan(&uncommittedCategories); err != nil {
		t.Fatal(err)
	}
	if uncommittedCategories != 0 {
		t.Fatalf("failed starter apply persisted %d categories", uncommittedCategories)
	}

	choices := map[int64]string{
		review.Transactions[0].ID: "Interest income",
		review.Transactions[1].ID: "Groceries",
	}
	handler, err := NewHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{"category": categories, "rule": rules}
	reviewRequest := httptest.NewRequest(http.MethodPost, "/settings/starter-set/preview", strings.NewReader(form.Encode()))
	reviewRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reviewResponse := httptest.NewRecorder()
	handler.Routes().ServeHTTP(reviewResponse, reviewRequest)
	if reviewResponse.Code != http.StatusOK ||
		!strings.Contains(reviewResponse.Body.String(), "Choose between matching rules") ||
		!strings.Contains(reviewResponse.Body.String(), "Keep or replace manual category") {
		t.Fatalf("starter conflict review = %d, body %q", reviewResponse.Code, reviewResponse.Body.String())
	}
	form.Set("confirm", "yes")
	for id, choice := range choices {
		form.Set(fmt.Sprintf("transaction_category[%d]", id), choice)
	}
	applyRequest := httptest.NewRequest(http.MethodPost, "/settings/starter-set/apply", strings.NewReader(form.Encode()))
	applyRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	applyResponse := httptest.NewRecorder()
	handler.Routes().ServeHTTP(applyResponse, applyRequest)
	if applyResponse.Code != http.StatusSeeOther {
		t.Fatalf("apply reviewed starter selection = %d, body %q", applyResponse.Code, applyResponse.Body.String())
	}
	for id, expected := range choices {
		var actual string
		var manual int
		if err := db.QueryRow(`SELECT c.name, t.is_manual_category FROM transactions t
			JOIN categories c ON c.id = t.category_id WHERE t.id = ?`, id).Scan(&actual, &manual); err != nil {
			t.Fatal(err)
		}
		if actual != expected {
			t.Errorf("transaction %d category = %q, want %q", id, actual, expected)
		}
		if expected == "Groceries" && manual != 1 {
			t.Errorf("kept manual category was not preserved: manual=%d", manual)
		}
	}
}

func TestStarterSetRejectsRuleWithoutItsCategory(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := categorization.PreviewStarterSelection(context.Background(), db, nil,
		[]string{"Salary description terms"}); err == nil {
		t.Fatal("starter rule without a category was accepted")
	}
}
