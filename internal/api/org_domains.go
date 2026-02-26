package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// OrgDomainHandler handles per-tenant tracking & unsubscribe domain config.
type OrgDomainHandler struct {
	Store *store.Store
}

func NewOrgDomainHandler(st *store.Store) *OrgDomainHandler {
	return &OrgDomainHandler{Store: st}
}

// systemHostname returns the canonical hostname used for CNAME targets.
// Falls back to the configured MainHostname.
func (h *OrgDomainHandler) systemHostname() string {
	settings, err := h.Store.GetSettings()
	if err == nil && settings.MainHostname != "" {
		return settings.MainHostname
	}
	return "localhost"
}

// requireOrgAccess verifies the caller can access the given org and returns orgID.
// SuperAdmins can always access any org. Org members can access their own.
func requireOrgAccess(w http.ResponseWriter, r *http.Request) (uint, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id"})
		return 0, false
	}
	orgID := uint(id)

	admin := getAdminFromContext(r.Context())
	if admin != nil && admin.IsSuperAdmin {
		return orgID, true
	}

	org := getOrganizationFromContext(r.Context())
	if org != nil && org.ID == orgID {
		return orgID, true
	}

	writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
	return 0, false
}

// GET /api/v2/organizations/{id}/domain-config
func (h *OrgDomainHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgAccess(w, r)
	if !ok {
		return
	}

	cfg, err := h.Store.GetOrgDomainConfig(orgID)
	if err == store.ErrNotFound {
		// Return empty stub with CNAME targets pre-populated
		sysHost := h.systemHostname()
		cfg = &models.OrganizationDomainConfig{
			OrganizationID:      orgID,
			TrackingCNAMETarget: "track." + sysHost,
			UnsubCNAMETarget:    "unsub." + sysHost,
		}
		writeJSON(w, http.StatusOK, cfg)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get config"})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// PUT /api/v2/organizations/{id}/domain-config
func (h *OrgDomainHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgAccess(w, r)
	if !ok {
		return
	}

	var req struct {
		TrackingDomain    string `json:"tracking_domain"`
		UnsubscribeDomain string `json:"unsubscribe_domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	sysHost := h.systemHostname()

	// Load existing or create new
	cfg, err := h.Store.GetOrgDomainConfig(orgID)
	if err == store.ErrNotFound {
		cfg = &models.OrganizationDomainConfig{
			OrganizationID: orgID,
		}
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load config"})
		return
	}

	// Update fields — reset verified flags whenever domain changes
	if req.TrackingDomain != "" && req.TrackingDomain != cfg.TrackingDomain {
		cfg.TrackingDomain = strings.ToLower(strings.TrimSpace(req.TrackingDomain))
		cfg.TrackingDomainVerified = false
		cfg.TrackingCNAMETarget = "track." + sysHost
	}
	if req.UnsubscribeDomain != "" && req.UnsubscribeDomain != cfg.UnsubscribeDomain {
		cfg.UnsubscribeDomain = strings.ToLower(strings.TrimSpace(req.UnsubscribeDomain))
		cfg.UnsubscribeDomainVerified = false
		cfg.UnsubCNAMETarget = "unsub." + sysHost
	}

	cfg.UpdatedAt = time.Now()

	if err := h.Store.UpsertOrgDomainConfig(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save config"})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// POST /api/v2/organizations/{id}/domain-config/verify-tracking
func (h *OrgDomainHandler) VerifyTracking(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgAccess(w, r)
	if !ok {
		return
	}

	cfg, err := h.Store.GetOrgDomainConfig(orgID)
	if err == store.ErrNotFound || cfg.TrackingDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tracking domain not configured"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load config"})
		return
	}

	verified, verifyErr := verifyCNAME(cfg.TrackingDomain, cfg.TrackingCNAMETarget)
	cfg.TrackingDomainVerified = verified
	cfg.UpdatedAt = time.Now()
	h.Store.UpsertOrgDomainConfig(cfg)

	resp := map[string]interface{}{
		"verified": verified,
		"domain":   cfg.TrackingDomain,
		"expected": cfg.TrackingCNAMETarget,
	}
	if verifyErr != nil {
		resp["error"] = verifyErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /api/v2/organizations/{id}/domain-config/verify-unsubscribe
func (h *OrgDomainHandler) VerifyUnsubscribe(w http.ResponseWriter, r *http.Request) {
	orgID, ok := requireOrgAccess(w, r)
	if !ok {
		return
	}

	cfg, err := h.Store.GetOrgDomainConfig(orgID)
	if err == store.ErrNotFound || cfg.UnsubscribeDomain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsubscribe domain not configured"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load config"})
		return
	}

	verified, verifyErr := verifyCNAME(cfg.UnsubscribeDomain, cfg.UnsubCNAMETarget)
	cfg.UnsubscribeDomainVerified = verified
	cfg.UpdatedAt = time.Now()
	h.Store.UpsertOrgDomainConfig(cfg)

	resp := map[string]interface{}{
		"verified": verified,
		"domain":   cfg.UnsubscribeDomain,
		"expected": cfg.UnsubCNAMETarget,
	}
	if verifyErr != nil {
		resp["error"] = verifyErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// verifyCNAME performs a DNS CNAME lookup and checks if it resolves to expectedTarget.
func verifyCNAME(domain, expectedTarget string) (bool, error) {
	cname, err := net.LookupCNAME(domain)
	if err != nil {
		return false, err
	}
	// net.LookupCNAME returns the canonical name with trailing dot
	return strings.TrimSuffix(cname, ".") == strings.TrimSuffix(expectedTarget, "."), nil
}
