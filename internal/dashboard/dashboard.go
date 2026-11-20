package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Filter struct {
	Start, End time.Time
	CategoryID int64
}

type CategoryOption struct {
	ID   int64
	Name string
}

type Summary struct {
	Income   string
	Spending string
	Saving   string
}

type MonthlyTotal struct {
	Month    string
	Income   string
	Spending string
	Saving   string
}

type CategoryTotal struct {
	CategoryID   int64
	CategoryName string
	Total        string
}

type AccountBalance struct {
	AccountID int64
	Bank      string
	Number    string
	Name      string
	Currency  string
	Balance   string
	Date      string
	Known     bool
}

type BalancePoint struct {
	Month            string
	Balance          string
	ObservedAccounts int
}

type AccountBalancePoint struct {
	AccountID int64
	Label     string
	Month     string
	Balance   string
}

type View struct {
	Start              string
	End                string
	HasActivity        bool
	Categories         []CategoryOption
	SelectedCategoryID int64
	Summary            Summary
	Monthly            []MonthlyTotal
	SpendingByCategory []CategoryTotal
	IncomeByCategory   []CategoryTotal
	Accounts           []AccountBalance
	BalanceHistory     []BalancePoint
	AccountHistory     []AccountBalancePoint
	CombinedBalance    string
	BalanceDatesDiffer bool
}

var decimalRE = regexp.MustCompile(`^[+-]?\d+(?:\.\d+)?$`)

type decimalSum struct {
	value *big.Rat
	scale int
}

func (s *decimalSum) add(text string) error {
	return s.addValue(text, false)
}

func (s *decimalSum) addMagnitude(text string) error {
	return s.addValue(text, true)
}

func (s *decimalSum) addValue(text string, magnitude bool) error {
	text = strings.TrimSpace(text)
	if !decimalRE.MatchString(text) {
		return errors.New("invalid decimal amount")
	}
	rat, ok := new(big.Rat).SetString(text)
	if !ok {
		return errors.New("invalid decimal amount")
	}
	if magnitude {
		rat.Abs(rat)
	}
	if s.value == nil {
		s.value = new(big.Rat)
	}
	s.value.Add(s.value, rat)
	if dot := strings.IndexByte(text, '.'); dot >= 0 && len(text)-dot-1 > s.scale {
		s.scale = len(text) - dot - 1
	}
	return nil
}

func (s decimalSum) string() string {
	if s.value == nil {
		return "0"
	}
	out := s.value.FloatString(s.scale)
	if strings.Contains(out, ".") {
		out = strings.TrimRight(strings.TrimRight(out, "0"), ".")
	}
	if out == "-0" || out == "" {
		return "0"
	}
	return out
}

func Load(ctx context.Context, db *sql.DB, filter Filter) (View, error) {
	var view View
	view.SelectedCategoryID = filter.CategoryID
	if !filter.Start.IsZero() {
		view.Start = filter.Start.Format("2006-01-02")
	}
	if !filter.End.IsZero() {
		view.End = filter.End.Format("2006-01-02")
	}
	if !filter.Start.IsZero() && !filter.End.IsZero() && view.Start > view.End {
		return view, fmt.Errorf("start date must not be after end date")
	}

	if err := loadCategories(ctx, db, &view); err != nil {
		return view, err
	}
	categoryNames := make(map[int64]string, len(view.Categories))
	for _, category := range view.Categories {
		categoryNames[category.ID] = category.Name
	}
	if filter.CategoryID != 0 {
		if _, ok := categoryNames[filter.CategoryID]; !ok {
			return view, fmt.Errorf("category %d does not exist", filter.CategoryID)
		}
	}
	if err := loadTransactions(ctx, db, filter, categoryNames, &view); err != nil {
		return view, err
	}
	if err := loadAccounts(ctx, db, &view); err != nil {
		return view, err
	}
	if err := loadBalanceHistory(ctx, db, filter, &view); err != nil {
		return view, err
	}
	return view, nil
}

func loadCategories(ctx context.Context, db *sql.DB, view *View) error {
	rows, err := db.QueryContext(ctx, `SELECT id, name FROM categories ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return fmt.Errorf("load dashboard categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var category CategoryOption
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			return fmt.Errorf("scan dashboard category: %w", err)
		}
		view.Categories = append(view.Categories, category)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dashboard categories: %w", err)
	}
	return nil
}

type monthlySums struct {
	income   decimalSum
	spending decimalSum
}

func loadTransactions(ctx context.Context, db *sql.DB, filter Filter, names map[int64]string, view *View) error {
	query := `SELECT id, booking_date, eur_amount, direction, category_id
		FROM transactions
		WHERE pending = 0 AND transfer_id IS NULL
			AND direction IN ('incoming', 'outgoing')
			AND (? = '' OR date(booking_date) >= date(?))
			AND (? = '' OR date(booking_date) <= date(?))
			AND (? = 0 OR category_id = ?)
		ORDER BY booking_date, id`
	rows, err := db.QueryContext(ctx, query,
		view.Start, view.Start, view.End, view.End, filter.CategoryID, filter.CategoryID)
	if err != nil {
		return fmt.Errorf("load dashboard transactions: %w", err)
	}
	defer rows.Close()

	var income, spending decimalSum
	monthly := make(map[string]*monthlySums)
	spendingByCategory := make(map[int64]decimalSum)
	incomeByCategory := make(map[int64]decimalSum)
	for rows.Next() {
		var id, categoryID int64
		var bookingDate, amount, direction string
		if err := rows.Scan(&id, &bookingDate, &amount, &direction, &categoryID); err != nil {
			return fmt.Errorf("scan dashboard transaction: %w", err)
		}
		view.HasActivity = true
		month := bookingDate[:min(7, len(bookingDate))]
		monthSums := monthly[month]
		if monthSums == nil {
			monthSums = &monthlySums{}
			monthly[month] = monthSums
		}
		var err error
		if direction == "incoming" {
			err = income.addMagnitude(amount)
			if err == nil {
				err = monthSums.income.addMagnitude(amount)
			}
			if err == nil {
				sum := incomeByCategory[categoryID]
				err = sum.addMagnitude(amount)
				incomeByCategory[categoryID] = sum
			}
		} else {
			err = spending.addMagnitude(amount)
			if err == nil {
				err = monthSums.spending.addMagnitude(amount)
			}
			if err == nil {
				sum := spendingByCategory[categoryID]
				err = sum.addMagnitude(amount)
				spendingByCategory[categoryID] = sum
			}
		}
		if err != nil {
			return fmt.Errorf("sum dashboard transaction %d: %w", id, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dashboard transactions: %w", err)
	}
	view.Summary = Summary{
		Income: income.string(), Spending: spending.string(),
		Saving: subtract(income, spending).string(),
	}

	months := make([]string, 0, len(monthly))
	for month := range monthly {
		months = append(months, month)
	}
	sort.Strings(months)
	if !filter.Start.IsZero() && !filter.End.IsZero() {
		months = completeMonths(filter.Start, filter.End)
	} else if len(months) > 0 {
		first, err := time.Parse("2006-01", months[0])
		if err != nil {
			return fmt.Errorf("parse first dashboard month: %w", err)
		}
		last, err := time.Parse("2006-01", months[len(months)-1])
		if err != nil {
			return fmt.Errorf("parse last dashboard month: %w", err)
		}
		months = completeMonths(first, last)
	}
	for _, month := range months {
		sums := monthly[month]
		if sums == nil {
			sums = &monthlySums{}
		}
		view.Monthly = append(view.Monthly, MonthlyTotal{
			Month: month, Income: sums.income.string(), Spending: sums.spending.string(),
			Saving: subtract(sums.income, sums.spending).string(),
		})
	}
	view.SpendingByCategory = categoryTotals(spendingByCategory, names)
	view.IncomeByCategory = categoryTotals(incomeByCategory, names)
	return nil
}

func subtract(left, right decimalSum) decimalSum {
	out := decimalSum{scale: max(left.scale, right.scale)}
	out.value = new(big.Rat)
	if left.value != nil {
		out.value.Add(out.value, left.value)
	}
	if right.value != nil {
		out.value.Sub(out.value, right.value)
	}
	return out
}

func completeMonths(start, end time.Time) []string {
	start = time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	var months []string
	for month := start; !month.After(end); month = month.AddDate(0, 1, 0) {
		months = append(months, month.Format("2006-01"))
	}
	return months
}

func categoryTotals(sums map[int64]decimalSum, names map[int64]string) []CategoryTotal {
	totals := make([]CategoryTotal, 0, len(sums))
	for id, sum := range sums {
		totals = append(totals, CategoryTotal{CategoryID: id, CategoryName: names[id], Total: sum.string()})
	}
	sort.Slice(totals, func(i, j int) bool {
		left := sums[totals[i].CategoryID]
		right := sums[totals[j].CategoryID]
		if comparison := left.value.Cmp(right.value); comparison != 0 {
			return comparison > 0
		}
		if totals[i].CategoryName != totals[j].CategoryName {
			return strings.ToLower(totals[i].CategoryName) < strings.ToLower(totals[j].CategoryName)
		}
		return totals[i].CategoryID < totals[j].CategoryID
	})
	return totals
}

func loadAccounts(ctx context.Context, db *sql.DB, view *View) error {
	const query = `SELECT
			a.id, a.bank, a.account_number, a.name, a.currency, a.balance_anchor_amount,
			date(a.balance_anchor_date),
			(SELECT t.balance_after FROM transactions t
				WHERE t.account_id = a.id AND t.balance_after IS NOT NULL
				ORDER BY date(t.booking_date) DESC, t.booking_date DESC, t.id DESC LIMIT 1),
			(SELECT date(t.booking_date) FROM transactions t
				WHERE t.account_id = a.id AND t.balance_after IS NOT NULL
				ORDER BY date(t.booking_date) DESC, t.booking_date DESC, t.id DESC LIMIT 1)
		FROM accounts a ORDER BY a.bank COLLATE NOCASE, a.name COLLATE NOCASE, a.id`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("load dashboard accounts: %w", err)
	}
	defer rows.Close()

	var combined decimalSum
	var componentDates []string
	for rows.Next() {
		var account AccountBalance
		var anchorAmount, anchorDate, transactionAmount, transactionDate sql.NullString
		if err := rows.Scan(&account.AccountID, &account.Bank, &account.Number, &account.Name, &account.Currency,
			&anchorAmount, &anchorDate, &transactionAmount, &transactionDate); err != nil {
			return fmt.Errorf("scan dashboard account: %w", err)
		}
		if anchorAmount.Valid {
			account.Balance, account.Date, account.Known = anchorAmount.String, anchorDate.String, true
		}
		if transactionAmount.Valid &&
			(!account.Known || (transactionDate.Valid && (!anchorDate.Valid || transactionDate.String >= anchorDate.String))) {
			account.Balance, account.Date, account.Known = transactionAmount.String, transactionDate.String, true
		}
		if account.Known {
			if err := combined.add(account.Balance); err != nil {
				return fmt.Errorf("sum account %d balance: %w", account.AccountID, err)
			}
			componentDates = append(componentDates, account.Date)
		} else {
			account.Balance = ""
		}
		view.Accounts = append(view.Accounts, account)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dashboard accounts: %w", err)
	}
	view.CombinedBalance = combined.string()
	if len(componentDates) > 1 {
		for _, date := range componentDates[1:] {
			if date != componentDates[0] {
				view.BalanceDatesDiffer = true
				break
			}
		}
	}
	return nil
}

func loadBalanceHistory(ctx context.Context, db *sql.DB, filter Filter, view *View) error {
	query := `SELECT account_id, booking_date, amount, account_balance
		FROM (
			SELECT account_id, booking_date, balance_after AS amount, 1 AS account_balance, id AS source_id
			FROM transactions
			WHERE balance_after IS NOT NULL
			UNION ALL
			SELECT id AS account_id, date(balance_anchor_date) AS booking_date,
				balance_anchor_amount AS amount, 0 AS account_balance, id AS source_id
			FROM accounts
			WHERE balance_anchor_amount IS NOT NULL AND balance_anchor_date IS NOT NULL
		)
		WHERE (? = '' OR date(booking_date) >= date(?))
			AND (? = '' OR date(booking_date) <= date(?))
		ORDER BY booking_date, account_balance, source_id`
	rows, err := db.QueryContext(ctx, query, view.Start, view.Start, view.End, view.End)
	if err != nil {
		return fmt.Errorf("load dashboard balance history: %w", err)
	}
	defer rows.Close()

	labels := make(map[int64]string, len(view.Accounts))
	for _, account := range view.Accounts {
		name := account.Name
		if name == "" {
			name = account.Number
		}
		labels[account.AccountID] = account.Bank + " · " + name
	}
	latest := make(map[string]map[int64]balanceObservation)
	for rows.Next() {
		var accountID int64
		var bookingDate, amount string
		var sourcePriority int
		if err := rows.Scan(&accountID, &bookingDate, &amount, &sourcePriority); err != nil {
			return fmt.Errorf("scan dashboard balance history: %w", err)
		}
		month := bookingDate[:min(7, len(bookingDate))]
		accounts := latest[month]
		if accounts == nil {
			accounts = make(map[int64]balanceObservation)
			latest[month] = accounts
		}
		accounts[accountID] = balanceObservation{amount: amount}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate dashboard balance history: %w", err)
	}
	months := make([]string, 0, len(latest))
	for month := range latest {
		months = append(months, month)
	}
	sort.Strings(months)
	for _, month := range months {
		var total decimalSum
		var observed int
		for _, account := range view.Accounts {
			observation, exists := latest[month][account.AccountID]
			if !exists {
				continue
			}
			if err := total.add(observation.amount); err != nil {
				return fmt.Errorf("sum %s account balance history: %w", month, err)
			}
			observed++
			view.AccountHistory = append(view.AccountHistory, AccountBalancePoint{
				AccountID: account.AccountID, Label: labels[account.AccountID],
				Month: month, Balance: observation.amount,
			})
		}
		view.BalanceHistory = append(view.BalanceHistory, BalancePoint{
			Month: month, Balance: total.string(), ObservedAccounts: observed,
		})
	}
	return nil
}

type balanceObservation struct {
	amount string
}
