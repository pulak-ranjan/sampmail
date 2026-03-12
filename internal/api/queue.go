package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
)

// GET /api/queue - List queue messages with optional domain filter
func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	domainFilter := r.URL.Query().Get("domain")

	limit := 100
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
		limit = l
	}

	messages, err := core.GetQueueMessages(limit, domainFilter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read queue"})
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

// GET /api/queue/stats - Get queue statistics
func (s *Server) handleGetQueueStats(w http.ResponseWriter, r *http.Request) {
	stats, err := core.GetQueueStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get queue stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// DELETE /api/queue/{id} - Delete a specific message
func (s *Server) handleDeleteQueueMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if err := core.DeleteQueueMessage(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// POST /api/queue/flush - Flush all deferred messages
func (s *Server) handleFlushQueue(w http.ResponseWriter, r *http.Request) {
	if err := core.FlushQueue(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "flushed"})
}

// POST /api/queue/retry/{id} - Retry a specific message
func (s *Server) handleRetryQueueMessage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}

	if err := core.RetryQueueMessage(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "retried"})
}

// POST /api/queue/retry/all - Retry all deferred messages
func (s *Server) handleRetryAllDeferred(w http.ResponseWriter, r *http.Request) {
	if err := core.RetryAllDeferred(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "all retried"})
}

// DELETE /api/queue/bounced - Delete all bounced messages
func (s *Server) handleDeleteBounced(w http.ResponseWriter, r *http.Request) {
	if err := core.DeleteBounced(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "bounced deleted"})
}

// GET /api/queue/domains/kumo - Get real-time per-domain statistics from KumoMTA
func (s *Server) handleGetKumoDomainStats(w http.ResponseWriter, r *http.Request) {
	stats, err := core.GetDomainStats()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get domain stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// GET /api/kumo/status - Get KumoMTA service status
func (s *Server) handleGetKumoStatus(w http.ResponseWriter, r *http.Request) {
	status, err := core.GetKumoMTAStatus()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get KumoMTA status"})
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// POST /api/kumo/start - Start KumoMTA service
func (s *Server) handleStartKumo(w http.ResponseWriter, r *http.Request) {
	if err := core.StartKumoMTA(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

// POST /api/kumo/stop - Stop KumoMTA service
func (s *Server) handleStopKumo(w http.ResponseWriter, r *http.Request) {
	if err := core.StopKumoMTA(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// POST /api/kumo/restart - Restart KumoMTA service
func (s *Server) handleRestartKumo(w http.ResponseWriter, r *http.Request) {
	if err := core.RestartKumoMTA(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// POST /api/kumo/reload - Reload KumoMTA config
func (s *Server) handleReloadKumo(w http.ResponseWriter, r *http.Request) {
	if err := core.ReloadKumoMTA(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}

// GET /api/kumo/logs - Get KumoMTA logs
func (s *Server) handleGetKumoLogs(w http.ResponseWriter, r *http.Request) {
	lines := 50
	if l, err := strconv.Atoi(r.URL.Query().Get("lines")); err == nil && l > 0 {
		lines = l
	}

	logs, err := core.GetKumoLogs(lines)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get logs"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
	})
}
