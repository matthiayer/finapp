package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/matthiayer/finapp/internal/domain"
	"github.com/matthiayer/finapp/internal/importflow"
)

type previewPageData struct {
	Preview    importflow.Preview
	FormError  string
	Actions    map[string]string
	Categories map[string]string
	Accounts   map[int]bool
	Manual     map[string]string
	Directions map[string]string
	Rates      map[string]string
}

type importResultData struct {
	JobID         int64
	CreatedAt     string
	Files         []string
	ImportedCount int
	SkippedCount  int
	RowCount      int
}

type importLandingData struct {
	Jobs []importJobSummary
}

type importJobSummary struct {
	JobID         int64
	CreatedAt     string
	Files         []string
	ImportedCount int
	SkippedCount  int
	DateFrom      string
	DateTo        string
}

func (h *Handler) importForm(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.QueryContext(r.Context(), `
		SELECT j.id, j.created_at, j.source_files_json, j.imported_count, j.skipped_count,
			COALESCE(MIN(t.booking_date), ''), COALESCE(MAX(t.booking_date), '')
		FROM import_jobs j
		LEFT JOIN transactions t ON t.import_job_id = j.id
		GROUP BY j.id
		ORDER BY j.id DESC
		LIMIT 20`)
	if err != nil {
		h.logger.Error("load recent import jobs", "error", err)
		http.Error(w, "unable to load import history", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	data := importLandingData{}
	for rows.Next() {
		var job importJobSummary
		var sourceFiles string
		if err := rows.Scan(&job.JobID, &job.CreatedAt, &sourceFiles, &job.ImportedCount,
			&job.SkippedCount, &job.DateFrom, &job.DateTo); err != nil {
			h.logger.Error("read recent import job", "error", err)
			http.Error(w, "unable to load import history", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal([]byte(sourceFiles), &job.Files); err != nil {
			h.logger.Error("decode recent import filenames", "job_id", job.JobID, "error", err)
			http.Error(w, "unable to load import history", http.StatusInternalServerError)
			return
		}
		data.Jobs = append(data.Jobs, job)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("iterate recent import jobs", "error", err)
		http.Error(w, "unable to load import history", http.StatusInternalServerError)
		return
	}
	h.render(w, "upload", data)
}

func (h *Handler) importPreview(w http.ResponseWriter, r *http.Request) {
	if !verifyRequestOrigin(w, r) {
		return
	}
	uploads, err := readUploads(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	preview, err := h.imports.Preview(r.Context(), uploads)
	if err != nil {
		h.logger.Error("build import preview", "error", err)
		http.Error(w, "unable to prepare the import", http.StatusInternalServerError)
		return
	}
	h.render(w, "preview", previewPageData{Preview: preview})
}

func (h *Handler) importConfirm(w http.ResponseWriter, r *http.Request) {
	if !verifyRequestOrigin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid import confirmation form", http.StatusBadRequest)
		return
	}
	token := r.FormValue("token")
	preview, available := h.imports.GetPreview(token)
	if !available {
		http.Error(w, "import preview expired; select the files again", http.StatusBadRequest)
		return
	}

	page := previewPageData{
		Preview:    preview,
		Actions:    make(map[string]string),
		Categories: make(map[string]string),
		Accounts:   make(map[int]bool),
		Manual:     make(map[string]string),
		Directions: make(map[string]string),
		Rates:      make(map[string]string),
	}
	decisions := make(map[string]importflow.Decision, len(preview.Rows))
	for _, row := range preview.Rows {
		action := r.FormValue("action[" + row.Key + "]")
		manual := r.FormValue("manual[" + row.Key + "]")
		direction := domain.Direction(r.FormValue("direction[" + row.Key + "]"))
		categoryID := int64(0)
		if value := r.FormValue("category[" + row.Key + "]"); value != "" {
			parsedCategoryID, err := strconv.ParseInt(value, 10, 64)
			if err != nil || parsedCategoryID < 1 {
				page.FormError = "Choose one of the proposed categories for each conflicting row."
			} else {
				categoryID = parsedCategoryID
			}
		}
		page.Actions[row.Key] = action
		page.Categories[row.Key] = strconv.FormatInt(categoryID, 10)
		page.Manual[row.Key] = manual
		page.Directions[row.Key] = string(direction)
		decisions[row.Key] = importflow.Decision{
			Action: action, ManualResolution: manual, Direction: direction, CategoryID: categoryID,
		}
	}
	rates := make(map[string]string, len(preview.RateRequirements))
	for _, requirement := range preview.RateRequirements {
		value := r.FormValue("rate[" + requirement.Key + "]")
		page.Rates[requirement.Key] = value
		rates[requirement.Key] = value
	}
	if page.FormError != "" {
		h.render(w, "preview", page)
		return
	}
	for i := range preview.Files {
		confirmed := r.FormValue(fmt.Sprintf("account_confirm[%d]", i)) == "yes"
		page.Accounts[i] = confirmed
		if !confirmed {
			page.FormError = "Confirm the proposed account for each selected file."
		}
	}
	if page.FormError != "" {
		h.render(w, "preview", page)
		return
	}

	result, err := h.imports.Confirm(r.Context(), token, decisions, rates)
	if err != nil {
		page.FormError = err.Error()
		h.render(w, "preview", page)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/imports/result?job=%d", result.JobID), http.StatusSeeOther)
}

func (h *Handler) importResult(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.URL.Query().Get("job"), 10, 64)
	if err != nil || jobID < 1 {
		http.Error(w, "invalid import job", http.StatusBadRequest)
		return
	}
	var data importResultData
	var sourceFiles string
	err = h.db.QueryRowContext(r.Context(), `
		SELECT id, created_at, source_files_json, imported_count, skipped_count, row_count
		FROM import_jobs WHERE id = ?`, jobID,
	).Scan(
		&data.JobID, &data.CreatedAt, &sourceFiles, &data.ImportedCount,
		&data.SkippedCount, &data.RowCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.logger.Error("load import job result", "job_id", jobID, "error", err)
		http.Error(w, "unable to load import result", http.StatusInternalServerError)
		return
	}
	if err := json.Unmarshal([]byte(sourceFiles), &data.Files); err != nil {
		h.logger.Error("decode import job filenames", "job_id", jobID, "error", err)
		http.Error(w, "unable to load import result", http.StatusInternalServerError)
		return
	}
	h.render(w, "result", data)
}

func readUploads(r *http.Request) ([]importflow.Upload, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, errors.New("submit CSV files using the upload form")
	}
	var uploads []importflow.Upload
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("unable to read uploaded files")
		}
		if part.FormName() != "files" || part.FileName() == "" {
			_ = part.Close()
			return nil, errors.New("unexpected upload form field")
		}
		data, readErr := io.ReadAll(io.LimitReader(part, importflow.MaxFileSize+1))
		closeErr := part.Close()
		if readErr != nil {
			return nil, errors.New("unable to read uploaded file")
		}
		if closeErr != nil {
			return nil, errors.New("unable to finish reading uploaded file")
		}
		uploads = append(uploads, importflow.Upload{Name: part.FileName(), Data: data})
	}
	for _, upload := range uploads {
		if len(upload.Data) > importflow.MaxFileSize {
			return uploads, nil
		}
	}
	if len(uploads) == 0 {
		return nil, errors.New("select at least one CSV file")
	}
	return uploads, nil
}

func (h *Handler) transactions(w http.ResponseWriter, r *http.Request) {
	var jobID int64
	if job := r.URL.Query().Get("job"); job != "" {
		parsed, err := strconv.ParseInt(job, 10, 64)
		if err != nil || parsed < 1 {
			http.Error(w, "invalid import job", http.StatusBadRequest)
			return
		}
		jobID = parsed
	}

	query := `
		SELECT t.booking_date, t.amount, t.currency, t.eur_amount, t.description,
			t.payee, a.bank, a.account_number, c.name
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id`
	args := []any{}
	if jobID > 0 {
		query += ` JOIN transaction_imports ti ON ti.transaction_id = t.id
			WHERE ti.import_job_id = ?`
		args = append(args, jobID)
	}
	query += ` ORDER BY t.booking_date DESC, t.id DESC LIMIT 100`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		h.logger.Error("load transactions", "job_id", jobID, "error", err)
		http.Error(w, "unable to load transactions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	data := struct {
		JobID        int64
		Transactions []transactionRow
	}{JobID: jobID}
	for rows.Next() {
		var row transactionRow
		var eurAmount, payee sql.NullString
		if err := rows.Scan(
			&row.BookingDate, &row.Amount, &row.Currency, &eurAmount,
			&row.Description, &payee, &row.Bank, &row.AccountNumber, &row.Category,
		); err != nil {
			h.logger.Error("read transaction", "error", err)
			http.Error(w, "unable to load transactions", http.StatusInternalServerError)
			return
		}
		if eurAmount.Valid {
			row.EURAmount = eurAmount.String
		}
		if payee.Valid {
			row.Payee = payee.String
		}
		data.Transactions = append(data.Transactions, row)
	}
	if err := rows.Err(); err != nil {
		h.logger.Error("iterate transactions", "error", err)
		http.Error(w, "unable to load transactions", http.StatusInternalServerError)
		return
	}
	h.render(w, "transactions", data)
}

type transactionRow struct {
	BookingDate   string
	Amount        string
	Currency      string
	EURAmount     string
	Description   string
	Payee         string
	Bank          string
	AccountNumber string
	Category      string
}
