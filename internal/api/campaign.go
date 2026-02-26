package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"github.com/pulak-ranjan/sampmail/internal/validation"
)

type CampaignHandler struct {
	Store   *store.Store
	Service *core.CampaignService
}

func NewCampaignHandler(st *store.Store) *CampaignHandler {
	return &CampaignHandler{
		Store:   st,
		Service: core.NewCampaignService(st),
	}
}

// Routes registers the campaign API routes
func (h *CampaignHandler) Routes(r chi.Router) {
	r.Get("/", h.listCampaigns)
	r.Get("/stats", h.getCampaignStats)
	r.Post("/", h.createCampaign)
	r.Post("/{id}/import", h.importRecipients)
	r.Post("/{id}/send", h.startCampaign)
	r.Get("/{id}", h.getCampaign)
}

func (h *CampaignHandler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	admin := getAdminFromContext(r.Context())

	q := h.Store.DB.Order("created_at desc")
	if org != nil && org.ID > 0 {
		q = q.Where("organization_id = ?", org.ID)
	} else if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "organization context required"})
		return
	}

	var campaigns []models.Campaign
	if err := q.Find(&campaigns).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, campaigns)
}

func (h *CampaignHandler) createCampaign(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	admin := getAdminFromContext(r.Context())

	var organizationID uint
	if org != nil && org.ID > 0 {
		organizationID = org.ID
	} else if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name     string `json:"name"`
		Subject  string `json:"subject"`
		Body     string `json:"body"`
		SenderID uint   `json:"sender_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	v := validation.New()
	v.Required("name", req.Name).MaxLength("name", req.Name, 200)
	v.Required("subject", req.Subject).MaxLength("subject", req.Subject, 500).NoScriptTags("subject", req.Subject)
	v.Required("body", req.Body).NoScriptTags("body", req.Body)

	if req.SenderID == 0 {
		v.AddError("sender_id", "is required")
	}

	if !v.Valid() {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"errors": v.Errors()})
		return
	}

	// Verify sender exists and user has access to it
	var sender models.Sender
	if err := h.Store.DB.Preload("Domain").First(&sender, req.SenderID).Error; err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sender not found"})
		return
	}

	// Non-superadmin: verify sender belongs to same org
	if organizationID > 0 && sender.OrganizationID != organizationID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "sender not accessible"})
		return
	}

	campaign := models.Campaign{
		Name:           req.Name,
		Subject:        req.Subject,
		Body:           req.Body,
		SenderID:       req.SenderID,
		Status:         "draft",
		OrganizationID: organizationID,
	}

	if err := h.Store.DB.Create(&campaign).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create campaign"})
		return
	}

	writeJSON(w, http.StatusCreated, campaign)
}

func (h *CampaignHandler) importRecipients(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	// Verify campaign ownership
	if orgID > 0 {
		var campaign models.Campaign
		if err := h.Store.DB.First(&campaign, id).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
			return
		}
		if campaign.OrganizationID != orgID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Max 10MB CSV
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too big"})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file"})
		return
	}
	defer file.Close()

	// Use Service to process CSV
	if err := h.Service.ImportRecipientsFromCSV(uint(id), file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("import failed: %v", err)})
		return
	}

	// Count recipients
	var count int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ?", id).Count(&count)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "imported",
		"count":  count,
	})
}

func (h *CampaignHandler) startCampaign(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	// Verify campaign ownership
	if orgID > 0 {
		var campaign models.Campaign
		if err := h.Store.DB.First(&campaign, id).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
			return
		}
		if campaign.OrganizationID != orgID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	if err := h.Service.StartCampaign(uint(id)); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *CampaignHandler) getCampaign(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	var campaign models.Campaign
	if err := h.Store.DB.Preload("Recipients").First(&campaign, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if orgID > 0 && campaign.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

func (h *CampaignHandler) getCampaignStats(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	admin := getAdminFromContext(r.Context())

	var orgID uint
	if org != nil && org.ID > 0 {
		orgID = org.ID
	} else if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "organization context required"})
		return
	}

	var stats struct {
		TotalCampaigns     int64 `json:"total_campaigns"`
		ActiveCampaigns    int64 `json:"active_campaigns"`
		DraftCampaigns     int64 `json:"draft_campaigns"`
		CompletedCampaigns int64 `json:"completed_campaigns"`
		ScheduledCampaigns int64 `json:"scheduled_campaigns"`
		PausedCampaigns    int64 `json:"paused_campaigns"`
		TotalSent          int64 `json:"total_sent"`
		TotalOpen          int64 `json:"total_open"`
		TotalClick         int64 `json:"total_click"`
		TotalBounce        int64 `json:"total_bounce"`
	}

	base := h.Store.DB.Model(&models.Campaign{}).Scopes(store.WithOrgFilterScope(orgID))
	base.Count(&stats.TotalCampaigns)
	base.Where("status = ?", "sending").Count(&stats.ActiveCampaigns)
	base.Where("status = ?", "draft").Count(&stats.DraftCampaigns)
	base.Where("status = ?", "completed").Count(&stats.CompletedCampaigns)
	base.Where("status = ?", "scheduled").Count(&stats.ScheduledCampaigns)
	base.Where("status = ?", "paused").Count(&stats.PausedCampaigns)

	h.Store.DB.Model(&models.CampaignRecipient{}).Count(&stats.TotalSent)

	writeJSON(w, http.StatusOK, stats)
}
