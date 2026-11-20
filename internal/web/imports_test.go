package web

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/matthiayer/finapp/internal/categorization"
	"github.com/matthiayer/finapp/internal/storage"
)

func TestMultiFileImportHTTPFlow(t *testing.T) {
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

	body, contentType := multiBankMultipart(t)
	request := httptest.NewRequest(http.MethodPost, "/imports/preview", body)
	request.Header.Set("Content-Type", contentType)
	preview := httptest.NewRecorder()
	routes.ServeHTTP(preview, request)
	if preview.Code != http.StatusOK || !strings.Contains(preview.Body.String(), "Review import") {
		t.Fatalf("preview response = %d %q", preview.Code, preview.Body.String())
	}
	tokenMatch := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(preview.Body.String())
	if len(tokenMatch) != 2 {
		t.Fatal("preview response did not contain a confirmation token")
	}

	form := url.Values{
		"token":              {tokenMatch[1]},
		"account_confirm[0]": {"yes"},
		"account_confirm[1]": {"yes"},
	}
	confirmRequest := httptest.NewRequest(
		http.MethodPost, "/imports/confirm", strings.NewReader(form.Encode()),
	)
	confirmRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	confirm := httptest.NewRecorder()
	routes.ServeHTTP(confirm, confirmRequest)
	if confirm.Code != http.StatusSeeOther || !strings.HasPrefix(confirm.Header().Get("Location"), "/imports/result?job=") {
		t.Fatalf("confirm response = %d, location %q, body %q",
			confirm.Code, confirm.Header().Get("Location"), confirm.Body.String())
	}

	result := httptest.NewRecorder()
	routes.ServeHTTP(result, httptest.NewRequest(http.MethodGet, confirm.Header().Get("Location"), nil))
	if result.Code != http.StatusOK || !strings.Contains(result.Body.String(), "Import complete") {
		t.Fatalf("result response = %d %q", result.Code, result.Body.String())
	}

	transactions := httptest.NewRecorder()
	routes.ServeHTTP(transactions, httptest.NewRequest(http.MethodGet, "/transactions", nil))
	if transactions.Code != http.StatusOK || !strings.Contains(transactions.Body.String(), "Synthetic payment") {
		t.Fatalf("transactions response = %d %q", transactions.Code, transactions.Body.String())
	}
	if !strings.Contains(transactions.Body.String(), "Synthetic detail") {
		t.Fatal("job transaction list did not include the second bank file")
	}

	imports := httptest.NewRecorder()
	routes.ServeHTTP(imports, httptest.NewRequest(http.MethodGet, "/imports", nil))
	if imports.Code != http.StatusOK || !strings.Contains(imports.Body.String(), "Recent imports") ||
		!strings.Contains(imports.Body.String(), "triodos.csv") ||
		!strings.Contains(imports.Body.String(), "ing.csv") {
		t.Fatalf("import history response = %d %q", imports.Code, imports.Body.String())
	}
}

func TestConflictingRulesRequireAChoiceInImportPreview(t *testing.T) {
	db, err := storage.Open(context.Background(), filepath.Join(t.TempDir(), "finapp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	categoryIDs := make([]int64, 2)
	for i, name := range []string{"Salary", "Other income"} {
		result, err := db.Exec(`INSERT INTO categories (name) VALUES (?)`, name)
		if err != nil {
			t.Fatal(err)
		}
		categoryIDs[i], err = result.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := categorization.Store(context.Background(), db, categorization.Rule{
			Name: name + " terms", CategoryID: categoryIDs[i], Category: name,
			Combine: "all", Enabled: true,
			Conditions: []categorization.Condition{
				{Field: "direction", Operator: "equals", Value: "incoming"},
				{Field: "description", Operator: "contains_any", Values: []string{"synthetic payment"}},
			},
		}); err != nil {
			t.Fatal(err)
		}
	}
	handler, err := NewHandler(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	routes := handler.Routes()
	body, contentType := singleTriodosMultipart(t)
	request := httptest.NewRequest(http.MethodPost, "/imports/preview", body)
	request.Header.Set("Content-Type", contentType)
	preview := httptest.NewRecorder()
	routes.ServeHTTP(preview, request)
	if preview.Code != http.StatusOK ||
		!strings.Contains(preview.Body.String(), "Conflicting rule categories") ||
		!strings.Contains(preview.Body.String(), "Category totals") {
		t.Fatalf("conflicting preview = %d %q", preview.Code, preview.Body.String())
	}
	token := regexp.MustCompile(`name="token" value="([^"]+)"`).FindStringSubmatch(preview.Body.String())
	if len(token) != 2 {
		t.Fatal("preview has no token")
	}

	form := url.Values{
		"token":              {token[1]},
		"account_confirm[0]": {"yes"},
	}
	confirm := httptest.NewRequest(http.MethodPost, "/imports/confirm", strings.NewReader(form.Encode()))
	confirm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rejected := httptest.NewRecorder()
	routes.ServeHTTP(rejected, confirm)
	if rejected.Code != http.StatusOK || !strings.Contains(rejected.Body.String(), "choose a category") {
		t.Fatalf("confirmation without category choice = %d %q", rejected.Code, rejected.Body.String())
	}

	form.Set("category[row-0]", strconv.FormatInt(categoryIDs[1], 10))
	confirm = httptest.NewRequest(http.MethodPost, "/imports/confirm", strings.NewReader(form.Encode()))
	confirm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	accepted := httptest.NewRecorder()
	routes.ServeHTTP(accepted, confirm)
	if accepted.Code != http.StatusSeeOther {
		t.Fatalf("confirmation with category choice = %d %q", accepted.Code, accepted.Body.String())
	}
	var savedCategory int64
	if err := db.QueryRow("SELECT category_id FROM transactions").Scan(&savedCategory); err != nil {
		t.Fatal(err)
	}
	if savedCategory != categoryIDs[1] {
		t.Fatalf("saved category = %d, want %d", savedCategory, categoryIDs[1])
	}
}

func multiBankMultipart(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	files := []struct {
		name string
		data string
	}{
		{
			name: "triodos.csv",
			data: `"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","Synthetic payment","112,50"`,
		},
		{
			name: "ing.csv",
			data: "Rekeningnummer;Naam van de rekening;Rekening tegenpartij;Omzetnummer;Boekingsdatum;Valutadatum;Bedrag;Munteenheid;Omschrijving;Detail van de omzet;Bericht\n" +
				"BE100;Synthetic Account;BE200;TX-1;13/03/2025;14/03/2025;-10,25;EUR;Card;Shop;Synthetic detail",
		},
	}

	for _, file := range files {
		part, err := writer.CreateFormFile("files", file.name)
		if err != nil {
			t.Fatalf("create multipart file: %v", err)
		}
		if _, err := io.WriteString(part, file.data); err != nil {
			t.Fatalf("write multipart file: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func singleTriodosMultipart(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "triodos.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part,
		`"12-03-2025","BE001","12,50","BE002","BICXXX","Synthetic Sender","","SCT","Synthetic payment","112,50"`); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return &body, writer.FormDataContentType()
}
