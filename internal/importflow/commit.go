package importflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/matthiayer/finapp/internal/domain"
)

func (s *Service) commit(ctx context.Context, state *previewState, rows []preparedRow) (Result, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Result{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback()

	fileNames := make([]string, len(state.files))
	for i, file := range state.files {
		fileNames[i] = file.name
	}
	sourceFiles, err := json.Marshal(fileNames)
	if err != nil {
		return Result{}, fmt.Errorf("encode import filenames: %w", err)
	}

	importedCount, skippedCount := countImportActions(rows)
	jobResult, err := tx.ExecContext(ctx, `
		INSERT INTO import_jobs (source_files_json, file_count, row_count, imported_count, skipped_count)
		VALUES (?, ?, ?, ?, ?)`,
		string(sourceFiles), len(state.files), len(rows), importedCount, skippedCount,
	)
	if err != nil {
		return Result{}, fmt.Errorf("create import job: %w", err)
	}
	jobID, err := jobResult.LastInsertId()
	if err != nil {
		return Result{}, fmt.Errorf("get import job ID: %w", err)
	}

	accountIDs := make([]int64, len(state.files))
	for i, file := range state.files {
		accountIDs[i], err = ensureAccount(ctx, tx, file.parsed.Account)
		if err != nil {
			return Result{}, fmt.Errorf("save account for %s: %w", file.name, err)
		}
	}

	for _, row := range rows {
		action := row.decision.Action
		if row.duplicate != nil && action == "skip" {
			continue
		}
		if row.duplicate != nil && action == "update" &&
			row.duplicate.ManualEdit && row.decision.ManualResolution == "keep-edits" {
			continue
		}

		sourceAction := "inserted"
		switch {
		case row.duplicate != nil && action == "keep-both":
			sourceAction = "kept_both"
			if err := insertTransaction(ctx, tx, accountIDs[row.fileIndex], jobID, row); err != nil {
				return Result{}, fmt.Errorf("save transaction from %s row %d: %w", row.fileName, row.tx.SourceRow, err)
			}
		case row.duplicate != nil && action == "update":
			sourceAction = "updated"
			if err := updateTransaction(ctx, tx, accountIDs[row.fileIndex], jobID, row); err != nil {
				return Result{}, fmt.Errorf("update transaction from %s row %d: %w", row.fileName, row.tx.SourceRow, err)
			}
		default:
			if err := insertTransaction(ctx, tx, accountIDs[row.fileIndex], jobID, row); err != nil {
				return Result{}, fmt.Errorf("save transaction from %s row %d: %w", row.fileName, row.tx.SourceRow, err)
			}
		}

		transactionID, err := rowTransactionID(ctx, tx, accountIDs[row.fileIndex], jobID, row, sourceAction)
		if err != nil {
			return Result{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO transaction_imports
				(transaction_id, import_job_id, source_file_name, source_row, action)
			VALUES (?, ?, ?, ?, ?)`,
			transactionID, jobID, row.fileName, row.tx.SourceRow, sourceAction,
		); err != nil {
			return Result{}, fmt.Errorf("record import provenance: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit import job: %w", err)
	}
	return Result{JobID: jobID, ImportedCount: importedCount, SkippedCount: skippedCount}, nil
}

func countImportActions(rows []preparedRow) (int, int) {
	imported, skipped := 0, 0
	for _, row := range rows {
		switch {
		case row.duplicate == nil:
			imported++
		case row.decision.Action == "skip":
			skipped++
		case row.decision.Action == "update" &&
			row.duplicate.ManualEdit && row.decision.ManualResolution == "keep-edits":
			skipped++
		default:
			imported++
		}
	}
	return imported, skipped
}

func ensureAccount(ctx context.Context, tx *sql.Tx, account domain.Account) (int64, error) {
	currency := strings.ToUpper(account.Currency)
	if currency == "" {
		currency = "EUR"
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO accounts (bank, account_number, name, currency)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (bank, account_number) DO UPDATE SET
			name = CASE WHEN excluded.name <> '' THEN excluded.name ELSE accounts.name END`,
		string(account.Bank), account.Number, account.Name, currency,
	)
	if err != nil {
		return 0, err
	}

	var id int64
	if err := tx.QueryRowContext(ctx,
		"SELECT id FROM accounts WHERE bank = ? AND account_number = ?",
		string(account.Bank), account.Number,
	).Scan(&id); err != nil {
		return 0, err
	}
	if account.Balance != nil && account.BalanceAsOf != nil {
		balance, ok := normalizeDecimal(*account.Balance)
		if !ok {
			return 0, errors.New("account balance is invalid")
		}
		date := account.BalanceAsOf.Format(time.RFC3339)
		if _, err := tx.ExecContext(ctx, `
			UPDATE accounts SET balance_anchor_amount = ?, balance_anchor_date = ?
			WHERE id = ? AND (balance_anchor_date IS NULL OR balance_anchor_date <= ?)`,
			balance, date, id, date,
		); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func insertTransaction(
	ctx context.Context,
	tx *sql.Tx,
	accountID, jobID int64,
	row preparedRow,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			account_id, booking_date, value_date, amount, currency, eur_amount,
			exchange_rate, fees, description, payee, counterparty_account, reference,
			balance_after, direction, category_id, import_job_id, source_file_name,
			source_row, pending
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, row.tx.BookingDate.Format("2006-01-02"), nullableDate(row.tx.ValueDate),
		row.tx.Amount, row.tx.Currency, row.eurAmount,
		nullableText(exchangeRate(row)), nullableString(row.tx.Fees),
		row.tx.Description, row.tx.CounterpartyName, row.tx.CounterpartyAccount,
		row.tx.Reference, nullableString(row.tx.BalanceAfter), string(row.tx.Direction),
		row.categoryID, jobID, row.fileName, row.tx.SourceRow, boolInt(row.tx.Pending),
	)
	return err
}

func updateTransaction(
	ctx context.Context,
	tx *sql.Tx,
	accountID, jobID int64,
	row preparedRow,
) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE transactions SET
			account_id = ?, booking_date = ?, value_date = ?, amount = ?, currency = ?,
			eur_amount = ?, exchange_rate = ?, fees = ?, description = ?, payee = ?,
			counterparty_account = ?, reference = ?, balance_after = ?, direction = ?,
			category_id = ?, import_job_id = ?, source_file_name = ?, source_row = ?, pending = ?,
			is_manually_edited = 0
		WHERE id = ?`,
		accountID, row.tx.BookingDate.Format("2006-01-02"), nullableDate(row.tx.ValueDate),
		row.tx.Amount, row.tx.Currency, row.eurAmount,
		nullableText(exchangeRate(row)), nullableString(row.tx.Fees),
		row.tx.Description, row.tx.CounterpartyName, row.tx.CounterpartyAccount,
		row.tx.Reference, nullableString(row.tx.BalanceAfter), string(row.tx.Direction),
		row.categoryID, jobID, row.fileName, row.tx.SourceRow, boolInt(row.tx.Pending),
		row.duplicate.TransactionID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("duplicate transaction changed after preview; review the import again")
	}
	return nil
}

func rowTransactionID(
	ctx context.Context,
	tx *sql.Tx,
	accountID, jobID int64,
	row preparedRow,
	action string,
) (int64, error) {
	if action == "updated" {
		return row.duplicate.TransactionID, nil
	}
	var id int64
	err := tx.QueryRowContext(ctx, `
		SELECT id FROM transactions
		WHERE account_id = ? AND import_job_id = ? AND source_file_name = ?
		  AND source_row = ? ORDER BY id DESC LIMIT 1`,
		accountID, jobID, row.fileName, row.tx.SourceRow,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("find imported transaction for provenance: %w", err)
	}
	return id, nil
}

func exchangeRate(row preparedRow) string {
	if row.exchangeRate != "" {
		return row.exchangeRate
	}
	if row.tx.ExchangeRate != nil {
		return *row.tx.ExchangeRate
	}
	return ""
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}

func nullableString(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
