package core

import (
	"context"
	"fmt"
	"mime"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// FEEDBACK LOOP (FBL) PROCESSOR
// =====================================
// Processes spam complaints from email providers' feedback loops
// Supports ARF (Abuse Reporting Format) and JMRP formats

// FBLReport represents a parsed feedback loop report
type FBLReport struct {
	OriginalRecipient   string    `json:"original_recipient"`
	OriginalMessageID   string    `json:"original_message_id"`
	OriginalSubject     string    `json:"original_subject"`
	FeedbackType        string    `json:"feedback_type"` // abuse, fraud, not-spam, opt-out, other, virus
	ReceivedDate        time.Time `json:"received_date"`
	ReportingMTA        string    `json:"reporting_mta"`
	SourceIP            string    `json:"source_ip"`
	ArrivalDate         time.Time `json:"arrival_date"`
	OriginalMailFrom    string    `json:"original_mail_from"`
	OriginalReturnPath  string    `json:"original_return_path"`
	CampaignID          uint      `json:"campaign_id"`
	UserAgent           string    `json:"user_agent"`
	Provider            string    `json:"provider"` // gmail, outlook, yahoo, etc.
	ReportType          string    `json:"report_type"` // arf, jmrp, aol
}

// FBLProcessor handles feedback loop processing
type FBLProcessor struct {
	store *store.Store
}

// NewFBLProcessor creates a new FBL processor
func NewFBLProcessor(st *store.Store) *FBLProcessor {
	return &FBLProcessor{store: st}
}

// ParseFBLReport parses an FBL email message
func (fp *FBLProcessor) ParseFBLReport(rawMessage []byte) (*FBLReport, error) {
	report := &FBLReport{
		ReceivedDate: time.Now(),
	}
	
	// Parse the email message
	msg, err := mail.ReadMessage(strings.NewReader(string(rawMessage)))
	if err != nil {
		return nil, fmt.Errorf("failed to parse message: %w", err)
	}
	
	// Get content type
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, _ := mime.ParseMediaType(contentType)
	
	// Detect report type based on content type and headers
	if strings.Contains(mediaType, "multipart/report") {
		report.ReportType = "arf"
		fp.parseARFReport(msg, params, report)
	} else if strings.Contains(strings.ToLower(msg.Header.Get("X-AOL-IP")), ".") {
		report.ReportType = "aol"
		fp.parseAOLReport(msg, report)
	} else if strings.Contains(msg.Header.Get("X-Loop"), "jmrp") {
		report.ReportType = "jmrp"
		fp.parseJMRPReport(msg, report)
	} else {
		// Try to parse as generic complaint
		fp.parseGenericComplaint(msg, report)
	}
	
	// Detect provider
	report.Provider = detectProvider(msg.Header.Get("From"), msg.Header.Get("Received"))
	
	// Extract campaign ID from headers if present
	report.CampaignID = extractCampaignIDFromHeaders(msg.Header)
	
	return report, nil
}

// parseARFReport parses an ARF (Abuse Reporting Format) report
func (fp *FBLProcessor) parseARFReport(msg *mail.Message, params map[string]string, report *FBLReport) {
	// ARF reports have 3 parts: human-readable, machine-readable, original message
	// The report type is in the machine-readable part
	
	// Parse common ARF headers
	report.FeedbackType = msg.Header.Get("Feedback-Type")
	report.OriginalRecipient = msg.Header.Get("Original-Rcpt-To")
	report.OriginalMessageID = msg.Header.Get("Original-Message-ID")
	report.OriginalMailFrom = msg.Header.Get("Original-Mail-From")
	report.OriginalReturnPath = msg.Header.Get("Original-Return-Path")
	report.ReportingMTA = msg.Header.Get("Reporting-MTA")
	report.SourceIP = msg.Header.Get("Source-IP")
	report.UserAgent = msg.Header.Get("User-Agent")
	
	// Parse arrival date
	if arrivalStr := msg.Header.Get("Arrival-Date"); arrivalStr != "" {
		if t, err := mail.ParseDate(arrivalStr); err == nil {
			report.ArrivalDate = t
		}
	}
	
	// Parse received date
	if receivedStr := msg.Header.Get("Received-Date"); receivedStr != "" {
		if t, err := mail.ParseDate(receivedStr); err == nil {
			report.ReceivedDate = t
		}
	}
}

// parseAOLReport parses AOL-specific FBL format
func (fp *FBLProcessor) parseAOLReport(msg *mail.Message, report *FBLReport) {
	report.FeedbackType = "abuse"
	report.OriginalRecipient = msg.Header.Get("X-AOL-IP") // AOL sometimes puts recipient here
	report.SourceIP = msg.Header.Get("X-AOL-IP")
	report.UserAgent = "AOL"
	
	// AOL FBL has specific headers
	if rcpt := msg.Header.Get("X-Original-To"); rcpt != "" {
		report.OriginalRecipient = rcpt
	}
}

// parseJMRPReport parses JMRP (Junk Mail Reporting Program) format (Microsoft)
func (fp *FBLProcessor) parseJMRPReport(msg *mail.Message, report *FBLReport) {
	report.FeedbackType = "abuse"
	report.Provider = "outlook"
	report.UserAgent = "JMRP"
	
	// JMRP specific headers
	if rcpt := msg.Header.Get("X-Original-To"); rcpt != "" {
		report.OriginalRecipient = rcpt
	}
	if from := msg.Header.Get("X-Original-From"); from != "" {
		report.OriginalMailFrom = from
	}
}

// parseGenericComplaint attempts to parse a generic complaint email
func (fp *FBLProcessor) parseGenericComplaint(msg *mail.Message, report *FBLReport) {
	report.FeedbackType = "abuse"
	
	// Try to extract recipient from subject or body
	subject := msg.Header.Get("Subject")
	if strings.Contains(strings.ToLower(subject), "complaint") {
		report.FeedbackType = "abuse"
	}
	
	// Extract from Return-Path
	if returnPath := msg.Header.Get("Return-Path"); returnPath != "" {
		report.OriginalReturnPath = strings.Trim(returnPath, "<>")
	}
}

// ProcessFBLReport processes an FBL report and takes action
func (fp *FBLProcessor) ProcessFBLReport(report *FBLReport) error {
	log := logger.WithComponent("fbl_processor")
	
	if report.OriginalRecipient == "" {
		return fmt.Errorf("no recipient in FBL report")
	}
	
	// Normalize email
	report.OriginalRecipient = strings.ToLower(strings.TrimSpace(report.OriginalRecipient))
	
	log.Info("processing FBL report",
		"recipient", report.OriginalRecipient,
		"feedback_type", report.FeedbackType,
		"provider", report.Provider,
		"campaign_id", report.CampaignID)
	
	// 1. Add to suppression list
	reason := fmt.Sprintf("complaint_%s", report.FeedbackType)
	source := fmt.Sprintf("fbl_%s", report.Provider)
	
	// orgID=0: FBL suppressions apply system-wide across all orgs
	if err := fp.store.AddSuppression(0, report.OriginalRecipient, reason, source); err != nil {
		log.Warn("failed to add suppression", "error", err)
	}
	
	// 2. Update contact status
	if err := fp.store.DB.Model(&models.ContactV2{}).
		Where("email = ?", report.OriginalRecipient).
		Update("status", "complained").Error; err != nil {
		log.Debug("contact not found for complaint update", "email", report.OriginalRecipient)
	}
	
	// 3. Log the complaint for analytics
	complaintLog := &models.ComplaintLog{
		Email:       report.OriginalRecipient,
		FeedbackType: report.FeedbackType,
		Provider:    report.Provider,
		CampaignID:  report.CampaignID,
		ReceivedAt:  report.ReceivedDate,
		SourceIP:    report.SourceIP,
	}
	
	if err := fp.store.DB.Create(complaintLog).Error; err != nil {
		log.Warn("failed to log complaint", "error", err)
	}
	
	// 4. Update sender reputation if we can identify the sender
	if report.CampaignID > 0 {
		var campaign models.Campaign
		if err := fp.store.DB.First(&campaign, report.CampaignID).Error; err == nil {
			// Decrement sender reputation
			fp.updateSenderReputation(campaign.SenderID, -20)
		}
	}
	
	// 5. Check complaint rate and alert if high
	fp.checkComplaintRate(report)
	
	return nil
}

// updateSenderReputation updates the sender's reputation score
func (fp *FBLProcessor) updateSenderReputation(senderID uint, delta int) {
	if senderID == 0 {
		return
	}
	
	// Update reputation score (lower is worse)
	fp.store.DB.Model(&models.Sender{}).
		Where("id = ?", senderID).
		UpdateColumn("reputation_score", 
			fp.store.DB.Raw("GREATEST(0, reputation_score + ?)", delta))
}

// checkComplaintRate checks if complaint rate exceeds threshold
func (fp *FBLProcessor) checkComplaintRate(report *FBLReport) {
	if report.CampaignID == 0 {
		return
	}
	
	// Get campaign stats
	var campaign models.Campaign
	if err := fp.store.DB.First(&campaign, report.CampaignID).Error; err != nil {
		return
	}
	
	if campaign.TotalSent < 100 {
		return // Not enough data
	}
	
	// Count complaints for this campaign
	var complaintCount int64
	fp.store.DB.Model(&models.ComplaintLog{}).
		Where("campaign_id = ?", report.CampaignID).
		Count(&complaintCount)
	
	complaintRate := float64(complaintCount) / float64(campaign.TotalSent) * 100
	
	if complaintRate > MaxComplaintRatePercent {
		logger.WithComponent("fbl_processor").Warn("high complaint rate detected",
			"campaign_id", report.CampaignID,
			"complaint_rate", fmt.Sprintf("%.3f%%", complaintRate),
			"threshold", fmt.Sprintf("%.3f%%", MaxComplaintRatePercent),
			"complaints", complaintCount,
			"sent", campaign.TotalSent)
		
		// Could trigger: pause campaign, alert admin, etc.
	}
}

// =====================================
// PROVIDER DETECTION
// =====================================

var providerPatterns = map[string]*regexp.Regexp{
	"gmail":    regexp.MustCompile(`(?i)(gmail|googlemail)\.com`),
	"outlook":  regexp.MustCompile(`(?i)(outlook|hotmail|live|msn)\.com`),
	"yahoo":    regexp.MustCompile(`(?i)(yahoo|aol|verizon)\.(com|net|co\.[a-z]{2})`),
	"apple":    regexp.MustCompile(`(?i)(icloud|me|mac)\.com`),
	"comcast":  regexp.MustCompile(`(?i)comcast\.net`),
	"cox":      regexp.MustCompile(`(?i)cox\.net`),
}

func detectProvider(from string, received string) string {
	fromLower := strings.ToLower(from)
	receivedLower := strings.ToLower(received)
	
	for provider, pattern := range providerPatterns {
		if pattern.MatchString(fromLower) || pattern.MatchString(receivedLower) {
			return provider
		}
	}
	
	return "unknown"
}

// extractCampaignIDFromHeaders extracts campaign ID from email headers
func extractCampaignIDFromHeaders(headers mail.Header) uint {
	// Check common campaign tracking headers
	campaignHeaders := []string{
		"X-Campaign-ID",
		"X-Campaign",
		"X-Mailer-Campaign",
		"X-Entity-ID",
	}
	
	for _, h := range campaignHeaders {
		if val := headers.Get(h); val != "" {
			var id uint
			if _, err := fmt.Sscanf(val, "%d", &id); err == nil {
				return id
			}
		}
	}
	
	return 0
}

// =====================================
// IMAP FBL MONITOR
// =====================================

// FBLMonitor monitors an IMAP mailbox for FBL reports
type FBLMonitor struct {
	processor *FBLProcessor
	config    FBLMonitorConfig
	stopCh    chan struct{}
}

// FBLMonitorConfig holds IMAP configuration
type FBLMonitorConfig struct {
	Server   string
	Port     int
	Username string
	Password string
	Mailbox  string // Default: "INBOX"
	PollInterval time.Duration
}

// NewFBLMonitor creates a new FBL monitor
func NewFBLMonitor(processor *FBLProcessor, config FBLMonitorConfig) *FBLMonitor {
	if config.Mailbox == "" {
		config.Mailbox = "INBOX"
	}
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Minute
	}
	
	return &FBLMonitor{
		processor: processor,
		config:    config,
		stopCh:    make(chan struct{}),
	}
}

// Start begins monitoring for FBL reports
func (fm *FBLMonitor) Start(ctx context.Context) error {
	log := logger.WithComponent("fbl_monitor")
	
	log.Info("starting FBL monitor",
		"server", fm.config.Server,
		"mailbox", fm.config.Mailbox)
	
	ticker := time.NewTicker(fm.config.PollInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-fm.stopCh:
			log.Info("FBL monitor stopped")
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := fm.pollMailbox(ctx); err != nil {
				log.Warn("failed to poll FBL mailbox", "error", err)
			}
		}
	}
}

// Stop stops the FBL monitor
func (fm *FBLMonitor) Stop() {
	close(fm.stopCh)
}

// pollMailbox polls the IMAP mailbox for new FBL reports
func (fm *FBLMonitor) pollMailbox(ctx context.Context) error {
	// Note: This is a placeholder. In production, you would:
	// 1. Connect to IMAP server
	// 2. Select the FBL mailbox
	// 3. Search for unseen messages
	// 4. Fetch and parse each message
	// 5. Process the FBL report
	// 6. Mark as read/delete
	
	// For now, we'll just log that we would poll
	logger.WithComponent("fbl_monitor").Debug("polling FBL mailbox")
	
	return nil
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// IsComplaint checks if an email is a complaint/FBL report
func IsComplaint(msg *mail.Message) bool {
	contentType := strings.ToLower(msg.Header.Get("Content-Type"))
	
	// Check for multipart/report (ARF)
	if strings.Contains(contentType, "multipart/report") {
		return true
	}
	
	// Check for FBL-specific headers
	fblHeaders := []string{
		"Feedback-Type",
		"X-AOL-IP",
		"X-Complaint-To",
		"X-Jmrp-Original-Recipient",
	}
	
	for _, h := range fblHeaders {
		if msg.Header.Get(h) != "" {
			return true
		}
	}
	
	// Check subject for complaint indicators
	subject := strings.ToLower(msg.Header.Get("Subject"))
	complaintIndicators := []string{
		"complaint",
		"abuse report",
		"spam report",
		"feedback report",
		"junk mail report",
	}
	
	for _, indicator := range complaintIndicators {
		if strings.Contains(subject, indicator) {
			return true
		}
	}
	
	return false
}
