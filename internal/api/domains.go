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
)

// Helper to get current user email
func (s *Server) getUser(r *http.Request) string {
	admin := getAdminFromContext(r.Context())
	if admin != nil {
		return admin.Email
	}
	return "unknown"
}

// getOrgID extracts the organization ID from request context.
// For superadmins without an org context, returns 0 (no filter = see all).
// For non-superadmins without an org context, writes 403 and returns false.
func getOrgID(w http.ResponseWriter, r *http.Request) (uint, bool) {
	admin := getAdminFromContext(r.Context())
	org := getOrganizationFromContext(r.Context())
	if org != nil && org.ID > 0 {
		return org.ID, true
	}
	if admin != nil && admin.IsSuperAdmin {
		return 0, true // superadmin: no filter
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "organization context required"})
	return 0, false
}

// GET /api/domains
func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	domains, err := s.Store.ListDomains(orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list domains"})
		return
	}
	if domains == nil {
		domains = []models.Domain{}
	}

	for i := range domains {
		for j := range domains[i].Senders {
			snd := &domains[i].Senders[j]
			snd.HasDKIM = core.DKIMKeyExists(domains[i].Name, snd.LocalPart)
		}
	}

	writeJSON(w, http.StatusOK, domains)
}

// POST /api/domains
func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var d models.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if d.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	d.OrganizationID = orgID
	if d.MailHost == "" {
		d.MailHost = "mail." + d.Name
	}
	if d.BounceHost == "" {
		d.BounceHost = "bounce." + d.Name
	}
	if d.DMARCPolicy == "" {
		d.DMARCPolicy = "none"
	}
	if d.DMARCPercentage == 0 {
		d.DMARCPercentage = 100
	}

	if err := s.Store.CreateDomain(&d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create domain"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Create Domain", fmt.Sprintf("Domain: %s", d.Name), s.getUser(r))

	writeJSON(w, http.StatusCreated, d)
}

// GET /api/domains/{id}
func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	domain, err := s.Store.GetDomainByID(uint(id))
	if err != nil {
		if err == store.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get domain"})
		return
	}

	// Verify ownership: non-superadmin must own the domain
	if orgID > 0 && domain.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	writeJSON(w, http.StatusOK, domain)
}

// PUT /api/domains/{id}
func (s *Server) handleUpdateDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	domain, err := s.Store.GetDomainByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	if orgID > 0 && domain.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	var update models.Domain
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if update.Name != "" {
		domain.Name = update.Name
	}
	if update.MailHost != "" {
		domain.MailHost = update.MailHost
	}
	if update.BounceHost != "" {
		domain.BounceHost = update.BounceHost
	}
	if update.DMARCPolicy != "" {
		domain.DMARCPolicy = update.DMARCPolicy
	}
	if update.DMARCRua != "" {
		domain.DMARCRua = update.DMARCRua
	}
	if update.DMARCRuf != "" {
		domain.DMARCRuf = update.DMARCRuf
	}
	if update.DMARCPercentage > 0 {
		domain.DMARCPercentage = update.DMARCPercentage
	}

	if err := s.Store.UpdateDomain(domain); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update domain"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Update Domain", fmt.Sprintf("Updated %s", domain.Name), s.getUser(r))

	writeJSON(w, http.StatusOK, domain)
}

// DELETE /api/domains/{id}
func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	// Fetch first to verify ownership and get name for log
	d, err := s.Store.GetDomainByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	if orgID > 0 && d.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	if err := s.Store.DeleteDomain(uint(id)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete domain"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Delete Domain", fmt.Sprintf("Deleted domain %s (ID: %d)", d.Name, id), s.getUser(r))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ----------------------
// Senders
// ----------------------

// GET /api/domains/{domainID}/senders
func (s *Server) handleListSenders(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	domainIDStr := chi.URLParam(r, "domainID")
	domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain id"})
		return
	}

	senders, err := s.Store.ListSendersByDomain(uint(domainID), orgID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list senders"})
		return
	}
	if senders == nil {
		senders = []models.Sender{}
	}

	domain, _ := s.Store.GetDomainByID(uint(domainID))
	if domain != nil {
		for i := range senders {
			senders[i].HasDKIM = core.DKIMKeyExists(domain.Name, senders[i].LocalPart)
		}
	}

	writeJSON(w, http.StatusOK, senders)
}

// POST /api/domains/{domainID}/senders
func (s *Server) handleCreateSender(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	domainIDStr := chi.URLParam(r, "domainID")
	domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain id"})
		return
	}

	domain, err := s.Store.GetDomainByID(uint(domainID))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	// Verify domain ownership
	if orgID > 0 && domain.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	var snd models.Sender
	if err := json.NewDecoder(r.Body).Decode(&snd); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	snd.DomainID = uint(domainID)
	snd.OrganizationID = orgID
	if snd.LocalPart != "" && snd.Email == "" {
		snd.Email = snd.LocalPart + "@" + domain.Name
	}
	if snd.BounceUsername == "" {
		snd.BounceUsername = "b-" + snd.LocalPart
	}

	if err := s.Store.CreateSender(&snd); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create sender"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Create Sender", fmt.Sprintf("Added sender %s to %s", snd.Email, domain.Name), s.getUser(r))

	writeJSON(w, http.StatusCreated, snd)
}

// GET /api/senders/{id}
func (s *Server) handleGetSender(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	sender, err := s.Store.GetSenderByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sender not found"})
		return
	}

	if orgID > 0 && sender.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	writeJSON(w, http.StatusOK, sender)
}

// PUT /api/senders/{id}
func (s *Server) handleUpdateSender(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	sender, err := s.Store.GetSenderByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sender not found"})
		return
	}

	if orgID > 0 && sender.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	var update models.Sender
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if update.LocalPart != "" {
		sender.LocalPart = update.LocalPart
	}
	if update.Email != "" {
		sender.Email = update.Email
	}
	if update.IP != "" {
		sender.IP = update.IP
	}
	if update.SMTPPassword != "" {
		sender.SMTPPassword = update.SMTPPassword
	}
	if update.BounceUsername != "" {
		sender.BounceUsername = update.BounceUsername
	}

	if err := s.Store.UpdateSender(sender); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update sender"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Update Sender", fmt.Sprintf("Updated sender %s", sender.Email), s.getUser(r))

	writeJSON(w, http.StatusOK, sender)
}

// DELETE /api/senders/{id}
func (s *Server) handleDeleteSender(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	// Get info for log and ownership check
	snd, err := s.Store.GetSenderByID(uint(id))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sender not found"})
		return
	}

	if orgID > 0 && snd.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	if err := s.Store.DeleteSender(uint(id)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete sender"})
		return
	}

	// AUDIT LOG
	go s.WS.SendAuditLog("Delete Sender", fmt.Sprintf("Deleted sender %s (ID: %d)", snd.Email, id), s.getUser(r))

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/domains/{domainID}/senders/{id}/setup
func (s *Server) handleSetupSender(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	domainIDStr := chi.URLParam(r, "domainID")
	domainID, err := strconv.ParseUint(domainIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain id"})
		return
	}

	senderIDStr := chi.URLParam(r, "id")
	senderID, err := strconv.ParseUint(senderIDStr, 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sender id"})
		return
	}

	domain, err := s.Store.GetDomainByID(uint(domainID))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}

	if orgID > 0 && domain.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	sender, err := s.Store.GetSenderByID(uint(senderID))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "sender not found"})
		return
	}

	// 1. Generate DKIM key
	dkimErr := core.GenerateDKIMKey(domain.Name, sender.LocalPart)

	// 2. Create bounce account
	bounceUser := sender.BounceUsername
	if bounceUser == "" {
		bounceUser = "b-" + sender.LocalPart
	}
	bounceErr := core.CreateBounceAccount(bounceUser, domain.Name, s.Store)

	// AUDIT LOG
	go s.WS.SendAuditLog("Setup Sender", fmt.Sprintf("Auto-configured DKIM/Bounce for %s", sender.Email), s.getUser(r))

	result := map[string]interface{}{
		"dkim_generated": dkimErr == nil,
		"bounce_created": bounceErr == nil,
		"bounce_user":    bounceUser + "@" + domain.Name,
		"selector":       sender.LocalPart,
	}

	if dkimErr != nil {
		result["dkim_error"] = dkimErr.Error()
	}
	if bounceErr != nil {
		result["bounce_error"] = bounceErr.Error()
	}

	writeJSON(w, http.StatusOK, result)
}
