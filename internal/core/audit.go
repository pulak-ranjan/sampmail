package core

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/middleware/custom"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// AuditAction represents the type of action being audited
type AuditAction string

const (
	// Authentication actions
	AuditActionLogin          AuditAction = "login"
	AuditActionLoginFailed    AuditAction = "login_failed"
	AuditActionLogout         AuditAction = "logout"
	AuditActionRegister       AuditAction = "register"
	AuditAction2FAEnabled     AuditAction = "2fa_enabled"
	AuditAction2FADisabled    AuditAction = "2fa_disabled"

	// Domain actions
	AuditActionDomainCreate   AuditAction = "domain_create"
	AuditActionDomainUpdate   AuditAction = "domain_update"
	AuditActionDomainDelete   AuditAction = "domain_delete"

	// Campaign actions
	AuditActionCampaignCreate AuditAction = "campaign_create"
	AuditActionCampaignStart  AuditAction = "campaign_start"
	AuditActionCampaignPause  AuditAction = "campaign_pause"
	AuditActionCampaignDelete AuditAction = "campaign_delete"

	// Settings actions
	AuditActionSettingsUpdate AuditAction = "settings_update"
	AuditActionAPIKeyCreate   AuditAction = "api_key_create"
	AuditActionAPIKeyDelete   AuditAction = "api_key_delete"

	// Security actions
	AuditActionIPBlocked      AuditAction = "ip_blocked"
	AuditActionConfigBackup   AuditAction = "config_backup"
	AuditActionConfigApply    AuditAction = "config_apply"

	// Data actions
	AuditActionBulkImport     AuditAction = "bulk_import"
	AuditActionBulkDelete     AuditAction = "bulk_delete"
	AuditActionExport         AuditAction = "export"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           uint        `json:"id" gorm:"primaryKey"`
	Timestamp    time.Time   `json:"timestamp" gorm:"index"`
	Actor        string      `json:"actor" gorm:"index"` // User email or "system"
	Action       AuditAction `json:"action" gorm:"index"`
	ResourceType string      `json:"resource_type"` // domain, campaign, settings, etc.
	ResourceID   string      `json:"resource_id,omitempty"`
	Details      string      `json:"details,omitempty"` // JSON details
	IPAddress    string      `json:"ip_address,omitempty"`
	UserAgent    string      `json:"user_agent,omitempty"`
	RequestID    string      `json:"request_id,omitempty"`
}

// AuditService handles audit logging
type AuditService struct {
	store *store.Store
}

// NewAuditService creates a new audit service
func NewAuditService(st *store.Store) *AuditService {
	return &AuditService{store: st}
}

// Log creates an audit log entry
func (s *AuditService) Log(ctx context.Context, action AuditAction, resourceType string, resourceID string, details map[string]interface{}) {
	log := logger.SecurityLogger()

	entry := AuditLog{
		Timestamp:    time.Now().UTC(),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Actor:        "system", // Default to system
	}

	// Extract context information
	if requestID := custom.GetRequestID(ctx); requestID != "" {
		entry.RequestID = requestID
	}
	if clientIP := custom.GetClientIP(ctx); clientIP != "" {
		entry.IPAddress = clientIP
	}

	// Extract actor from context if available
	if actor := ctx.Value("audit_actor"); actor != nil {
		if actorStr, ok := actor.(string); ok {
			entry.Actor = actorStr
		}
	}

	// Serialize details
	if details != nil {
		detailsJSON, err := json.Marshal(details)
		if err == nil {
			entry.Details = string(detailsJSON)
		}
	}

	// Log to structured logger
	log.Info("audit event",
		"action", string(action),
		"resource_type", resourceType,
		"resource_id", resourceID,
		"actor", entry.Actor,
		"ip_address", entry.IPAddress,
		"request_id", entry.RequestID,
	)

	// Store in database (async to not block requests)
	go s.storeLog(entry)
}

// storeLog saves the audit log to database
func (s *AuditService) storeLog(entry AuditLog) {
	if err := s.store.DB.Create(&entry).Error; err != nil {
		logger.SecurityLogger().Error("failed to store audit log",
			"error", err,
			"action", entry.Action,
		)
	}
}

// Query retrieves audit logs with filters
func (s *AuditService) Query(filters AuditQueryFilters) ([]AuditLog, int64, error) {
	var logs []AuditLog
	var total int64

	query := s.store.DB.Model(&AuditLog{})

	if filters.Actor != "" {
		query = query.Where("actor = ?", filters.Actor)
	}
	if filters.Action != "" {
		query = query.Where("action = ?", filters.Action)
	}
	if filters.ResourceType != "" {
		query = query.Where("resource_type = ?", filters.ResourceType)
	}
	if !filters.StartTime.IsZero() {
		query = query.Where("timestamp >= ?", filters.StartTime)
	}
	if !filters.EndTime.IsZero() {
		query = query.Where("timestamp <= ?", filters.EndTime)
	}

	// Count total
	query.Count(&total)

	// Apply pagination
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 1000 {
		filters.Limit = 1000
	}

	err := query.Order("timestamp DESC").
		Offset(filters.Offset).
		Limit(filters.Limit).
		Find(&logs).Error

	return logs, total, err
}

// AuditQueryFilters holds query parameters for audit logs
type AuditQueryFilters struct {
	Actor        string
	Action       string
	ResourceType string
	StartTime    time.Time
	EndTime      time.Time
	Limit        int
	Offset       int
}

// Global audit service
var globalAuditService *AuditService

// InitAuditService initializes the global audit service
func InitAuditService(st *store.Store) {
	globalAuditService = NewAuditService(st)
}

// Audit is a convenience function for logging audit events
func Audit(ctx context.Context, action AuditAction, resourceType string, resourceID string, details map[string]interface{}) {
	if globalAuditService != nil {
		globalAuditService.Log(ctx, action, resourceType, resourceID, details)
	}
}

// AuditWithActor is a convenience function that sets the actor
func AuditWithActor(ctx context.Context, actor string, action AuditAction, resourceType string, resourceID string, details map[string]interface{}) {
	ctx = context.WithValue(ctx, "audit_actor", actor)
	Audit(ctx, action, resourceType, resourceID, details)
}
