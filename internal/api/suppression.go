package api

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type SuppressionHandler struct {
	Store *store.Store
}

func NewSuppressionHandler(st *store.Store) *SuppressionHandler {
	return &SuppressionHandler{Store: st}
}

// GET /api/suppressions - List suppressions with pagination
func (h *SuppressionHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	sups, total, err := h.Store.ListSuppressions(orgID, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"suppressions": sups,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
	})
}

// POST /api/suppressions - Add email(s) to suppression list
func (h *SuppressionHandler) HandleAdd(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var req struct {
		Email  string   `json:"email"`
		Emails []string `json:"emails"`
		Reason string   `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	reason := req.Reason
	if reason == "" {
		reason = "manual"
	}

	// Collect emails to add
	emails := req.Emails
	if req.Email != "" {
		emails = append(emails, req.Email)
	}

	if len(emails) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no emails provided"})
		return
	}

	added := 0
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" || !strings.Contains(email, "@") {
			continue
		}
		if err := h.Store.AddSuppression(orgID, email, reason, "manual"); err == nil {
			added++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"added": added,
		"total": len(emails),
	})
}

// DELETE /api/suppressions/{email} - Remove from suppression list
func (h *SuppressionHandler) HandleRemove(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	email := chi.URLParam(r, "email")
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email required"})
		return
	}

	if err := h.Store.RemoveSuppression(orgID, email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// POST /api/suppressions/check - Check if emails are suppressed
func (h *SuppressionHandler) HandleCheck(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var req struct {
		Emails []string `json:"emails"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	result, err := h.Store.BulkCheckSuppressed(orgID, req.Emails)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "check failed"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"suppressed": result,
	})
}

// POST /api/suppressions/import - Import suppressions from CSV
func (h *SuppressionHandler) HandleImport(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	// Max 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file"})
		return
	}
	defer file.Close()

	reason := r.FormValue("reason")
	if reason == "" {
		reason = "import"
	}

	reader := csv.NewReader(file)
	added := 0
	skipped := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(record) < 1 {
			continue
		}

		email := strings.TrimSpace(record[0])
		if email == "" || !strings.Contains(email, "@") {
			skipped++
			continue
		}

		if err := h.Store.AddSuppression(orgID, email, reason, "csv_import"); err == nil {
			added++
		} else {
			skipped++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"added":   added,
		"skipped": skipped,
	})
}

// GET /api/suppressions/export - Export suppressions as CSV
func (h *SuppressionHandler) HandleExport(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	sups, _, err := h.Store.ListSuppressions(orgID, 100000, 0) // Get all
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "export failed"})
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=suppressions.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"email", "reason", "source", "created_at"})

	for _, sup := range sups {
		writer.Write([]string{
			sup.Email,
			sup.Reason,
			sup.Source,
			sup.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	writer.Flush()
}

// GET /api/suppressions/stats - Get suppression statistics
func (h *SuppressionHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var stats struct {
		Total        int64 `json:"total"`
		Bounces      int64 `json:"bounces"`
		Unsubscribes int64 `json:"unsubscribes"`
		Complaints   int64 `json:"complaints"`
		Manual       int64 `json:"manual"`
	}

	base := h.Store.DB.Model(&models.Suppression{}).Scopes(store.WithOrgFilterScope(orgID))
	base.Count(&stats.Total)
	base.Where("reason = ?", "hard_bounce").Count(&stats.Bounces)
	base.Where("reason = ?", "unsubscribe").Count(&stats.Unsubscribes)
	base.Where("reason = ?", "complaint").Count(&stats.Complaints)
	base.Where("reason = ?", "manual").Count(&stats.Manual)

	writeJSON(w, http.StatusOK, stats)
}

func suppressionRoutes(h *SuppressionHandler) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", h.HandleList)
		r.Post("/", h.HandleAdd)
		r.Delete("/{email}", h.HandleRemove)
		r.Post("/check", h.HandleCheck)
		r.Post("/import", h.HandleImport)
		r.Get("/export", h.HandleExport)
		r.Get("/stats", h.HandleStats)
	}
}
