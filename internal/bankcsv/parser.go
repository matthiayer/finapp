package bankcsv

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/matthiayer/finapp/internal/domain"
)

var errUnrecognized = errors.New("unrecognized or unreadable bank CSV format")
var ErrUnsupportedAccountType = errors.New("unsupported account type")

type record struct {
	fields []string
	row    int
}

// ParseFile detects and parses a supported Belgian bank CSV export.
func ParseFile(filename string, data []byte) (domain.ParsedFile, error) {
	_ = filename
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if len(bytes.TrimSpace(data)) == 0 {
		return domain.ParsedFile{}, errUnrecognized
	}

	// ING and VDK have distinctive semicolon-delimited layouts.
	if rows, err := readRecords(data, ';'); err == nil {
		if isINGHeader(rows) {
			return parseING(rows)
		}
		if isVDKLayout(rows) {
			return parseVDK(rows)
		}
	}

	for _, delimiter := range []rune{',', ';'} {
		rows, err := readRecords(data, delimiter)
		if err != nil || !isTriodosLayout(rows) {
			continue
		}
		return parseTriodos(rows)
	}
	return domain.ParsedFile{}, errUnrecognized
}

func readRecords(data []byte, delimiter rune) ([]record, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	var rows []record
	for {
		fields, err := reader.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		line, _ := reader.FieldPos(0)
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		rows = append(rows, record{fields: fields, row: line})
	}
}

func isINGHeader(rows []record) bool {
	if len(rows) == 0 || len(rows[0].fields) != 11 {
		return false
	}
	expected := []string{
		"Rekeningnummer", "Naam van de rekening", "Rekening tegenpartij",
		"Omzetnummer", "Boekingsdatum", "Valutadatum", "Bedrag",
		"Munteenheid", "Omschrijving", "Detail van de omzet", "Bericht",
	}
	for i := range expected {
		if rows[0].fields[i] != expected[i] {
			return false
		}
	}
	return true
}

func isVDKLayout(rows []record) bool {
	if len(rows) < 7 || len(rows[0].fields) != 2 || len(rows[5].fields) != 2 || len(rows[6].fields) != 18 {
		return false
	}
	// Summary labels are bank-localized; order and the transaction header shape
	// are stable across the described export.
	header := strings.ToLower(strings.Join(rows[6].fields, " "))
	return strings.Contains(header, "datum") &&
		(strings.Contains(header, "bedrag") || strings.Contains(header, "amount")) &&
		(strings.Contains(header, "rekening") || strings.Contains(header, "tegenpartij"))
}

func isTriodosLayout(rows []record) bool {
	if len(rows) == 0 {
		return false
	}
	start := 0
	if len(rows[0].fields) == 10 && looksLikeTriodosHeader(rows[0].fields) {
		start = 1
	}
	if start == len(rows) {
		return start == 1
	}
	for _, row := range rows[start:] {
		if len(row.fields) == 10 {
			return true
		}
	}
	return false
}

func looksLikeTriodosHeader(fields []string) bool {
	firstField := strings.ToLower(fields[0])
	return strings.Contains(firstField, "datum") || strings.Contains(firstField, "date")
}

func parseING(rows []record) (domain.ParsedFile, error) {
	result := domain.ParsedFile{
		Bank: domain.BankING,
		Account: domain.Account{
			Bank: domain.BankING, Currency: "EUR",
		},
	}
	for _, row := range rows[1:] {
		if len(row.fields) != 11 {
			result.Errors = append(result.Errors, rowError(row.row, "expected 11 fields"))
			continue
		}
		tx, field := parseINGTransaction(row.fields)
		if field != "" {
			result.Errors = append(result.Errors, rowError(row.row, "invalid "+field))
			continue
		}
		if result.Account.Number == "" {
			result.Account.Number = row.fields[0]
			result.Account.Name = row.fields[1]
		}
		tx.SourceRow = row.row
		result.Transactions = append(result.Transactions, tx)
	}
	return result, nil
}

func parseINGTransaction(fields []string) (domain.Transaction, string) {
	bookingDate, ok := parseDate(fields[4], "02/01/2006")
	if !ok {
		return domain.Transaction{}, "booking date"
	}
	var valueDate *time.Time
	if fields[5] != "" {
		parsed, ok := parseDate(fields[5], "02/01/2006")
		if !ok {
			return domain.Transaction{}, "value date"
		}
		valueDate = &parsed
	}
	amount, ok := normalizeAmount(fields[6])
	if !ok {
		return domain.Transaction{}, "amount"
	}
	currency := strings.ToUpper(fields[7])
	if currency == "" {
		currency = "EUR"
	}
	tx := makeTransaction(bookingDate, valueDate, fields[0], fields[2], "",
		joinDescription(fields[8], fields[9], fields[10]), amount, currency)
	tx.Reference = fields[3]
	return tx, ""
}

func parseTriodos(rows []record) (domain.ParsedFile, error) {
	start := 0
	if len(rows[0].fields) == 10 && looksLikeTriodosHeader(rows[0].fields) {
		start = 1
	}
	result := domain.ParsedFile{Bank: domain.BankTriodos}
	for _, row := range rows[start:] {
		if len(row.fields) != 10 {
			result.Errors = append(result.Errors, rowError(row.row, "expected 10 fields"))
			continue
		}
		fields := row.fields
		bookingDate, ok := parseDate(fields[0], "02-01-2006")
		if !ok {
			result.Errors = append(result.Errors, rowError(row.row, "invalid booking date"))
			continue
		}
		amount, ok := normalizeAmount(fields[2])
		if !ok {
			result.Errors = append(result.Errors, rowError(row.row, "invalid amount"))
			continue
		}
		balance, ok := normalizeAmount(fields[9])
		if !ok {
			result.Errors = append(result.Errors, rowError(row.row, "invalid balance"))
			continue
		}
		if result.Account.Number == "" {
			result.Account = domain.Account{
				Bank: domain.BankTriodos, Number: fields[1], Currency: "EUR",
			}
		}
		tx := makeTransaction(bookingDate, nil, fields[1], fields[3], fields[5],
			fields[8], amount, "EUR")
		tx.SourceRow = row.row
		tx.Reference = fields[7]
		tx.BalanceAfter = stringPointer(balance)
		if result.Account.BalanceAsOf == nil || !bookingDate.Before(*result.Account.BalanceAsOf) {
			result.Account.Balance = stringPointer(balance)
			result.Account.BalanceAsOf = &bookingDate
		}
		result.Transactions = append(result.Transactions, tx)
	}
	if result.Account.Bank == "" {
		result.Account = domain.Account{Bank: domain.BankTriodos, Currency: "EUR"}
	}
	return result, nil
}

func parseVDK(rows []record) (domain.ParsedFile, error) {
	accountNumber := rows[0].fields[1]
	accountName := rows[1].fields[1]
	if !supportedVDKAccountType(rows[2].fields[1]) {
		return domain.ParsedFile{}, ErrUnsupportedAccountType
	}
	accountBalance, ok := normalizeAmount(rows[3].fields[1])
	if !ok {
		return domain.ParsedFile{}, errUnrecognized
	}
	balanceAsOf, ok := parseDate(rows[4].fields[1], "02/01/2006 15:04:05")
	if !ok {
		return domain.ParsedFile{}, errUnrecognized
	}
	result := domain.ParsedFile{
		Bank: domain.BankVDK,
		Account: domain.Account{
			Bank: domain.BankVDK, Number: accountNumber, Name: accountName,
			Currency: "EUR", Balance: stringPointer(accountBalance), BalanceAsOf: &balanceAsOf,
		},
	}
	for _, row := range rows[7:] {
		if len(row.fields) != 18 {
			result.Errors = append(result.Errors, rowError(row.row, "expected 18 fields"))
			continue
		}
		fields := row.fields
		bookingDate, valid := parseDate(fields[0], "02/01/2006")
		if !valid {
			result.Errors = append(result.Errors, rowError(row.row, "invalid execution date"))
			continue
		}
		var valueDate *time.Time
		if fields[1] != "" {
			parsed, valid := parseDate(fields[1], "02/01/2006")
			if !valid {
				result.Errors = append(result.Errors, rowError(row.row, "invalid value date"))
				continue
			}
			valueDate = &parsed
		}
		amount, valid := normalizeAmount(fields[14])
		if !valid {
			result.Errors = append(result.Errors, rowError(row.row, "invalid amount"))
			continue
		}
		balanceAfter, valid := normalizeOptionalAmount(fields[15])
		if !valid {
			result.Errors = append(result.Errors, rowError(row.row, "invalid balance"))
			continue
		}
		exchangeRate, valid := normalizeOptionalAmount(fields[16])
		if !valid {
			result.Errors = append(result.Errors, rowError(row.row, "invalid exchange rate"))
			continue
		}
		fees, valid := normalizeOptionalAmount(fields[17])
		if !valid {
			result.Errors = append(result.Errors, rowError(row.row, "invalid fees"))
			continue
		}
		tx := makeTransaction(bookingDate, valueDate, accountNumber, fields[6], fields[8],
			joinDescription(fields[5], fields[13]), amount, "EUR")
		tx.Reference = fields[4]
		tx.BalanceAfter = balanceAfter
		tx.ExchangeRate = exchangeRate
		tx.Fees = fees
		tx.SourceRow = row.row
		result.Transactions = append(result.Transactions, tx)
	}
	return result, nil
}

func supportedVDKAccountType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "current", "checking", "current account", "checking account",
		"zichtrekening", "compte à vue":
		return true
	default:
		return false
	}
}

func makeTransaction(bookingDate time.Time, valueDate *time.Time, account, counterpartyAccount, counterpartyName, description, amount, currency string) domain.Transaction {
	direction := domain.DirectionUnknown
	switch {
	case strings.HasPrefix(amount, "-"):
		direction = domain.DirectionOutgoing
	case amount != "0":
		direction = domain.DirectionIncoming
	}
	return domain.Transaction{
		BookingDate: bookingDate, ValueDate: valueDate, AccountNumber: account,
		CounterpartyAccount: counterpartyAccount, CounterpartyName: counterpartyName,
		Description: description, Amount: amount, Currency: currency, Direction: direction,
	}
}

func parseDate(value, layout string) (time.Time, bool) {
	parsed, err := time.Parse(layout, value)
	return parsed, err == nil
}

func normalizeOptionalAmount(value string) (*string, bool) {
	if strings.TrimSpace(value) == "" {
		return nil, true
	}
	normalized, ok := normalizeAmount(value)
	if !ok {
		return nil, false
	}
	return stringPointer(normalized), true
}

func normalizeAmount(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\u00a0", "")
	if value == "" {
		return "", false
	}
	negative := false
	if value[0] == '-' || value[0] == '+' {
		negative = value[0] == '-'
		value = value[1:]
	}
	if value == "" || strings.ContainsAny(value, "+-") {
		return "", false
	}
	if strings.Count(value, ",") > 1 {
		return "", false
	}
	if strings.Contains(value, ",") {
		parts := strings.SplitN(value, ",", 2)
		if len(parts[1]) == 0 || !allDigits(parts[1]) {
			return "", false
		}
		integer, ok := removeGrouping(parts[0])
		if !ok {
			return "", false
		}
		value = integer + "." + parts[1]
	} else if strings.Count(value, ".") > 1 {
		integer, ok := removeGrouping(value)
		if !ok {
			return "", false
		}
		value = integer
	} else if strings.Contains(value, ".") {
		parts := strings.SplitN(value, ".", 2)
		if !allDigits(parts[0]) || !allDigits(parts[1]) {
			return "", false
		}
	}
	if !validDecimal(value) {
		return "", false
	}
	parts := strings.SplitN(value, ".", 2)
	integer := strings.TrimLeft(parts[0], "0")
	if integer == "" {
		integer = "0"
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = strings.TrimRight(parts[1], "0")
	}
	if fraction != "" {
		value = integer + "." + fraction
	} else {
		value = integer
	}
	if negative && value != "0" {
		value = "-" + value
	}
	return value, true
}

func removeGrouping(value string) (string, bool) {
	if !strings.Contains(value, ".") {
		if !allDigits(value) {
			return "", false
		}
		return value, true
	}
	groups := strings.Split(value, ".")
	if len(groups[0]) < 1 || len(groups[0]) > 3 || !allDigits(groups[0]) {
		return "", false
	}
	for _, group := range groups[1:] {
		if len(group) != 3 || !allDigits(group) {
			return "", false
		}
	}
	return strings.Join(groups, ""), true
}

func validDecimal(value string) bool {
	parts := strings.Split(value, ".")
	return len(parts) <= 2 && allDigits(parts[0]) &&
		(len(parts) == 1 || allDigits(parts[1]))
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) || r > '9' {
			return false
		}
	}
	return true
}

func joinDescription(values ...string) string {
	var nonempty []string
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			nonempty = append(nonempty, value)
		}
	}
	return strings.Join(nonempty, " | ")
}

func rowError(row int, message string) domain.RowError {
	return domain.RowError{Row: row, Message: message}
}

func stringPointer(value string) *string {
	return &value
}
