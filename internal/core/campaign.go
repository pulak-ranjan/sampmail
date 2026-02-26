package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// CampaignService handles bulk sending logic with worker pool
type CampaignService struct {
	Store       *store.Store
	WorkerCount int
}

func NewCampaignService(st *store.Store) *CampaignService {
	cfg := config.Get()
	return &CampaignService{
		Store:       st,
		WorkerCount: cfg.CampaignWorkers,
	}
}

// recipientJob represents a single email to send
type recipientJob struct {
	Recipient    models.CampaignRecipient
	Campaign     models.Campaign
	Sender       models.Sender
	BaseURL      string  // tracking base URL
	UnsubBaseURL string  // unsubscribe base URL (may differ from tracking)
	Subject      string
}

// SendResult tracks the outcome of a send attempt
type SendResult struct {
	RecipientID uint
	Status      string
	Error       string
	SentAt      time.Time
}

// ImportRecipientsFromCSV imports recipients from CSV file
func (cs *CampaignService) ImportRecipientsFromCSV(campaignID uint, r io.Reader) error {
	log := logger.CampaignLogger(campaignID)
	cfg := config.Get()

	// Get org context for suppression checks
	var campaign models.Campaign
	var orgID uint
	if err := cs.Store.DB.First(&campaign, campaignID).Error; err == nil {
		orgID = campaign.OrganizationID
	}

	reader := csv.NewReader(r)
	batchSize := cfg.ImportBatchSize

	var recipients []models.CampaignRecipient
	var emailBatch []string
	lineNum := 0
	imported := 0
	skippedSuppressed := 0
	skippedInvalid := 0

	tx := cs.Store.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Error("panic during CSV import", "error", r)
		}
	}()

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++

		if lineNum > MaxRecipientsPerImport {
			tx.Rollback()
			return fmt.Errorf("import exceeds maximum recipient limit of %d", MaxRecipientsPerImport)
		}

		if err != nil {
			log.Warn("invalid CSV line", "line", lineNum, "error", err)
			skippedInvalid++
			continue
		}

		if len(record) < 1 {
			continue
		}
		email := strings.TrimSpace(strings.ToLower(record[0]))
		if email == "" || !strings.Contains(email, "@") || !isValidEmailFormat(email) {
			skippedInvalid++
			continue
		}

		emailBatch = append(emailBatch, email)
		recipients = append(recipients, models.CampaignRecipient{
			CampaignID: campaignID,
			Email:      email,
			Status:     "pending",
		})

		// Batch insert when we hit the threshold
		if len(recipients) >= batchSize {
			// Check suppression list before inserting
			suppressedMap, _ := cs.Store.BulkCheckSuppressed(orgID, emailBatch)

			var validRecipients []models.CampaignRecipient
			for _, rec := range recipients {
				if suppressedMap[strings.ToLower(rec.Email)] {
					skippedSuppressed++
					continue
				}
				validRecipients = append(validRecipients, rec)
			}

			if len(validRecipients) > 0 {
				if err := tx.Create(&validRecipients).Error; err != nil {
					log.Error("batch import failed", "error", err)
					tx.Rollback()
					return err
				}
				imported += len(validRecipients)
			}
			recipients = nil
			emailBatch = nil
		}
	}

	// Insert remaining recipients (with suppression check)
	if len(recipients) > 0 {
		suppressedMap, _ := cs.Store.BulkCheckSuppressed(orgID, emailBatch)

		var validRecipients []models.CampaignRecipient
		for _, rec := range recipients {
			if suppressedMap[strings.ToLower(rec.Email)] {
				skippedSuppressed++
				continue
			}
			validRecipients = append(validRecipients, rec)
		}

		if len(validRecipients) > 0 {
			if err := tx.Create(&validRecipients).Error; err != nil {
				log.Error("final batch import failed", "error", err)
				tx.Rollback()
				return err
			}
			imported += len(validRecipients)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return err
	}

	log.Info("CSV import completed",
		"lines_read", lineNum,
		"imported", imported,
		"skipped_suppressed", skippedSuppressed,
		"skipped_invalid", skippedInvalid)
	return nil
}

// isValidEmailFormat performs basic email format validation
func isValidEmailFormat(email string) bool {
	// Basic validation: must have exactly one @, local part and domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if len(local) == 0 || len(local) > 64 {
		return false
	}
	if len(domain) < 3 || !strings.Contains(domain, ".") {
		return false
	}
	// Check for common invalid patterns
	if strings.HasPrefix(email, ".") || strings.HasSuffix(email, ".") {
		return false
	}
	if strings.Contains(email, "..") {
		return false
	}
	return true
}

// Campaign thresholds
const (
	MaxBounceRatePercent       = 5.0
	MinEmailsBeforeBounceCheck = 100
	MaxComplaintRatePercent    = 0.1
	MaxRecipientsPerImport     = 500000
)

// StartCampaign starts sending emails for a campaign
func (cs *CampaignService) StartCampaign(campaignID uint) error {
	var campaign models.Campaign
	if err := cs.Store.DB.Preload("Sender").Preload("Sender.Domain").First(&campaign, campaignID).Error; err != nil {
		return err
	}

	if campaign.Status == "sending" || campaign.Status == "completed" {
		return fmt.Errorf("campaign is already %s", campaign.Status)
	}

	// Anti-spam: Verify sender has DKIM configured
	if campaign.Sender.Domain.ID > 0 {
		if !DKIMKeyExists(campaign.Sender.Domain.Name, campaign.Sender.LocalPart) {
			return fmt.Errorf("DKIM not configured for sender %s@%s - generate DKIM keys before sending",
				campaign.Sender.LocalPart, campaign.Sender.Domain.Name)
		}
	}

	// Anti-spam: Check if there are any recipients
	var recipientCount int64
	cs.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ? AND status = 'pending'", campaignID).Count(&recipientCount)
	if recipientCount == 0 {
		return fmt.Errorf("no pending recipients to send to")
	}

	campaign.Status = "sending"
	cs.Store.DB.Save(&campaign)

	go cs.processCampaignWithPool(campaign)

	return nil
}

// processCampaignWithPool uses a worker pool for concurrent sending
func (cs *CampaignService) processCampaignWithPool(c models.Campaign) {
	log := logger.CampaignLogger(c.ID)
	cfg := config.Get()

	defer func() {
		if r := recover(); r != nil {
			log.Error("panic in campaign processor", "error", r)
			c.Status = "failed"
			cs.Store.DB.Save(&c)
		}
	}()

	log.Info("starting campaign",
		"name", c.Name,
		"workers", cs.WorkerCount)

	sender := c.Sender
	safeSubject := strings.ReplaceAll(c.Subject, "\r", "")
	safeSubject = strings.ReplaceAll(safeSubject, "\n", "")

	// Determine Base URL with per-org custom domain support
	baseURL := buildBaseURL(cs.Store, c.OrganizationID, "tracking")
	unsubBaseURL := buildBaseURL(cs.Store, c.OrganizationID, "unsub")

	// =====================================
	// WARMUP INTEGRATION
	// =====================================
	// Check if sender is in warmup mode and apply rate limiting
	var warmupTicker *time.Ticker
	var warmupInterval time.Duration
	
	if sender.WarmupEnabled {
		warmupRate := GetWarmupRateForSender(&sender)
		warmupInterval = warmupRate.Interval
		
		log.Info("warmup mode enabled",
			"day", sender.WarmupDay,
			"plan", sender.WarmupPlan,
			"rate", fmt.Sprintf("%d/hr", warmupRate.Count),
			"interval", warmupInterval)
		
		// Set domain limiter warmup factor
		if dl := GetDomainLimiter(); dl != nil {
			warmupFactor := GetWarmupFactor(&sender)
			dl.SetWarmupFactor(sender.Domain.Name, warmupFactor)
		}
		
		// Create ticker for warmup throttling
		warmupTicker = time.NewTicker(warmupInterval)
		defer warmupTicker.Stop()
	}

	// Create channels
	jobs := make(chan recipientJob, cs.WorkerCount*2)
	results := make(chan SendResult, cs.WorkerCount*2)

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < cs.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			cs.sendWorker(ctx, workerID, jobs, results, cfg.SMTPAddr)
		}(i)
	}

	// Start result collector with bounce rate monitoring
	var totalSent, totalFailed, totalBounced atomic.Int64
	var campaignPaused atomic.Bool
	var resultsWg sync.WaitGroup
	resultsWg.Add(1)
	go func() {
		defer resultsWg.Done()
		for result := range results {
			// Update recipient in database
			update := map[string]interface{}{
				"status": result.Status,
			}
			if result.Error != "" {
				update["error"] = result.Error
			}
			if result.Status == "sent" {
				update["sent_at"] = result.SentAt
				totalSent.Add(1)
			} else {
				totalFailed.Add(1)
				// Track bounces separately for rate calculation
				if strings.Contains(strings.ToLower(result.Error), "bounce") ||
					strings.Contains(result.Error, "550") ||
					strings.Contains(result.Error, "551") ||
					strings.Contains(result.Error, "552") ||
					strings.Contains(result.Error, "553") ||
					strings.Contains(result.Error, "554") {
					totalBounced.Add(1)
				}
			}
			cs.Store.DB.Model(&models.CampaignRecipient{}).
				Where("id = ?", result.RecipientID).
				Updates(update)

			// Anti-spam: Check bounce rate and pause if too high
			sent := totalSent.Load()
			bounced := totalBounced.Load()
			if sent >= MinEmailsBeforeBounceCheck && !campaignPaused.Load() {
				bounceRate := float64(bounced) / float64(sent) * 100
				if bounceRate > MaxBounceRatePercent {
					campaignPaused.Store(true)
					log.Warn("campaign paused due to high bounce rate",
						"bounce_rate", fmt.Sprintf("%.2f%%", bounceRate),
						"threshold", fmt.Sprintf("%.2f%%", MaxBounceRatePercent),
						"sent", sent,
						"bounced", bounced)
					// Signal workers to stop
					cancel()
				}
			}
		}
	}()

	// Feed jobs to workers
	batchSize := 500
	for {
		var recipients []models.CampaignRecipient
		if err := cs.Store.DB.Where("campaign_id = ? AND status = 'pending'", c.ID).
			Limit(batchSize).Find(&recipients).Error; err != nil {
			log.Error("failed to fetch recipients", "error", err)
			break
		}

		if len(recipients) == 0 {
			break
		}

		// Bulk check suppression
		emails := make([]string, len(recipients))
		for i, r := range recipients {
			emails[i] = r.Email
		}
		suppressedMap, _ := cs.Store.BulkCheckSuppressed(c.OrganizationID, emails)

		for _, r := range recipients {
			// Skip suppressed
			if suppressedMap[strings.ToLower(r.Email)] {
				results <- SendResult{
					RecipientID: r.ID,
					Status:      "skipped",
					Error:       "suppressed",
				}
				continue
			}

			// =====================================
			// WARMUP & DOMAIN RATE LIMITING
			// =====================================
			// Wait for warmup ticker if enabled
			if warmupTicker != nil {
				<-warmupTicker.C
			}
			
			// Check per-domain rate limit
			destDomain := ExtractDomain(r.Email)
			if allowed, waitTime, err := CheckDomainRateLimit(ctx, destDomain); err == nil && !allowed {
				log.Debug("domain rate limit reached, waiting",
					"domain", destDomain,
					"wait_time", waitTime)
				time.Sleep(waitTime)
			}

			jobs <- recipientJob{
				Recipient:    r,
				Campaign:     c,
				Sender:       sender,
				BaseURL:      baseURL,
				UnsubBaseURL: unsubBaseURL,
				Subject:      safeSubject,
			}
		}

		// Periodically update campaign stats
		cs.Store.DB.Model(&c).Updates(map[string]interface{}{
			"total_sent":   totalSent.Load(),
			"total_failed": totalFailed.Load(),
		})
	}

	// Close jobs channel to signal workers to finish
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	// Close results channel and wait for collector
	close(results)
	resultsWg.Wait()

	// Update final campaign status
	c.TotalSent = int(totalSent.Load())
	c.TotalFailed = int(totalFailed.Load())

	// Set status based on whether campaign was paused due to bounce rate
	if campaignPaused.Load() {
		c.Status = "paused"
		log.Warn("campaign paused due to high bounce rate",
			"sent", c.TotalSent,
			"failed", c.TotalFailed,
			"bounced", totalBounced.Load())
	} else {
		c.Status = "completed"
		log.Info("campaign completed",
			"sent", c.TotalSent,
			"failed", c.TotalFailed)
	}
	cs.Store.DB.Save(&c)
}

// sendWorker is a worker goroutine that sends emails
func (cs *CampaignService) sendWorker(ctx context.Context, id int, jobs <-chan recipientJob, results chan<- SendResult, smtpAddr string) {
	log := logger.WithComponent("send_worker").With("worker_id", id)

	// Create persistent connection
	var client *smtp.Client
	var conn net.Conn

	reconnect := func() error {
		if client != nil {
			client.Close()
		}
		if conn != nil {
			conn.Close()
		}

		var err error
		conn, err = net.DialTimeout("tcp", smtpAddr, 5*time.Second)
		if err != nil {
			return err
		}

		client, err = smtp.NewClient(conn, "localhost")
		return err
	}

	if err := reconnect(); err != nil {
		log.Error("failed to connect to SMTP", "error", err)
		return
	}
	defer func() {
		if client != nil {
			client.Quit()
		}
	}()

	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		result := cs.sendSingleRecipient(client, job, &reconnect)
		results <- result
	}
}

// sendSingleRecipient sends to one recipient
func (cs *CampaignService) sendSingleRecipient(client *smtp.Client, job recipientJob, reconnect *func() error) SendResult {
	result := SendResult{
		RecipientID: job.Recipient.ID,
		Status:      "failed",
	}

	// Try MAIL command, reconnect if needed
	if err := client.Mail(job.Sender.Email); err != nil {
		if err = (*reconnect)(); err != nil {
			result.Error = "connection failed: " + err.Error()
			return result
		}
		if err = client.Mail(job.Sender.Email); err != nil {
			result.Error = "mail cmd failed: " + err.Error()
			return result
		}
	}

	if err := client.Rcpt(job.Recipient.Email); err != nil {
		result.Error = "rcpt rejected: " + err.Error()
		client.Reset()
		return result
	}

	wc, err := client.Data()
	if err != nil {
		result.Error = "data failed: " + err.Error()
		client.Reset()
		return result
	}

	// Build message with proper headers and personalization
	unsubToken := generateUnsubToken(job.Recipient.ID)
	unsubURL := fmt.Sprintf("%s/api/unsubscribe/%s", job.UnsubBaseURL, unsubToken)

	openID := strconv.FormatUint(uint64(job.Recipient.ID), 10)
	openSig := SignLink(openID)
	trackingOpenURL := fmt.Sprintf("%s/api/track/open/%d?sig=%s", job.BaseURL, job.Recipient.ID, openSig)
	pixel := fmt.Sprintf(`<img src="%s" alt="" width="1" height="1" style="display:none" />`, trackingOpenURL)

	// Apply personalization to email body
	pe := NewPersonalizationEngine()
	
	// Build personalization context
	ctx := &PersonalizationContext{
		CustomData: map[string]interface{}{
			"email": job.Recipient.Email,
		},
		UnsubscribeURL: unsubURL,
		TrackingPixel:  pixel,
		CurrentDate:    time.Now(),
	}
	
	// Try to load contact data for personalization
	if job.Recipient.ContactID > 0 {
		var contact models.ContactV2
		if err := cs.Store.DB.First(&contact, job.Recipient.ContactID).Error; err == nil {
			ctx.Contact = &contact
			// Mirror common fields into CustomData for templates using direct variables.
			ctx.CustomData["first_name"] = contact.FirstName
			ctx.CustomData["last_name"] = contact.LastName
			ctx.CustomData["company"] = contact.Company
		}
	}
	
	// Render personalized content
	personalizedBody, err := pe.Render(job.Campaign.Body, ctx)
	if err != nil {
		// Log but continue with original body
		personalizedBody = job.Campaign.Body
	}
	
	// Rewrite links for tracking
	bodyWithLinks := rewriteLinks(personalizedBody, job.BaseURL, job.Recipient.ID)
	
	// Generate plain text version from HTML
	textBody := htmlToPlainText(bodyWithLinks)
	
	// Build multipart message
	boundary := fmt.Sprintf("----=_Part_%d_%d", job.Recipient.ID, time.Now().UnixNano())
	messageID := fmt.Sprintf("<%d.%d@%s>", job.Recipient.ID, time.Now().UnixNano(), senderDomainForMessageID(job.Sender.Email))
	
	// Build headers with all required fields
	headers := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Message-ID: %s\r\n"+
		"Date: %s\r\n"+
		"MIME-Version: 1.0\r\n"+
		"X-Campaign: %d\r\n"+
		"List-Unsubscribe: <%s>\r\n"+
		"List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n"+
		"Content-Type: multipart/alternative; boundary=\"%s\"\r\n"+
		"\r\n",
		job.Sender.Email, job.Recipient.Email, job.Subject, messageID, 
		time.Now().Format(time.RFC1123Z), job.Campaign.ID, unsubURL, boundary)

	// Build multipart body
	multipartBody := fmt.Sprintf("--%s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"Content-Transfer-Encoding: quoted-printable\r\n"+
		"\r\n%s\r\n\r\n"+
		"--%s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n"+
		"Content-Transfer-Encoding: quoted-printable\r\n"+
		"\r\n%s\r\n\r\n"+
		"--%s--\r\n",
		boundary, textBody, boundary, bodyWithLinks+"\n"+pixel, boundary)

	msg := headers + multipartBody

	if _, err = wc.Write([]byte(msg)); err != nil {
		result.Error = "write failed: " + err.Error()
		wc.Close()
		return result
	}

	if err = wc.Close(); err != nil {
		result.Error = "close failed: " + err.Error()
		return result
	}

	result.Status = "sent"
	result.SentAt = time.Now()
	return result
}

// htmlToPlainText converts HTML to plain text for multipart emails
func htmlToPlainText(html string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, " ")
	
	// Decode HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	
	// Clean up whitespace
	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")
	
	return strings.TrimSpace(text)
}

// senderDomainForMessageID extracts domain from sender address for Message-ID.
// Kept local to this file to avoid collisions with stats helper names.
func senderDomainForMessageID(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return "localhost"
}

// ResumeInterruptedCampaigns finds campaigns stuck in "sending" and restarts them
func (cs *CampaignService) ResumeInterruptedCampaigns() error {
	log := logger.WithComponent("campaign")

	var campaigns []models.Campaign
	if err := cs.Store.DB.Where("status = 'sending'").Find(&campaigns).Error; err != nil {
		return err
	}

	for _, c := range campaigns {
		if err := cs.Store.DB.Preload("Sender").Preload("Sender.Domain").First(&c, c.ID).Error; err != nil {
			log.Error("failed to reload campaign", "id", c.ID, "error", err)
			continue
		}
		log.Info("resuming interrupted campaign", "id", c.ID, "name", c.Name)
		go cs.processCampaignWithPool(c)
	}
	return nil
}

// StartScheduledCampaigns finds campaigns ready to send and launches them
func (cs *CampaignService) StartScheduledCampaigns() error {
	log := logger.WithComponent("campaign")

	var campaigns []models.Campaign
	now := time.Now()

	if err := cs.Store.DB.Where("status = 'scheduled' AND scheduled_at <= ?", now).Find(&campaigns).Error; err != nil {
		return err
	}

	for _, c := range campaigns {
		// Atomic update to prevent race conditions
		result := cs.Store.DB.Model(&c).Where("status = 'scheduled'").Update("status", "sending")
		if result.RowsAffected == 0 {
			continue
		}

		if err := cs.Store.DB.Preload("Sender").Preload("Sender.Domain").First(&c, c.ID).Error; err != nil {
			log.Error("failed to load scheduled campaign", "id", c.ID, "error", err)
			continue
		}

		log.Info("starting scheduled campaign", "id", c.ID, "name", c.Name)
		go cs.processCampaignWithPool(c)
	}
	return nil
}

// Helper functions

func generateUnsubToken(recipientID uint) string {
	timestamp := time.Now().Unix()
	data := fmt.Sprintf("%d:%d", recipientID, timestamp)
	signature := signUnsubData(data)
	return fmt.Sprintf("%d:%d:%s", recipientID, timestamp, signature)
}

func signUnsubData(data string) string {
	key, err := GetEncryptionKey()
	if err != nil {
		return ""
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func rewriteLinks(html string, baseURL string, recipientID uint) string {
	re := regexp.MustCompile(`href=["'](http[^"']+)["']`)

	return re.ReplaceAllStringFunc(html, func(match string) string {
		quote := match[5:6]
		originalURL := match[6 : len(match)-1]
		encodedURL := url.QueryEscape(originalURL)
		signature := SignLink(originalURL)
		trackingURL := fmt.Sprintf("%s/api/track/click/%d?url=%s&sig=%s",
			baseURL, recipientID, encodedURL, signature)
		return fmt.Sprintf("href=%s%s%s", quote, trackingURL, quote)
	})
}

// buildBaseURL returns the appropriate base URL for tracking or unsubscribe links.
// Priority: verified org custom domain > AppSettings.MainHostname > "http://localhost:9000"
func buildBaseURL(st *store.Store, orgID uint, domainType string) string {
	// 1. Try org-level custom domain config (verified only)
	if orgID > 0 {
		if cfg, err := st.GetOrgDomainConfig(orgID); err == nil {
			switch domainType {
			case "tracking":
				if cfg.TrackingDomainVerified && cfg.TrackingDomain != "" {
					return "https://" + cfg.TrackingDomain
				}
			case "unsub":
				if cfg.UnsubscribeDomainVerified && cfg.UnsubscribeDomain != "" {
					return "https://" + cfg.UnsubscribeDomain
				}
			}
		}
	}

	// 2. Fall back to system settings
	if settings, err := st.GetSettings(); err == nil && settings.MainHostname != "" {
		protocol := "https"
		if settings.MainHostname == "localhost" {
			protocol = "http"
		}
		return fmt.Sprintf("%s://%s", protocol, settings.MainHostname)
	}

	// 3. Final fallback
	return "http://localhost:9000"
}

// SendSingleEmail sends a transactional email
func (cs *CampaignService) SendSingleEmail(to, subject, body string, senderID uint) error {
	cfg := config.Get()

	var sender models.Sender
	if err := cs.Store.DB.Preload("Domain").First(&sender, senderID).Error; err != nil {
		return fmt.Errorf("sender not found: %v", err)
	}

	conn, err := net.DialTimeout("tcp", cfg.SMTPAddr, 5*time.Second)
	if err != nil {
		return err
	}

	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Quit()

	if err := client.Mail(sender.Email); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}

	wc, err := client.Data()
	if err != nil {
		return err
	}

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n",
		sender.Email, to, subject)

	if _, err = wc.Write([]byte(headers + body)); err != nil {
		return err
	}
	return wc.Close()
}
