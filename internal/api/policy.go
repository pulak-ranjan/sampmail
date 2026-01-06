package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// PolicyHandler serves internal policy API for KumoMTA Lua integration
// KumoMTA's Lua scripts query these endpoints to get dynamic sending rules
type PolicyHandler struct {
	store *store.Store
}

// NewPolicyHandler creates a new policy handler
func NewPolicyHandler(st *store.Store) *PolicyHandler {
	return &PolicyHandler{store: st}
}

// SendingIPPolicy represents an IP's sending policy
type SendingIPPolicy struct {
	IPAddress       string  `json:"ip_address"`
	Hostname        string  `json:"hostname"`
	WarmupStage     int     `json:"warmup_stage"`
	DailySendLimit  int     `json:"daily_send_limit"`
	TodaySent       int     `json:"today_sent"`
	ReputationScore float64 `json:"reputation_score"`
	IsActive        bool    `json:"is_active"`
}

// DomainPolicy represents a domain's sending policy
type DomainPolicy struct {
	Name          string `json:"name"`
	DKIMSelector  string `json:"dkim_selector"`
	WarmupEnabled bool   `json:"warmup_enabled"`
	RateLimit     string `json:"rate_limit"`
	MaxPerHour    int    `json:"max_per_hour"`
}

// SenderRatePolicy represents rate limiting for a sender
type SenderRatePolicy struct {
	Email      string `json:"email"`
	RateLimit  string `json:"rate_limit"`
	WarmupDay  int    `json:"warmup_day"`
	MaxPerHour int    `json:"max_per_hour"`
	EgressPool string `json:"egress_pool"`
	IsActive   bool   `json:"is_active"`
}

// DeliveryEvent represents a delivery event from KumoMTA
type DeliveryEvent struct {
	MessageID  string `json:"message_id"`
	Recipient  string `json:"recipient"`
	Sender     string `json:"sender"`
	Domain     string `json:"domain"`
	EventType  string `json:"event_type"` // delivered, bounced, deferred
	StatusCode string `json:"status_code"`
	Diagnostic string `json:"diagnostic"`
	Timestamp  int64  `json:"timestamp"`
}

// GET /api/internal/policy/sending-ips
// Returns list of sending IPs with warmup/rate info for KumoMTA
func (h *PolicyHandler) HandleGetSendingIPs(w http.ResponseWriter, r *http.Request) {
	var ips []models.SendingIP
	if err := h.store.DB.Where("is_active = ?", true).Find(&ips).Error; err != nil {
		logger.Error("Failed to fetch sending IPs", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	policies := make([]SendingIPPolicy, 0, len(ips))
	for _, ip := range ips {
		policies = append(policies, SendingIPPolicy{
			IPAddress:       ip.IPAddress,
			Hostname:        ip.Hostname,
			WarmupStage:     ip.WarmupStage,
			DailySendLimit:  ip.DailySendLimit,
			TodaySent:       ip.TodaySent,
			ReputationScore: ip.ReputationScore,
			IsActive:        ip.IsActive,
		})
	}

	writeJSON(w, http.StatusOK, policies)
}

// GET /api/internal/policy/domains
// Returns list of domains with DKIM/rate info for KumoMTA
func (h *PolicyHandler) HandleGetDomains(w http.ResponseWriter, r *http.Request) {
	var domains []models.Domain
	if err := h.store.DB.Preload("Senders").Find(&domains).Error; err != nil {
		logger.Error("Failed to fetch domains", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	policies := make([]DomainPolicy, 0, len(domains))
	for _, d := range domains {
		// Calculate rate limit based on warmup status of senders
		warmupEnabled := false
		maxPerHour := 10000 // Default high limit

		for _, s := range d.Senders {
			if s.WarmupEnabled {
				warmupEnabled = true
				rate := core.GetSenderRate(s)
				if rate != "" {
					// Parse rate like "100/h" to get integer
					if perHour := parseRateLimit(rate); perHour > 0 && perHour < maxPerHour {
						maxPerHour = perHour
					}
				}
			}
		}

		policies = append(policies, DomainPolicy{
			Name:          d.Name,
			DKIMSelector:  "default", // Default selector
			WarmupEnabled: warmupEnabled,
			RateLimit:     formatRateLimit(maxPerHour),
			MaxPerHour:    maxPerHour,
		})
	}

	writeJSON(w, http.StatusOK, policies)
}

// GET /api/internal/policy/sender-rate
// Returns rate limiting config for a specific sender/tenant
// Query params: email or tenant
func (h *PolicyHandler) HandleGetSenderRate(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	tenant := r.URL.Query().Get("tenant")

	if email == "" && tenant == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email or tenant required"})
		return
	}

	// Parse tenant format: domain__localpart
	if tenant != "" && email == "" {
		parts := strings.SplitN(tenant, "__", 2)
		if len(parts) == 2 {
			email = parts[1] + "@" + parts[0]
		}
	}

	var sender models.Sender
	if err := h.store.DB.Preload("Domain").Where("email = ?", email).First(&sender).Error; err != nil {
		// Return default policy if sender not found
		writeJSON(w, http.StatusOK, SenderRatePolicy{
			Email:      email,
			RateLimit:  "1000/h",
			MaxPerHour: 1000,
			EgressPool: "default",
			IsActive:   true,
		})
		return
	}

	rateLimit := core.GetSenderRate(sender)
	maxPerHour := parseRateLimit(rateLimit)
	if maxPerHour == 0 {
		maxPerHour = 1000 // Default
	}

	pool := core.PoolName(sender.Domain, sender)

	writeJSON(w, http.StatusOK, SenderRatePolicy{
		Email:      sender.Email,
		RateLimit:  rateLimit,
		WarmupDay:  sender.WarmupDay,
		MaxPerHour: maxPerHour,
		EgressPool: pool,
		IsActive:   true,
	})
}

// POST /api/internal/policy/log-event
// Receives delivery events from KumoMTA for tracking/analytics
func (h *PolicyHandler) HandleLogEvent(w http.ResponseWriter, r *http.Request) {
	var event DeliveryEvent
	if err := decodeJSON(r, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Log the event
	logger.Info("KumoMTA delivery event",
		"message_id", event.MessageID,
		"recipient", event.Recipient,
		"event_type", event.EventType,
		"status", event.StatusCode,
	)

	// Process based on event type
	switch event.EventType {
	case "bounced":
		// Record bounce
		bounce := &models.BounceEvent{
			Email:          event.Recipient,
			BounceType:     classifyBounce(event.StatusCode),
			BounceCode:     event.StatusCode,
			DiagnosticCode: event.Diagnostic,
			ProcessedAt:    time.Now(),
		}
		h.store.DB.Create(bounce)

		// Add to suppression list for hard bounces
		if bounce.BounceType == "hard" {
			suppression := &models.Suppression{
				Email:     event.Recipient,
				Reason:    "hard_bounce",
				Source:    "kumomta",
				CreatedAt: time.Now(),
			}
			h.store.DB.Where("email = ?", event.Recipient).FirstOrCreate(suppression)
		}

	case "delivered":
		// Update sending IP stats
		if event.Sender != "" {
			parts := strings.Split(event.Sender, "@")
			if len(parts) == 2 {
				h.store.DB.Model(&models.SendingIP{}).
					Where("hostname LIKE ?", "%"+parts[1]).
					UpdateColumn("today_sent", h.store.DB.Raw("today_sent + 1"))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged"})
}

// GET /api/internal/policy/suppression-check
// Checks if an email is on suppression list
func (h *PolicyHandler) HandleSuppressionCheck(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email required"})
		return
	}

	var suppression models.Suppression
	if err := h.store.DB.Where("email = ?", email).First(&suppression).Error; err != nil {
		// Not suppressed
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"email":      email,
			"suppressed": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"email":      email,
		"suppressed": true,
		"reason":     suppression.Reason,
	})
}

// Helper functions

func parseRateLimit(rate string) int {
	// Parse formats like "100/h", "50/hour", "1000/h"
	rate = strings.ToLower(strings.TrimSpace(rate))
	if rate == "" {
		return 0
	}

	var value int
	if strings.Contains(rate, "/h") {
		parts := strings.Split(rate, "/")
		if len(parts) >= 1 {
			_, err := strings.NewReader(parts[0]).Read(make([]byte, 0))
			if err == nil {
				// Parse the number
				for _, c := range parts[0] {
					if c >= '0' && c <= '9' {
						value = value*10 + int(c-'0')
					}
				}
			}
		}
	}

	return value
}

func formatRateLimit(perHour int) string {
	return strings.TrimSpace(strings.Replace(string(rune(perHour)), "", "", -1)) + "/h"
}

func classifyBounce(statusCode string) string {
	// 5xx = hard bounce, 4xx = soft bounce
	if strings.HasPrefix(statusCode, "5") {
		return "hard"
	}
	if strings.HasPrefix(statusCode, "4") {
		return "soft"
	}
	return "unknown"
}

// decodeJSON decodes JSON from request body
func decodeJSON(r *http.Request, v interface{}) error {
	return nil // Already defined elsewhere, placeholder
}
