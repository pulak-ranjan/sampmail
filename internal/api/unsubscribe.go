package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type UnsubscribeHandler struct {
	Store *store.Store
}

func NewUnsubscribeHandler(st *store.Store) *UnsubscribeHandler {
	return &UnsubscribeHandler{Store: st}
}

// GenerateUnsubscribeToken creates a signed token for unsubscribe links
// Format: recipientID:timestamp:signature
func GenerateUnsubscribeToken(recipientID uint) string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%d:%d", recipientID, timestamp)
	signature := signUnsubscribeData(data)
	return fmt.Sprintf("%d:%d:%s", recipientID, timestamp, signature)
}

// GenerateUnsubscribeURL creates the full unsubscribe URL
func GenerateUnsubscribeURL(baseURL string, recipientID uint) string {
	token := GenerateUnsubscribeToken(recipientID)
	return fmt.Sprintf("%s/api/unsubscribe/%s", baseURL, token)
}

func signUnsubscribeData(data string) string {
	key, err := core.GetEncryptionKey()
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))[:16] // Truncate for shorter URLs
}

func verifyUnsubscribeToken(token string) (uint, bool) {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return 0, false
	}

	recipientID, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return 0, false
	}

	timestamp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, false
	}

	// Token expires after 90 days
	if time.Now().Unix()-timestamp > 90*24*60*60 {
		return 0, false
	}

	// Verify signature
	data := fmt.Sprintf("%d:%d", recipientID, timestamp)
	expectedSig := signUnsubscribeData(data)
	if expectedSig == "" || !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return 0, false
	}

	return uint(recipientID), true
}

// GET /api/unsubscribe/{token} - Show unsubscribe confirmation page
func (h *UnsubscribeHandler) HandleUnsubscribePage(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	recipientID, valid := verifyUnsubscribeToken(token)

	if !valid {
		h.renderPage(w, "Invalid or Expired Link", 
			"This unsubscribe link is invalid or has expired. Please contact support if you need assistance.", 
			false)
		return
	}

	// Get recipient info
	var recipient models.CampaignRecipient
	if err := h.Store.DB.First(&recipient, recipientID).Error; err != nil {
		h.renderPage(w, "Error", "Unable to find subscription information.", false)
		return
	}

	// Check if already unsubscribed
	suppressed, _ := h.Store.IsSuppressed(recipient.Email)
	if suppressed {
		h.renderPage(w, "Already Unsubscribed", 
			fmt.Sprintf("The email address %s has already been unsubscribed.", maskEmail(recipient.Email)), 
			true)
		return
	}

	// Show confirmation page
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Unsubscribe</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
               max-width: 500px; margin: 50px auto; padding: 20px; text-align: center; }
        .btn { background: #dc3545; color: white; padding: 12px 24px; border: none; 
               border-radius: 4px; cursor: pointer; font-size: 16px; text-decoration: none; }
        .btn:hover { background: #c82333; }
        .email { font-weight: bold; color: #333; }
    </style>
</head>
<body>
    <h1>Unsubscribe</h1>
    <p>You are about to unsubscribe <span class="email">%s</span> from our mailing list.</p>
    <p>You will no longer receive marketing emails from us.</p>
    <form method="POST" action="/api/unsubscribe/%s">
        <button type="submit" class="btn">Confirm Unsubscribe</button>
    </form>
</body>
</html>`, maskEmail(recipient.Email), token)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// POST /api/unsubscribe/{token} - Process unsubscribe (also handles List-Unsubscribe-Post)
func (h *UnsubscribeHandler) HandleUnsubscribeConfirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	recipientID, valid := verifyUnsubscribeToken(token)

	if !valid {
		// For one-click unsubscribe, return appropriate status
		if r.Header.Get("List-Unsubscribe") == "One-Click" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.renderPage(w, "Invalid Link", "This unsubscribe link is invalid or has expired.", false)
		return
	}

	// Get recipient info
	var recipient models.CampaignRecipient
	if err := h.Store.DB.First(&recipient, recipientID).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Add to suppression list
	err := h.Store.AddSuppression(recipient.Email, "unsubscribe", fmt.Sprintf("campaign:%d", recipient.CampaignID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// For one-click unsubscribe (RFC 8058), just return 200
	if r.Header.Get("List-Unsubscribe") == "One-Click" || 
	   r.URL.Query().Get("List-Unsubscribe") == "One-Click" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Show success page
	h.renderPage(w, "Unsubscribed Successfully", 
		fmt.Sprintf("The email address %s has been unsubscribed. You will no longer receive marketing emails from us.", 
			maskEmail(recipient.Email)), 
		true)
}

func (h *UnsubscribeHandler) renderPage(w http.ResponseWriter, title, message string, success bool) {
	color := "#dc3545"
	if success {
		color = "#28a745"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; 
               max-width: 500px; margin: 50px auto; padding: 20px; text-align: center; }
        h1 { color: %s; }
    </style>
</head>
<body>
    <h1>%s</h1>
    <p>%s</p>
</body>
</html>`, title, color, title, message)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// maskEmail partially hides an email for privacy
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}
	local := parts[0]
	domain := parts[1]
	
	if len(local) <= 2 {
		return local + "***@" + domain
	}
	return local[:2] + "***@" + domain
}
