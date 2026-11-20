package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/matthiayer/finapp/internal/categorization"
)

type starterSetPageData struct {
	Categories []starterCategoryChoice
	Rules      []starterRuleChoice
	Error      string
	Applied    bool
}

type starterCategoryChoice struct {
	Name     string
	Exists   bool
	Selected bool
}

type starterRuleChoice struct {
	categorization.StarterRule
	Enabled bool
}

type starterSetPreviewData struct {
	Preview    categorization.StarterPreview
	Categories []string
	Rules      []string
	Choices    map[int64]string
	FormError  string
}

func (h *Handler) starterSet(w http.ResponseWriter, r *http.Request) {
	data, err := h.loadStarterSetPage(r.Context(), nil, nil)
	if err != nil {
		h.logger.Error("load starter set options", "error", err)
		http.Error(w, "unable to load starter categories and rules", http.StatusInternalServerError)
		return
	}
	data.Applied = r.URL.Query().Get("applied") == "1"
	h.render(w, "starter-set", data)
}

func (h *Handler) starterSetPreview(w http.ResponseWriter, r *http.Request) {
	if !verifyRequestOrigin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid starter set form", http.StatusBadRequest)
		return
	}
	categories := r.Form["category"]
	rules := r.Form["rule"]
	if len(categories) == 0 && len(rules) == 0 {
		h.renderStarterSetError(w, r, categories, rules, "Select at least one category or rule.")
		return
	}
	preview, err := categorization.PreviewStarterSelection(r.Context(), h.db, categories, rules)
	if err != nil {
		h.renderStarterSetError(w, r, categories, rules, err.Error())
		return
	}
	h.render(w, "starter-set-preview", starterSetPreviewData{
		Preview: preview, Categories: categories, Rules: rules,
		Choices: make(map[int64]string),
	})
}

func (h *Handler) starterSetApply(w http.ResponseWriter, r *http.Request) {
	if !verifyRequestOrigin(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid starter set confirmation form", http.StatusBadRequest)
		return
	}
	categories := r.Form["category"]
	rules := r.Form["rule"]
	if r.FormValue("confirm") != "yes" {
		h.renderStarterSetError(w, r, categories, rules, "Confirm the selected starter categories and rules to apply them.")
		return
	}
	if len(categories) == 0 && len(rules) == 0 {
		h.renderStarterSetError(w, r, categories, rules, "Select at least one category or rule.")
		return
	}
	preview, err := categorization.PreviewStarterSelection(r.Context(), h.db, categories, rules)
	if err != nil {
		h.renderStarterSetError(w, r, categories, rules, err.Error())
		return
	}
	choices := make(map[int64]string, len(preview.Transactions))
	for _, match := range preview.Transactions {
		if selected := r.FormValue(fmt.Sprintf("transaction_category[%d]", match.ID)); selected != "" {
			choices[match.ID] = selected
		}
	}
	if err := categorization.ApplyStarterSelection(r.Context(), h.db, categories, rules, choices); err != nil {
		data := starterSetPreviewData{
			Preview: preview, Categories: categories, Rules: rules,
			Choices: choices, FormError: err.Error(),
		}
		h.render(w, "starter-set-preview", data)
		return
	}
	http.Redirect(w, r, "/settings/starter-set?applied=1", http.StatusSeeOther)
}

func (h *Handler) renderStarterSetError(w http.ResponseWriter, r *http.Request, categories, rules []string, message string) {
	data, err := h.loadStarterSetPage(r.Context(), categories, rules)
	if err != nil {
		h.logger.Error("reload starter set options", "error", err)
		http.Error(w, "unable to load starter categories and rules", http.StatusInternalServerError)
		return
	}
	data.Error = message
	h.render(w, "starter-set", data)
}

func (h *Handler) loadStarterSetPage(
	ctx context.Context,
	selectedCategories, selectedRules []string,
) (starterSetPageData, error) {
	rows, err := h.db.QueryContext(ctx, "SELECT name FROM categories")
	if err != nil {
		return starterSetPageData{}, err
	}
	existingCategories := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return starterSetPageData{}, err
		}
		existingCategories[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return starterSetPageData{}, err
	}
	if err := rows.Close(); err != nil {
		return starterSetPageData{}, err
	}

	enabledRules, err := categorization.Enabled(ctx, h.db)
	if err != nil {
		return starterSetPageData{}, err
	}
	selectedCategorySet := stringSet(selectedCategories)
	selectedRuleSet := stringSet(selectedRules)
	enabledRuleSet := make(map[string]bool, len(enabledRules))
	for _, rule := range enabledRules {
		enabledRuleSet[strings.ToLower(rule.Name)] = true
	}

	data := starterSetPageData{
		Categories: make([]starterCategoryChoice, 0, len(categorization.StarterCategories())),
		Rules:      make([]starterRuleChoice, 0, len(categorization.StarterRules())),
	}
	for _, name := range categorization.StarterCategories() {
		data.Categories = append(data.Categories, starterCategoryChoice{
			Name: name, Exists: existingCategories[strings.ToLower(name)],
			Selected: selectedCategorySet[strings.ToLower(name)],
		})
	}
	for _, rule := range categorization.StarterRules() {
		key := strings.ToLower(rule.Name)
		data.Rules = append(data.Rules, starterRuleChoice{
			StarterRule: rule,
			Enabled:     selectedRuleSet[key] || (selectedRules == nil && enabledRuleSet[key]),
		})
	}
	return data, nil
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[strings.ToLower(strings.TrimSpace(value))] = true
	}
	return result
}
