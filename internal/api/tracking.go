package api

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// Transparent 1x1 GIF
var pixelGIF, _ = base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")

type TrackingHandler struct {
	Store *store.Store
}

func NewTrackingHandler(st *store.Store) *TrackingHandler {
	return &TrackingHandler{Store: st}
}

// GET /api/track/open/{id}?sig=...
func (h *TrackingHandler) HandleTrackOpen(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)
	signature := r.URL.Query().Get("sig")

	// Validate ID is positive
	if id <= 0 {
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Write(pixelGIF)
		return
	}

	// Verify signature to prevent fake tracking
	// The signature should be HMAC of the recipient ID
	expectedSig := core.SignLink(idStr)
	if signature == "" || !hmacEqual(expectedSig, signature) {
		// Still return pixel but don't record (prevents tracking manipulation)
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Write(pixelGIF)
		return
	}

	// Basic bot detection
	userAgent := strings.ToLower(r.UserAgent())
	if isBot(userAgent) {
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Write(pixelGIF)
		return
	}

	go h.recordOpen(uint(id), r)

	// Return transparent pixel
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(pixelGIF)
}

// GET /api/track/click/{id}?url=...&sig=...
func (h *TrackingHandler) HandleTrackClick(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, _ := strconv.Atoi(idStr)
	targetURL := r.URL.Query().Get("url")
	signature := r.URL.Query().Get("sig")

	// Security: Prevent Open Redirect using HMAC
	// We verify that the 'url' param was signed by our system.
	if targetURL == "" || !core.VerifyLinkSignature(targetURL, signature) {
		http.Error(w, "Invalid link signature", http.StatusForbidden)
		return
	}

	// Defense-in-depth: validate URL scheme even after HMAC verification
	parsedURL, err := url.Parse(targetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		http.Error(w, "Invalid redirect", http.StatusForbidden)
		return
	}

	// Basic bot detection
	userAgent := strings.ToLower(r.UserAgent())
	if !isBot(userAgent) && id > 0 {
		go h.recordClick(uint(id))
	}

	http.Redirect(w, r, targetURL, http.StatusFound)
}

// hmacEqual performs constant-time comparison for HMAC signatures
func hmacEqual(expected, actual string) bool {
	if len(expected) != len(actual) {
		return false
	}
	result := true
	for i := range expected {
		if expected[i] != actual[i] {
			result = false
		}
	}
	return result
}

// isBot detects common bots and crawlers
func isBot(userAgent string) bool {
	botPatterns := []string{
		"bot", "crawler", "spider", "scraper",
		"googlebot", "bingbot", "slurp", "duckduckbot",
		"baiduspider", "yandexbot", "facebookexternalhit",
		"twitterbot", "linkedinbot", "pinterest",
		"preview", "fetcher", "monitor", "check",
	}
	
	for _, pattern := range botPatterns {
		if strings.Contains(userAgent, pattern) {
			return true
		}
	}
	return false
}

func (h *TrackingHandler) recordOpen(id uint, r *http.Request) {
	var recip models.CampaignRecipient
	if err := h.Store.DB.First(&recip, id).Error; err != nil {
		return
	}

	// Update Recipient
	now := time.Now()
	if recip.OpenedAt == nil {
		recip.OpenedAt = &now
		h.Store.DB.Save(&recip)

		// Increment Campaign Stats
		var camp models.Campaign
		if err := h.Store.DB.First(&camp, recip.CampaignID).Error; err == nil {
			camp.TotalOpens++
			h.Store.DB.Save(&camp)
		}

		// Update Contact Score (AI Superlead)
		if recip.ContactID > 0 {
			var contact models.Contact
			if err := h.Store.DB.First(&contact, recip.ContactID).Error; err == nil {
				contact.TotalOpens++
				contact.Score += 1 // +1 for open
				h.Store.DB.Save(&contact)
			}
		}
	}
}

func (h *TrackingHandler) recordClick(id uint) {
	var recip models.CampaignRecipient
	if err := h.Store.DB.First(&recip, id).Error; err != nil {
		return
	}

	now := time.Now()
	if recip.ClickedAt == nil {
		recip.ClickedAt = &now
		h.Store.DB.Save(&recip)

		// Increment Campaign Stats
		var camp models.Campaign
		if err := h.Store.DB.First(&camp, recip.CampaignID).Error; err == nil {
			camp.TotalClicks++
			h.Store.DB.Save(&camp)
		}

		// Update Contact Score
		if recip.ContactID > 0 {
			var contact models.Contact
			if err := h.Store.DB.First(&contact, recip.ContactID).Error; err == nil {
				contact.TotalClicks++
				contact.Score += 5 // +5 for click (higher intent)
				h.Store.DB.Save(&contact)
			}
		}
	}
}
