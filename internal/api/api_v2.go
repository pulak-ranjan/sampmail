package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
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
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")

	query := h.store.DB.Model(&models.EmailTemplate{})

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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.EmailTemplate
	if err := h.store.DB.First(&template, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	writeJSON(w, http.StatusOK, template)
}

// CreateTemplate creates a new template
func (h *TemplateHandlerV2) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var template models.EmailTemplate
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var template models.EmailTemplate
	if err := h.store.DB.First(&template, id).Error; err != nil {
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

	if err := h.store.DB.Model(&template).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, template)
}

// DeleteTemplate deletes a template
func (h *TemplateHandlerV2) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	if err := h.store.DB.Delete(&models.EmailTemplate{}, id).Error; err != nil {
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
	status := r.URL.Query().Get("status")

	query := h.store.DB.Model(&models.AutomationV2{})
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var automation models.AutomationV2
	if err := h.store.DB.First(&automation, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	writeJSON(w, http.StatusOK, automation)
}

// CreateAutomation creates a new automation
func (h *AutomationHandlerV2) CreateAutomation(w http.ResponseWriter, r *http.Request) {
	var automation models.AutomationV2
	if err := json.NewDecoder(r.Body).Decode(&automation); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	automation.Status = "draft"

	if err := h.store.DB.Create(&automation).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, automation)
}

// UpdateAutomation updates an automation
func (h *AutomationHandlerV2) UpdateAutomation(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var automation models.AutomationV2
	if err := h.store.DB.First(&automation, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "automation not found"})
		return
	}

	var updates models.AutomationV2
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.store.DB.Model(&automation).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	// Unregister if running
	if h.engine != nil {
		h.engine.UnregisterWorkflow(uint(id))
	}

	if err := h.store.DB.Delete(&models.AutomationV2{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ActivateAutomation activates an automation
func (h *AutomationHandlerV2) ActivateAutomation(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var automation models.AutomationV2
	if err := h.store.DB.First(&automation, id).Error; err != nil {
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var automation models.AutomationV2
	if err := h.store.DB.First(&automation, id).Error; err != nil {
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var automation models.AutomationV2
	if err := h.store.DB.First(&automation, id).Error; err != nil {
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))
	status := r.URL.Query().Get("status")

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
	status := r.URL.Query().Get("status")

	query := h.store.DB.Model(&models.CampaignV2{})
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
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var campaign models.CampaignV2
	if err := h.store.DB.First(&campaign, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

// CreateCampaign creates a new campaign
func (h *CampaignHandlerV2) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var campaign models.CampaignV2
	if err := json.NewDecoder(r.Body).Decode(&campaign); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	campaign.Status = "draft"
	campaign.TrackOpens = true
	campaign.TrackClicks = true

	if err := h.store.DB.Create(&campaign).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, campaign)
}

// UpdateCampaign updates a campaign
func (h *CampaignHandlerV2) UpdateCampaign(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var campaign models.CampaignV2
	if err := h.store.DB.First(&campaign, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	if campaign.Status != "draft" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "can only edit draft campaigns"})
		return
	}

	var updates models.CampaignV2
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.store.DB.Model(&campaign).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, campaign)
}

// PreviewCampaign renders a campaign preview for a contact
func (h *CampaignHandlerV2) PreviewCampaign(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var campaign models.CampaignV2
	if err := h.store.DB.First(&campaign, id).Error; err != nil {
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
	// In a real implementation, this would come from the auth context
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// UpdateOrganization updates organization settings
func (h *OrganizationHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
	}

	var updates models.Organization
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.store.DB.Model(&org).Updates(updates).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// GetOrganizationUsage returns usage statistics
func (h *OrganizationHandler) GetOrganizationUsage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var org models.Organization
	if err := h.store.DB.First(&org, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
		return
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
