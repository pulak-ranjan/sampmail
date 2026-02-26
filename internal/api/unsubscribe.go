package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type UnsubscribeHandler struct {
	Store *store.Store
}

func NewUnsubscribeHandler(st *store.Store) *UnsubscribeHandler {
	return &UnsubscribeHandler{Store: st}
}

// =====================================
// RFC 8058 ONE-CLICK UNSUBSCRIBE
// =====================================
// Implements Gmail, Outlook, Yahoo one-click unsubscribe
// https://datatracker.ietf.org/doc/html/rfc8058

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

// GenerateRFC8058Headers generates the List-Unsubscribe headers for one-click unsubscribe
// Returns the headers to be included in the email
func GenerateRFC8058Headers(baseURL string, recipientID uint, senderEmail string) string {
	unsubscribeURL := GenerateUnsubscribeURL(baseURL, recipientID)
	
	// RFC 8058 requires both List-Unsubscribe and List-Unsubscribe-Post headers
	// The URL must support POST requests
	return fmt.Sprintf(
		"List-Unsubscribe: <%s>\r\n"+
		"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n",
		unsubscribeURL,
	)
}

// GenerateRFC8058HeadersWithMailto generates headers with both HTTPS and mailto options
func GenerateRFC8058HeadersWithMailto(baseURL string, recipientID uint, senderEmail string) string {
	unsubscribeURL := GenerateUnsubscribeURL(baseURL, recipientID)
	mailto := fmt.Sprintf("mailto:%s?subject=Unsubscribe%%20Request", senderEmail)
	
	return fmt.Sprintf(
		"List-Unsubscribe: <%s>, <%s>\r\n"+
		"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n",
		unsubscribeURL, mailto,
	)
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

	// Resolve org context from the campaign so suppression is properly scoped
	var campaign models.Campaign
	var recipientOrgID uint
	if err := h.Store.DB.Select("organization_id").First(&campaign, recipient.CampaignID).Error; err == nil {
		recipientOrgID = campaign.OrganizationID
	}

	// Check if already unsubscribed
	suppressed, _ := h.Store.IsSuppressed(recipientOrgID, recipient.Email)
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
// This endpoint handles both:
// 1. Regular form-based unsubscribe (user clicks link, confirms on page)
// 2. RFC 8058 one-click unsubscribe (Gmail, Outlook, Yahoo buttons)
func (h *UnsubscribeHandler) HandleUnsubscribeConfirm(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	
	// Check if this is an RFC 8058 one-click unsubscribe
	isOneClick := isRFC8058Request(r)
	
	log := logger.WithComponent("unsubscribe")
	
	recipientID, valid := verifyUnsubscribeToken(token)

	if !valid {
		if isOneClick {
			// RFC 8058: Return 400 for invalid token
			log.Warn("RFC 8058: invalid token")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h.renderPage(w, "Invalid Link", "This unsubscribe link is invalid or has expired.", false)
		return
	}

	// Get recipient info
	var recipient models.CampaignRecipient
	if err := h.Store.DB.First(&recipient, recipientID).Error; err != nil {
		if isOneClick {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h.renderPage(w, "Error", "Unable to find subscription information.", false)
		return
	}

	// Resolve org context for scoped suppression
	var unsub_campaign models.Campaign
	var confirmOrgID uint
	if err := h.Store.DB.Select("organization_id").First(&unsub_campaign, recipient.CampaignID).Error; err == nil {
		confirmOrgID = unsub_campaign.OrganizationID
	}

	// Add to suppression list (org-scoped)
	err := h.Store.AddSuppression(confirmOrgID, recipient.Email, "unsubscribe", fmt.Sprintf("campaign:%d", recipient.CampaignID))
	if err != nil {
		if isOneClick {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		h.renderPage(w, "Error", "An error occurred. Please try again later.", false)
		return
	}
	
	// Update contact status
	h.Store.DB.Model(&models.ContactV2{}).
		Where("email = ?", recipient.Email).
		Update("status", "unsubscribed")

	// Log the unsubscribe
	log.Info("unsubscribe processed",
		"email", maskEmail(recipient.Email),
		"campaign_id", recipient.CampaignID,
		"one_click", isOneClick)

	// For RFC 8058 one-click unsubscribe, return 200 OK
	// This is what Gmail, Outlook, Yahoo expect
	if isOneClick {
		log.Debug("RFC 8058 one-click unsubscribe successful")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Show success page for regular unsubscribe
	h.renderPage(w, "Unsubscribed Successfully", 
		fmt.Sprintf("The email address %s has been unsubscribed. You will no longer receive marketing emails from us.", 
			maskEmail(recipient.Email)), 
		true)
}

// isRFC8058Request checks if this is an RFC 8058 one-click unsubscribe request
func isRFC8058Request(r *http.Request) bool {
	// Check for List-Unsubscribe-Post header (RFC 8058)
	if r.Header.Get("List-Unsubscribe-Post") == "List-Unsubscribe=One-Click" {
		return true
	}
	
	// Some providers send it as a query parameter
	if r.URL.Query().Get("List-Unsubscribe-Post") == "List-Unsubscribe=One-Click" {
		return true
	}
	
	// Check Content-Type for form-urlencoded (typical for one-click)
	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		// Check if body has List-Unsubscribe parameter
		r.ParseForm()
		if r.FormValue("List-Unsubscribe") == "One-Click" {
			return true
		}
	}
	
	return false
}

// HandleOneClickUnsubscribe handles explicit RFC 8058 POST requests
// Endpoint: POST /api/unsubscribe/one-click
func (h *UnsubscribeHandler) HandleOneClickUnsubscribe(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// Get parameters
	// Format: email + campaign_id + signature
	email := r.FormValue("email")
	campaignIDStr := r.FormValue("campaign")
	signature := r.FormValue("sig")
	
	if email == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// Verify signature
	if !VerifyOneClickSignature(email, campaignIDStr, signature) {
		logger.WithComponent("unsubscribe").Warn("one-click: invalid signature")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	
	// Parse campaign ID
	var campaignID uint
	if campaignIDStr != "" {
		id, err := strconv.ParseUint(campaignIDStr, 10, 32)
		if err == nil {
			campaignID = uint(id)
		}
	}
	
	// Resolve org context for scoped suppression
	var oneClickOrgID uint
	if campaignID > 0 {
		var oc models.Campaign
		if err := h.Store.DB.Select("organization_id").First(&oc, campaignID).Error; err == nil {
			oneClickOrgID = oc.OrganizationID
		}
	}

	// Add to suppression (org-scoped)
	source := "rfc8058_one_click"
	if campaignID > 0 {
		source = fmt.Sprintf("campaign:%d", campaignID)
	}

	if err := h.Store.AddSuppression(oneClickOrgID, email, "unsubscribe", source); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	
	// Update contact status
	h.Store.DB.Model(&models.ContactV2{}).
		Where("email = ?", email).
		Update("status", "unsubscribed")
	
	logger.WithComponent("unsubscribe").Info("RFC 8058 one-click unsubscribe processed",
		"email", maskEmail(email),
		"campaign_id", campaignID)
	
	// Return 200 OK as per RFC 8058
	w.WriteHeader(http.StatusOK)
}

// GenerateOneClickURL generates a URL for one-click unsubscribe
func GenerateOneClickURL(baseURL, email string, campaignID uint) string {
	sig := SignOneClickEmail(email, campaignID)
	params := url.Values{
		"email":    {email},
		"campaign": {fmt.Sprintf("%d", campaignID)},
		"sig":      {sig},
	}
	return fmt.Sprintf("%s/api/unsubscribe/one-click?%s", baseURL, params.Encode())
}

// SignOneClickEmail signs an email for one-click unsubscribe
func SignOneClickEmail(email string, campaignID uint) string {
	data := fmt.Sprintf("%s:%d", strings.ToLower(email), campaignID)
	key, err := core.GetEncryptionKey()
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

// VerifyOneClickSignature verifies a one-click unsubscribe signature
func VerifyOneClickSignature(email, campaignIDStr, signature string) bool {
	if signature == "" {
		return false
	}
	
	var campaignID uint
	if id, err := strconv.ParseUint(campaignIDStr, 10, 32); err == nil {
		campaignID = uint(id)
	}
	
	expectedSig := SignOneClickEmail(email, campaignID)
	if expectedSig == "" {
		return false
	}
	
	return hmac.Equal([]byte(expectedSig), []byte(signature))
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
