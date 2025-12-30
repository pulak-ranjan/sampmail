// Package store provides database abstraction with PostgreSQL support
package store

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/pulak-ranjan/sampmail/internal/models"
)

// PostgresConfig holds PostgreSQL connection settings
type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DefaultPostgresConfig returns production-ready defaults
func DefaultPostgresConfig() *PostgresConfig {
	return &PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		SSLMode:         "require",
		MaxOpenConns:    100,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// NewPostgresStore creates a new Store with PostgreSQL backend
func NewPostgresStore(cfg *PostgresConfig) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		PrepareStmt: true, // Cache prepared statements
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Get underlying sql.DB for connection pool settings
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &Store{DB: db}, nil
}

// MigratePostgres runs migrations for PostgreSQL
// This uses GORM AutoMigrate which handles all table creation
// V2 raw SQL migrations are NOT needed for PostgreSQL as GORM handles it
func MigratePostgres(st *Store) error {
	return st.DB.AutoMigrate(
		// Core models
		&models.AppSettings{},
		&models.Domain{},
		&models.Sender{},
		&models.AdminUser{},
		&models.AuthSession{},
		&models.BounceAccount{},
		&models.SystemIP{},
		&models.EmailStats{},
		&models.WebhookLog{},
		&models.APIKey{},
		&models.ChatLog{},
		&models.ContactList{},
		&models.Contact{},
		&models.Campaign{},
		&models.CampaignRecipient{},
		&models.AutomationWorkflow{},
		&models.WhatsAppMessage{},
		&models.Suppression{},
		&models.BounceEvent{},
		&models.ProcessedLogFile{},
		// Phase 2: Template & Subscriber Management
		&models.Template{},
		&models.CustomField{},
		&models.SubscriberFieldValue{},
		&models.Tag{},
		&models.SubscriberTag{},
		&models.Segment{},
		// Phase 2: A/B Testing & Analytics
		&models.ABTest{},
		&models.CampaignAnalytics{},
		&models.DomainReputation{},
		// Phase 2: Enhanced Automation
		&models.AutomationWorkflowV2{},
		&models.AutomationRun{},
		&models.SubscriberActivity{},
		&models.TrackingEvent{},
		// V2 Models with Organization support
		&models.AutomationV2{},
		&models.AutomationRunV2{},
		// Proxy and IP Management
		&models.Proxy{},
		&models.SendingIP{},
		// V2 Multi-tenancy models
		&models.Organization{},
		&models.OrganizationUser{},
		&models.ContactV2{},
		&models.EmailTemplate{},
		&models.TemplateBlock{},
		&models.CampaignV2{},
		&models.CampaignRecipientV2{},
		&models.AutomationActionLog{},
		&models.APIKeyV2{},
		&models.WebhookV2{},
	)
}
