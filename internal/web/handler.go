package web

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/matthiayer/finapp/internal/dashboard"
	"github.com/matthiayer/finapp/internal/importflow"
)

//go:embed templates/*.html static/*
var files embed.FS

type Handler struct {
	db         *sql.DB
	logger     *slog.Logger
	imports    *importflow.Service
	templates  *template.Template
	staticFile http.Handler
}

func NewHandler(db *sql.DB, logger *slog.Logger) (*Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	templates, err := template.ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse embedded HTML templates: %w", err)
	}
	staticFS, err := fs.Sub(files, "static")
	if err != nil {
		return nil, fmt.Errorf("open embedded static files: %w", err)
	}
	return &Handler{
		db:         db,
		logger:     logger,
		imports:    importflow.NewService(db),
		templates:  templates,
		staticFile: http.FileServer(http.FS(staticFS)),
	}, nil
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /", h.home)
	mux.HandleFunc("GET /imports", h.importForm)
	mux.HandleFunc("GET /settings/starter-set", h.starterSet)
	mux.HandleFunc("POST /settings/starter-set/preview", h.starterSetPreview)
	mux.HandleFunc("POST /settings/starter-set/apply", h.starterSetApply)
	mux.HandleFunc("POST /imports/preview", h.importPreview)
	mux.HandleFunc("POST /imports/confirm", h.importConfirm)
	mux.HandleFunc("GET /imports/result", h.importResult)
	mux.HandleFunc("GET /transactions", h.transactions)
	mux.HandleFunc("POST /accounts/balance", h.updateBalance)
	mux.Handle("GET /static/", http.StripPrefix("/static/", h.staticFile))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	if err := h.db.PingContext(r.Context()); err != nil {
		h.logger.Error("database health check failed", "error", err)
		http.Error(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	filter, selectedRange, err := dashboardFilter(r.URL.Query(), time.Now().UTC())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	view, err := dashboard.Load(r.Context(), h.db, filter)
	if err != nil {
		h.logger.Error("load dashboard", "error", err)
		if filter.CategoryID != 0 {
			http.Error(w, "selected category is not available", http.StatusBadRequest)
			return
		}
		http.Error(w, "unable to load dashboard", http.StatusInternalServerError)
		return
	}
	chartData, err := dashboardChartData(view)
	if err != nil {
		h.logger.Error("encode dashboard charts", "error", err)
		http.Error(w, "unable to load dashboard", http.StatusInternalServerError)
		return
	}
	hasKnownBalance := false
	for _, account := range view.Accounts {
		hasKnownBalance = hasKnownBalance || account.Known
	}
	h.render(w, "dashboard", dashboardPageData{
		View: view, Range: selectedRange, ChartData: template.JS(chartData),
		HasTransactions: view.HasActivity,
		HasBalance:      len(view.BalanceHistory) > 0,
		HasKnownBalance: hasKnownBalance,
		BalanceUpdated:  r.URL.Query().Get("balance") == "updated",
		BalanceFormDate: time.Now().UTC().Format("2006-01-02"),
	})
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("render HTML template", "template", name, "error", err)
		http.Error(w, "unable to render page", http.StatusInternalServerError)
	}
}

type dashboardPageData struct {
	View            dashboard.View
	Range           string
	ChartData       template.JS
	HasTransactions bool
	HasBalance      bool
	HasKnownBalance bool
	BalanceUpdated  bool
	BalanceFormDate string
}

var accountBalancePattern = regexp.MustCompile(`^[+-]?\d+(?:\.\d{1,2})?$`)

func (h *Handler) updateBalance(w http.ResponseWriter, r *http.Request) {
	if !verifyRequestOrigin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid balance form", http.StatusBadRequest)
		return
	}
	accountID, err := strconv.ParseInt(r.FormValue("account_id"), 10, 64)
	if err != nil || accountID < 1 {
		http.Error(w, "invalid account", http.StatusBadRequest)
		return
	}
	amount := strings.TrimSpace(r.FormValue("amount"))
	if !accountBalancePattern.MatchString(amount) {
		http.Error(w, "enter a balance with no more than two decimal places", http.StatusBadRequest)
		return
	}
	date, err := time.Parse("2006-01-02", r.FormValue("date"))
	if err != nil {
		http.Error(w, "enter a valid effective date", http.StatusBadRequest)
		return
	}
	result, err := h.db.ExecContext(r.Context(), `
		UPDATE accounts SET balance_anchor_amount = ?, balance_anchor_date = ?
		WHERE id = ?`, amount, date.Format("2006-01-02"), accountID)
	if err != nil {
		h.logger.Error("update account balance", "account_id", accountID, "error", err)
		http.Error(w, "unable to update account balance", http.StatusInternalServerError)
		return
	}
	updated, err := result.RowsAffected()
	if err != nil {
		h.logger.Error("check account balance update", "account_id", accountID, "error", err)
		http.Error(w, "unable to update account balance", http.StatusInternalServerError)
		return
	}
	if updated != 1 {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, "/?balance=updated", http.StatusSeeOther)
}

func verifyRequestOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = r.Referer()
		if origin == "" {
			return true
		}
	}
	parsed, err := url.Parse(origin)
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	if err != nil || parsed.Host == "" ||
		!strings.EqualFold(parsed.Host, r.Host) || parsed.Scheme != expectedScheme {
		http.Error(w, "cross-origin request rejected", http.StatusForbidden)
		return false
	}
	return true
}

func dashboardFilter(query url.Values, now time.Time) (dashboard.Filter, string, error) {
	selectedRange := query.Get("range")
	if selectedRange == "" {
		selectedRange = "12m"
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	filter := dashboard.Filter{End: today}
	switch selectedRange {
	case "3m":
		filter.Start = monthsBefore(today, 3)
	case "6m":
		filter.Start = monthsBefore(today, 6)
	case "12m":
		filter.Start = monthsBefore(today, 12)
	case "ytd":
		filter.Start = time.Date(today.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	case "previous-year":
		filter.Start = time.Date(today.Year()-1, time.January, 1, 0, 0, 0, 0, time.UTC)
		filter.End = time.Date(today.Year()-1, time.December, 31, 0, 0, 0, 0, time.UTC)
	case "3y":
		filter.Start = monthsBefore(today, 36)
	case "5y":
		filter.Start = monthsBefore(today, 60)
	case "all":
		filter.Start = time.Time{}
		filter.End = time.Time{}
	case "custom":
		start, err := time.Parse("2006-01-02", query.Get("start"))
		if err != nil {
			return dashboard.Filter{}, "", fmt.Errorf("enter a valid custom start date")
		}
		end, err := time.Parse("2006-01-02", query.Get("end"))
		if err != nil {
			return dashboard.Filter{}, "", fmt.Errorf("enter a valid custom end date")
		}
		if start.After(end) {
			return dashboard.Filter{}, "", fmt.Errorf("custom start date must not be after end date")
		}
		filter.Start, filter.End = start, end
	default:
		return dashboard.Filter{}, "", fmt.Errorf("unknown dashboard time range")
	}
	if category := query.Get("category"); category != "" {
		categoryID, err := strconv.ParseInt(category, 10, 64)
		if err != nil || categoryID < 0 {
			return dashboard.Filter{}, "", fmt.Errorf("invalid dashboard category")
		}
		filter.CategoryID = categoryID
	}
	return filter, selectedRange, nil
}

func monthsBefore(date time.Time, months int) time.Time {
	targetMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, time.UTC).
		AddDate(0, -months, 0)
	day := min(date.Day(), time.Date(targetMonth.Year(), targetMonth.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
	return time.Date(targetMonth.Year(), targetMonth.Month(), day, 0, 0, 0, 0, time.UTC)
}

func dashboardChartData(view dashboard.View) ([]byte, error) {
	type balanceSeries struct {
		Label  string   `json:"label"`
		Values []string `json:"values"`
	}
	type chartData struct {
		Months         []string        `json:"months"`
		Income         []string        `json:"income"`
		Spending       []string        `json:"spending"`
		Saving         []string        `json:"saving"`
		BalanceMonths  []string        `json:"balanceMonths"`
		Balances       []string        `json:"balances"`
		BalanceSeries  []balanceSeries `json:"balanceSeries"`
		SpendingLabels []string        `json:"spendingLabels"`
		SpendingValues []string        `json:"spendingValues"`
		IncomeLabels   []string        `json:"incomeLabels"`
		IncomeValues   []string        `json:"incomeValues"`
	}
	data := chartData{
		Months:         make([]string, 0, len(view.Monthly)),
		Income:         make([]string, 0, len(view.Monthly)),
		Spending:       make([]string, 0, len(view.Monthly)),
		Saving:         make([]string, 0, len(view.Monthly)),
		BalanceMonths:  make([]string, 0, len(view.BalanceHistory)),
		Balances:       make([]string, 0, len(view.BalanceHistory)),
		BalanceSeries:  make([]balanceSeries, 0),
		SpendingLabels: make([]string, 0, len(view.SpendingByCategory)),
		SpendingValues: make([]string, 0, len(view.SpendingByCategory)),
		IncomeLabels:   make([]string, 0, len(view.IncomeByCategory)),
		IncomeValues:   make([]string, 0, len(view.IncomeByCategory)),
	}
	for _, month := range view.Monthly {
		data.Months = append(data.Months, month.Month)
		data.Income = append(data.Income, month.Income)
		data.Spending = append(data.Spending, month.Spending)
		data.Saving = append(data.Saving, month.Saving)
	}
	for _, point := range view.BalanceHistory {
		data.BalanceMonths = append(data.BalanceMonths, point.Month)
		data.Balances = append(data.Balances, point.Balance)
	}
	series := make(map[int64]*balanceSeries)
	seriesOrder := make([]int64, 0)
	for _, point := range view.AccountHistory {
		item := series[point.AccountID]
		if item == nil {
			item = &balanceSeries{Label: point.Label, Values: make([]string, len(view.BalanceHistory))}
			series[point.AccountID] = item
			seriesOrder = append(seriesOrder, point.AccountID)
		}
		for i, month := range view.BalanceHistory {
			if month.Month == point.Month {
				item.Values[i] = point.Balance
				break
			}
		}
	}
	for _, id := range seriesOrder {
		data.BalanceSeries = append(data.BalanceSeries, *series[id])
	}
	for _, total := range view.SpendingByCategory {
		data.SpendingLabels = append(data.SpendingLabels, total.CategoryName)
		data.SpendingValues = append(data.SpendingValues, total.Total)
	}
	for _, total := range view.IncomeByCategory {
		data.IncomeLabels = append(data.IncomeLabels, total.CategoryName)
		data.IncomeValues = append(data.IncomeValues, total.Total)
	}
	return json.Marshal(data)
}
