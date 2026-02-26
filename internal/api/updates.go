package api

import (
	"context"
	"net/http"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/core"
)

// =====================================
// VERSION & UPDATE API HANDLERS
// =====================================

// GET /api/version
// Returns current version information
func (s *Server) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	updater := core.GetUpdater()
	info := updater.GetVersionInfo()
	writeJSON(w, http.StatusOK, info)
}

// GET /api/updates/status
// Returns current update status
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	updater := core.GetUpdater()
	status := updater.GetStatus()
	writeJSON(w, http.StatusOK, status)
}

// POST /api/updates/check
// Manually trigger update check
func (s *Server) handleCheckForUpdates(w http.ResponseWriter, r *http.Request) {
	updater := core.GetUpdater()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	status, err := updater.CheckForUpdates(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"status":  status,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"status":  status,
	})
}

// POST /api/updates/download
// Download the available update
func (s *Server) handleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	updater := core.GetUpdater()

	// Check if update is available
	status := updater.GetStatus()
	if !status.Available {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "no update available",
		})
		return
	}

	// Start download in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		updater.DownloadUpdate(ctx)
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"message": "download started",
		"version": status.LatestVersion,
	})
}

// POST /api/updates/apply
// Apply the downloaded update (requires restart)
func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	updater := core.GetUpdater()

	// Check if update is ready
	status := updater.GetStatus()
	if !status.DownloadReady {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "update not downloaded yet",
		})
		return
	}

	// Apply update
	if err := updater.ApplyUpdate(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "update applied, restarting...",
		"version":          status.LatestVersion,
		"restart_required": true,
	})
}

// POST /api/updates/rollback
// Rollback to previous version
func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	admin := getAdminFromContext(r.Context())
	if admin == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "not authenticated"})
		return
	}
	if !admin.IsSuperAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "superadmin access required"})
		return
	}

	updater := core.GetUpdater()

	if err := updater.Rollback(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":          "rollback completed, restarting...",
		"restart_required": true,
	})
}

// GET /api/updates/changelog
// Get changelog for available update
func (s *Server) handleUpdateChangelog(w http.ResponseWriter, r *http.Request) {
	updater := core.GetUpdater()
	status := updater.GetStatus()

	if !status.Available || status.ReleaseInfo == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error": "no update available",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":       status.ReleaseInfo.Version,
		"release_date":  status.ReleaseInfo.ReleaseDate,
		"release_notes": status.ReleaseInfo.ReleaseNotes,
		"is_lts":        status.ReleaseInfo.IsLTS,
		"size":          status.ReleaseInfo.Size,
	})
}
