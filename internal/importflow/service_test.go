package importflow

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matthiayer/finapp/internal/categorization"
	"github.com/matthiayer/finapp/internal/domain"
	"github.com/matthiayer/finapp/internal/storage"
)

func TestPreviewRejectsInvalidFileWithoutPersistingData(t *testing.T) {
	db := openTestDB(t)
	service := NewService(db)
	preview, err := service.Preview(context.Background(), []Upload{
		syntheticTriodosUpload(),
		{
			Name: "invalid.csv",
			Data: []byte(`"not-a-date","BE001","12,50","BE002","BICXXX","Sender","","SCT","Synthetic payment","112,50"`),
		},
	})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.InvalidCount == 0 || len(preview.Errors) == 0 {
		t.Fatalf("preview = %#v, want validation errors", preview)
	}

	var accounts, jobs, transactions int
	for query, destination := range map[string]*int{
		"SELECT COUNT(*) FROM accounts":     &accounts,
		"SELECT COUNT(*) FROM import_jobs":  &jobs,
		"SELECT COUNT(*) FROM transactions": &transactions,
	} {
		if err := db.QueryRowContext(context.Background(), query).Scan(destination); err != nil {
			t.Fatalf("query %q: %v", query, err)
		}
	}
	if accounts != 0 || jobs != 0 || transactions != 0 {
		t.Fatalf("invalid preview persisted data: accounts=%d jobs=%d transactions=%d", accounts, jobs, transactions)
	}
}

func TestConfirmImportsAndRequiresDuplicateDecision(t *testing.T) {
	db := openTestDB(t)
	service := NewService(db)
	upload := syntheticTriodosUpload()

	first, err := service.Preview(context.Background(), []Upload{upload})
	if err != nil || len(first.Errors) != 0 {
		t.Fatalf("first Preview() = %#v, error %v", first, err)
	}
	result, err := service.Confirm(context.Background(), first.Token, nil, nil)
	if err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	if result.ImportedCount != 1 || result.SkippedCount != 0 {
		t.Fatalf("result = %#v, want one imported transaction", result)
	}

	second, err := service.Preview(context.Background(), []Upload{upload})
	if err != nil || len(second.Rows) != 1 || second.Rows[0].Duplicate == nil {
		t.Fatalf("duplicate Preview() = %#v, error %v", second, err)
	}
	if _, err := service.Confirm(context.Background(), second.Token, nil, nil); err == nil {
		t.Fatal("Confirm() succeeded without a duplicate decision")
	}
	skipped, err := service.Confirm(context.Background(), second.Token, map[string]Decision{
		second.Rows[0].Key: {Action: "skip"},
	}, nil)
	if err != nil {
		t.Fatalf("Confirm(skip) error = %v", err)
	}
	if skipped.ImportedCount != 0 || skipped.SkippedCount != 1 {
		t.Fatalf("skip result = %#v, want one skipped transaction", skipped)
	}

	var transactionCount int
	if err := db.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM transactions").Scan(&transactionCount); err != nil {
		t.Fatal(err)
	}
	if transactionCount != 1 {
		t.Fatalf("transaction count = %d, want 1", transactionCount)
	}
}

func TestDuplicateMatchRequiresSameAccountAmountDescriptionAndBookingDate(t *testing.T) {
	tests := []struct {
		name        string
		account     string
		date        string
		amount      string
		description string
	}{
		{name: "account", account: "BE999", date: "12-03-2025", amount: "12,50", description: "Synthetic payment"},
		{name: "amount", account: "BE001", date: "12-03-2025", amount: "13,50", description: "Synthetic payment"},
		{name: "description", account: "BE001", date: "12-03-2025", amount: "12,50", description: "Different payment"},
		{name: "booking date", account: "BE001", date: "13-03-2025", amount: "12,50", description: "Synthetic payment"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openTestDB(t)
			service := NewService(db)
			first, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Confirm(context.Background(), first.Token, nil, nil); err != nil {
				t.Fatal(err)
			}

			candidate, err := service.Preview(context.Background(), []Upload{
				syntheticTriodosUploadWith(test.account, test.date, test.amount, test.description),
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(candidate.Rows) != 1 || candidate.Rows[0].Duplicate != nil {
				t.Fatalf("candidate = %#v, want no duplicate match", candidate.Rows)
			}
		})
	}
}

func TestConfirmKeepBothAndUpdateDuplicate(t *testing.T) {
	t.Run("keep both", func(t *testing.T) {
		db := openTestDB(t)
		service := NewService(db)
		first, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Confirm(context.Background(), first.Token, nil, nil); err != nil {
			t.Fatal(err)
		}

		duplicate, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Confirm(context.Background(), duplicate.Token, map[string]Decision{
			duplicate.Rows[0].Key: {Action: "keep-both"},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.ImportedCount != 1 || result.SkippedCount != 0 {
			t.Fatalf("result = %#v, want one kept transaction", result)
		}
		assertTransactionCount(t, db, 2)
	})

	t.Run("update", func(t *testing.T) {
		db := openTestDB(t)
		service := NewService(db)
		first, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.Confirm(context.Background(), first.Token, nil, nil); err != nil {
			t.Fatal(err)
		}

		changedPayee := syntheticTriodosUpload()
		changedPayee.Data = []byte(strings.ReplaceAll(string(changedPayee.Data),
			`"Synthetic Sender"`, `"Updated Sender"`))
		duplicate, err := service.Preview(context.Background(), []Upload{changedPayee})
		if err != nil || len(duplicate.Rows) != 1 || duplicate.Rows[0].Duplicate == nil {
			t.Fatalf("duplicate preview = %#v, error %v", duplicate, err)
		}
		result, err := service.Confirm(context.Background(), duplicate.Token, map[string]Decision{
			duplicate.Rows[0].Key: {Action: "update"},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.ImportedCount != 1 || result.SkippedCount != 0 {
			t.Fatalf("result = %#v, want one updated transaction", result)
		}
		assertTransactionCount(t, db, 1)
		var payee string
		if err := db.QueryRow("SELECT payee FROM transactions").Scan(&payee); err != nil {
			t.Fatal(err)
		}
		if payee != "Updated Sender" {
			t.Fatalf("updated payee = %q, want Updated Sender", payee)
		}
	})
}

func TestConfirmManualEditDuplicateRequiresResolution(t *testing.T) {
	for _, resolution := range []string{"keep-edits", "use-imported"} {
		t.Run(resolution, func(t *testing.T) {
			db := openTestDB(t)
			service := NewService(db)
			first, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Confirm(context.Background(), first.Token, nil, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`UPDATE transactions
				SET payee = 'Manually edited sender', is_manually_edited = 1`); err != nil {
				t.Fatal(err)
			}

			changedPayee := syntheticTriodosUpload()
			changedPayee.Data = []byte(strings.ReplaceAll(string(changedPayee.Data),
				`"Synthetic Sender"`, `"Imported sender"`))
			preview, err := service.Preview(context.Background(), []Upload{changedPayee})
			if err != nil || len(preview.Rows) != 1 || preview.Rows[0].Duplicate == nil ||
				!preview.Rows[0].Duplicate.ManualEdit ||
				preview.Rows[0].Duplicate.Payee != "Manually edited sender" {
				t.Fatalf("manual-edit preview = %#v, error %v", preview, err)
			}
			if _, err := service.Confirm(context.Background(), preview.Token, map[string]Decision{
				preview.Rows[0].Key: {Action: "update"},
			}, nil); err == nil {
				t.Fatal("Confirm() updated a manual edit without a resolution")
			}

			result, err := service.Confirm(context.Background(), preview.Token, map[string]Decision{
				preview.Rows[0].Key: {
					Action: "update", ManualResolution: resolution,
				},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			wantImported, wantSkipped, wantPayee, wantEdited := 1, 0, "Imported sender", 0
			if resolution == "keep-edits" {
				wantImported, wantSkipped, wantPayee, wantEdited = 0, 1, "Manually edited sender", 1
			}
			if result.ImportedCount != wantImported || result.SkippedCount != wantSkipped {
				t.Fatalf("result = %#v, want imported=%d skipped=%d",
					result, wantImported, wantSkipped)
			}
			var payee string
			var manuallyEdited int
			if err := db.QueryRow("SELECT payee, is_manually_edited FROM transactions").
				Scan(&payee, &manuallyEdited); err != nil {
				t.Fatal(err)
			}
			if payee != wantPayee || manuallyEdited != wantEdited {
				t.Fatalf("stored manual edit = (%q, %d), want (%q, %d)",
					payee, manuallyEdited, wantPayee, wantEdited)
			}
		})
	}
}

func TestKnownTotalsArePartialForUnknownDirection(t *testing.T) {
	rows := []stagedRow{{
		tx: domain.Transaction{
			Amount: "12.5", Currency: "EUR", Direction: domain.DirectionUnknown,
		},
	}}
	incoming, outgoing, partial := knownTotals(rows)
	if incoming != "0.00" || outgoing != "0.00" || !partial {
		t.Fatalf("knownTotals() = (%s, %s, %t), want (0.00, 0.00, true)",
			incoming, outgoing, partial)
	}
}

func TestUnknownDirectionRequiresImportChoice(t *testing.T) {
	db := openTestDB(t)
	service := NewService(db)
	upload := syntheticTriodosUploadWith("BE001", "12-03-2025", "0,00", "Zero-value transaction")
	preview, err := service.Preview(context.Background(), []Upload{upload})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Rows) != 1 || preview.Rows[0].Transaction.Direction != domain.DirectionUnknown ||
		!preview.TotalsPartial || !preview.CategoryPartial {
		t.Fatalf("ambiguous preview = %#v", preview)
	}
	if _, err := service.Confirm(context.Background(), preview.Token, nil, nil); err == nil {
		t.Fatal("Confirm() succeeded without a direction choice")
	}
	result, err := service.Confirm(context.Background(), preview.Token, map[string]Decision{
		preview.Rows[0].Key: {Direction: domain.DirectionIncoming},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var direction string
	if err := db.QueryRow("SELECT direction FROM transactions WHERE import_job_id = ?",
		result.JobID).Scan(&direction); err != nil {
		t.Fatal(err)
	}
	if direction != string(domain.DirectionIncoming) {
		t.Fatalf("stored direction = %q, want incoming", direction)
	}
}

func TestConfirmRequiresExchangeRateForNonEURTransactions(t *testing.T) {
	db := openTestDB(t)
	service := NewService(db)
	upload := Upload{
		Name: "ing.csv",
		Data: []byte("Rekeningnummer;Naam van de rekening;Rekening tegenpartij;Omzetnummer;Boekingsdatum;Valutadatum;Bedrag;Munteenheid;Omschrijving;Detail van de omzet;Bericht\n" +
			"BE100;Synthetic Account;BE200;TX-1;13/03/2025;14/03/2025;10,00;USD;Card;Shop;Synthetic detail"),
	}
	preview, err := service.Preview(context.Background(), []Upload{upload})
	if err != nil || len(preview.Errors) != 0 || len(preview.RateRequirements) != 1 {
		t.Fatalf("Preview() = %#v, error %v", preview, err)
	}
	if _, err := service.Confirm(context.Background(), preview.Token, nil, nil); err == nil {
		t.Fatal("Confirm() succeeded without a required exchange rate")
	}

	result, err := service.Confirm(context.Background(), preview.Token, nil, map[string]string{
		preview.RateRequirements[0].Key: "0.9",
	})
	if err != nil {
		t.Fatalf("Confirm() with exchange rate error = %v", err)
	}
	if result.ImportedCount != 1 {
		t.Fatalf("imported count = %d, want 1", result.ImportedCount)
	}
	var eurAmount string
	if err := db.QueryRowContext(context.Background(),
		"SELECT eur_amount FROM transactions").Scan(&eurAmount); err != nil {
		t.Fatal(err)
	}
	if eurAmount != "9.00" {
		t.Fatalf("eur_amount = %q, want 9.00", eurAmount)
	}

	euroUpload := Upload{
		Name: "ing-eur.csv",
		Data: []byte(strings.ReplaceAll(string(upload.Data), ";USD;", ";EUR;")),
	}
	duplicate, err := service.Preview(context.Background(), []Upload{euroUpload})
	if err != nil || len(duplicate.Rows) != 1 || duplicate.Rows[0].Duplicate == nil {
		t.Fatalf("same-amount currency variant should match duplicate criteria: %#v, error %v",
			duplicate, err)
	}
}

func TestPreviewAppliesRuleAndReportsCategoryTotals(t *testing.T) {
	db := openTestDB(t)
	category, err := db.Exec(`INSERT INTO categories (name) VALUES ('Salary')`)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, err := category.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	_, err = categorization.Store(context.Background(), db, categorization.Rule{
		Name: "Salary terms", CategoryID: categoryID, Category: "Salary",
		Combine: "all", Enabled: true,
		Conditions: []categorization.Condition{
			{Field: "direction", Operator: "equals", Value: "incoming"},
			{Field: "description", Operator: "contains_any", Values: []string{"synthetic payment"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	service := NewService(db)
	preview, err := service.Preview(context.Background(), []Upload{{
		Name: "triodos.csv",
		Data: []byte(
			`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","Synthetic payment","112,50"` + "\n" +
				`"13-03-2025","BE001","5,00","BE002","BICXXX","Synthetic Sender","","SCT","Reimbursement","117,50"`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Rows) != 2 || preview.Rows[0].Category != "Salary" ||
		preview.Rows[0].CategoryID != categoryID || preview.Rows[0].Conflict {
		t.Fatalf("rule assignment = %#v", preview.Rows)
	}
	if len(preview.CategoryTotals) != 2 ||
		preview.CategoryTotals[0] != (CategoryTotal{
			CategoryID: categoryID, CategoryName: "Salary", Incoming: "12.50", Outgoing: "0.00",
		}) || preview.CategoryTotals[1] != (CategoryTotal{
		CategoryID: 1, CategoryName: "Uncategorized", Incoming: "5.00", Outgoing: "0.00",
	}) || preview.CategoryPartial {
		t.Fatalf("category totals = %#v, partial %t", preview.CategoryTotals, preview.CategoryPartial)
	}

	result, err := service.Confirm(context.Background(), preview.Token, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var storedCategoryID int64
	if err := db.QueryRow(`SELECT category_id FROM transactions
		WHERE import_job_id = ? AND description = 'Synthetic payment'`, result.JobID).Scan(&storedCategoryID); err != nil {
		t.Fatal(err)
	}
	if storedCategoryID != categoryID {
		t.Fatalf("stored category = %d, want %d", storedCategoryID, categoryID)
	}
}

func TestPreviewRequiresChoiceForConflictingRules(t *testing.T) {
	db := openTestDB(t)
	categoryIDs := make([]int64, 2)
	for i, name := range []string{"Salary", "Other income"} {
		result, err := db.Exec("INSERT INTO categories (name) VALUES (?)", name)
		if err != nil {
			t.Fatal(err)
		}
		categoryIDs[i], err = result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		_, err = categorization.Store(context.Background(), db, categorization.Rule{
			Name: name + " terms", CategoryID: categoryIDs[i], Category: name,
			Combine: "all", Enabled: true,
			Conditions: []categorization.Condition{
				{Field: "direction", Operator: "equals", Value: "incoming"},
				{Field: "description", Operator: "contains_any", Values: []string{"synthetic payment"}},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	service := NewService(db)
	preview, err := service.Preview(context.Background(), []Upload{syntheticTriodosUpload()})
	if err != nil {
		t.Fatal(err)
	}
	if preview.UnresolvedRules != 1 || !preview.Rows[0].Conflict ||
		len(preview.Rows[0].RuleMatches) != 2 || !preview.CategoryPartial {
		t.Fatalf("conflicting preview = %#v", preview)
	}
	if _, err := service.Confirm(context.Background(), preview.Token, nil, nil); err == nil {
		t.Fatal("Confirm() accepted conflicting rules without a category choice")
	}
	result, err := service.Confirm(context.Background(), preview.Token, map[string]Decision{
		preview.Rows[0].Key: {CategoryID: categoryIDs[1]},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var storedCategoryID int64
	if err := db.QueryRow("SELECT category_id FROM transactions WHERE import_job_id = ?",
		result.JobID).Scan(&storedCategoryID); err != nil {
		t.Fatal(err)
	}
	if storedCategoryID != categoryIDs[1] {
		t.Fatalf("stored category = %d, want chosen category %d", storedCategoryID, categoryIDs[1])
	}
}

func TestPreviewEnforcesPerFileLimit(t *testing.T) {
	service := NewService(openTestDB(t))
	preview, err := service.Preview(context.Background(), []Upload{{
		Name: "large.csv",
		Data: make([]byte, MaxFileSize+1),
	}})
	if err != nil {
		t.Fatalf("Preview() error = %v", err)
	}
	if preview.InvalidCount != 1 || preview.Token != "" {
		t.Fatalf("preview = %#v, want one rejected file and no token", preview)
	}
}

func TestPreviewAcceptsFileAtPerFileLimit(t *testing.T) {
	prefix := []byte(`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","`)
	suffix := []byte(`","112,50"`)
	data := append([]byte(nil), prefix...)
	data = append(data, bytes.Repeat([]byte{'x'}, MaxFileSize-len(prefix)-len(suffix))...)
	data = append(data, suffix...)
	if len(data) != MaxFileSize {
		t.Fatalf("test data size = %d, want %d", len(data), MaxFileSize)
	}
	preview, err := NewService(openTestDB(t)).Preview(context.Background(), []Upload{{
		Name: "limit.csv", Data: data,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if preview.InvalidCount != 0 || len(preview.Rows) != 1 || preview.Token == "" {
		t.Fatalf("preview = %#v, want a valid file at the size limit", preview)
	}
}

func assertTransactionCount(t *testing.T, db *sql.DB, want int) {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("transaction count = %d, want %d", count, want)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})
	return db
}

func syntheticTriodosUpload() Upload {
	return Upload{
		Name: "triodos.csv",
		Data: []byte(`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","Synthetic payment","112,50"`),
	}
}

func syntheticTriodosUploadWith(account, date, amount, description string) Upload {
	return Upload{
		Name: "triodos.csv",
		Data: []byte(fmt.Sprintf(
			`"%s","%s","%s","BE002","BICXXX","Synthetic Sender","","SCT","%s","112,50"`,
			date, account, amount, description,
		)),
	}
}
