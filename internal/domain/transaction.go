package domain

import "time"

type Bank string

const (
	BankTriodos Bank = "Triodos Belgium"
	BankING     Bank = "ING Belgium"
	BankVDK     Bank = "VDK Belgium"
)

type Direction string

const (
	DirectionUnknown  Direction = "unknown"
	DirectionIncoming Direction = "incoming"
	DirectionOutgoing Direction = "outgoing"
)

type Account struct {
	Bank        Bank
	Number      string
	Name        string
	Currency    string
	Balance     *string
	BalanceAsOf *time.Time
}

type Transaction struct {
	SourceRow           int
	BookingDate         time.Time
	ValueDate           *time.Time
	AccountNumber       string
	CounterpartyAccount string
	CounterpartyName    string
	Description         string
	Reference           string
	Amount              string
	Currency            string
	EURAmount           *string
	ExchangeRate        *string
	Fees                *string
	BalanceAfter        *string
	Direction           Direction
	Pending             bool
}

type RowError struct {
	Row     int
	Message string
}

type ParsedFile struct {
	Bank         Bank
	Account      Account
	Transactions []Transaction
	Errors       []RowError
}
