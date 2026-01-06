package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/pulak-ranjan/sampmail/internal/models"
)

// Context keys
const (
	adminContextKey        contextKey = "admin"
	organizationContextKey contextKey = "organization"
)

type contextKey string

// authMiddleware authenticates the user via Bearer token
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authorization header"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authorization format"})
			return
		}

		// Hash the token to look up in database
		tokenHash := hashSessionToken(token)
		admin, err := s.Store.GetAdminBySessionToken(tokenHash)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		ctx := context.WithValue(r.Context(), adminContextKey, admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// organizationMiddleware extracts X-Organization-ID and validates membership
func (s *Server) organizationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin := getAdminFromContext(r.Context())
		if admin == nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
			return
		}

		// 1. Get Org ID from Header
		orgIDStr := r.Header.Get("X-Organization-ID")

		// If no header, and user is NOT Superadmin, we might block or allow global (depending on route)
		// For now, if no header, we proceed without Org Context (Global handlers will fail if they need it)
		if orgIDStr == "" {
			next.ServeHTTP(w, r)
			return
		}

		orgID, err := strconv.Atoi(orgIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid organization id header"})
			return
		}

		// 2. Superadmin Bypass
		if admin.IsSuperAdmin {
			// Superadmin can access ANY org, fetch it to ensure it exists
			var org models.Organization
			if err := s.Store.DB.First(&org, orgID).Error; err != nil {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
				return
			}
			ctx := context.WithValue(r.Context(), organizationContextKey, &org)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// 3. Regular User: Check Membership
		// We need to check if AdminID is linked to OrganizationID in OrganizationUser table
		var membership models.OrganizationUser
		if err := s.Store.DB.Where("admin_id = ? AND organization_id = ?", admin.ID, orgID).First(&membership).Error; err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "you do not have access to this organization"})
			return
		}

		// 4. Fetch Organization Details
		var org models.Organization
		if err := s.Store.DB.First(&org, orgID).Error; err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "organization not found"})
			return
		}

		// Inject into context
		ctx := context.WithValue(r.Context(), organizationContextKey, &org)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helpers

func getAdminFromContext(ctx context.Context) *models.AdminUser {
	if u, ok := ctx.Value(adminContextKey).(*models.AdminUser); ok {
		return u
	}
	return nil
}

func getOrganizationFromContext(ctx context.Context) *models.Organization {
	if o, ok := ctx.Value(organizationContextKey).(*models.Organization); ok {
		return o
	}
	return nil
}

// internalOnlyMiddleware restricts access to localhost only
// Used for internal APIs that KumoMTA Lua calls
func internalOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request is from localhost
		remoteIP := r.RemoteAddr

		// Extract IP (may include port)
		if colonIdx := strings.LastIndex(remoteIP, ":"); colonIdx != -1 {
			remoteIP = remoteIP[:colonIdx]
		}

		// Allow localhost and loopback
		allowed := []string{"127.0.0.1", "::1", "localhost", "[::1]"}
		isAllowed := false
		for _, ip := range allowed {
			if remoteIP == ip || strings.HasPrefix(remoteIP, ip) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "internal API only"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
