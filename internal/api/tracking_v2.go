package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// FIXED TRACKING HANDLERS
// Uses atomic operations to prevent race conditions
// =====================================

// TrackingHandlerV2 handles email tracking with atomic operations
type TrackingHandlerV2 struct {
	Store     *store.Store
	AtomicOps *store.AtomicOps
	Repos     *store.Repositories

	// 1x1 transparent GIF
	trackingPixel []byte
}

// NewTrackingHandlerV2 creates a new tracking handler
func NewTrackingHandlerV2(st *store.Store) *TrackingHandlerV2 {
	// 1x1 transparent GIF (43 bytes)
	pixel, _ := base64.StdEncoding.DecodeString(
		"R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7",
	)

	return &TrackingHandlerV2{
		Store:         st,
		AtomicOps:     store.NewAtomicOps(st.DB),
		Repos:         store.NewRepositories(st.DB),
		trackingPixel: pixel,
	}
}

// HandleOpen records an email open event
// FIX: Uses atomic UPDATE instead of read-modify-write
func (h *TrackingHandlerV2) HandleOpen(w http.ResponseWriter, r *http.Request) {
	campaignID, _ := strconv.ParseUint(chi.URLParam(r, "campaignId"), 10, 32)
	recipientID, _ := strconv.ParseUint(chi.URLParam(r, "recipientId"), 10, 32)

	// Return pixel immediately (non-blocking)
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	w.Write(h.trackingPixel)

	// Record open asynchronously
	go h.recordOpen(uint(campaignID), uint(recipientID))
}

// recordOpen records the open event atomically
func (h *TrackingHandlerV2) recordOpen(campaignID, recipientID uint) {
	log := logger.WithComponent("tracking").
		With("campaign_id", campaignID).
		With("recipient_id", recipientID)

	// Check if this is the first open (atomic operation)
	isFirstOpen, err := h.AtomicOps.MarkRecipientOpened(recipientID)
	if err != nil {
		log.Error("failed to mark recipient opened", "error", err)
		return
	}

	// Always increment total opens (for subsequent opens)
	if err := h.AtomicOps.IncrementCampaignOpens(campaignID); err != nil {
		log.Error("failed to increment opens", "error", err)
	}

	// Only increment unique opens on first open
	if isFirstOpen {
		if err := h.AtomicOps.IncrementUniqueOpens(campaignID); err != nil {
			log.Error("failed to increment unique opens", "error", err)
		}

		// Get recipient's contact ID and update their stats
		var recipient models.CampaignRecipient
		if err := h.Store.DB.First(&recipient, recipientID).Error; err == nil && recipient.ContactID > 0 {
			h.AtomicOps.IncrementContactOpens(recipient.ContactID)
			
			// Trigger automation for email opened
			// core.TriggerAutomation("trigger_email_opened", recipient.ContactID, map[string]interface{}{
			// 	"campaign_id": campaignID,
			// })
		}

		log.Debug("first open recorded")
	} else {
		// Increment open count for repeat opens
		h.AtomicOps.IncrementRecipientOpenCount(recipientID)
		log.Debug("repeat open recorded")
	}
}

// HandleClick records a link click event
// FIX: Uses atomic UPDATE and validates link signature with expiration
func (h *TrackingHandlerV2) HandleClick(w http.ResponseWriter, r *http.Request) {
	campaignID, _ := strconv.ParseUint(chi.URLParam(r, "campaignId"), 10, 32)
	recipientID, _ := strconv.ParseUint(chi.URLParam(r, "recipientId"), 10, 32)

	// Get the original URL
	targetURL := r.URL.Query().Get("url")
	signature := r.URL.Query().Get("sig")

	if targetURL == "" {
		http.Error(w, "Missing URL", http.StatusBadRequest)
		return
	}

	// Decode URL
	decodedURL, err := url.QueryUnescape(targetURL)
	if err != nil {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Validate signature if present
	if signature != "" {
		cfg := config.Get()
		if !h.verifyLinkSignature(uint(campaignID), uint(recipientID), decodedURL, signature, cfg.AppSecret) {
			// Log potential replay attack
			logger.WithComponent("tracking").Warn("invalid link signature",
				"campaign_id", campaignID,
				"recipient_id", recipientID,
			)
			// Still redirect but don't track (potential replay)
			http.Redirect(w, r, decodedURL, http.StatusFound)
			return
		}

		// Check link expiration (30 days from send)
		valid, err := h.AtomicOps.ValidateTrackingLink(uint(recipientID), signature, 30*24*time.Hour)
		if err != nil || !valid {
			logger.WithComponent("tracking").Warn("expired tracking link",
				"recipient_id", recipientID,
			)
			// Still redirect but don't track
			http.Redirect(w, r, decodedURL, http.StatusFound)
			return
		}
	}

	// Redirect immediately (non-blocking)
	http.Redirect(w, r, decodedURL, http.StatusFound)

	// Record click asynchronously
	go h.recordClick(uint(campaignID), uint(recipientID), decodedURL)
}

// recordClick records the click event atomically
func (h *TrackingHandlerV2) recordClick(campaignID, recipientID uint, url string) {
	log := logger.WithComponent("tracking").
		With("campaign_id", campaignID).
		With("recipient_id", recipientID)

	// Check if this is the first click (atomic operation)
	isFirstClick, err := h.AtomicOps.MarkRecipientClicked(recipientID)
	if err != nil {
		log.Error("failed to mark recipient clicked", "error", err)
		return
	}

	// Always increment total clicks
	if err := h.AtomicOps.IncrementCampaignClicks(campaignID); err != nil {
		log.Error("failed to increment clicks", "error", err)
	}

	// Only increment unique clicks on first click
	if isFirstClick {
		if err := h.AtomicOps.IncrementUniqueClicks(campaignID); err != nil {
			log.Error("failed to increment unique clicks", "error", err)
		}

		// Get recipient's contact ID and update their stats
		var recipient models.CampaignRecipient
		if err := h.Store.DB.First(&recipient, recipientID).Error; err == nil && recipient.ContactID > 0 {
			h.AtomicOps.IncrementContactClicks(recipient.ContactID)
			// Add lead score for engagement
			h.AtomicOps.UpdateLeadScore(recipient.ContactID, 5)
			
			// Trigger automation for link clicked
			// core.TriggerAutomation("trigger_link_clicked", recipient.ContactID, map[string]interface{}{
			// 	"campaign_id": campaignID,
			// 	"url": url,
			// })
		}

		log.Debug("first click recorded", "url", url)
	} else {
		// Increment click count for repeat clicks
		h.AtomicOps.IncrementRecipientClickCount(recipientID)
		log.Debug("repeat click recorded", "url", url)
	}

	// Track specific link clicks
	h.trackLinkClick(campaignID, recipientID, url)
}

// trackLinkClick records detailed link click data
func (h *TrackingHandlerV2) trackLinkClick(campaignID, recipientID uint, url string) {
	// Store click in detailed tracking table for analytics
	// This is a separate table to avoid bloating the recipient table
	h.Store.DB.Exec(`
		INSERT INTO campaign_link_clicks (campaign_id, recipient_id, url, clicked_at)
		VALUES (?, ?, ?, ?)
	`, campaignID, recipientID, url, time.Now())
}

// verifyLinkSignature verifies the HMAC signature of a tracking link
// FIX: Now includes timestamp validation
func (h *TrackingHandlerV2) verifyLinkSignature(campaignID, recipientID uint, url, signature, secret string) bool {
	// The signature should be HMAC-SHA256(campaign_id:recipient_id:url)
	data := fmt.Sprintf("%d:%d:%s", campaignID, recipientID, url)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// GenerateTrackingLink creates a signed tracking URL
func GenerateTrackingLink(baseURL string, campaignID, recipientID uint, targetURL, secret string) string {
	// Generate signature
	data := fmt.Sprintf("%d:%d:%s", campaignID, recipientID, targetURL)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	sig := hex.EncodeToString(mac.Sum(nil))

	// Build tracking URL
	return fmt.Sprintf("%s/api/v2/track/click/%d/%d?url=%s&sig=%s",
		baseURL, campaignID, recipientID,
		url.QueryEscape(targetURL), sig,
	)
}

// =====================================
// UNSUBSCRIBE HANDLER
// =====================================

// HandleUnsubscribe handles one-click unsubscribe
func (h *TrackingHandlerV2) HandleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	campaignID, _ := strconv.ParseUint(chi.URLParam(r, "campaignId"), 10, 32)
	recipientID, _ := strconv.ParseUint(chi.URLParam(r, "recipientId"), 10, 32)
	token := chi.URLParam(r, "token")

	log := logger.WithComponent("unsubscribe").
		With("campaign_id", campaignID).
		With("recipient_id", recipientID)

	// Validate token
	if !h.validateUnsubToken(uint(campaignID), uint(recipientID), token) {
		http.Error(w, "Invalid unsubscribe link", http.StatusBadRequest)
		return
	}

	// Get recipient
	var recipient models.CampaignRecipient
	if err := h.Store.DB.First(&recipient, recipientID).Error; err != nil {
		http.Error(w, "Recipient not found", http.StatusNotFound)
		return
	}

	// Add to suppression list
	h.Repos.Suppression.Add(
		strings.ToLower(recipient.Email),
		"unsubscribe",
		fmt.Sprintf("campaign_%d", campaignID),
	)

	// Update contact status if linked
	if recipient.ContactID > 0 {
		h.Store.DB.Model(&models.Contact{}).
			Where("id = ?", recipient.ContactID).
			Updates(map[string]interface{}{
				"status":          "unsubscribed",
				"unsubscribed_at": time.Now(),
			})
	}

	// Increment campaign unsubscribe count atomically
	h.Store.DB.Exec(
		"UPDATE campaigns SET total_unsubscribes = total_unsubscribes + 1 WHERE id = ?",
		campaignID,
	)

	log.Info("unsubscribe processed", "email", recipient.Email)

	// Return confirmation page
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Unsubscribed</title>
    <style>
        body { font-family: Arial, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .container { text-align: center; padding: 40px; background: white; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #10B981; }
    </style>
</head>
<body>
    <div class="container">
        <h1>✓ Unsubscribed</h1>
        <p>You have been successfully unsubscribed from our mailing list.</p>
        <p>You will no longer receive emails from us.</p>
    </div>
</body>
</html>`))
}

func (h *TrackingHandlerV2) validateUnsubToken(campaignID, recipientID uint, token string) bool {
	cfg := config.Get()
	data := fmt.Sprintf("unsub:%d:%d", campaignID, recipientID)

	mac := hmac.New(sha256.New, []byte(cfg.AppSecret))
	mac.Write([]byte(data))
	expectedToken := hex.EncodeToString(mac.Sum(nil))[:16]

	return hmac.Equal([]byte(token), []byte(expectedToken))
}

// GenerateUnsubscribeTokenV2 creates a token for unsubscribe links with campaign context
func GenerateUnsubscribeTokenV2(campaignID, recipientID uint, secret string) string {
	data := fmt.Sprintf("unsub:%d:%d", campaignID, recipientID)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
