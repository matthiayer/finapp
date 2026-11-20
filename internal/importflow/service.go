package importflow

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/matthiayer/finapp/internal/bankcsv"
	"github.com/matthiayer/finapp/internal/categorization"
	"github.com/matthiayer/finapp/internal/domain"
)

const previewLifetime = 20 * time.Minute

type Service struct {
	db       *sql.DB
	mu       sync.Mutex
	previews map[string]*previewState
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db:       db,
		previews: make(map[string]*previewState),
	}
}

func (s *Service) Preview(ctx context.Context, uploads []Upload) (Preview, error) {
	preview := Preview{}
	if len(uploads) == 0 {
		preview.Errors = append(preview.Errors, FileError{Message: "Select at least one CSV file."})
		preview.InvalidCount = 1
		return preview, nil
	}

	state := &previewState{
		createdAt: time.Now(),
		files:     make([]stagedFile, 0, len(uploads)),
	}
	rules, err := categorization.Enabled(ctx, s.db)
	if err != nil {
		return Preview{}, err
	}
	uncategorizedID, err := uncategorizedCategoryID(ctx, s.db)
	if err != nil {
		return Preview{}, err
	}
	rateKeys := make(map[string]RateRequirement)
	var firstDate, lastDate time.Time

	for _, upload := range uploads {
		name := safeFileName(upload.Name)
		if name == "" {
			preview.Errors = append(preview.Errors, FileError{Message: "A selected file has no filename."})
			preview.InvalidCount++
			continue
		}
		if len(upload.Data) > MaxFileSize {
			preview.Errors = append(preview.Errors, FileError{
				FileName: name,
				Message:  fmt.Sprintf("File exceeds the %d-byte per-file limit.", MaxFileSize),
			})
			preview.InvalidCount++
			continue
		}

		parsed, err := bankcsv.ParseFile(name, upload.Data)
		if err != nil {
			message := "File is not a supported or readable bank CSV."
			if errors.Is(err, bankcsv.ErrUnsupportedAccountType) {
				message = "Only current/checking accounts are supported."
			}
			preview.Errors = append(preview.Errors, FileError{
				FileName: name,
				Message:  message,
			})
			preview.InvalidCount++
			continue
		}
		if parsed.Account.Number == "" {
			preview.Errors = append(preview.Errors, FileError{
				FileName: name,
				Message:  "The account number could not be identified.",
			})
			preview.InvalidCount++
			continue
		}
		if parsed.Account.Currency == "" {
			parsed.Account.Currency = "EUR"
		}
		if !strings.EqualFold(parsed.Account.Currency, "EUR") {
			preview.Errors = append(preview.Errors, FileError{
				FileName: name,
				Message:  "Only EUR accounts are supported.",
			})
			preview.InvalidCount++
			continue
		}

		fileIndex := len(state.files)
		state.files = append(state.files, stagedFile{name: name, parsed: parsed})
		filePreview := FilePreview{
			Name:          name,
			Bank:          parsed.Bank,
			AccountNumber: parsed.Account.Number,
			AccountName:   parsed.Account.Name,
			RowCount:      len(parsed.Transactions),
		}
		preview.Files = append(preview.Files, filePreview)
		for _, rowErr := range parsed.Errors {
			preview.Errors = append(preview.Errors, FileError{
				FileName: name,
				Row:      rowErr.Row,
				Message:  rowErr.Message,
			})
		}
		preview.ValidCount += len(parsed.Transactions)
		preview.InvalidCount += len(parsed.Errors)

		accountID, err := findAccountID(ctx, s.db, parsed.Account.Bank, parsed.Account.Number)
		if err != nil {
			return Preview{}, fmt.Errorf("look up account for import preview: %w", err)
		}
		for txIndex, tx := range parsed.Transactions {
			if tx.Pending {
				continue
			}
			if tx.AccountNumber == "" {
				tx.AccountNumber = parsed.Account.Number
			}
			if tx.AccountNumber != parsed.Account.Number {
				preview.Errors = append(preview.Errors, FileError{
					FileName: name,
					Row:      tx.SourceRow,
					Message:  "Transaction account does not match the account summary.",
				})
				preview.InvalidCount++
				continue
			}
			if tx.BookingDate.IsZero() {
				preview.Errors = append(preview.Errors, FileError{
					FileName: name, Row: tx.SourceRow, Message: "Invalid booking date.",
				})
				preview.InvalidCount++
				continue
			}
			amount, ok := normalizeDecimal(tx.Amount)
			if !ok {
				preview.Errors = append(preview.Errors, FileError{
					FileName: name, Row: tx.SourceRow, Message: "Invalid amount.",
				})
				preview.InvalidCount++
				continue
			}
			tx.Amount = amount
			if tx.Currency == "" {
				tx.Currency = "EUR"
			}
			tx.Currency = strings.ToUpper(tx.Currency)
			if len(tx.Currency) != 3 {
				preview.Errors = append(preview.Errors, FileError{
					FileName: name, Row: tx.SourceRow, Message: "Invalid currency code.",
				})
				preview.InvalidCount++
				continue
			}
			if tx.Direction == domain.DirectionUnknown {
				value, _ := decimalRat(amount)
				if value.Sign() > 0 {
					tx.Direction = domain.DirectionIncoming
				} else if value.Sign() < 0 {
					tx.Direction = domain.DirectionOutgoing
				}
			}
			if tx.Direction != domain.DirectionIncoming &&
				tx.Direction != domain.DirectionOutgoing &&
				tx.Direction != domain.DirectionUnknown {
				preview.Errors = append(preview.Errors, FileError{
					FileName: name, Row: tx.SourceRow, Message: "Invalid transaction direction.",
				})
				preview.InvalidCount++
				continue
			}
			if tx.SourceRow < 1 {
				tx.SourceRow = txIndex + 1
			}

			duplicate, err := findDuplicate(ctx, s.db, accountID, tx)
			if err != nil {
				return Preview{}, fmt.Errorf("check for duplicate transaction: %w", err)
			}
			rowKey := fmt.Sprintf("row-%d", len(state.rows))
			rateKey := ""
			if tx.Currency != "EUR" && tx.EURAmount == nil {
				rateKey = rateKeyFor(tx.Currency, tx.BookingDate)
				rateKeys[rateKey] = RateRequirement{
					Key: rateKey, Currency: tx.Currency,
					Date: tx.BookingDate.Format("2006-01-02"),
				}
			}
			categoryID, categoryName, ruleMatches, conflict := matchRules(
				rules, tx.Direction, tx.Description, uncategorizedID,
			)
			if conflict {
				preview.UnresolvedRules++
			}
			state.rows = append(state.rows, stagedRow{
				key: rowKey, fileIndex: fileIndex, fileName: name,
				account: parsed.Account, tx: tx, duplicate: duplicate, rateKey: rateKey,
				categoryID: categoryID, category: categoryName,
				ruleMatches: ruleMatches, conflict: conflict,
			})
			if duplicate != nil {
				preview.DuplicateCount++
			}
			preview.Rows = append(preview.Rows, RowPreview{
				Key: rowKey, FileName: name, SourceRow: tx.SourceRow,
				Account: parsed.Account, Transaction: tx,
				Duplicate: duplicate, RateKey: rateKey, CategoryID: categoryID,
				Category: categoryName, RuleMatches: ruleMatches, Conflict: conflict,
			})
			if firstDate.IsZero() || tx.BookingDate.Before(firstDate) {
				firstDate = tx.BookingDate
			}
			if lastDate.IsZero() || tx.BookingDate.After(lastDate) {
				lastDate = tx.BookingDate
			}
		}
	}

	if !firstDate.IsZero() {
		preview.DateFrom = firstDate.Format("2006-01-02")
		preview.DateTo = lastDate.Format("2006-01-02")
	}
	for _, requirement := range rateKeys {
		preview.RateRequirements = append(preview.RateRequirements, requirement)
	}
	sort.Slice(preview.RateRequirements, func(i, j int) bool {
		return preview.RateRequirements[i].Key < preview.RateRequirements[j].Key
	})
	preview.IncomingTotal, preview.OutgoingTotal, preview.TotalsPartial = knownTotals(state.rows)
	preview.CategoryTotals, preview.CategoryPartial = knownCategoryTotals(state.rows)
	if preview.InvalidCount > 0 {
		return preview, nil
	}
	if len(state.rows) == 0 && len(state.files) == 0 {
		preview.Errors = append(preview.Errors, FileError{Message: "No valid files were selected."})
		preview.InvalidCount++
		return preview, nil
	}

	token, err := newToken()
	if err != nil {
		return Preview{}, fmt.Errorf("create import preview token: %w", err)
	}
	preview.Token = token
	state.view = preview
	s.mu.Lock()
	s.removeExpiredLocked(time.Now())
	s.previews[token] = state
	state.expiry = time.AfterFunc(previewLifetime, func() {
		s.mu.Lock()
		if s.previews[token] == state {
			delete(s.previews, token)
		}
		s.mu.Unlock()
	})
	s.mu.Unlock()
	return preview, nil
}

func (s *Service) GetPreview(token string) (Preview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.previews[token]
	if !ok || time.Since(state.createdAt) > previewLifetime {
		if ok && state.expiry != nil {
			state.expiry.Stop()
		}
		delete(s.previews, token)
		return Preview{}, false
	}
	return state.view, true
}

func (s *Service) Confirm(
	ctx context.Context,
	token string,
	decisions map[string]Decision,
	rates map[string]string,
) (Result, error) {
	s.mu.Lock()
	state, ok := s.previews[token]
	if !ok || time.Since(state.createdAt) > previewLifetime {
		if ok && state.expiry != nil {
			state.expiry.Stop()
		}
		delete(s.previews, token)
		s.mu.Unlock()
		return Result{}, errors.New("import preview expired; select the files again")
	}
	s.mu.Unlock()

	effectiveRows, err := prepareRows(state.rows, decisions, rates)
	if err != nil {
		return Result{}, err
	}

	s.mu.Lock()
	if s.previews[token] != state {
		s.mu.Unlock()
		return Result{}, errors.New("import preview is no longer available")
	}
	delete(s.previews, token)
	s.mu.Unlock()

	result, err := s.commit(ctx, state, effectiveRows)
	if err != nil {
		s.mu.Lock()
		if time.Since(state.createdAt) <= previewLifetime {
			s.previews[token] = state
		} else if state.expiry != nil {
			state.expiry.Stop()
		}
		s.mu.Unlock()
		return Result{}, err
	}
	if state.expiry != nil {
		state.expiry.Stop()
	}
	return result, nil
}

func (s *Service) removeExpiredLocked(now time.Time) {
	for token, preview := range s.previews {
		if now.Sub(preview.createdAt) > previewLifetime {
			if preview.expiry != nil {
				preview.expiry.Stop()
			}
			delete(s.previews, token)
		}
	}
}

type preparedRow struct {
	stagedRow
	decision     Decision
	eurAmount    string
	exchangeRate string
}

func prepareRows(rows []stagedRow, decisions map[string]Decision, rates map[string]string) ([]preparedRow, error) {
	knownKeys := make(map[string]struct{}, len(rows))
	requiredRates := make(map[string]struct{})
	for _, row := range rows {
		knownKeys[row.key] = struct{}{}
		if row.rateKey != "" {
			requiredRates[row.rateKey] = struct{}{}
		}
	}
	for key := range decisions {
		if _, ok := knownKeys[key]; !ok {
			return nil, errors.New("import preview contains an unknown transaction choice")
		}
	}
	for key := range rates {
		if _, ok := requiredRates[key]; !ok {
			return nil, errors.New("import preview contains an unexpected exchange rate")
		}
	}

	for key := range requiredRates {
		rate, ok := rates[key]
		if !ok {
			return nil, fmt.Errorf("an exchange rate is required for %s", strings.ReplaceAll(key, "|", " on "))
		}
		value, ok := decimalRat(rate)
		if !ok || value.Sign() <= 0 {
			return nil, fmt.Errorf("exchange rate for %s must be a positive decimal", strings.ReplaceAll(key, "|", " on "))
		}
	}

	prepared := make([]preparedRow, 0, len(rows))
	for _, row := range rows {
		decision := decisions[row.key]
		if row.tx.Direction == domain.DirectionUnknown {
			if decision.Direction != domain.DirectionIncoming && decision.Direction != domain.DirectionOutgoing {
				return nil, fmt.Errorf("choose incoming or outgoing for %s row %d", row.fileName, row.tx.SourceRow)
			}
			row.tx.Direction = decision.Direction
		}
		if row.conflict {
			validCategory := false
			for _, category := range row.ruleMatches {
				if category.ID == decision.CategoryID {
					validCategory = true
					row.categoryID, row.category = category.ID, category.Name
					break
				}
			}
			if !validCategory {
				return nil, fmt.Errorf("choose a category for conflicting rules in %s row %d", row.fileName, row.tx.SourceRow)
			}
		} else if decision.CategoryID != 0 {
			return nil, errors.New("a category choice was provided when no rule conflict exists")
		}
		if row.duplicate != nil {
			switch decision.Action {
			case "skip", "keep-both":
			case "update":
				if row.duplicate.ManualEdit &&
					decision.ManualResolution != "keep-edits" &&
					decision.ManualResolution != "use-imported" {
					return nil, fmt.Errorf("choose whether to keep manual edits or use imported values for %s row %d", row.fileName, row.tx.SourceRow)
				}
			default:
				return nil, fmt.Errorf("choose how to handle the possible duplicate in %s row %d", row.fileName, row.tx.SourceRow)
			}
		} else if decision.Action != "" {
			return nil, errors.New("a duplicate choice was provided for a transaction that has no duplicate")
		}

		eurAmount, err := convertToEUR(row.tx, rates[row.rateKey])
		if err != nil {
			return nil, fmt.Errorf("%s row %d: %w", row.fileName, row.tx.SourceRow, err)
		}
		prepared = append(prepared, preparedRow{
			stagedRow: row, decision: decision, eurAmount: eurAmount,
			exchangeRate: rates[row.rateKey],
		})
	}
	return prepared, nil
}

func convertToEUR(tx domain.Transaction, rate string) (string, error) {
	if tx.EURAmount != nil {
		amount, ok := normalizeDecimal(*tx.EURAmount)
		if !ok {
			return "", errors.New("invalid bank-provided EUR amount")
		}
		return amount, nil
	}
	if tx.Currency == "EUR" {
		return tx.Amount, nil
	}
	amount, ok := decimalRat(tx.Amount)
	if !ok {
		return "", errors.New("invalid transaction amount")
	}
	exchangeRate, ok := decimalRat(rate)
	if !ok || exchangeRate.Sign() <= 0 {
		return "", errors.New("invalid exchange rate")
	}
	return new(big.Rat).Mul(amount, exchangeRate).FloatString(2), nil
}

func knownTotals(rows []stagedRow) (string, string, bool) {
	incoming := new(big.Rat)
	outgoing := new(big.Rat)
	partial := false
	for _, row := range rows {
		var amount string
		switch {
		case row.tx.EURAmount != nil:
			amount = *row.tx.EURAmount
		case row.tx.Currency == "EUR":
			amount = row.tx.Amount
		default:
			partial = true
			continue
		}
		value, ok := decimalRat(amount)
		if !ok {
			partial = true
			continue
		}
		switch row.tx.Direction {
		case domain.DirectionIncoming:
			incoming.Add(incoming, value)
		case domain.DirectionOutgoing:
			outgoing.Add(outgoing, new(big.Rat).Abs(value))
		default:
			partial = true
		}
	}
	return incoming.FloatString(2), outgoing.FloatString(2), partial
}

func knownCategoryTotals(rows []stagedRow) ([]CategoryTotal, bool) {
	type totals struct {
		incoming *big.Rat
		outgoing *big.Rat
	}
	byCategory := make(map[int64]*totals)
	names := make(map[int64]string)
	partial := false
	for _, row := range rows {
		if row.conflict {
			partial = true
			continue
		}
		var amount string
		switch {
		case row.tx.EURAmount != nil:
			amount = *row.tx.EURAmount
		case row.tx.Currency == "EUR":
			amount = row.tx.Amount
		default:
			partial = true
			continue
		}
		value, ok := decimalRat(amount)
		if !ok {
			partial = true
			continue
		}
		category := byCategory[row.categoryID]
		if category == nil {
			category = &totals{incoming: new(big.Rat), outgoing: new(big.Rat)}
			byCategory[row.categoryID] = category
		}
		names[row.categoryID] = row.category
		switch row.tx.Direction {
		case domain.DirectionIncoming:
			category.incoming.Add(category.incoming, value)
		case domain.DirectionOutgoing:
			category.outgoing.Add(category.outgoing, new(big.Rat).Abs(value))
		default:
			partial = true
		}
	}
	result := make([]CategoryTotal, 0, len(byCategory))
	for id, sum := range byCategory {
		result = append(result, CategoryTotal{
			CategoryID: id, CategoryName: names[id],
			Incoming: sum.incoming.FloatString(2), Outgoing: sum.outgoing.FloatString(2),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if strings.EqualFold(result[i].CategoryName, result[j].CategoryName) {
			return result[i].CategoryID < result[j].CategoryID
		}
		return strings.ToLower(result[i].CategoryName) < strings.ToLower(result[j].CategoryName)
	})
	return result, partial
}

func matchRules(
	rules []categorization.Rule,
	direction domain.Direction,
	description string,
	uncategorizedID int64,
) (int64, string, []CategoryOption, bool) {
	byCategory := make(map[int64]string)
	for _, rule := range rules {
		if categorization.Matches(rule, direction, description) {
			byCategory[rule.CategoryID] = rule.Category
		}
	}
	if len(byCategory) == 0 {
		return uncategorizedID, "Uncategorized", nil, false
	}
	matches := make([]CategoryOption, 0, len(byCategory))
	for id, name := range byCategory {
		matches = append(matches, CategoryOption{ID: id, Name: name})
	}
	sort.Slice(matches, func(i, j int) bool {
		if strings.EqualFold(matches[i].Name, matches[j].Name) {
			return matches[i].ID < matches[j].ID
		}
		return strings.ToLower(matches[i].Name) < strings.ToLower(matches[j].Name)
	})
	if len(matches) > 1 {
		return 0, "", matches, true
	}
	return matches[0].ID, matches[0].Name, matches, false
}

func uncategorizedCategoryID(ctx context.Context, db *sql.DB) (int64, error) {
	var id int64
	if err := db.QueryRowContext(ctx,
		"SELECT id FROM categories WHERE name = 'Uncategorized'",
	).Scan(&id); err != nil {
		return 0, fmt.Errorf("find Uncategorized category: %w", err)
	}
	return id, nil
}

func findAccountID(ctx context.Context, db *sql.DB, bank domain.Bank, number string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx,
		"SELECT id FROM accounts WHERE bank = ? AND account_number = ?",
		string(bank), number,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func findDuplicate(ctx context.Context, db *sql.DB, accountID int64, tx domain.Transaction) (*Duplicate, error) {
	if accountID == 0 {
		return nil, nil
	}
	var duplicate Duplicate
	var manualEdit int
	err := db.QueryRowContext(ctx, `
		SELECT id, amount, currency, description, payee, COALESCE(value_date, ''), is_manually_edited
		FROM transactions
		WHERE account_id = ? AND amount = ? AND description = ?
		  AND booking_date = ? AND pending = 0
		ORDER BY id LIMIT 1`,
		accountID, tx.Amount, tx.Description, tx.BookingDate.Format("2006-01-02"),
	).Scan(
		&duplicate.TransactionID, &duplicate.Amount, &duplicate.Currency,
		&duplicate.Description, &duplicate.Payee, &duplicate.ValueDate, &manualEdit,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	duplicate.ManualEdit = manualEdit == 1
	return &duplicate, nil
}

func safeFileName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	if name == "." || name == "/" {
		return ""
	}
	return name
}

func rateKeyFor(currency string, date time.Time) string {
	return currency + "|" + date.Format("2006-01-02")
}

func newToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
