package api

import (
	"encoding/json"
	"net/http"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// handleGetWebhookSettings returns current webhook configuration
func (s *Server) handleGetWebhookSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Store.GetSettings()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"webhook_url":      "",
			"webhook_enabled":  false,
			"bounce_alert_pct": 5.0,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"webhook_url":      settings.WebhookURL,
		"webhook_enabled":  settings.WebhookEnabled,
		"bounce_alert_pct": settings.BounceAlertPct,
	})
}

// handleSetWebhookSettings saves webhook configuration
func (s *Server) handleSetWebhookSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL     string  `json:"webhook_url"`
		WebhookEnabled bool    `json:"webhook_enabled"`
		BounceAlertPct float64 `json:"bounce_alert_pct"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	settings, err := s.Store.GetSettings()
	if err != nil {
		settings = &models.AppSettings{}
	}

	if req.WebhookURL != "" {
		if err := core.ValidateWebhookURL(req.WebhookURL); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook URL: " + err.Error()})
			return
		}
	}

	settings.WebhookURL = req.WebhookURL
	settings.WebhookEnabled = req.WebhookEnabled
	settings.BounceAlertPct = req.BounceAlertPct

	if settings.BounceAlertPct <= 0 {
		settings.BounceAlertPct = 5.0
	}

	if err := s.Store.UpsertSettings(settings); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save settings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// handleTestWebhook sends a test message to the webhook URL
func (s *Server) handleTestWebhook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL string `json:"webhook_url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if req.WebhookURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook_url required"})
		return
	}

	if err := core.ValidateWebhookURL(req.WebhookURL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid webhook URL: " + err.Error()})
		return
	}

	if err := s.WS.SendTestWebhook(req.WebhookURL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// handleGetWebhookLogs returns recent webhook logs
func (s *Server) handleGetWebhookLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.Store.ListWebhookLogs(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get logs"})
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// handleCheckBounces triggers bounce rate check
func (s *Server) handleCheckBounces(w http.ResponseWriter, r *http.Request) {
	if err := s.WS.CheckBounceRates(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to check bounces"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "checked"})
}
