package bankcsv

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/matthiayer/finapp/internal/domain"
)

func TestParseTriodos(t *testing.T) {
	data := `"12-03-2025","BE001","12,50","BE002","BICXXX","Sample Sender","","SCT","Synthetic payment","112,50"`
	parsed, err := ParseFile("triodos.csv", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Bank != domain.BankTriodos || parsed.Account.Number != "BE001" ||
		parsed.Account.Currency != "EUR" || parsed.Account.Balance == nil ||
		*parsed.Account.Balance != "112.5" {
		t.Fatalf("unexpected Triodos account: %#v", parsed.Account)
	}
	if len(parsed.Transactions) != 1 {
		t.Fatalf("got %d transactions, want 1", len(parsed.Transactions))
	}
	tx := parsed.Transactions[0]
	if tx.Amount != "12.5" || tx.Direction != domain.DirectionIncoming ||
		tx.CounterpartyName != "Sample Sender" || tx.CounterpartyAccount != "BE002" ||
		tx.Reference != "SCT" || tx.Description != "Synthetic payment" ||
		tx.BalanceAfter == nil || *tx.BalanceAfter != "112.5" {
		t.Fatalf("unexpected Triodos transaction: %#v", tx)
	}
}

func TestParseTriodosDelimitersAndHeaders(t *testing.T) {
	tests := []struct {
		name        string
		data        string
		sourceRow   int
		description string
	}{
		{
			name:        "comma without header",
			data:        `"12-03-2025","BE001","12,50","BE002","BICXXX","Sender","","SCT","Synthetic payment","112,50"`,
			sourceRow:   1,
			description: "Synthetic payment",
		},
		{
			name: "comma with header",
			data: `"Boekingsdatum","Rekening","Bedrag","Tegenrekening","BIC","Naam","Adres","Code","Omschrijving","Saldo"
"12-03-2025","BE001","12,50","BE002","BICXXX","Sender","","SCT","Synthetic payment","112,50"`,
			sourceRow:   2,
			description: "Synthetic payment",
		},
		{
			name:        "semicolon without header",
			data:        `"12-03-2025";"BE001";"12,50";"BE002";"BICXXX";"Sender";"";"SCT";"Synthetic payment";"112,50"`,
			sourceRow:   1,
			description: "Synthetic payment",
		},
		{
			name: "semicolon with header",
			data: `"Boekingsdatum";"Rekening";"Bedrag";"Tegenrekening";"BIC";"Naam";"Adres";"Code";"Omschrijving";"Saldo"
"12-03-2025";"BE001";"12,50";"BE002";"BICXXX";"Sender";"";"SCT";"Synthetic; payment";"112,50"`,
			sourceRow:   2,
			description: "Synthetic; payment",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseFile("triodos.csv", []byte(test.data))
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Bank != domain.BankTriodos || len(parsed.Transactions) != 1 {
				t.Fatalf("parsed file = %#v, want one Triodos transaction", parsed)
			}
			tx := parsed.Transactions[0]
			if tx.SourceRow != test.sourceRow || tx.Description != test.description ||
				tx.Amount != "12.5" || parsed.Account.Number != "BE001" {
				t.Fatalf("parsed transaction/account = %#v / %#v", tx, parsed.Account)
			}
		})
	}
}

func TestTriodosUsesLatestDatedBalance(t *testing.T) {
	data := `"12-03-2025","BE001","12,50","BE002","BICXXX","Sender","","SCT","Later","112,50"
"10-03-2025","BE001","5,00","BE002","BICXXX","Sender","","SCT","Earlier","105,00"`
	parsed, err := ParseFile("triodos.csv", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	wantDate := time.Date(2025, time.March, 12, 0, 0, 0, 0, time.UTC)
	if parsed.Account.Balance == nil || *parsed.Account.Balance != "112.5" ||
		parsed.Account.BalanceAsOf == nil || !parsed.Account.BalanceAsOf.Equal(wantDate) {
		t.Fatalf("unexpected latest Triodos balance: %#v", parsed.Account)
	}
}

func TestParseINGAndReportBadRows(t *testing.T) {
	data := `Rekeningnummer;Naam van de rekening;Rekening tegenpartij;Omzetnummer;Boekingsdatum;Valutadatum;Bedrag;Munteenheid;Omschrijving;Detail van de omzet;Bericht
BE100;Synthetic Account;BE200;TX-1;13/03/2025;14/03/2025;-10,25;EUR;Card;Shop;Synthetic detail
BE100;Synthetic Account;BE200;SECRET-TX;bad-date;14/03/2025;5,00;EUR;Hidden description;;`
	parsed, err := ParseFile("ing.csv", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Bank != domain.BankING || parsed.Account.Number != "BE100" ||
		parsed.Account.Name != "Synthetic Account" {
		t.Fatalf("unexpected ING account: %#v", parsed.Account)
	}
	if len(parsed.Transactions) != 1 || len(parsed.Errors) != 1 {
		t.Fatalf("got %d transactions and %d errors, want 1 each", len(parsed.Transactions), len(parsed.Errors))
	}
	tx := parsed.Transactions[0]
	if tx.Amount != "-10.25" || tx.Direction != domain.DirectionOutgoing ||
		tx.Currency != "EUR" || tx.Reference != "TX-1" ||
		tx.Description != "Card | Shop | Synthetic detail" || tx.ValueDate == nil {
		t.Fatalf("unexpected ING transaction: %#v", tx)
	}
	if parsed.Errors[0].Row != 3 || !strings.Contains(parsed.Errors[0].Message, "booking date") ||
		strings.Contains(parsed.Errors[0].Message, "SECRET") ||
		strings.Contains(parsed.Errors[0].Message, "Hidden") {
		t.Fatalf("unexpected or sensitive row error: %#v", parsed.Errors[0])
	}
}

func TestParseVDK(t *testing.T) {
	data := `"Rekeningnummer";"BE300"
"Naam";"Synthetic VDK Account"
"Type";"Current"
"Saldo";"1.234,50"
"Datum saldo";"15/03/2025 09:30:00"
"Filter actief";"Ja"

"Uitvoeringsdatum";"Valutadatum";"Jaar";"Nummer";"VDK referentie";"Type verrichting";"Rekening tegenpartij";"BIC";"Naam tegenpartij";"Adres";"Postcode";"Stad";"Land";"Mededeling";"Bedrag";"Saldo na beweging";"Wisselkoers";"Kosten"
"16/03/2025";"17/03/2025";"2025";"2";"REF-2";"Overschrijving";"BE400";"BICYYY";"Synthetic Recipient";"";"";"";"BE";"Synthetic message";"-25,00";"1.209,50";"1,0000";"0,00"`
	parsed, err := ParseFile("vdk.csv", []byte(data))
	if err != nil {
		t.Fatal(err)
	}
	wantAsOf := time.Date(2025, time.March, 15, 9, 30, 0, 0, time.UTC)
	if parsed.Bank != domain.BankVDK || parsed.Account.Number != "BE300" ||
		parsed.Account.Name != "Synthetic VDK Account" || parsed.Account.Balance == nil ||
		*parsed.Account.Balance != "1234.5" || parsed.Account.BalanceAsOf == nil ||
		!parsed.Account.BalanceAsOf.Equal(wantAsOf) {
		t.Fatalf("unexpected VDK account: %#v", parsed.Account)
	}
	if len(parsed.Transactions) != 1 {
		t.Fatalf("got %d transactions, want 1", len(parsed.Transactions))
	}
	tx := parsed.Transactions[0]
	if tx.Amount != "-25" || tx.Direction != domain.DirectionOutgoing ||
		tx.CounterpartyAccount != "BE400" || tx.CounterpartyName != "Synthetic Recipient" ||
		tx.Reference != "REF-2" || tx.Description != "Overschrijving | Synthetic message" ||
		tx.BalanceAfter == nil || *tx.BalanceAfter != "1209.5" ||
		tx.ExchangeRate == nil || *tx.ExchangeRate != "1" ||
		tx.Fees == nil || *tx.Fees != "0" {
		t.Fatalf("unexpected VDK transaction: %#v", tx)
	}
}

func TestVDKRejectsUnsupportedAccountType(t *testing.T) {
	data := `"Rekeningnummer";"BE300"
"Naam";"Synthetic VDK Account"
"Type";"Savings"
"Saldo";"1.234,50"
"Datum saldo";"15/03/2025 09:30:00"
"Filter actief";"Ja"

"Uitvoeringsdatum";"Valutadatum";"Jaar";"Nummer";"VDK referentie";"Type verrichting";"Rekening tegenpartij";"BIC";"Naam tegenpartij";"Adres";"Postcode";"Stad";"Land";"Mededeling";"Bedrag";"Saldo na beweging";"Wisselkoers";"Kosten"`
	_, err := ParseFile("vdk.csv", []byte(data))
	if !errors.Is(err, ErrUnsupportedAccountType) {
		t.Fatalf("ParseFile() error = %v, want unsupported account type", err)
	}
}

func TestParseUnknownFormat(t *testing.T) {
	_, err := ParseFile("unknown.csv", []byte("one;two\nthree;four\n"))
	if err == nil {
		t.Fatal("expected an error for an unknown format")
	}
}
