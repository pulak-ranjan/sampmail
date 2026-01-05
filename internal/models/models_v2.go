package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// =====================================
// MULTI-TENANCY MODELS
// =====================================

// Organization represents a tenant in multi-tenant setup
type Organization struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `json:"name"`
	Slug        string `gorm:"uniqueIndex" json:"slug"` // URL-safe identifier
	Plan        string `json:"plan"`                    // free, starter, pro, enterprise
	MaxUsers    int    `json:"max_users"`
	MaxContacts int    `json:"max_contacts"`
	MaxEmails   int    `json:"max_emails_per_month"`

	// Branding
	LogoURL      string `json:"logo_url"`
	PrimaryColor string `json:"primary_color"`

	// Settings
	Settings JSONMap `json:"settings" gorm:"type:text"`

	// Usage tracking
	ContactsUsed int `json:"contacts_used"`
	EmailsSent   int `json:"emails_sent_this_month"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Users []OrganizationUser `json:"users,omitempty" gorm:"foreignKey:OrganizationID"`
}

// OrganizationUser represents membership in an organization
type OrganizationUser struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	OrganizationID uint       `gorm:"index" json:"organization_id"`
	AdminID        uint       `gorm:"index" json:"admin_id"`
	Role           string     `json:"role"` // owner, admin, editor, viewer
	Permissions    JSONMap    `json:"permissions" gorm:"type:text"`
	InvitedAt      time.Time  `json:"invited_at"`
	JoinedAt       *time.Time `json:"joined_at"`

	// Relations
	Organization Organization `json:"organization" gorm:"foreignKey:OrganizationID"`
}

// =====================================
// ENHANCED CONTACT MODEL
// =====================================

// ContactV2 extends Contact with more fields for personalization
type ContactV2 struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`
	ListID         uint `gorm:"index" json:"list_id"`

	// Core fields
	Email     string `gorm:"index" json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Company   string `json:"company"`
	JobTitle  string `json:"job_title"`

	// Address
	Address1   string `json:"address1"`
	Address2   string `json:"address2"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country"`

	// Custom fields (JSON)
	CustomFields JSONMap `json:"custom_fields" gorm:"type:text"`

	// Engagement
	LeadScore     int        `json:"lead_score"`
	TotalOpens    int        `json:"total_opens"`
	TotalClicks   int        `json:"total_clicks"`
	LastOpenedAt  *time.Time `json:"last_opened_at"`
	LastClickedAt *time.Time `json:"last_clicked_at"`

	// Status
	Status         string     `json:"status"` // active, unsubscribed, bounced, complained
	SubscribedAt   time.Time  `json:"subscribed_at"`
	UnsubscribedAt *time.Time `json:"unsubscribed_at"`

	// Verification
	IsVerified bool `json:"is_verified"`
	RiskScore  int  `json:"risk_score"`

	// Timezone for send time optimization
	Timezone string `json:"timezone"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// =====================================
// DRAG & DROP EMAIL TEMPLATE BUILDER
// =====================================

// EmailTemplate represents a drag & drop email template
type EmailTemplate struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name        string `json:"name"`
	Subject     string `json:"subject"`
	PreviewText string `json:"preview_text"`

	// Builder JSON (GrapeJS/MJML compatible)
	BuilderJSON string `json:"builder_json" gorm:"type:text"`

	// Compiled outputs
	HTMLContent string `json:"html_content" gorm:"type:text"`
	TextContent string `json:"text_content" gorm:"type:text"`
	MJMLSource  string `json:"mjml_source" gorm:"type:text"`

	// Template metadata
	Category     string `json:"category"`    // newsletter, welcome, promotional, transactional
	Subcategory  string `json:"subcategory"` // ecommerce, saas, agency, etc.
	ThumbnailURL string `json:"thumbnail_url"`

	// Variables used in template
	Variables StringArray `json:"variables" gorm:"type:text"`

	// Template library
	IsPublic   bool    `json:"is_public"`
	IsBuiltIn  bool    `json:"is_built_in"`
	IsFavorite bool    `json:"is_favorite"`
	UsageCount int     `json:"usage_count"`
	Rating     float64 `json:"rating"`

	// AI generated
	AIGenerated bool   `json:"ai_generated"`
	AIPrompt    string `json:"ai_prompt"`

	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateBlock represents reusable content blocks
type TemplateBlock struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name     string `json:"name"`
	Category string `json:"category"` // header, footer, cta, image, text, social

	// Block content
	BuilderJSON  string `json:"builder_json" gorm:"type:text"`
	HTMLContent  string `json:"html_content" gorm:"type:text"`
	ThumbnailURL string `json:"thumbnail_url"`

	IsGlobal bool `json:"is_global"` // Available to all organizations

	CreatedAt time.Time `json:"created_at"`
}

// =====================================
// CAMPAIGN V2 WITH PERSONALIZATION
// =====================================

// CampaignV2 extends Campaign with personalization and A/B testing
type CampaignV2 struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name string `json:"name"`
	Type string `json:"type"` // regular, ab_test, automated, rss

	// Content
	Subject     string `json:"subject"`
	PreviewText string `json:"preview_text"`
	TemplateID  uint   `json:"template_id"`
	HTMLContent string `json:"html_content" gorm:"type:text"`
	TextContent string `json:"text_content" gorm:"type:text"`

	// Sender
	SenderID uint   `json:"sender_id"`
	ReplyTo  string `json:"reply_to"`

	// Recipients
	ListIDs        StringArray `json:"list_ids" gorm:"type:text"`
	SegmentIDs     StringArray `json:"segment_ids" gorm:"type:text"`
	ExcludeListIDs StringArray `json:"exclude_list_ids" gorm:"type:text"`

	// A/B Testing
	ABTestEnabled  bool    `json:"ab_test_enabled"`
	ABTestConfig   JSONMap `json:"ab_test_config" gorm:"type:text"`
	WinningVariant string  `json:"winning_variant"`

	// Scheduling
	Status               string     `json:"status"` // draft, scheduled, sending, paused, completed, failed
	ScheduledAt          *time.Time `json:"scheduled_at"`
	SendTimezone         string     `json:"send_timezone"`
	SendTimeOptimization bool       `json:"send_time_optimization"` // AI-optimized send time

	// Tracking
	TrackOpens      bool    `json:"track_opens"`
	TrackClicks     bool    `json:"track_clicks"`
	GoogleAnalytics bool    `json:"google_analytics"`
	UTMParams       JSONMap `json:"utm_params" gorm:"type:text"`

	// Stats
	TotalRecipients   int `json:"total_recipients"`
	TotalSent         int `json:"total_sent"`
	TotalDelivered    int `json:"total_delivered"`
	TotalOpens        int `json:"total_opens"`
	UniqueOpens       int `json:"unique_opens"`
	TotalClicks       int `json:"total_clicks"`
	UniqueClicks      int `json:"unique_clicks"`
	TotalBounces      int `json:"total_bounces"`
	TotalComplaints   int `json:"total_complaints"`
	TotalUnsubscribes int `json:"total_unsubscribes"`

	CreatedBy   uint       `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	SentAt      *time.Time `json:"sent_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// CampaignRecipientV2 with personalization data
type CampaignRecipientV2 struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	CampaignID uint   `gorm:"index" json:"campaign_id"`
	ContactID  uint   `gorm:"index" json:"contact_id"`
	Email      string `gorm:"index" json:"email"`

	// Personalization data snapshot
	PersonalizationData JSONMap `json:"personalization_data" gorm:"type:text"`

	// Rendered content (for debugging)
	RenderedSubject string `json:"rendered_subject"`

	// Status
	Status    string `json:"status"`     // pending, queued, sent, delivered, opened, clicked, bounced, failed
	ABVariant string `json:"ab_variant"` // For A/B tests

	// Timestamps
	QueuedAt     *time.Time `json:"queued_at"`
	SentAt       *time.Time `json:"sent_at"`
	DeliveredAt  *time.Time `json:"delivered_at"`
	FirstOpenAt  *time.Time `json:"first_open_at"`
	LastOpenAt   *time.Time `json:"last_open_at"`
	FirstClickAt *time.Time `json:"first_click_at"`

	// Engagement
	OpenCount    int         `json:"open_count"`
	ClickCount   int         `json:"click_count"`
	ClickedLinks StringArray `json:"clicked_links" gorm:"type:text"`

	// Errors
	ErrorCode    string `json:"error_code"`
	ErrorMessage string `json:"error_message"`
}

// =====================================
// AUTOMATION WORKFLOW BUILDER
// =====================================

// AutomationV2 represents a visual automation workflow (like n8n/Mautic)
type AutomationV2 struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name        string `json:"name"`
	Description string `json:"description"`

	// Workflow definition (React Flow compatible)
	Nodes    string `json:"nodes" gorm:"type:text"`    // JSON array of nodes
	Edges    string `json:"edges" gorm:"type:text"`    // JSON array of connections
	Viewport string `json:"viewport" gorm:"type:text"` // Canvas viewport state

	// Trigger configuration
	TriggerType   string  `json:"trigger_type"` // contact_added, tag_added, email_opened, link_clicked, webhook, scheduled, form_submitted
	TriggerConfig JSONMap `json:"trigger_config" gorm:"type:text"`

	// Filters
	EntryFilters   JSONMap `json:"entry_filters" gorm:"type:text"`   // Who can enter
	ExitConditions JSONMap `json:"exit_conditions" gorm:"type:text"` // When to exit

	// Settings
	Status       string  `json:"status"` // draft, active, paused, archived
	AllowReentry bool    `json:"allow_reentry"`
	ReentryDelay int     `json:"reentry_delay"` // Hours before re-entry allowed
	GoalTracking bool    `json:"goal_tracking"`
	GoalConfig   JSONMap `json:"goal_config" gorm:"type:text"`

	// Stats
	TotalEntered   int     `json:"total_entered"`
	TotalCompleted int     `json:"total_completed"`
	TotalActive    int     `json:"total_active"`
	ConversionRate float64 `json:"conversion_rate"`

	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AutomationNode represents a single node in the workflow
type AutomationNode struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"` // trigger, action, condition, delay, goal
	Position Position `json:"position"`
	Data     NodeData `json:"data"`
}

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type NodeData struct {
	Label       string  `json:"label"`
	NodeType    string  `json:"nodeType"` // Specific type: send_email, add_tag, wait, split, webhook
	Config      JSONMap `json:"config"`   // Node-specific configuration
	Description string  `json:"description"`
}

// AutomationEdge represents a connection between nodes
type AutomationEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"` // For condition branches
	TargetHandle string `json:"targetHandle,omitempty"`
	Label        string `json:"label,omitempty"`
	Animated     bool   `json:"animated,omitempty"`
}

// AutomationRun tracks a contact's journey through automation
type AutomationRunV2 struct {
	ID           uint `gorm:"primaryKey" json:"id"`
	AutomationID uint `gorm:"index" json:"automation_id"`
	ContactID    uint `gorm:"index" json:"contact_id"`

	// Current state
	CurrentNodeID string `json:"current_node_id"`
	Status        string `json:"status"` // active, completed, failed, exited, paused

	// History
	NodesVisited StringArray `json:"nodes_visited" gorm:"type:text"`
	ActionsLog   string      `json:"actions_log" gorm:"type:text"` // JSON log of all actions

	// Timing
	EnteredAt    time.Time  `json:"entered_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	NextActionAt *time.Time `json:"next_action_at"` // For delayed actions

	// Context data passed through workflow
	ContextData JSONMap `json:"context_data" gorm:"type:text"`

	// Error tracking
	ErrorCount int    `json:"error_count"`
	LastError  string `json:"last_error"`
}

// AutomationActionLog tracks individual actions in a run
type AutomationActionLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	RunID      uint      `gorm:"index" json:"run_id"`
	NodeID     string    `json:"node_id"`
	NodeType   string    `json:"node_type"`
	Action     string    `json:"action"` // started, completed, failed, skipped
	Result     JSONMap   `json:"result" gorm:"type:text"`
	ExecutedAt time.Time `json:"executed_at"`
	DurationMs int       `json:"duration_ms"`
}

// =====================================
// AUTOMATION NODE TYPES
// =====================================

const (
	// Triggers
	NodeTypeTriggerContactAdded   = "trigger_contact_added"
	NodeTypeTriggerTagAdded       = "trigger_tag_added"
	NodeTypeTriggerEmailOpened    = "trigger_email_opened"
	NodeTypeTriggerLinkClicked    = "trigger_link_clicked"
	NodeTypeTriggerFormSubmitted  = "trigger_form_submitted"
	NodeTypeTriggerWebhook        = "trigger_webhook"
	NodeTypeTriggerSchedule       = "trigger_schedule"
	NodeTypeTriggerSegmentEntered = "trigger_segment_entered"
	NodeTypeTriggerDateField      = "trigger_date_field"

	// Actions
	NodeTypeActionSendEmail      = "action_send_email"
	NodeTypeActionSendSMS        = "action_send_sms"
	NodeTypeActionAddTag         = "action_add_tag"
	NodeTypeActionRemoveTag      = "action_remove_tag"
	NodeTypeActionAddToList      = "action_add_to_list"
	NodeTypeActionRemoveFromList = "action_remove_from_list"
	NodeTypeActionUpdateField    = "action_update_field"
	NodeTypeActionWebhook        = "action_webhook"
	NodeTypeActionNotify         = "action_notify"
	NodeTypeActionScore          = "action_update_score"
	NodeTypeActionCopyToList     = "action_copy_to_list"
	NodeTypeActionCreateTask     = "action_create_task"

	// Flow Control
	NodeTypeDelay       = "delay"
	NodeTypeCondition   = "condition"
	NodeTypeABSplit     = "ab_split"
	NodeTypeRandomSplit = "random_split"
	NodeTypeGoal        = "goal"
	NodeTypeExit        = "exit"

	// Advanced
	NodeTypeAIDecision    = "ai_decision"
	NodeTypeRunAutomation = "run_automation"
)

// =====================================
// API MODELS
// =====================================

// APIKeyV2 with organization support
type APIKeyV2 struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"` // First 8 chars for display
	KeyHash   string `json:"-"`          // Hashed full key

	// Permissions
	Scopes StringArray `json:"scopes" gorm:"type:text"` // contacts:read, campaigns:write, etc.

	// Rate limiting
	RateLimitPerMinute int `json:"rate_limit_per_minute"`
	RateLimitPerHour   int `json:"rate_limit_per_hour"`

	// Usage
	LastUsedAt    *time.Time `json:"last_used_at"`
	TotalRequests int64      `json:"total_requests"`

	// Restrictions
	AllowedIPs StringArray `json:"allowed_ips" gorm:"type:text"`
	ExpiresAt  *time.Time  `json:"expires_at"`

	IsActive  bool      `json:"is_active"`
	CreatedBy uint      `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// WebhookV2 for incoming/outgoing webhooks
type WebhookV2 struct {
	ID             uint `gorm:"primaryKey" json:"id"`
	OrganizationID uint `gorm:"index" json:"organization_id"`

	Name      string `json:"name"`
	Direction string `json:"direction"` // incoming, outgoing

	// Outgoing webhook config
	URL     string      `json:"url"`
	Method  string      `json:"method"` // POST, PUT
	Headers JSONMap     `json:"headers" gorm:"type:text"`
	Events  StringArray `json:"events" gorm:"type:text"` // contact.created, campaign.sent, etc.

	// Incoming webhook config
	IncomingToken string `json:"incoming_token"` // For verification

	// Auth
	AuthType   string  `json:"auth_type"` // none, basic, bearer, hmac
	AuthConfig JSONMap `json:"auth_config" gorm:"type:text"`

	// Status
	IsActive      bool       `json:"is_active"`
	LastTriggered *time.Time `json:"last_triggered"`
	SuccessCount  int        `json:"success_count"`
	FailureCount  int        `json:"failure_count"`

	CreatedAt time.Time `json:"created_at"`
}

// =====================================
// HELPER TYPES
// =====================================

// JSONMap is a helper for storing JSON objects
type JSONMap map[string]interface{}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

// StringArray is a helper for storing string arrays
type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *StringArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}
