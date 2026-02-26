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
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// hashAPIKey returns SHA256 hash of the API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// ValidAPIKeyScopes contains allowed API key scopes
var ValidAPIKeyScopes = map[string]bool{
	"relay":     true,
	"verify":    true,
	"campaigns": true,
	"lists":     true,
	"analytics": true,
	"admin":     true,
}

// validateScopes returns true if all scopes in the string are valid
func validateScopes(scopes string) bool {
	if scopes == "" {
		return true
	}
	for _, scope := range strings.Split(scopes, ",") {
		scope = strings.TrimSpace(scope)
		if scope != "" && !ValidAPIKeyScopes[scope] {
			return false
		}
	}
	return true
}

// HasScope returns true if the API key has the required scope
func HasScope(apiKey *models.APIKey, requiredScope string) bool {
	if apiKey == nil || apiKey.Scopes == "" {
		return false
	}
	if strings.Contains(apiKey.Scopes, "admin") {
		return true
	}
	for _, scope := range strings.Split(apiKey.Scopes, ",") {
		if strings.TrimSpace(scope) == requiredScope {
			return true
		}
	}
	return false
}

// handleListKeys returns all API keys with masked values
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var keys []models.APIKey
	if err := s.Store.DB.Scopes(store.WithOrgFilterScope(orgID)).Find(&keys).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	type maskedKey struct {
		ID        uint      `json:"id"`
		Name      string    `json:"name"`
		KeyPrefix string    `json:"key_prefix"`
		Scopes    string    `json:"scopes"`
		CreatedAt time.Time `json:"created_at"`
	}

	masked := make([]maskedKey, len(keys))
	for i, k := range keys {
		prefix := k.Key
		if len(prefix) > 12 {
			prefix = prefix[:12] + "..."
		}
		masked[i] = maskedKey{
			ID:        k.ID,
			Name:      k.Name,
			KeyPrefix: prefix,
			Scopes:    k.Scopes,
			CreatedAt: k.CreatedAt,
		}
	}
	writeJSON(w, http.StatusOK, masked)
}

// handleCreateKey creates a new API key
func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	var req struct {
		Name   string `json:"name"`
		Scopes string `json:"scopes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}

	if !validateScopes(req.Scopes) {
		validScopes := make([]string, 0, len(ValidAPIKeyScopes))
		for scope := range ValidAPIKeyScopes {
			validScopes = append(validScopes, scope)
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":        "invalid scopes",
			"valid_scopes": strings.Join(validScopes, ","),
		})
		return
	}

	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "crypto error"})
		return
	}
	keyStr := "kumo_" + hex.EncodeToString(keyBytes)
	keyHash := hashAPIKey(keyStr)

	apiKey := &models.APIKey{
		Name:           req.Name,
		Key:            keyHash,
		Scopes:         req.Scopes,
		OrganizationID: orgID,
		CreatedAt:      time.Now(),
	}

	if err := s.Store.DB.Create(apiKey).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save key"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":         apiKey.ID,
		"name":       apiKey.Name,
		"key":        keyStr,
		"scopes":     apiKey.Scopes,
		"created_at": apiKey.CreatedAt,
		"warning":    "Save this key now. It will not be shown again.",
	})
}

// handleDeleteKey deletes an API key by ID
func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	orgID, ok := getOrgID(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)

	// Verify ownership before delete
	var key models.APIKey
	if err := s.Store.DB.First(&key, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
		return
	}

	if orgID > 0 && key.OrganizationID != orgID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	if err := s.Store.DB.Delete(&models.APIKey{}, id).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
