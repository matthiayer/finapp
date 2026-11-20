package web

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/matthiayer/finapp/internal/storage"
)

func TestHandlerHealthAndHome(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	handler, err := NewHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("create handler: %v", err)
	}
	routes := handler.Routes()

	health := httptest.NewRecorder()
	routes.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK || strings.TrimSpace(health.Body.String()) != "ok" {
		t.Fatalf("health response = %d %q, want 200 ok", health.Code, health.Body.String())
	}

	home := httptest.NewRecorder()
	routes.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	if home.Code != http.StatusOK || !strings.Contains(home.Body.String(), "<title>Overview · Finapp</title>") ||
		!strings.Contains(home.Body.String(), "No transactions in this period.") {
		t.Fatalf("home response = %d, body %q", home.Code, home.Body.String())
	}
	if home.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("dashboard cache policy = %q, want no-store", home.Header().Get("Cache-Control"))
	}
}

func TestDashboardFilterPresetsAndCustomDates(t *testing.T) {
	now := time.Date(2026, time.May, 15, 17, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		query     url.Values
		wantRange string
		wantStart string
		wantEnd   string
		wantErr   bool
	}{
		{name: "default rolling year", query: url.Values{}, wantRange: "12m", wantStart: "2025-05-15", wantEnd: "2026-05-15"},
		{name: "three months", query: url.Values{"range": {"3m"}}, wantRange: "3m", wantStart: "2026-02-15", wantEnd: "2026-05-15"},
		{name: "six months", query: url.Values{"range": {"6m"}}, wantRange: "6m", wantStart: "2025-11-15", wantEnd: "2026-05-15"},
		{name: "year to date", query: url.Values{"range": {"ytd"}}, wantRange: "ytd", wantStart: "2026-01-01", wantEnd: "2026-05-15"},
		{name: "previous year", query: url.Values{"range": {"previous-year"}}, wantRange: "previous-year", wantStart: "2025-01-01", wantEnd: "2025-12-31"},
		{name: "three years", query: url.Values{"range": {"3y"}}, wantRange: "3y", wantStart: "2023-05-15", wantEnd: "2026-05-15"},
		{name: "five years", query: url.Values{"range": {"5y"}}, wantRange: "5y", wantStart: "2021-05-15", wantEnd: "2026-05-15"},
		{name: "all history", query: url.Values{"range": {"all"}}, wantRange: "all"},
		{name: "custom", query: url.Values{"range": {"custom"}, "start": {"2024-01-02"}, "end": {"2024-12-30"}}, wantRange: "custom", wantStart: "2024-01-02", wantEnd: "2024-12-30"},
		{name: "invalid custom order", query: url.Values{"range": {"custom"}, "start": {"2025-01-02"}, "end": {"2025-01-01"}}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, selected, err := dashboardFilter(test.query, now)
			if (err != nil) != test.wantErr {
				t.Fatalf("dashboardFilter() error = %v, wantErr %t", err, test.wantErr)
			}
			if err != nil {
				return
			}
			gotStart, gotEnd := "", ""
			if !got.Start.IsZero() {
				gotStart = got.Start.Format("2006-01-02")
			}
			if !got.End.IsZero() {
				gotEnd = got.End.Format("2006-01-02")
			}
			if selected != test.wantRange || gotStart != test.wantStart || gotEnd != test.wantEnd {
				t.Fatalf("dashboardFilter() = (%s, %q, %q)", selected,
					gotStart, gotEnd)
			}
		})
	}
}

func TestDashboardRangeClampsEndOfMonth(t *testing.T) {
	now := time.Date(2024, time.May, 31, 17, 0, 0, 0, time.UTC)
	start := monthsBefore(now, 3)
	if got, want := start.Format("2006-01-02"), "2024-02-29"; got != want {
		t.Fatalf("three months before 2024-05-31 = %s, want %s", got, want)
	}
}

func TestDashboardEmbedsCategoryNamesAsSafeJSON(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	category, err := db.Exec(`INSERT INTO categories (name) VALUES (?)`, `</script><script>alert(1)</script>`)
	if err != nil {
		t.Fatal(err)
	}
	categoryID, err := category.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.Exec(`INSERT INTO accounts (bank, account_number, name, currency)
		VALUES ('Test', 'A-1', 'Current', 'EUR')`)
	if err != nil {
		t.Fatal(err)
	}
	accountID, err := account.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	job, err := db.Exec(`INSERT INTO import_jobs
		(source_files_json, file_count, row_count, imported_count, skipped_count)
		VALUES ('["test.csv"]', 1, 1, 1, 0)`)
	if err != nil {
		t.Fatal(err)
	}
	jobID, err := job.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO transactions
		(account_id, booking_date, amount, currency, eur_amount, direction, category_id,
		 import_job_id, source_file_name, source_row)
		VALUES (?, '2026-01-01', '1', 'EUR', '1', 'incoming', ?, ?, 'test.csv', 1)`,
		accountID, categoryID, jobID); err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("dashboard response = %d, body %q", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), `</script><script>alert(1)`) {
		t.Fatal("category name escaped the dashboard JSON script element")
	}
}

func TestChartAssetIsServedLocally(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	handler, err := NewHandler(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response,
		httptest.NewRequest(http.MethodGet, "/static/vendor/chart.umd.min.js", nil))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("Chart.js response = %d, size %d", response.Code, response.Body.Len())
	}
}

func TestAccountBalanceCanBeRecordedWithEffectiveDate(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	account, err := db.Exec(`INSERT INTO accounts (bank, account_number, name, currency)
		VALUES ('Test', 'A-1', 'Current', 'EUR')`)
	if err != nil {
		t.Fatal(err)
	}
	accountID, err := account.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"account_id": {strconv.FormatInt(accountID, 10)},
		"amount":     {"1234.56"},
		"date":       {"2026-03-10"},
	}
	crossOrigin := httptest.NewRequest(http.MethodPost, "/accounts/balance", strings.NewReader(form.Encode()))
	crossOrigin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	crossOrigin.Header.Set("Origin", "http://attacker.example")
	rejected := httptest.NewRecorder()
	handler.Routes().ServeHTTP(rejected, crossOrigin)
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("cross-origin balance update response = %d, want forbidden", rejected.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/accounts/balance", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/?balance=updated" {
		t.Fatalf("balance update response = %d, location %q", response.Code, response.Header().Get("Location"))
	}
	var amount, date string
	if err := db.QueryRow(`SELECT balance_anchor_amount, date(balance_anchor_date)
		FROM accounts WHERE id = ?`, accountID).Scan(&amount, &date); err != nil {
		t.Fatal(err)
	}
	if amount != "1234.56" || date != "2026-03-10" {
		t.Fatalf("saved balance = %q on %q, want 1234.56 on 2026-03-10", amount, date)
	}
}
