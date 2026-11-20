package importflow

import (
	"time"

	"github.com/matthiayer/finapp/internal/domain"
)

const MaxFileSize = 1_000_000

type Upload struct {
	Name          string
	Data          []byte
	AccountChoice *AccountChoice
}

type Preview struct {
	Token            string
	Files            []FilePreview
	Rows             []RowPreview
	Errors           []FileError
	RateRequirements []RateRequirement
	ValidCount       int
	InvalidCount     int
	DuplicateCount   int
	DateFrom         string
	DateTo           string
	IncomingTotal    string
	OutgoingTotal    string
	TotalsPartial    bool
	CategoryTotals   []CategoryTotal
	CategoryPartial  bool
	UnresolvedRules  int
	NeedsAccountMapping bool
}

type CategoryTotal struct {
	CategoryID   int64
	CategoryName string
	Incoming     string
	Outgoing     string
}

type FilePreview struct {
	Name          string
	Bank          domain.Bank
	AccountNumber string
	AccountName   string
	RowCount      int
	UploadIndex   int
	NeedsAccountMapping bool
	ExistingAccounts []AccountOption
}

type AccountOption struct {
	ID     int64
	Number string
	Name   string
}

type AccountChoice struct {
	ExistingAccountID int64
	AccountNumber     string
	AccountName       string
}

type RowPreview struct {
	Key         string
	FileName    string
	SourceRow   int
	Account     domain.Account
	Transaction domain.Transaction
	Duplicate   *Duplicate
	RateKey     string
	CategoryID  int64
	Category    string
	RuleMatches []CategoryOption
	Conflict    bool
}

type CategoryOption struct {
	ID   int64
	Name string
}

type Duplicate struct {
	TransactionID int64
	Amount        string
	Currency      string
	Description   string
	Payee         string
	ValueDate     string
	ManualEdit    bool
	Differences   []FieldDifference
}

type FieldDifference struct {
	Field    string
	Existing string
	Incoming string
}

type FileError struct {
	FileName string
	Row      int
	Message  string
}

type RateRequirement struct {
	Key      string
	Currency string
	Date     string
}

type Decision struct {
	Action           string
	ManualResolution string
	Direction        domain.Direction
	CategoryID       int64
}

type Result struct {
	JobID         int64
	ImportedCount int
	SkippedCount  int
}

type stagedFile struct {
	name   string
	parsed domain.ParsedFile
}

type stagedRow struct {
	key         string
	fileIndex   int
	fileName    string
	account     domain.Account
	tx          domain.Transaction
	duplicate   *Duplicate
	rateKey     string
	categoryID  int64
	category    string
	ruleMatches []CategoryOption
	conflict    bool
}

type previewState struct {
	createdAt time.Time
	files     []stagedFile
	rows      []stagedRow
	view      Preview
	expiry    *time.Timer
	uploads   []Upload
}
