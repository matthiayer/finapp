package dashboard

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	schema := `
		CREATE TABLE categories (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE accounts (
			id INTEGER PRIMARY KEY, bank TEXT NOT NULL, account_number TEXT NOT NULL,
			name TEXT NOT NULL, currency TEXT NOT NULL, balance_anchor_amount TEXT,
			balance_anchor_date TEXT
		);
		CREATE TABLE transactions (
			id INTEGER PRIMARY KEY, account_id INTEGER NOT NULL, booking_date TEXT NOT NULL,
			amount TEXT NOT NULL, currency TEXT NOT NULL, eur_amount TEXT NOT NULL,
			balance_after TEXT, direction TEXT NOT NULL, category_id INTEGER NOT NULL,
			pending INTEGER NOT NULL DEFAULT 0, transfer_id INTEGER
		);
		INSERT INTO categories VALUES (1, 'Food'), (2, 'Salary'), (3, 'Other');
		INSERT INTO accounts VALUES
			(1, 'Bank A', 'A-1', 'Current', 'EUR', '1000', '2024-01-31'),
			(2, 'Bank B', 'B-1', 'Savings', 'EUR', '80', '2024-02-01');`
	if _, err := db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return db
}

func addTransaction(t *testing.T, db *sql.DB, accountID int, date, amount, eurAmount, balance, direction string, categoryID, pending int, transferID any) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO transactions
		(account_id, booking_date, amount, currency, eur_amount, balance_after,
		 direction, category_id, pending, transfer_id)
		VALUES (?, ?, ?, 'USD', ?, ?, ?, ?, ?, ?)`,
		accountID, date, amount, eurAmount, nullable(balance), direction, categoryID, pending, transferID)
	if err != nil {
		t.Fatal(err)
	}
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func TestLoadEURTotalsExclusionsAndCategoryFilter(t *testing.T) {
	db := testDB(t)
	addTransaction(t, db, 1, "2024-02-01", "999.00", "100.25", "", "incoming", 2, 0, nil)
	addTransaction(t, db, 1, "2024-02-02", "0.10", "0.1", "", "incoming", 2, 0, nil)
	addTransaction(t, db, 1, "2024-02-03", "0.20", "0.2", "", "incoming", 2, 0, nil)
	addTransaction(t, db, 1, "2024-02-04", "-12.10", "-12.10", "", "outgoing", 1, 0, nil)
	addTransaction(t, db, 1, "2024-02-05", "-9.99", "-9.99", "", "outgoing", 1, 0, nil)
	addTransaction(t, db, 1, "2024-02-05", "-5", "-5", "", "outgoing", 3, 0, nil)
	addTransaction(t, db, 1, "2024-02-06", "500.00", "500.00", "", "incoming", 2, 1, nil)
	addTransaction(t, db, 1, "2024-02-07", "-200.00", "200.00", "", "outgoing", 1, 0, 7)
	addTransaction(t, db, 1, "2024-02-08", "50.00", "50.00", "", "unknown", 3, 0, nil)

	filter := Filter{
		Start: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
	}
	got, err := Load(context.Background(), db, filter)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != (Summary{Income: "100.55", Spending: "27.09", Saving: "73.46"}) {
		t.Fatalf("unexpected summary: %+v", got.Summary)
	}
	if len(got.Monthly) != 1 || got.Monthly[0] != (MonthlyTotal{
		Month: "2024-02", Income: "100.55", Spending: "27.09", Saving: "73.46",
	}) {
		t.Fatalf("unexpected monthly totals: %+v", got.Monthly)
	}
	if !reflect.DeepEqual(got.SpendingByCategory, []CategoryTotal{
		{CategoryID: 1, CategoryName: "Food", Total: "22.09"},
		{CategoryID: 3, CategoryName: "Other", Total: "5"},
	}) {
		t.Fatalf("unexpected spending categories: %+v", got.SpendingByCategory)
	}
	if !reflect.DeepEqual(got.IncomeByCategory, []CategoryTotal{
		{CategoryID: 2, CategoryName: "Salary", Total: "100.55"},
	}) {
		t.Fatalf("unexpected income categories: %+v", got.IncomeByCategory)
	}
	if len(got.Categories) != 3 {
		t.Fatalf("expected all categories in filter options, got %+v", got.Categories)
	}

	filter.CategoryID = 1
	got, err = Load(context.Background(), db, filter)
	if err != nil {
		t.Fatal(err)
	}
	if got.Summary != (Summary{Income: "0", Spending: "22.09", Saving: "-22.09"}) {
		t.Fatalf("category filter did not scope summary: %+v", got.Summary)
	}
	if got.SelectedCategoryID != 1 || len(got.IncomeByCategory) != 0 {
		t.Fatalf("category filter not reflected in view: %+v", got)
	}
}

func TestLoadAccountBalancesAndObservedHistory(t *testing.T) {
	db := testDB(t)
	addTransaction(t, db, 1, "2024-01-31", "-5", "5", "505.00", "outgoing", 1, 0, 9)
	addTransaction(t, db, 1, "2024-02-10", "-20", "20", "525.25", "outgoing", 1, 0, nil)
	addTransaction(t, db, 2, "2024-01-20", "10", "10", "70", "incoming", 2, 0, nil)
	filter := Filter{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
	}

	got, err := Load(context.Background(), db, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Accounts) != 2 {
		t.Fatalf("unexpected accounts: %+v", got.Accounts)
	}
	if got.Accounts[0].Balance != "525.25" || got.Accounts[0].Date != "2024-02-10" || !got.Accounts[0].Known {
		t.Fatalf("transaction balance should supersede older anchor: %+v", got.Accounts[0])
	}
	if got.Accounts[1].Balance != "80" || got.Accounts[1].Date != "2024-02-01" {
		t.Fatalf("older transaction balance should not supersede account anchor: %+v", got.Accounts[1])
	}
	if got.CombinedBalance != "605.25" || !got.BalanceDatesDiffer {
		t.Fatalf("unexpected combined balance/date state: %s %t", got.CombinedBalance, got.BalanceDatesDiffer)
	}
	if !reflect.DeepEqual(got.BalanceHistory, []BalancePoint{
		{Month: "2024-01", Balance: "575", ObservedAccounts: 2},
		{Month: "2024-02", Balance: "605.25", ObservedAccounts: 2},
	}) {
		t.Fatalf("unexpected balance history: %+v", got.BalanceHistory)
	}
	if len(got.AccountHistory) != 4 {
		t.Fatalf("expected per-account observed balance history, got %+v", got.AccountHistory)
	}
}

func TestCompleteMonthsIncludesEmptyMonths(t *testing.T) {
	start, _ := time.Parse("2006-01-02", "2024-01-15")
	end, _ := time.Parse("2006-01-02", "2024-03-02")
	if got, want := completeMonths(start, end), []string{"2024-01", "2024-02", "2024-03"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("completeMonths() = %v, want %v", got, want)
	}
}
