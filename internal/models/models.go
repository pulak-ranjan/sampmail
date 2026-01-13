package models

import (
	"fmt"
	"time"
)

// Represents global application settings.
type AppSettings struct {
	ID uint `gorm:"primaryKey" json:"id"`

	MainHostname string `json:"main_hostname"`
	MainServerIP string `json:"main_server_ip"`
	MailWizzIP   string `json:"mailwizz_ip"` // optional relay IP

	// NEW: Listener Binding (e.g., "127.0.0.1:25" or "0.0.0.0:25")
	SMTPListenAddr string `json:"smtp_listen_addr"`

	AIProvider string `json:"ai_provider"` // "openai", "deepseek"
	AIAPIKey   string `json:"ai_api_key"`  // encrypted or blank

	// Webhook Settings
	WebhookURL     string  `json:"webhook_url"`
	WebhookEnabled bool    `json:"webhook_enabled"`
	BounceAlertPct float64 `json:"bounce_alert_pct"`

	// List Cleaner Proxy
	ProxyURL string `json:"proxy_url"` // SOCKS5 or HTTP proxy for fallback

	// Reacher Backend (for Gmail/Yahoo/etc verification)
	ReacherURL     string `json:"reacher_url"`      // HTTP API URL (e.g., "http://localhost:8080")
	ReacherAPIKey  string `json:"reacher_api_key"`  // API key for hosted Reacher
	ReacherBinPath string `json:"reacher_bin_path"` // Path to check_if_email_exists binary (preferred, no port conflicts)

	// Verification Configuration
	VerificationIPs string `json:"verification_ips"` // Comma/newline separated IPs for verification (NOT sending IPs!)
	EnableCatchall  bool   `json:"enable_catchall"`  // Enable catch-all detection (requires proxy for safety)
	UseReacherOnly  bool   `json:"use_reacher_only"` // Skip local SMTP, use Reacher for all domains

	// CORS
	AllowedOrigins string `json:"allowed_origins"` // Comma-separated list of origins

	// WhatsApp Configuration
	WhatsAppPhoneNumberID string `json:"whatsapp_phone_number_id"`
	WhatsAppAccessToken   string `json:"whatsapp_access_token"` // Should be encrypted
	WhatsAppVerifyToken   string `json:"whatsapp_verify_token"`
}

// A domain managed by the system
type Domain struct {
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"uniqueIndex" json:"name"`

	MailHost   string `json:"mail_host"`
	BounceHost string `json:"bounce_host"`

	// DMARC Settings
	DMARCPolicy     string `json:"dmarc_policy"`     // none, quarantine, reject
	DMARCRua        string `json:"dmarc_rua"`        // Aggregate report email
	DMARCRuf        string `json:"dmarc_ruf"`        // Forensic report email
	DMARCPercentage int    `json:"dmarc_percentage"` // 0-100

	Senders []Sender `gorm:"constraint:OnDelete:CASCADE" json:"senders"`
}

// AdminUser represents a panel admin account
type AdminUser struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Email        string `gorm:"uniqueIndex" json:"email"`
	PasswordHash string `json:"-"` // Never return hash

	// 2FA Support
	TwoFactorSecret  string `json:"-"`
	TwoFactorEnabled bool   `json:"has_2fa"`

	// User Preferences
	Theme        string `json:"theme"`
	IsSuperAdmin bool   `json:"is_super_admin"` // Multi-tenancy support
}

// Auth Sessions for multi-device support
type AuthSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	AdminID   uint      `gorm:"index" json:"admin_id"`
	Token     string    `gorm:"uniqueIndex" json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	DeviceIP  string    `json:"device_ip"`
	UserAgent string    `json:"user_agent"`
}

// BounceAccount represents a system user for handling bounced emails
type BounceAccount struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"uniqueIndex" json:"username"`
	PasswordHash string `json:"-"`
	Domain       string `json:"domain"`
	Notes        string `json:"notes"`
}

// A sender identity associated with a domain
type Sender struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	DomainID uint   `gorm:"index" json:"domain_id"`
	Domain   Domain `json:"-" gorm:"foreignKey:DomainID"` // Relation for Warmup Logic

	LocalPart    string `json:"local_part"`
	Email        string `json:"email"`
	IP           string `json:"ip"` // specific IP for this sender
	SMTPPassword string `json:"smtp_password,omitempty"`

	BounceUsername string `json:"bounce_username"`

	// Warmup State (NEW)
	WarmupEnabled    bool      `json:"warmup_enabled"`
	WarmupPlan       string    `json:"warmup_plan"`        // "conservative", "standard"
	WarmupDay        int       `json:"warmup_day"`         // 1, 2, 3...
	WarmupLastUpdate time.Time `json:"warmup_last_update"` // Last time we bumped the day

	// Virtual field for DKIM check (computed at runtime)
	HasDKIM bool `gorm:"-" json:"has_dkim"`
}

// Inventory of IPs available on the server
type SystemIP struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Value     string    `gorm:"uniqueIndex" json:"value"` // IPv4 address
	Netmask   string    `json:"netmask"`                  // e.g. /24
	Interface string    `json:"interface"`                // e.g. eth0 (optional)
	CreatedAt time.Time `json:"created_at"`

	// Virtual Field: True if this IP is actually configured on the OS
	IsActive bool `gorm:"-" json:"is_active"`
}

// =====================================
// PROXY MANAGEMENT
// =====================================

// Proxy represents a SOCKS5/HTTP proxy for email verification and sending
type Proxy struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`                          // Friendly name
	Type     string `json:"type"`                          // "socks5", "http", "https"
	Host     string `json:"host"`                          // Proxy hostname/IP
	Port     int    `json:"port"`                          // Proxy port
	Username string `json:"username,omitempty"`            // Optional auth
	Password string `json:"-"`                             // Hidden in JSON
	IsActive bool   `gorm:"default:true" json:"is_active"` // Enable/disable
	UseFor   string `json:"use_for"`                       // "verification", "sending", "both"
	Priority int    `json:"priority"`                      // Lower = higher priority

	// Health tracking
	LastCheck   time.Time `json:"last_check"`
	IsHealthy   bool      `json:"is_healthy"`
	FailCount   int       `json:"fail_count"`
	SuccessRate float64   `json:"success_rate"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProxyURL returns the full proxy URL string
func (p *Proxy) ProxyURL() string {
	auth := ""
	if p.Username != "" {
		auth = p.Username
		if p.Password != "" {
			auth += ":" + p.Password
		}
		auth += "@"
	}
	return fmt.Sprintf("%s://%s%s:%d", p.Type, auth, p.Host, p.Port)
}

// SendingIP represents an IP address used for outbound mail
type SendingIP struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	IPAddress string `gorm:"uniqueIndex" json:"ip_address"` // IPv4/IPv6
	Hostname  string `json:"hostname"`                      // PTR record / rDNS
	IsActive  bool   `gorm:"default:true" json:"is_active"`

	// Warmup tracking
	WarmupStage    int `json:"warmup_stage"` // 0=cold, 1-10=warming, 10=warm
	DailySendLimit int `json:"daily_send_limit"`
	TodaySent      int `json:"today_sent"`

	// Reputation
	ReputationScore float64 `json:"reputation_score"` // 0-100
	LastBounceRate  float64 `json:"last_bounce_rate"`

	// Assignment
	DomainID uint `json:"domain_id,omitempty"` // Optional: dedicated to domain

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EmailStats stores aggregated sending statistics
type EmailStats struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Domain    string    `gorm:"index" json:"domain"`
	Date      time.Time `gorm:"index" json:"date"` // Date only (no time)
	Sent      int64     `json:"sent"`
	Delivered int64     `json:"delivered"`
	Bounced   int64     `json:"bounced"`
	Deferred  int64     `json:"deferred"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QueueMessage represents a message in the mail queue (RESTORED)
type QueueMessage struct {
	ID          string    `json:"id"`
	Sender      string    `json:"sender"`
	Recipient   string    `json:"recipient"`
	Subject     string    `json:"subject"`
	Size        int64     `json:"size"`
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"` // queued, deferred, etc.
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt"`
	NextRetry   time.Time `json:"next_retry"`
	ErrorMsg    string    `json:"error_msg"`
}

// WebhookLog stores webhook delivery history
type WebhookLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	EventType string    `json:"event_type"` // bounce_alert, daily_summary
	Payload   string    `json:"payload"`    // JSON payload sent
	Status    int       `json:"status"`     // HTTP status code
	Response  string    `json:"response"`   // Response body
	CreatedAt time.Time `json:"created_at"`
}

// APIKey for external applications (NEW)
type APIKey struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`                   // e.g. "EmailVerifier App"
	Key       string    `gorm:"uniqueIndex" json:"key"` // The secret token
	Scopes    string    `json:"scopes"`                 // e.g. "verify,relay"
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
}

// ChatLog stores AI conversation history (NEW)
type ChatLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Role      string    `json:"role"` // "user", "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ContactList represents a managed list of contacts
type ContactList struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Contacts  []Contact `json:"contacts,omitempty" gorm:"foreignKey:ListID"`
}

// Contact represents a single person/lead
type Contact struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ListID    uint   `gorm:"index" json:"list_id"`
	Email     string `gorm:"index" json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`

	// Validation Status
	IsValid   bool   `json:"is_valid"`
	RiskScore int    `json:"risk_score"` // 0-100 (0=safe)
	VerifyLog string `json:"verify_log"` // "MX ok, SMTP failed"

	// Engagement (AI Superlead data)
	Score       int `json:"score"` // Lead Score
	TotalOpens  int `json:"total_opens"`
	TotalClicks int `json:"total_clicks"`

	CreatedAt time.Time `json:"created_at"`
}

// Campaign represents a bulk email job
type Campaign struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	Body     string `json:"body"`      // HTML Content
	SenderID uint   `json:"sender_id"` // From which Sender identity
	Sender   Sender `json:"-" gorm:"foreignKey:SenderID"`

	Status      string     `json:"status"`       // "draft", "scheduled", "sending", "completed", "failed"
	ScheduledAt *time.Time `json:"scheduled_at"` // Nullable

	TotalSent   int `json:"total_sent"`
	TotalFailed int `json:"total_failed"`
	TotalOpens  int `json:"total_opens"`
	TotalClicks int `json:"total_clicks"`

	CreatedAt  time.Time           `json:"created_at"`
	Recipients []CampaignRecipient `json:"recipients,omitempty" gorm:"foreignKey:CampaignID"`
}

// CampaignRecipient tracks individual status in a campaign
type CampaignRecipient struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	CampaignID uint   `gorm:"index" json:"campaign_id"`
	Email      string `gorm:"index" json:"email"`
	ContactID  uint   `gorm:"index" json:"contact_id"` // Optional link to persistent contact

	Status string    `json:"status"` // "pending", "sent", "failed"
	Error  string    `json:"error,omitempty"`
	SentAt time.Time `json:"sent_at,omitempty"`

	// Tracking
	OpenedAt  *time.Time `json:"opened_at"`
	ClickedAt *time.Time `json:"clicked_at"`
}

// AutomationWorkflow represents a visual automation flow
type AutomationWorkflow struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	Trigger   string    `json:"trigger"` // e.g. "contact_added", "email_opened"
	StepsJSON string    `json:"steps"`   // JSON array of steps (React Flow format)
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// WhatsAppMessage represents a message sent via WhatsApp Cloud API
type WhatsAppMessage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ContactID  uint      `gorm:"index" json:"contact_id"`
	ToNumber   string    `json:"to_number"`
	Body       string    `json:"body"`
	Status     string    `json:"status"` // sent, delivered, read
	MessageSID string    `json:"message_sid"`
	CreatedAt  time.Time `json:"created_at"`
}

// Suppression represents an email that should never be sent to
type Suppression struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex" json:"email"`
	Reason    string    `json:"reason"` // "hard_bounce", "unsubscribe", "complaint", "manual"
	Source    string    `json:"source"` // Campaign ID or "manual" or "fbl"
	CreatedAt time.Time `json:"created_at"`
}

// BounceEvent represents a processed bounce from the mail server
type BounceEvent struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Email          string    `gorm:"index" json:"email"`
	BounceType     string    `json:"bounce_type"` // "hard", "soft", "complaint"
	BounceCode     string    `json:"bounce_code"` // e.g., "550", "452"
	DiagnosticCode string    `json:"diagnostic_code"`
	CampaignID     uint      `gorm:"index" json:"campaign_id"`
	ProcessedAt    time.Time `json:"processed_at"`
	RawMessage     string    `json:"raw_message"` // Original bounce message (truncated)
}

// ProcessedLogFile tracks which log files have been processed and to what offset
type ProcessedLogFile struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	FilePath    string    `gorm:"uniqueIndex" json:"file_path"`
	LastOffset  int64     `json:"last_offset"`
	LastSize    int64     `json:"last_size"` // To detect log rotation
	ProcessedAt time.Time `json:"processed_at"`
}

// =====================================
// TEMPLATE SYSTEM
// =====================================

// Template represents an email template
type Template struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"index" json:"name"`
	Subject      string    `json:"subject"`
	HTMLContent  string    `json:"html_content"`
	TextContent  string    `json:"text_content"`
	ThumbnailURL string    `json:"thumbnail_url"`
	Category     string    `gorm:"index" json:"category"` // newsletter, transactional, welcome, etc.
	Variables    string    `json:"variables"`             // JSON array of variable names
	IsPublic     bool      `json:"is_public"`             // Available in template library
	IsBuiltIn    bool      `json:"is_built_in"`           // Pre-packaged template
	CreatedBy    uint      `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// =====================================
// SUBSCRIBER MANAGEMENT
// =====================================

// CustomField defines a custom field for subscribers
type CustomField struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `json:"name"`                         // Display name
	FieldKey     string    `gorm:"uniqueIndex" json:"field_key"` // Internal key (snake_case)
	FieldType    string    `json:"field_type"`                   // text, number, date, boolean, dropdown
	Options      string    `json:"options"`                      // JSON array for dropdown options
	IsRequired   bool      `json:"is_required"`
	DefaultValue string    `json:"default_value"`
	SortOrder    int       `json:"sort_order"`
	CreatedAt    time.Time `json:"created_at"`
}

// SubscriberFieldValue stores custom field values for subscribers
type SubscriberFieldValue struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	ContactID uint   `gorm:"uniqueIndex:idx_contact_field" json:"contact_id"`
	FieldID   uint   `gorm:"uniqueIndex:idx_contact_field" json:"field_id"`
	Value     string `json:"value"`
}

// Tag represents a subscriber tag
type Tag struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex" json:"name"`
	Color     string    `json:"color"` // Hex color
	CreatedAt time.Time `json:"created_at"`
}

// SubscriberTag is the many-to-many join table
type SubscriberTag struct {
	ContactID uint      `gorm:"primaryKey" json:"contact_id"`
	TagID     uint      `gorm:"primaryKey" json:"tag_id"`
	AddedAt   time.Time `json:"added_at"`
}

// Segment represents a dynamic subscriber segment
type Segment struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Conditions   string    `json:"conditions"`    // JSON array of conditions
	IsDynamic    bool      `json:"is_dynamic"`    // True = recalculate on query
	CachedCount  int       `json:"cached_count"`  // Last known count
	LastComputed time.Time `json:"last_computed"` // When count was last calculated
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// =====================================
// A/B TESTING
// =====================================

// ABTest represents an A/B test for a campaign
type ABTest struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	CampaignID       uint       `gorm:"uniqueIndex" json:"campaign_id"`
	TestType         string     `json:"test_type"`      // subject, content, send_time
	Variants         string     `json:"variants"`       // JSON array of variants
	WinnerVariant    string     `json:"winner_variant"` // A, B, etc.
	WinnerMetric     string     `json:"winner_metric"`  // opens, clicks
	WinnerSelectedAt *time.Time `json:"winner_selected_at"`
	SampleSizePct    int        `json:"sample_size_pct"` // % of list for test
	TestDurationHrs  int        `json:"test_duration_hrs"`
	CreatedAt        time.Time  `json:"created_at"`
}

// ABTestVariant represents a single variant (embedded in ABTest.Variants JSON)
type ABTestVariant struct {
	Name     string `json:"name"`      // A, B, C...
	Subject  string `json:"subject"`   // For subject tests
	Content  string `json:"content"`   // For content tests
	SendTime string `json:"send_time"` // For send time tests
	Weight   int    `json:"weight"`    // % weight (default 50/50)
	Sent     int    `json:"sent"`
	Opens    int    `json:"opens"`
	Clicks   int    `json:"clicks"`
}

// =====================================
// ANALYTICS
// =====================================

// CampaignAnalytics stores hourly aggregated stats
type CampaignAnalytics struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	CampaignID   uint      `gorm:"uniqueIndex:idx_campaign_hour" json:"campaign_id"`
	Hour         time.Time `gorm:"uniqueIndex:idx_campaign_hour" json:"hour"`
	Sent         int       `json:"sent"`
	Delivered    int       `json:"delivered"`
	Opened       int       `json:"opened"`
	UniqueOpens  int       `json:"unique_opens"`
	Clicked      int       `json:"clicked"`
	UniqueClicks int       `json:"unique_clicks"`
	Bounced      int       `json:"bounced"`
	Complained   int       `json:"complained"`
	Unsubscribed int       `json:"unsubscribed"`
}

// DomainReputation tracks domain health metrics
type DomainReputation struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	DomainID        uint      `gorm:"uniqueIndex:idx_domain_date" json:"domain_id"`
	Date            time.Time `gorm:"uniqueIndex:idx_domain_date" json:"date"`
	SentCount       int       `json:"sent_count"`
	DeliveredCount  int       `json:"delivered_count"`
	BounceRate      float64   `json:"bounce_rate"`
	ComplaintRate   float64   `json:"complaint_rate"`
	OpenRate        float64   `json:"open_rate"`
	ClickRate       float64   `json:"click_rate"`
	ReputationScore int       `json:"reputation_score"` // 0-100
}

// =====================================
// ENHANCED AUTOMATION
// =====================================

// AutomationWorkflowV2 is the enhanced visual workflow
type AutomationWorkflowV2 struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	TriggerType    string    `json:"trigger_type"`   // subscribe, tag_added, email_opened, etc.
	TriggerConfig  string    `json:"trigger_config"` // JSON config for trigger
	Nodes          string    `json:"nodes"`          // React Flow nodes JSON
	Edges          string    `json:"edges"`          // React Flow edges JSON
	IsActive       bool      `json:"is_active"`
	StatsEntered   int       `json:"stats_entered"`
	StatsCompleted int       `json:"stats_completed"`
	StatsActive    int       `json:"stats_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AutomationRun tracks individual subscriber runs through a workflow
type AutomationRun struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	WorkflowID   uint       `gorm:"index" json:"workflow_id"`
	ContactID    uint       `gorm:"index" json:"contact_id"`
	CurrentNode  string     `json:"current_node"` // Node ID currently at
	Status       string     `json:"status"`       // running, completed, failed, paused
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	NextActionAt *time.Time `json:"next_action_at"` // For wait nodes
	ErrorMessage string     `json:"error_message"`
}

// =====================================
// SUBSCRIBER ACTIVITY
// =====================================

// SubscriberActivity tracks all subscriber events
type SubscriberActivity struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ContactID  uint      `gorm:"index" json:"contact_id"`
	EventType  string    `gorm:"index" json:"event_type"` // subscribed, email_sent, email_opened, etc.
	EventData  string    `json:"event_data"`              // JSON with additional data
	CampaignID uint      `json:"campaign_id"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// TrackingEvent stores email open/click tracking events
type TrackingEvent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	CampaignID  uint      `gorm:"index" json:"campaign_id"`
	RecipientID uint      `gorm:"index" json:"recipient_id"`
	EventType   string    `gorm:"index" json:"event_type"` // "open", "click"
	URL         string    `json:"url"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `gorm:"index" json:"created_at"`
}
