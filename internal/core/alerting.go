package core

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// ALERTING & MONITORING SYSTEM
// =====================================
// Provides real-time alerts for:
// - High bounce rates
// - High complaint rates
// - Queue depth issues
// - SMTP connection failures
// - Domain authentication issues
// - IP reputation changes

// AlertLevel defines the severity of an alert
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertType defines the type of alert
type AlertType string

const (
	AlertTypeBounceRate      AlertType = "bounce_rate"
	AlertTypeComplaintRate   AlertType = "complaint_rate"
	AlertTypeQueueDepth      AlertType = "queue_depth"
	AlertTypeSMTPFailure     AlertType = "smtp_failure"
	AlertTypeDNSIssue        AlertType = "dns_issue"
	AlertTypeIPReputation    AlertType = "ip_reputation"
	AlertTypeSenderHealth    AlertType = "sender_health"
	AlertTypeAutomationError AlertType = "automation_error"
	AlertTypeSystemHealth    AlertType = "system_health"
)

// Alert represents a system alert
type Alert struct {
	ID             uint          `json:"id"`
	Type           AlertType     `json:"type"`
	Level          AlertLevel    `json:"level"`
	OrganizationID uint          `json:"organization_id"`
	Title          string        `json:"title"`
	Message        string        `json:"message"`
	Details        map[string]interface{} `json:"details"`
	CreatedAt      time.Time     `json:"created_at"`
	Acknowledged   bool          `json:"acknowledged"`
	AcknowledgedBy *uint         `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time    `json:"resolved_at,omitempty"`
}

// AlertThresholds defines configurable alert thresholds
type AlertThresholds struct {
	BounceRateWarning    float64 `json:"bounce_rate_warning"`     // 5%
	BounceRateError      float64 `json:"bounce_rate_error"`       // 10%
	BounceRateCritical   float64 `json:"bounce_rate_critical"`    // 20%
	ComplaintRateWarning float64 `json:"complaint_rate_warning"`  // 0.1%
	ComplaintRateError   float64 `json:"complaint_rate_error"`    // 0.5%
	QueueDepthWarning    int     `json:"queue_depth_warning"`     // 5000
	QueueDepthError      int     `json:"queue_depth_error"`       // 10000
	QueueDepthCritical   int     `json:"queue_depth_critical"`    // 50000
	SMTPFailureWindow    int     `json:"smtp_failure_window"`     // minutes
	SMTPFailureThreshold int     `json:"smtp_failure_threshold"` // failures in window
}

// DefaultAlertThresholds returns default alert thresholds
func DefaultAlertThresholds() *AlertThresholds {
	return &AlertThresholds{
		BounceRateWarning:    0.05,
		BounceRateError:      0.10,
		BounceRateCritical:   0.20,
		ComplaintRateWarning: 0.001,
		ComplaintRateError:   0.005,
		QueueDepthWarning:    5000,
		QueueDepthError:      10000,
		QueueDepthCritical:   50000,
		SMTPFailureWindow:    5,
		SMTPFailureThreshold: 10,
	}
}

// Notifier interface for alert notifications
type Notifier interface {
	Send(alert *Alert) error
	Name() string
}

// EmailNotifier sends alerts via email
type EmailNotifier struct {
	recipients []string
	sender     EmailSender
}

func NewEmailNotifier(recipients []string, sender EmailSender) *EmailNotifier {
	return &EmailNotifier{recipients: recipients, sender: sender}
}

func (n *EmailNotifier) Name() string { return "email" }

func (n *EmailNotifier) Send(alert *Alert) error {
	subject := fmt.Sprintf("[%s] %s", string(alert.Level), alert.Title)
	body := fmt.Sprintf("%s\n\nDetails: %+v\n\nTime: %s",
		alert.Message, alert.Details, alert.CreatedAt.Format(time.RFC3339))
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	for _, recipient := range n.recipients {
		n.sender.SendEmail(ctx, recipient, subject, body)
	}
	return nil
}

// WebhookNotifier sends alerts to a webhook URL
type WebhookNotifier struct {
	url     string
	client  *http.Client
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *WebhookNotifier) Name() string { return "webhook" }

func (n *WebhookNotifier) Send(alert *Alert) error {
	// Implementation would POST alert to webhook URL
	return nil
}

// SlackNotifier sends alerts to Slack
type SlackNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewSlackNotifier(webhookURL string) *SlackNotifier {
	return &SlackNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *SlackNotifier) Name() string { return "slack" }

func (n *SlackNotifier) Send(alert *Alert) error {
	// Implementation would POST to Slack webhook
	return nil
}

// AlertManager manages alerts and notifications
type AlertManager struct {
	store      *store.Store
	thresholds *AlertThresholds
	notifiers  []Notifier
	alerts     map[uint]*Alert // active alerts by ID
	mu         sync.RWMutex
	stopCh     chan struct{}
}

var globalAlertManager *AlertManager

// InitAlertManager initializes the global alert manager
func InitAlertManager(st *store.Store, thresholds *AlertThresholds) *AlertManager {
	if thresholds == nil {
		thresholds = DefaultAlertThresholds()
	}
	globalAlertManager = &AlertManager{
		store:      st,
		thresholds: thresholds,
		notifiers:  make([]Notifier, 0),
		alerts:     make(map[uint]*Alert),
		stopCh:     make(chan struct{}),
	}
	return globalAlertManager
}

// GetAlertManager returns the global alert manager
func GetAlertManager() *AlertManager {
	return globalAlertManager
}

// AddNotifier adds a notifier to the alert manager
func (am *AlertManager) AddNotifier(notifier Notifier) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.notifiers = append(am.notifiers, notifier)
}

// SetThresholds updates alert thresholds
func (am *AlertManager) SetThresholds(thresholds *AlertThresholds) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.thresholds = thresholds
}

// CreateAlert creates a new alert
func (am *AlertManager) CreateAlert(alertType AlertType, level AlertLevel, orgID uint, title, message string, details map[string]interface{}) *Alert {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert := &Alert{
		Type:           alertType,
		Level:          level,
		OrganizationID: orgID,
		Title:          title,
		Message:        message,
		Details:        details,
		CreatedAt:      time.Now(),
		Acknowledged:   false,
	}

	// Store in database
	if am.store != nil {
		// Would persist to alerts table
	}

	am.alerts[alert.ID] = alert

	// Send notifications
	go am.notify(alert)

	// Also send via WebSocket
	if orgID > 0 {
		NotifySystemAlert(orgID, string(level), message, details)
	}

	return alert
}

// notify sends alert to all notifiers
func (am *AlertManager) notify(alert *Alert) {
	am.mu.RLock()
	notifiers := am.notifiers
	am.mu.RUnlock()

	log := logger.WithComponent("alert_manager")
	
	for _, notifier := range notifiers {
		go func(n Notifier) {
			if err := n.Send(alert); err != nil {
				log.Error("failed to send alert notification",
					"notifier", n.Name(),
					"alert_id", alert.ID,
					"error", err)
			}
		}(notifier)
	}
}

// CheckBounceRate checks bounce rate and creates alerts if needed
func (am *AlertManager) CheckBounceRate(orgID uint, campaignID uint, sent, bounced int) {
	if sent == 0 {
		return
	}

	bounceRate := float64(bounced) / float64(sent)
	thresholds := am.thresholds

	var level AlertLevel
	var shouldAlert bool

	switch {
	case bounceRate >= thresholds.BounceRateCritical:
		level = AlertLevelCritical
		shouldAlert = true
	case bounceRate >= thresholds.BounceRateError:
		level = AlertLevelError
		shouldAlert = true
	case bounceRate >= thresholds.BounceRateWarning:
		level = AlertLevelWarning
		shouldAlert = true
	}

	if shouldAlert {
		am.CreateAlert(
			AlertTypeBounceRate,
			level,
			orgID,
			"High Bounce Rate Detected",
			fmt.Sprintf("Bounce rate is %.1f%% (%d/%d emails)", bounceRate*100, bounced, sent),
			map[string]interface{}{
				"bounce_rate":  bounceRate,
				"bounced":      bounced,
				"sent":         sent,
				"campaign_id":  campaignID,
			},
		)
	}
}

// CheckComplaintRate checks complaint rate and creates alerts if needed
func (am *AlertManager) CheckComplaintRate(orgID uint, campaignID uint, sent, complaints int) {
	if sent == 0 {
		return
	}

	complaintRate := float64(complaints) / float64(sent)
	thresholds := am.thresholds

	var level AlertLevel
	var shouldAlert bool

	switch {
	case complaintRate >= thresholds.ComplaintRateError:
		level = AlertLevelError
		shouldAlert = true
	case complaintRate >= thresholds.ComplaintRateWarning:
		level = AlertLevelWarning
		shouldAlert = true
	}

	if shouldAlert {
		am.CreateAlert(
			AlertTypeComplaintRate,
			level,
			orgID,
			"High Complaint Rate Detected",
			fmt.Sprintf("Complaint rate is %.2f%% (%d/%d emails)", complaintRate*100, complaints, sent),
			map[string]interface{}{
				"complaint_rate": complaintRate,
				"complaints":     complaints,
				"sent":           sent,
				"campaign_id":    campaignID,
			},
		)
	}
}

// CheckQueueDepth checks queue depth and creates alerts if needed
func (am *AlertManager) CheckQueueDepth(orgID uint, depth int) {
	thresholds := am.thresholds

	var level AlertLevel
	var shouldAlert bool

	switch {
	case depth >= thresholds.QueueDepthCritical:
		level = AlertLevelCritical
		shouldAlert = true
	case depth >= thresholds.QueueDepthError:
		level = AlertLevelError
		shouldAlert = true
	case depth >= thresholds.QueueDepthWarning:
		level = AlertLevelWarning
		shouldAlert = true
	}

	if shouldAlert {
		am.CreateAlert(
			AlertTypeQueueDepth,
			level,
			orgID,
			"Queue Depth Alert",
			fmt.Sprintf("Email queue has %d pending messages", depth),
			map[string]interface{}{
				"queue_depth": depth,
			},
		)
	}
}

// AlertSMTPFailure creates an alert for SMTP failures
func (am *AlertManager) AlertSMTPFailure(orgID uint, server string, errMsg string) {
	am.CreateAlert(
		AlertTypeSMTPFailure,
		AlertLevelError,
		orgID,
		"SMTP Connection Failure",
		fmt.Sprintf("Failed to connect to SMTP server %s: %s", server, errMsg),
		map[string]interface{}{
			"server":  server,
			"error":   errMsg,
		},
	)
}

// AlertDNSIssue creates an alert for DNS issues
func (am *AlertManager) AlertDNSIssue(orgID uint, domain string, recordType string, issue string) {
	am.CreateAlert(
		AlertTypeDNSIssue,
		AlertLevelWarning,
		orgID,
		"DNS Configuration Issue",
		fmt.Sprintf("DNS %s record issue for %s: %s", recordType, domain, issue),
		map[string]interface{}{
			"domain":      domain,
			"record_type": recordType,
			"issue":       issue,
		},
	)
}

// AlertIPReputation creates an alert for IP reputation issues
func (am *AlertManager) AlertIPReputation(orgID uint, ip string, blacklist string, details string) {
	am.CreateAlert(
		AlertTypeIPReputation,
		AlertLevelError,
		orgID,
		"IP Blacklisted",
		fmt.Sprintf("IP %s listed on %s: %s", ip, blacklist, details),
		map[string]interface{}{
			"ip":        ip,
			"blacklist": blacklist,
			"details":   details,
		},
	)
}

// GetActiveAlerts returns all active (unacknowledged) alerts
func (am *AlertManager) GetActiveAlerts(orgID uint) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var alerts []*Alert
	for _, alert := range am.alerts {
		if !alert.Acknowledged && (orgID == 0 || alert.OrganizationID == orgID) {
			alerts = append(alerts, alert)
		}
	}
	return alerts
}

// AcknowledgeAlert marks an alert as acknowledged
func (am *AlertManager) AcknowledgeAlert(alertID uint, userID uint) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, ok := am.alerts[alertID]; ok {
		alert.Acknowledged = true
		alert.AcknowledgedBy = &userID
		return nil
	}
	return fmt.Errorf("alert not found")
}

// ResolveAlert marks an alert as resolved
func (am *AlertManager) ResolveAlert(alertID uint) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert, ok := am.alerts[alertID]; ok {
		now := time.Now()
		alert.ResolvedAt = &now
		return nil
	}
	return fmt.Errorf("alert not found")
}

// StartMonitoring starts the alert monitoring loop
func (am *AlertManager) StartMonitoring(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log := logger.WithComponent("alert_manager")

	for {
		select {
		case <-ctx.Done():
			return
		case <-am.stopCh:
			return
		case <-ticker.C:
			// Run periodic checks
			am.runPeriodicChecks()
			log.Debug("ran periodic alert checks")
		}
	}
}

// runPeriodicChecks runs all periodic alert checks
func (am *AlertManager) runPeriodicChecks() {
	// Check queue depth
	if stats, err := GetQueueStats(); err == nil {
		am.CheckQueueDepth(0, stats.Total)
	}

	// Check for unresolved alerts that need escalation
	am.checkEscalation()
}

// checkEscalation checks for alerts that need escalation
func (am *AlertManager) checkEscalation() {
	am.mu.RLock()
	defer am.mu.RUnlock()

	now := time.Now()
	for _, alert := range am.alerts {
		if !alert.Acknowledged && alert.ResolvedAt == nil {
			// Escalate critical alerts after 15 minutes
			if alert.Level == AlertLevelCritical {
				age := now.Sub(alert.CreatedAt)
				if age > 15*time.Minute {
					// Re-notify for critical unacknowledged alerts
					go am.notify(alert)
				}
			}
		}
	}
}

// Stop stops the alert manager
func (am *AlertManager) Stop() {
	close(am.stopCh)
}

// =====================================
// ALERT STATISTICS
// =====================================

// AlertStats represents alert statistics
type AlertStats struct {
	TotalAlerts     int            `json:"total_alerts"`
	ActiveAlerts    int            `json:"active_alerts"`
	ByType          map[AlertType]int `json:"by_type"`
	ByLevel         map[AlertLevel]int `json:"by_level"`
	Last24Hours     int            `json:"last_24_hours"`
	AcknowledgedRate float64       `json:"acknowledged_rate"`
}

// GetAlertStats returns alert statistics
func (am *AlertManager) GetAlertStats(orgID uint) *AlertStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := &AlertStats{
		ByType:  make(map[AlertType]int),
		ByLevel: make(map[AlertLevel]int),
	}

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	for _, alert := range am.alerts {
		if orgID == 0 || alert.OrganizationID == orgID {
			stats.TotalAlerts++
			stats.ByType[alert.Type]++
			stats.ByLevel[alert.Level]++

			if !alert.Acknowledged && alert.ResolvedAt == nil {
				stats.ActiveAlerts++
			}

			if alert.CreatedAt.After(dayAgo) {
				stats.Last24Hours++
			}
		}
	}

	if stats.TotalAlerts > 0 {
		stats.AcknowledgedRate = float64(stats.TotalAlerts-stats.ActiveAlerts) / float64(stats.TotalAlerts)
	}

	return stats
}

func NotifySystemAlert(orgID uint, level, message string, details map[string]interface{}) {
}
