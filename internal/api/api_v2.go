package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// =====================================
// TEMPLATE API V2
// =====================================

// TemplateHandlerV2 handles email template operations
type TemplateHandlerV2 struct {
	store         *store.Store
	aiService     *core.AITemplateService
	renderService *core.TemplateRenderService
}

// NewTemplateHandlerV2 creates a new template handler
func NewTemplateHandlerV2(st *store.Store) *TemplateHandlerV2 {
	return &TemplateHandlerV2{
		store:         st,
		aiService:     core.NewAITemplateService(st),
		renderService: core.NewTemplateRenderService(),
	}
}

// ListTemplates returns all templates
func (h *TemplateHandlerV2) ListTemplates(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	query := h.store.DB.Model(&models.EmailTemplate{})
	query = query.Where("organization_id = ?", org.ID)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}

	var templates []models.EmailTemplate
	if err := query.Order("created_at DESC").Find(&templates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, templates)
}

// GetTemplate returns a single template
func (h *TemplateHandlerV2) GetTemplate(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.EmailTemplate
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&template).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	writeJSON(w, http.StatusOK, template)
}

// CreateTemplate creates a new template
func (h *TemplateHandlerV2) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var template models.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	template.ID = 0
	template.OrganizationID = org.ID
	if admin != nil {
		template.CreatedBy = admin.ID
	}

	// Extract variables
	pe := core.NewPersonalizationEngine()
	template.Variables = models.StringArray(pe.ExtractVariables(template.HTMLContent))

	if err := h.store.DB.Create(&template).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, template)
}

// UpdateTemplate updates a template
func (h *TemplateHandlerV2) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.EmailTemplate
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&template).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	var updates models.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	// Extract variables
	pe := core.NewPersonalizationEngine()
	updates.Variables = models.StringArray(pe.ExtractVariables(updates.HTMLContent))
	updates.ID = template.ID
	updates.OrganizationID = template.OrganizationID
	updates.CreatedBy = template.CreatedBy

	if err := h.store.DB.Model(&template).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, template)
}

// DeleteTemplate deletes a template
func (h *TemplateHandlerV2) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).Delete(&models.EmailTemplate{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GenerateWithAI generates a template using AI
func (h *TemplateHandlerV2) GenerateWithAI(w http.ResponseWriter, r *http.Request) {
	var req core.GenerateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Prompt == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt is required"})
		return
	}

	result, err := h.aiService.GenerateTemplate(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// PreviewTemplate renders a template with sample data
func (h *TemplateHandlerV2) PreviewTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HTMLContent string                 `json:"html_content"`
		Subject     string                 `json:"subject"`
		Data        map[string]interface{} `json:"data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	pe := core.NewPersonalizationEngine()

	// Use sample data if not provided
	if req.Data == nil {
		html, _ := pe.PreviewWithSampleData(req.HTMLContent)
		subject, _ := pe.PreviewWithSampleData(req.Subject)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"html":    html,
			"subject": subject,
		})
		return
	}

	ctx := &core.PersonalizationContext{
		CustomData: req.Data,
	}

	html, _ := pe.Render(req.HTMLContent, ctx)
	subject, _ := pe.Render(req.Subject, ctx)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"html":    html,
		"subject": subject,
	})
}

// GetBuiltInTemplates returns the template library
func (h *TemplateHandlerV2) GetBuiltInTemplates(w http.ResponseWriter, r *http.Request) {
	templates := core.GetBuiltInTemplates()
	writeJSON(w, http.StatusOK, templates)
}

// GetBuiltInTemplateContent returns content for a built-in template
func (h *TemplateHandlerV2) GetBuiltInTemplateContent(w http.ResponseWriter, r *http.Request) {
	templateID := chi.URLParam(r, "templateId")

	content, err := core.GetBuiltInTemplateContent(templateID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, content)
}

// GetVariables returns available template variables
func (h *TemplateHandlerV2) GetVariables(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"variables": core.GetAvailableVariables(),
		"filters":   core.GetAvailableFilters(),
	})
}

// =====================================
// AUTOMATION API V2
// =====================================

// AutomationHandlerV2 handles automation workflow operations
type AutomationHandlerV2 struct {
	store  *store.Store
	engine *core.AutomationEngine
}

// NewAutomationHandlerV2 creates a new automation handler
func NewAutomationHandlerV2(st *store.Store) *AutomationHandlerV2 {
	return &AutomationHandlerV2{
		store:  st,
		engine: core.GetAutomationEngine(),
	}
}

// ListAutomations returns all automations
func (h *AutomationHandlerV2) ListAutomations(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())
	status := r.URL.Query().Get("status")

	query := h.store.DB.Model(&models.AutomationV2{})
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var automations []models.AutomationV2
	if err := query.Order("created_at DESC").Find(&automations).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, automations)
}

// GetAutomation returns a single automation with full workflow data
func (h *AutomationHandlerV2) GetAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}

	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	writeJSON(w, http.StatusOK, automation)
}

// CreateAutomation creates a new automation
func (h *AutomationHandlerV2) CreateAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name           string         `json:"name"`
		Description    string         `json:"description"`
		Nodes          string         `json:"nodes"`
		Edges          string         `json:"edges"`
		Viewport       string         `json:"viewport"`
		TriggerType    string         `json:"trigger_type"`
		TriggerConfig  models.JSONMap `json:"trigger_config"`
		EntryFilters   models.JSONMap `json:"entry_filters"`
		ExitConditions models.JSONMap `json:"exit_conditions"`
		AllowReentry   bool           `json:"allow_reentry"`
		ReentryDelay   int            `json:"reentry_delay"`
		GoalTracking   bool           `json:"goal_tracking"`
		GoalConfig     models.JSONMap `json:"goal_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	automation := models.AutomationV2{
		OrganizationID: org.ID,
		Name:           req.Name,
		Description:    req.Description,
		Nodes:          req.Nodes,
		Edges:          req.Edges,
		Viewport:       req.Viewport,
		TriggerType:    req.TriggerType,
		TriggerConfig:  req.TriggerConfig,
		EntryFilters:   req.EntryFilters,
		ExitConditions: req.ExitConditions,
		Status:         "draft",
		AllowReentry:   req.AllowReentry,
		ReentryDelay:   req.ReentryDelay,
		GoalTracking:   req.GoalTracking,
		GoalConfig:     req.GoalConfig,
	}
	if admin != nil {
		automation.CreatedBy = admin.ID
	}

	if err := h.store.DB.Create(&automation).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, automation)
}

// UpdateAutomation updates an automation
func (h *AutomationHandlerV2) UpdateAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}

	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	var req struct {
		Name           *string         `json:"name"`
		Description    *string         `json:"description"`
		Nodes          *string         `json:"nodes"`
		Edges          *string         `json:"edges"`
		Viewport       *string         `json:"viewport"`
		TriggerType    *string         `json:"trigger_type"`
		TriggerConfig  *models.JSONMap `json:"trigger_config"`
		EntryFilters   *models.JSONMap `json:"entry_filters"`
		ExitConditions *models.JSONMap `json:"exit_conditions"`
		AllowReentry   *bool           `json:"allow_reentry"`
		ReentryDelay   *int            `json:"reentry_delay"`
		GoalTracking   *bool           `json:"goal_tracking"`
		GoalConfig     *models.JSONMap `json:"goal_config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Nodes != nil {
		updates["nodes"] = *req.Nodes
	}
	if req.Edges != nil {
		updates["edges"] = *req.Edges
	}
	if req.Viewport != nil {
		updates["viewport"] = *req.Viewport
	}
	if req.TriggerType != nil {
		updates["trigger_type"] = *req.TriggerType
	}
	if req.TriggerConfig != nil {
		updates["trigger_config"] = *req.TriggerConfig
	}
	if req.EntryFilters != nil {
		updates["entry_filters"] = *req.EntryFilters
	}
	if req.ExitConditions != nil {
		updates["exit_conditions"] = *req.ExitConditions
	}
	if req.AllowReentry != nil {
		updates["allow_reentry"] = *req.AllowReentry
	}
	if req.ReentryDelay != nil {
		updates["reentry_delay"] = *req.ReentryDelay
	}
	if req.GoalTracking != nil {
		updates["goal_tracking"] = *req.GoalTracking
	}
	if req.GoalConfig != nil {
		updates["goal_config"] = *req.GoalConfig
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no updates provided"})
		return
	}

	if err := h.store.DB.Model(&automation).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reload automation"})
		return
	}

	// Re-register if active
	if automation.Status == "active" && h.engine != nil {
		h.engine.UnregisterWorkflow(automation.ID)
		h.engine.RegisterWorkflow(&automation)
	}

	writeJSON(w, http.StatusOK, automation)
}

// DeleteAutomation deletes an automation
func (h *AutomationHandlerV2) DeleteAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}
	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	// Unregister if running
	if h.engine != nil {
		h.engine.UnregisterWorkflow(automation.ID)
	}

	if err := h.store.DB.Delete(&automation).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ActivateAutomation activates an automation
func (h *AutomationHandlerV2) ActivateAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}
	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	automation.Status = "active"
	h.store.DB.Save(&automation)

	// Register with engine
	if h.engine != nil {
		h.engine.RegisterWorkflow(&automation)
	}

	writeJSON(w, http.StatusOK, automation)
}

// PauseAutomation pauses an automation
func (h *AutomationHandlerV2) PauseAutomation(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}
	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	automation.Status = "paused"
	h.store.DB.Save(&automation)

	// Unregister from engine
	if h.engine != nil {
		h.engine.UnregisterWorkflow(uint(id))
	}

	writeJSON(w, http.StatusOK, automation)
}

// GetAutomationStats returns statistics for an automation
func (h *AutomationHandlerV2) GetAutomationStats(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}

	var automation models.AutomationV2
	query := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		query = query.Where("organization_id = ?", org.ID)
	} else {
		query = query.Where("organization_id = ?", org.ID)
	}
	if err := query.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	// Get run statistics
	var activeRuns, completedRuns, failedRuns int64
	h.store.DB.Model(&models.AutomationRunV2{}).Where("automation_id = ? AND status = ?", id, "active").Count(&activeRuns)
	h.store.DB.Model(&models.AutomationRunV2{}).Where("automation_id = ? AND status = ?", id, "completed").Count(&completedRuns)
	h.store.DB.Model(&models.AutomationRunV2{}).Where("automation_id = ? AND status = ?", id, "failed").Count(&failedRuns)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_entered":   automation.TotalEntered,
		"total_completed": automation.TotalCompleted,
		"total_active":    automation.TotalActive,
		"conversion_rate": automation.ConversionRate,
		"active_runs":     activeRuns,
		"completed_runs":  completedRuns,
		"failed_runs":     failedRuns,
	})
}

// GetAutomationRuns returns runs for an automation
func (h *AutomationHandlerV2) GetAutomationRuns(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid automation id"})
		return
	}
	status := r.URL.Query().Get("status")

	var automation models.AutomationV2
	automationQuery := h.store.DB.Model(&models.AutomationV2{}).Where("id = ?", id)
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}
	if admin != nil && !admin.IsSuperAdmin {
		automationQuery = automationQuery.Where("organization_id = ?", org.ID)
	} else {
		automationQuery = automationQuery.Where("organization_id = ?", org.ID)
	}
	if err := automationQuery.First(&automation).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	query := h.store.DB.Model(&models.AutomationRunV2{}).Where("automation_id = ?", id)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var runs []models.AutomationRunV2
	if err := query.Order("entered_at DESC").Limit(100).Find(&runs).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, runs)
}

// GetNodeTypes returns available automation node types
func (h *AutomationHandlerV2) GetNodeTypes(w http.ResponseWriter, r *http.Request) {
	nodeTypes := []map[string]interface{}{
		// Triggers
		{"type": "trigger_contact_added", "category": "trigger", "label": "Contact Added", "description": "Triggers when a contact is added to a list"},
		{"type": "trigger_tag_added", "category": "trigger", "label": "Tag Added", "description": "Triggers when a tag is added to a contact"},
		{"type": "trigger_email_opened", "category": "trigger", "label": "Email Opened", "description": "Triggers when an email is opened"},
		{"type": "trigger_link_clicked", "category": "trigger", "label": "Link Clicked", "description": "Triggers when a link is clicked"},
		{"type": "trigger_form_submitted", "category": "trigger", "label": "Form Submitted", "description": "Triggers when a form is submitted"},
		{"type": "trigger_webhook", "category": "trigger", "label": "Webhook", "description": "Triggers from external webhook"},
		{"type": "trigger_schedule", "category": "trigger", "label": "Schedule", "description": "Triggers on a schedule"},

		// Actions
		{"type": "action_send_email", "category": "action", "label": "Send Email", "description": "Send an email to the contact"},
		{"type": "action_add_tag", "category": "action", "label": "Add Tag", "description": "Add a tag to the contact"},
		{"type": "action_remove_tag", "category": "action", "label": "Remove Tag", "description": "Remove a tag from the contact"},
		{"type": "action_add_to_list", "category": "action", "label": "Add to List", "description": "Add contact to a list"},
		{"type": "action_remove_from_list", "category": "action", "label": "Remove from List", "description": "Remove contact from a list"},
		{"type": "action_update_field", "category": "action", "label": "Update Field", "description": "Update a contact field"},
		{"type": "action_webhook", "category": "action", "label": "Call Webhook", "description": "Call an external webhook"},
		{"type": "action_update_score", "category": "action", "label": "Update Score", "description": "Update contact's lead score"},
		{"type": "action_notify", "category": "action", "label": "Send Notification", "description": "Send internal notification"},

		// Flow Control
		{"type": "delay", "category": "flow", "label": "Delay", "description": "Wait for a period of time"},
		{"type": "condition", "category": "flow", "label": "Condition", "description": "Branch based on conditions"},
		{"type": "ab_split", "category": "flow", "label": "A/B Split", "description": "Split traffic for testing"},
		{"type": "goal", "category": "flow", "label": "Goal", "description": "Mark a conversion goal"},
		{"type": "exit", "category": "flow", "label": "Exit", "description": "Exit the automation"},
	}

	writeJSON(w, http.StatusOK, nodeTypes)
}

// =====================================
// CAMPAIGN API V2
// =====================================

// CampaignHandlerV2 handles campaign operations with personalization
type CampaignHandlerV2 struct {
	store         *store.Store
	renderService *core.TemplateRenderService
}

// NewCampaignHandlerV2 creates a new campaign handler
func NewCampaignHandlerV2(st *store.Store) *CampaignHandlerV2 {
	return &CampaignHandlerV2{
		store:         st,
		renderService: core.NewTemplateRenderService(),
	}
}

// ListCampaigns returns all campaigns
func (h *CampaignHandlerV2) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())
	status := r.URL.Query().Get("status")

	query := h.store.DB.Model(&models.CampaignV2{})
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		query = query.Where("organization_id = ?", org.ID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var campaigns []models.CampaignV2
	if err := query.Order("created_at DESC").Find(&campaigns).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, campaigns)
}

// GetCampaign returns a single campaign
func (h *CampaignHandlerV2) GetCampaign(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid campaign id"})
		return
	}

	var campaign models.CampaignV2
	query := h.store.DB.Model(&models.CampaignV2{}).Where("id = ?", id)
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		query = query.Where("organization_id = ?", org.ID)
	}

	if err := query.First(&campaign).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

// CreateCampaign creates a new campaign
func (h *CampaignHandlerV2) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name                 string             `json:"name"`
		Type                 string             `json:"type"`
		Subject              string             `json:"subject"`
		PreviewText          string             `json:"preview_text"`
		TemplateID           uint               `json:"template_id"`
		HTMLContent          string             `json:"html_content"`
		TextContent          string             `json:"text_content"`
		SenderID             uint               `json:"sender_id"`
		ReplyTo              string             `json:"reply_to"`
		ListIDs              models.StringArray `json:"list_ids"`
		SegmentIDs           models.StringArray `json:"segment_ids"`
		ExcludeListIDs       models.StringArray `json:"exclude_list_ids"`
		ABTestEnabled        bool               `json:"ab_test_enabled"`
		ABTestConfig         models.JSONMap     `json:"ab_test_config"`
		SendTimezone         string             `json:"send_timezone"`
		SendTimeOptimization bool               `json:"send_time_optimization"`
		GoogleAnalytics      bool               `json:"google_analytics"`
		UTMParams            models.JSONMap     `json:"utm_params"`
		TrackOpens           *bool              `json:"track_opens"`
		TrackClicks          *bool              `json:"track_clicks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	campaign := models.CampaignV2{
		OrganizationID:       org.ID,
		Name:                 req.Name,
		Type:                 req.Type,
		Subject:              req.Subject,
		PreviewText:          req.PreviewText,
		TemplateID:           req.TemplateID,
		HTMLContent:          req.HTMLContent,
		TextContent:          req.TextContent,
		SenderID:             req.SenderID,
		ReplyTo:              req.ReplyTo,
		ListIDs:              req.ListIDs,
		SegmentIDs:           req.SegmentIDs,
		ExcludeListIDs:       req.ExcludeListIDs,
		ABTestEnabled:        req.ABTestEnabled,
		ABTestConfig:         req.ABTestConfig,
		SendTimezone:         req.SendTimezone,
		SendTimeOptimization: req.SendTimeOptimization,
		GoogleAnalytics:      req.GoogleAnalytics,
		UTMParams:            req.UTMParams,
		Status:               "draft",
		TrackOpens:           true,
		TrackClicks:          true,
	}
	if req.TrackOpens != nil {
		campaign.TrackOpens = *req.TrackOpens
	}
	if req.TrackClicks != nil {
		campaign.TrackClicks = *req.TrackClicks
	}
	if admin != nil {
		campaign.CreatedBy = admin.ID
	}

	if err := h.store.DB.Create(&campaign).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, campaign)
}

// UpdateCampaign updates a campaign
func (h *CampaignHandlerV2) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid campaign id"})
		return
	}

	var campaign models.CampaignV2
	query := h.store.DB.Model(&models.CampaignV2{}).Where("id = ?", id)
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		query = query.Where("organization_id = ?", org.ID)
	}

	if err := query.First(&campaign).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	if campaign.Status != "draft" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "can only edit draft campaigns"})
		return
	}

	var req struct {
		Name                 *string             `json:"name"`
		Type                 *string             `json:"type"`
		Subject              *string             `json:"subject"`
		PreviewText          *string             `json:"preview_text"`
		TemplateID           *uint               `json:"template_id"`
		HTMLContent          *string             `json:"html_content"`
		TextContent          *string             `json:"text_content"`
		SenderID             *uint               `json:"sender_id"`
		ReplyTo              *string             `json:"reply_to"`
		ListIDs              *models.StringArray `json:"list_ids"`
		SegmentIDs           *models.StringArray `json:"segment_ids"`
		ExcludeListIDs       *models.StringArray `json:"exclude_list_ids"`
		ABTestEnabled        *bool               `json:"ab_test_enabled"`
		ABTestConfig         *models.JSONMap     `json:"ab_test_config"`
		SendTimezone         *string             `json:"send_timezone"`
		SendTimeOptimization *bool               `json:"send_time_optimization"`
		GoogleAnalytics      *bool               `json:"google_analytics"`
		UTMParams            *models.JSONMap     `json:"utm_params"`
		TrackOpens           *bool               `json:"track_opens"`
		TrackClicks          *bool               `json:"track_clicks"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Subject != nil {
		updates["subject"] = *req.Subject
	}
	if req.PreviewText != nil {
		updates["preview_text"] = *req.PreviewText
	}
	if req.TemplateID != nil {
		updates["template_id"] = *req.TemplateID
	}
	if req.HTMLContent != nil {
		updates["html_content"] = *req.HTMLContent
	}
	if req.TextContent != nil {
		updates["text_content"] = *req.TextContent
	}
	if req.SenderID != nil {
		updates["sender_id"] = *req.SenderID
	}
	if req.ReplyTo != nil {
		updates["reply_to"] = *req.ReplyTo
	}
	if req.ListIDs != nil {
		updates["list_ids"] = *req.ListIDs
	}
	if req.SegmentIDs != nil {
		updates["segment_ids"] = *req.SegmentIDs
	}
	if req.ExcludeListIDs != nil {
		updates["exclude_list_ids"] = *req.ExcludeListIDs
	}
	if req.ABTestEnabled != nil {
		updates["ab_test_enabled"] = *req.ABTestEnabled
	}
	if req.ABTestConfig != nil {
		updates["ab_test_config"] = *req.ABTestConfig
	}
	if req.SendTimezone != nil {
		updates["send_timezone"] = *req.SendTimezone
	}
	if req.SendTimeOptimization != nil {
		updates["send_time_optimization"] = *req.SendTimeOptimization
	}
	if req.GoogleAnalytics != nil {
		updates["google_analytics"] = *req.GoogleAnalytics
	}
	if req.UTMParams != nil {
		updates["utm_params"] = *req.UTMParams
	}
	if req.TrackOpens != nil {
		updates["track_opens"] = *req.TrackOpens
	}
	if req.TrackClicks != nil {
		updates["track_clicks"] = *req.TrackClicks
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no valid fields to update"})
		return
	}

	if err := h.store.DB.Model(&campaign).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.DB.First(&campaign, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reload campaign"})
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

// DeleteCampaign deletes a campaign
func (h *CampaignHandlerV2) DeleteCampaign(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid campaign id"})
		return
	}

	query := h.store.DB.Model(&models.CampaignV2{}).Where("id = ?", id)
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		query = query.Where("organization_id = ?", org.ID)
	}

	var campaign models.CampaignV2
	if err := query.First(&campaign).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	if err := h.store.DB.Delete(&campaign).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetCampaignStats returns aggregate campaign stats
func (h *CampaignHandlerV2) GetCampaignStats(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	query := h.store.DB.Model(&models.CampaignV2{})
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		// Superadmins can still scope to selected org.
		query = query.Where("organization_id = ?", org.ID)
	}

	type campaignStats struct {
		TotalCampaigns int64 `json:"total_campaigns"`
		TotalSent      int64 `json:"total_sent"`
		TotalOpens     int64 `json:"total_opens"`
		TotalClicks    int64 `json:"total_clicks"`
	}

	var stats campaignStats
	if err := query.Count(&stats.TotalCampaigns).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	type sumResult struct {
		Sent   int64
		Opens  int64
		Clicks int64
	}
	var sums sumResult
	if err := query.Select(
		"COALESCE(SUM(total_sent), 0) as sent, COALESCE(SUM(total_opens), 0) as opens, COALESCE(SUM(total_clicks), 0) as clicks",
	).Scan(&sums).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	stats.TotalSent = sums.Sent
	stats.TotalOpens = sums.Opens
	stats.TotalClicks = sums.Clicks

	openRate := 0.0
	clickRate := 0.0
	if stats.TotalSent > 0 {
		openRate = float64(stats.TotalOpens) * 100.0 / float64(stats.TotalSent)
		clickRate = float64(stats.TotalClicks) * 100.0 / float64(stats.TotalSent)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_campaigns": stats.TotalCampaigns,
		"total_sent":      stats.TotalSent,
		"total_opens":     stats.TotalOpens,
		"total_clicks":    stats.TotalClicks,
		"open_rate":       openRate,
		"click_rate":      clickRate,
	})
}

// PreviewCampaign renders a campaign preview for a contact
func (h *CampaignHandlerV2) PreviewCampaign(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid campaign id"})
		return
	}

	var campaign models.CampaignV2
	query := h.store.DB.Model(&models.CampaignV2{}).Where("id = ?", id)
	if admin != nil && !admin.IsSuperAdmin {
		if org == nil || org.ID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
			return
		}
		query = query.Where("organization_id = ?", org.ID)
	} else if org != nil && org.ID > 0 {
		query = query.Where("organization_id = ?", org.ID)
	}

	if err := query.First(&campaign).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	pe := core.NewPersonalizationEngine()
	html, _ := pe.PreviewWithSampleData(campaign.HTMLContent)
	subject, _ := pe.PreviewWithSampleData(campaign.Subject)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"html":    html,
		"subject": subject,
	})
}

func hashAPIKeyV2(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

type APIKeyHandlerV2 struct {
	store *store.Store
}

func NewAPIKeyHandlerV2(st *store.Store) *APIKeyHandlerV2 {
	return &APIKeyHandlerV2{store: st}
}

func (h *APIKeyHandlerV2) ListKeys(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var keys []models.APIKeyV2
	if err := h.store.DB.Where("organization_id = ?", org.ID).Order("created_at desc").Find(&keys).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, keys)
}

func parseScopes(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		clean := make([]string, 0, len(arr))
		for _, s := range arr {
			s = strings.TrimSpace(s)
			if s != "" {
				clean = append(clean, s)
			}
		}
		return clean, nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	parts := strings.Split(s, ",")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			clean = append(clean, p)
		}
	}
	return clean, nil
}

func (h *APIKeyHandlerV2) CreateKey(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	admin := getAdminFromContext(r.Context())

	var req struct {
		Name   string          `json:"name"`
		Scopes json.RawMessage `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	scopes, err := parseScopes(req.Scopes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scopes"})
		return
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "crypto error"})
		return
	}
	keyStr := "samp_" + hex.EncodeToString(keyBytes)
	keyHash := hashAPIKeyV2(keyStr)
	keyPrefix := keyStr
	if len(keyPrefix) > 8 {
		keyPrefix = keyPrefix[:8]
	}

	apiKey := &models.APIKeyV2{
		OrganizationID: org.ID,
		Name:           req.Name,
		KeyPrefix:      keyPrefix,
		KeyHash:        keyHash,
		Scopes:         scopes,
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	if admin != nil {
		apiKey.CreatedBy = admin.ID
	}

	if err := h.store.DB.Create(apiKey).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save key"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        keyStr,
		"key_prefix": apiKey.KeyPrefix,
		"scopes":     apiKey.Scopes,
		"created_at": apiKey.CreatedAt,
		"warning":    "Save this key now. It will not be shown again.",
	})
}

func (h *APIKeyHandlerV2) DeleteKey(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).Delete(&models.APIKeyV2{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type TagHandlerV2 struct {
	store *store.Store
}

func NewTagHandlerV2(st *store.Store) *TagHandlerV2 {
	return &TagHandlerV2{store: st}
}

func (h *TagHandlerV2) List(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var tags []models.TagV2
	if err := h.store.DB.Where("organization_id = ?", org.ID).Order("name").Find(&tags).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	type tagWithCount struct {
		models.TagV2
		ContactCount int `json:"contact_count"`
	}
	result := make([]tagWithCount, len(tags))
	for i, tag := range tags {
		var count int64
		h.store.DB.Model(&models.ContactTagV2{}).Where("tag_id = ?", tag.ID).Count(&count)
		result[i] = tagWithCount{TagV2: tag, ContactCount: int(count)}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *TagHandlerV2) Create(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	color := req.Color
	if strings.TrimSpace(color) == "" {
		color = "#6366f1"
	}

	tag := models.TagV2{
		OrganizationID: org.ID,
		Name:           req.Name,
		Color:          color,
		CreatedAt:      time.Now(),
	}

	if err := h.store.DB.Create(&tag).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "tag already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	writeJSON(w, http.StatusCreated, tag)
}

func (h *TagHandlerV2) Update(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var tag models.TagV2
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&tag).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tag not found"})
		return
	}

	var req struct {
		Name  *string `json:"name"`
		Color *string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		updates["name"] = *req.Name
	}
	if req.Color != nil && strings.TrimSpace(*req.Color) != "" {
		updates["color"] = *req.Color
	}
	if len(updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no valid fields to update"})
		return
	}

	if err := h.store.DB.Model(&tag).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, tag)
}

func (h *TagHandlerV2) Delete(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var tag models.TagV2
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&tag).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "tag not found"})
		return
	}

	h.store.DB.Where("tag_id = ?", tag.ID).Delete(&models.ContactTagV2{})
	if err := h.store.DB.Delete(&tag).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type SegmentConditionV2 struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Combiner string `json:"combiner"`
}

type SegmentHandlerV2 struct {
	store *store.Store
}

func NewSegmentHandlerV2(st *store.Store) *SegmentHandlerV2 {
	return &SegmentHandlerV2{store: st}
}

func (h *SegmentHandlerV2) List(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var segments []models.SegmentV2
	if err := h.store.DB.Where("organization_id = ?", org.ID).Order("name").Find(&segments).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, segments)
}

func (h *SegmentHandlerV2) Create(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Conditions  string `json:"conditions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	now := time.Now()
	segment := models.SegmentV2{
		OrganizationID: org.ID,
		Name:           req.Name,
		Description:    req.Description,
		Conditions:     req.Conditions,
		IsDynamic:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastComputed:   now,
	}
	segment.CachedCount = h.calculateSegmentCount(org.ID, segment.Conditions)

	if err := h.store.DB.Create(&segment).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "segment already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	writeJSON(w, http.StatusCreated, segment)
}

func (h *SegmentHandlerV2) Delete(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).Delete(&models.SegmentV2{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *SegmentHandlerV2) Refresh(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var segment models.SegmentV2
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&segment).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "segment not found"})
		return
	}

	now := time.Now()
	segment.CachedCount = h.calculateSegmentCount(org.ID, segment.Conditions)
	segment.LastComputed = now
	segment.UpdatedAt = now
	if err := h.store.DB.Save(&segment).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	writeJSON(w, http.StatusOK, segment)
}

func (h *SegmentHandlerV2) calculateSegmentCount(orgID uint, conditionsJSON string) int {
	var conditions []SegmentConditionV2
	if strings.TrimSpace(conditionsJSON) != "" {
		if err := json.Unmarshal([]byte(conditionsJSON), &conditions); err != nil {
			return 0
		}
	}

	query := h.store.DB.Model(&models.ContactV2{}).Where("organization_id = ?", orgID)
	if len(conditions) == 0 {
		var count int64
		query.Count(&count)
		return int(count)
	}

	query = h.applyConditions(query, orgID, conditions)
	var count int64
	query.Count(&count)
	return int(count)
}

func (h *SegmentHandlerV2) applyConditions(query *gorm.DB, orgID uint, conditions []SegmentConditionV2) *gorm.DB {
	for i, cond := range conditions {
		field := strings.TrimSpace(cond.Field)
		op := strings.TrimSpace(cond.Operator)
		val := strings.TrimSpace(cond.Value)
		comb := strings.TrimSpace(cond.Combiner)

		var (
			clauseStr string
			args      []interface{}
		)

		switch field {
		case "email":
			clauseStr, args = buildStringConditionV2("email", op, val)
		case "first_name":
			clauseStr, args = buildStringConditionV2("first_name", op, val)
		case "last_name":
			clauseStr, args = buildStringConditionV2("last_name", op, val)
		case "list_id":
			if op == "equals" || op == "" {
				clauseStr, args = "list_id = ?", []interface{}{val}
			} else if op == "not_equals" {
				clauseStr, args = "list_id != ?", []interface{}{val}
			}
		case "tag":
			sub := h.store.DB.Model(&models.ContactTagV2{}).
				Select("contact_tag_v2s.contact_id").
				Joins("JOIN tag_v2s t ON t.id = contact_tag_v2s.tag_id").
				Where("t.organization_id = ? AND t.name = ?", orgID, val)

			if op == "not_has" || op == "not_equals" {
				clauseStr, args = "id NOT IN (?)", []interface{}{sub}
			} else {
				clauseStr, args = "id IN (?)", []interface{}{sub}
			}
		case "created_at":
			clauseStr, args = buildDateConditionV2("created_at", op, val)
		}

		if clauseStr == "" {
			continue
		}

		if i == 0 || comb == "" || comb == "and" {
			query = query.Where(clauseStr, args...)
		} else if comb == "or" {
			query = query.Or(clauseStr, args...)
		}
	}
	return query
}

func buildStringConditionV2(field, operator, value string) (string, []interface{}) {
	switch operator {
	case "equals":
		return field + " = ?", []interface{}{value}
	case "not_equals":
		return field + " != ?", []interface{}{value}
	case "contains":
		return field + " LIKE ?", []interface{}{"%" + value + "%"}
	case "not_contains":
		return field + " NOT LIKE ?", []interface{}{"%" + value + "%"}
	case "starts_with":
		return field + " LIKE ?", []interface{}{value + "%"}
	case "ends_with":
		return field + " LIKE ?", []interface{}{"%" + value}
	case "is_empty":
		return "(" + field + " = '' OR " + field + " IS NULL)", nil
	case "is_not_empty":
		return field + " != '' AND " + field + " IS NOT NULL", nil
	}
	return "", nil
}

func buildDateConditionV2(field, operator, value string) (string, []interface{}) {
	switch operator {
	case "before":
		return field + " < ?", []interface{}{value}
	case "after":
		return field + " > ?", []interface{}{value}
	case "within_days":
		days, err := strconv.Atoi(value)
		if err != nil || days <= 0 {
			return "", nil
		}
		cutoff := time.Now().AddDate(0, 0, -days)
		return field + " >= ?", []interface{}{cutoff}
	}
	return "", nil
}

type SuppressionHandlerV2 struct {
	store *store.Store
}

func NewSuppressionHandlerV2(st *store.Store) *SuppressionHandlerV2 {
	return &SuppressionHandlerV2{store: st}
}

func (h *SuppressionHandlerV2) List(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	offset := (page - 1) * limit

	query := h.store.DB.Model(&models.SuppressionV2{}).Where("organization_id = ?", org.ID)
	if search != "" {
		query = query.Where("email LIKE ?", "%"+strings.ToLower(search)+"%")
	}

	var total int64
	query.Count(&total)

	var sups []models.SuppressionV2
	if err := query.Order("created_at desc").Limit(limit).Offset(offset).Find(&sups).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	totalPages := int((total + int64(limit) - 1) / int64(limit))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":       sups,
		"page":       page,
		"total":      total,
		"totalPages": totalPages,
	})
}

func (h *SuppressionHandlerV2) Add(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
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

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "manual"
	}

	emails := make([]string, 0, len(req.Emails)+1)
	for _, e := range req.Emails {
		e = strings.TrimSpace(e)
		if e != "" {
			emails = append(emails, e)
		}
	}
	if strings.TrimSpace(req.Email) != "" {
		emails = append(emails, strings.TrimSpace(req.Email))
	}
	if len(emails) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no emails provided"})
		return
	}

	added := 0
	now := time.Now()
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" || !strings.Contains(email, "@") {
			continue
		}

		sup := models.SuppressionV2{
			OrganizationID: org.ID,
			Email:          email,
			Reason:         reason,
			Source:         "manual",
			CreatedAt:      now,
		}
		if err := h.store.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&sup).Error; err == nil {
			added++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"added": added,
		"total": len(emails),
	})
}

func (h *SuppressionHandlerV2) Delete(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).Delete(&models.SuppressionV2{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type WebhookHandlerV2 struct {
	store *store.Store
	ws    *core.WebhookService
}

func NewWebhookHandlerV2(st *store.Store) *WebhookHandlerV2 {
	return &WebhookHandlerV2{
		store: st,
		ws:    core.NewWebhookService(st),
	}
}

func (h *WebhookHandlerV2) List(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var hooks []models.WebhookV2
	if err := h.store.DB.Where("organization_id = ?", org.ID).Order("created_at desc").Find(&hooks).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, hooks)
}

func (h *WebhookHandlerV2) Create(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	var req struct {
		Name    string   `json:"name"`
		URL     string   `json:"url"`
		Enabled bool     `json:"enabled"`
		Events  []string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
		return
	}
	if err := core.ValidateWebhookURL(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook URL: " + err.Error()})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "Default"
	}
	events := req.Events
	if len(events) == 0 {
		events = []string{"bounce", "delivery", "open", "click"}
	}

	hook := models.WebhookV2{
		OrganizationID: org.ID,
		Name:           name,
		Direction:      "outgoing",
		URL:            req.URL,
		Method:         "POST",
		Events:         events,
		IsActive:       req.Enabled,
		CreatedAt:      time.Now(),
	}

	if err := h.store.DB.Create(&hook).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save webhook"})
		return
	}
	writeJSON(w, http.StatusCreated, hook)
}

func (h *WebhookHandlerV2) Update(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var hook models.WebhookV2
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&hook).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	var req struct {
		Name    *string   `json:"name"`
		URL     *string   `json:"url"`
		Enabled *bool     `json:"enabled"`
		Events  *[]string `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.URL != nil {
		urlStr := strings.TrimSpace(*req.URL)
		if urlStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
			return
		}
		if err := core.ValidateWebhookURL(urlStr); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook URL: " + err.Error()})
			return
		}
		hook.URL = urlStr
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name != "" {
			hook.Name = name
		}
	}
	if req.Enabled != nil {
		hook.IsActive = *req.Enabled
	}
	if req.Events != nil {
		hook.Events = *req.Events
	}

	if err := h.store.DB.Save(&hook).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update webhook"})
		return
	}
	writeJSON(w, http.StatusOK, hook)
}

func (h *WebhookHandlerV2) Delete(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).Delete(&models.WebhookV2{}).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete webhook"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *WebhookHandlerV2) TestURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url required"})
		return
	}
	if err := core.ValidateWebhookURL(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook URL: " + err.Error()})
		return
	}
	if err := h.ws.SendTestWebhook(req.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *WebhookHandlerV2) TestByID(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil || org.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization context required"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var hook models.WebhookV2
	if err := h.store.DB.Where("id = ? AND organization_id = ?", id, org.ID).First(&hook).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	if strings.TrimSpace(hook.URL) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook url not set"})
		return
	}
	if err := h.ws.SendTestWebhook(hook.URL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// =====================================
// ORGANIZATION/MULTITENANCY API
// =====================================

// OrganizationHandler handles multi-tenant operations
type OrganizationHandler struct {
	store *store.Store
}

// NewOrganizationHandler creates a new organization handler
func NewOrganizationHandler(st *store.Store) *OrganizationHandler {
	return &OrganizationHandler{store: st}
}

// GetOrganization returns the current organization
func (h *OrganizationHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}

	if !admin.IsSuperAdmin {
		var membership models.OrganizationUser
		if err := h.store.DB.Where("admin_id = ? AND organization_id = ?", admin.ID, id).First(&membership).Error; err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	writeJSON(w, http.StatusOK, org)
}

// UpdateOrganization updates organization settings
func (h *OrganizationHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}

	if !admin.IsSuperAdmin {
		var membership models.OrganizationUser
		if err := h.store.DB.Where("admin_id = ? AND organization_id = ?", admin.ID, id).First(&membership).Error; err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
		if membership.Role != "owner" && membership.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
			return
		}
	}

	var req struct {
		Name         *string         `json:"name"`
		Slug         *string         `json:"slug"`
		Plan         *string         `json:"plan"`
		MaxUsers     *int            `json:"max_users"`
		MaxContacts  *int            `json:"max_contacts"`
		MaxEmails    *int            `json:"max_emails_per_month"`
		LogoURL      *string         `json:"logo_url"`
		PrimaryColor *string         `json:"primary_color"`
		Settings     *models.JSONMap `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.LogoURL != nil {
		updates["logo_url"] = *req.LogoURL
	}
	if req.PrimaryColor != nil {
		updates["primary_color"] = *req.PrimaryColor
	}
	if req.Settings != nil {
		updates["settings"] = *req.Settings
	}

	if admin.IsSuperAdmin {
		if req.Slug != nil {
			updates["slug"] = *req.Slug
		}
		if req.Plan != nil {
			updates["plan"] = *req.Plan
		}
		if req.MaxUsers != nil {
			updates["max_users"] = *req.MaxUsers
		}
		if req.MaxContacts != nil {
			updates["max_contacts"] = *req.MaxContacts
		}
		if req.MaxEmails != nil {
			updates["max_emails_per_month"] = *req.MaxEmails
		}
	}

	if len(updates) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no valid fields to update"})
		return
	}

	if err := h.store.DB.Model(&org).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to reload organization"})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// GetOrganizationUsage returns usage statistics
func (h *OrganizationHandler) GetOrganizationUsage(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return
	}

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}

	if !admin.IsSuperAdmin {
		var membership models.OrganizationUser
		if err := h.store.DB.Where("admin_id = ? AND organization_id = ?", admin.ID, id).First(&membership).Error; err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
			return
		}
	}

	// Get actual counts
	var contactCount, campaignCount int64
	h.store.DB.Model(&models.ContactV2{}).Where("organization_id = ?", id).Count(&contactCount)
	h.store.DB.Model(&models.CampaignV2{}).Where("organization_id = ?", id).Count(&campaignCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"contacts_used":     contactCount,
		"contacts_limit":    org.MaxContacts,
		"emails_sent":       org.EmailsSent,
		"emails_limit":      org.MaxEmails,
		"campaigns_created": campaignCount,
		"plan":              org.Plan,
	})
}

// ListOrganizations (Superadmin)
func (h *OrganizationHandler) ListOrganizations(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	var orgs []models.Organization
	if err := h.store.DB.Order("created_at DESC").Find(&orgs).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, orgs)
}

// CreateOrganization (Superadmin)
func (h *OrganizationHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
		Plan string `json:"plan"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	org := models.Organization{
		Name:      req.Name,
		Slug:      req.Slug,
		Plan:      req.Plan,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		// Defaults
		MaxUsers:    5,
		MaxContacts: 1000,
		MaxEmails:   10000,
	}

	if err := h.store.DB.Create(&org).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Add Creator as Owner
	membership := models.OrganizationUser{
		OrganizationID: org.ID,
		AdminID:        admin.ID,
		Role:           "owner",
		JoinedAt:       func() *time.Time { t := time.Now(); return &t }(),
	}
	h.store.DB.Create(&membership)

	writeJSON(w, http.StatusCreated, org)
}

// DeleteOrganization (Superadmin)
func (h *OrganizationHandler) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil || !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Delete Org (Cascading deletes should be handled by DB or explicit logic, for now simple delete)
	if err := h.store.DB.Delete(&models.Organization{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Cleanup memberships
	h.store.DB.Where("organization_id = ?", id).Delete(&models.OrganizationUser{})

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetOrganizationMembers returns all users in an organization
func (h *OrganizationHandler) GetOrganizationMembers(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var memberships []models.OrganizationUser
	if err := h.store.DB.Where("organization_id = ?", id).Find(&memberships).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Get admin details for each membership
	type MemberResponse struct {
		ID      uint   `json:"id"`
		AdminID uint   `json:"admin_id"`
		Email   string `json:"email"`
		Role    string `json:"role"`
	}

	var response []MemberResponse
	for _, m := range memberships {
		var admin models.AdminUser
		if err := h.store.DB.First(&admin, m.AdminID).Error; err == nil {
			response = append(response, MemberResponse{
				ID:      m.ID,
				AdminID: m.AdminID,
				Email:   admin.Email,
				Role:    m.Role,
			})
		}
	}

	writeJSON(w, http.StatusOK, response)
}
