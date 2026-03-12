package store

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// Migration represents a database migration
type Migration struct {
	Version     int
	Description string
	Up          func(*sql.DB, string) error // second param is dialect: "sqlite" or "postgres"
	Down        func(*sql.DB, string) error
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db         *sql.DB
	migrations []Migration
	dialect    string // "sqlite" or "postgres"
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB, dialect string) *MigrationManager {
	if dialect == "" {
		dialect = "sqlite"
	}
	return &MigrationManager{
		db:         db,
		migrations: registeredMigrations,
		dialect:    dialect,
	}
}

// autoIncrementSQL returns the correct auto-increment primary key syntax for the dialect
func (m *MigrationManager) autoIncrementSQL() string {
	if m.dialect == "postgres" {
		return "BIGSERIAL PRIMARY KEY"
	}
	return "INTEGER PRIMARY KEY AUTOINCREMENT"
}

// migrationTable creates the migrations tracking table
const migrationTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// Initialize creates the migrations table if it doesn't exist
func (m *MigrationManager) Initialize() error {
	_, err := m.db.Exec(migrationTable)
	return err
}

// CurrentVersion returns the current schema version
func (m *MigrationManager) CurrentVersion() (int, error) {
	var version int
	err := m.db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version)
	return version, err
}

// PendingMigrations returns migrations that haven't been applied
func (m *MigrationManager) PendingMigrations() ([]Migration, error) {
	current, err := m.CurrentVersion()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, migration := range m.migrations {
		if migration.Version > current {
			pending = append(pending, migration)
		}
	}

	// Sort by version
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

// Migrate runs all pending migrations
func (m *MigrationManager) Migrate() error {
	log := logger.WithComponent("migrations")

	if err := m.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize migrations: %w", err)
	}

	pending, err := m.PendingMigrations()
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	if len(pending) == 0 {
		log.Info("no pending migrations")
		return nil
	}

	log.Info("running migrations", "count", len(pending))

	for _, migration := range pending {
		log.Info("applying migration",
			"version", migration.Version,
			"description", migration.Description)

		// Begin transaction
		tx, err := m.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		// Run migration
		if err := migration.Up(m.db, m.dialect); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		// Record migration
		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)",
			migration.Version, migration.Description, time.Now(),
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration: %w", err)
		}

		log.Info("migration applied", "version", migration.Version)
	}

	return nil
}

// Rollback reverts the last migration
func (m *MigrationManager) Rollback() error {
	log := logger.WithComponent("migrations")

	current, err := m.CurrentVersion()
	if err != nil {
		return err
	}

	if current == 0 {
		log.Info("no migrations to rollback")
		return nil
	}

	// Find the current migration
	var migration *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == current {
			migration = &m.migrations[i]
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found", current)
	}

	if migration.Down == nil {
		return fmt.Errorf("migration %d has no rollback", current)
	}

	log.Info("rolling back migration",
		"version", migration.Version,
		"description", migration.Description)

	tx, err := m.db.Begin()
	if err != nil {
		return err
	}

	if err := migration.Down(m.db, m.dialect); err != nil {
		tx.Rollback()
		return fmt.Errorf("rollback failed: %w", err)
	}

	_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = ?", current)
	if err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// Status returns the current migration status
func (m *MigrationManager) Status() ([]MigrationStatus, error) {
	if err := m.Initialize(); err != nil {
		return nil, err
	}

	// Get applied migrations
	rows, err := m.db.Query("SELECT version, description, applied_at FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var desc string
		var appliedAt time.Time
		if err := rows.Scan(&version, &desc, &appliedAt); err != nil {
			return nil, err
		}
		applied[version] = appliedAt
	}

	// Build status
	var statuses []MigrationStatus
	for _, migration := range m.migrations {
		status := MigrationStatus{
			Version:     migration.Version,
			Description: migration.Description,
		}
		if appliedAt, ok := applied[migration.Version]; ok {
			status.Applied = true
			status.AppliedAt = &appliedAt
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int        `json:"version"`
	Description string     `json:"description"`
	Applied     bool       `json:"applied"`
	AppliedAt   *time.Time `json:"applied_at,omitempty"`
}

// registeredMigrations holds all migrations
// Add new migrations here
var registeredMigrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema (handled by GORM AutoMigrate)",
		Up: func(db *sql.DB, dialect string) error {
			// Initial schema is created by GORM AutoMigrate
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			// Cannot rollback initial schema
			return fmt.Errorf("cannot rollback initial schema")
		},
	},
	{
		Version:     2,
		Description: "Add indexes for performance",
		Up: func(db *sql.DB, dialect string) error {
			indexes := []string{
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_status ON campaign_recipients(status)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_campaign_status ON campaign_recipients(campaign_id, status)",
				"CREATE INDEX IF NOT EXISTS idx_bounce_events_email ON bounce_events(email)",
				"CREATE INDEX IF NOT EXISTS idx_bounce_events_processed_at ON bounce_events(processed_at)",
				"CREATE INDEX IF NOT EXISTS idx_email_stats_domain_date ON email_stats(domain, date)",
				"CREATE INDEX IF NOT EXISTS idx_subscriber_activity_contact_id ON subscriber_activities(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_subscriber_activity_created_at ON subscriber_activities(created_at)",
			}
			for _, idx := range indexes {
				if _, err := db.Exec(idx); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			indexes := []string{
				"DROP INDEX IF EXISTS idx_campaign_recipients_status",
				"DROP INDEX IF EXISTS idx_campaign_recipients_campaign_status",
				"DROP INDEX IF EXISTS idx_bounce_events_email",
				"DROP INDEX IF EXISTS idx_bounce_events_processed_at",
				"DROP INDEX IF EXISTS idx_email_stats_domain_date",
				"DROP INDEX IF EXISTS idx_subscriber_activity_contact_id",
				"DROP INDEX IF EXISTS idx_subscriber_activity_created_at",
			}
			for _, idx := range indexes {
				if _, err := db.Exec(idx); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:     3,
		Description: "Add audit logging table",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			_, err := db.Exec(fmt.Sprintf(`
				CREATE TABLE IF NOT EXISTS audit_logs (
					id %s,
					timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					actor TEXT NOT NULL,
					action TEXT NOT NULL,
					resource_type TEXT NOT NULL,
					resource_id TEXT,
					details TEXT,
					ip_address TEXT,
					user_agent TEXT
				)`, serial))
			if err != nil {
				return err
			}
			for _, idx := range []string{
				"CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)",
				"CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor)",
				"CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)",
			} {
				if _, err := db.Exec(idx); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			_, err := db.Exec("DROP TABLE IF EXISTS audit_logs")
			return err
		},
	},
	{
		Version:     4,
		Description: "Add proxies and sending_ips tables",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				fmt.Sprintf(`CREATE TABLE IF NOT EXISTS proxies (
					id %s,
					name TEXT,
					type TEXT NOT NULL DEFAULT 'socks5',
					host TEXT NOT NULL,
					port INTEGER NOT NULL,
					username TEXT,
					password TEXT,
					is_active INTEGER DEFAULT 1,
					use_for TEXT DEFAULT 'both',
					priority INTEGER DEFAULT 0,
					last_check TIMESTAMP,
					is_healthy INTEGER DEFAULT 1,
					fail_count INTEGER DEFAULT 0,
					success_rate REAL DEFAULT 0,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`, serial),
				fmt.Sprintf(`CREATE TABLE IF NOT EXISTS sending_ips (
					id %s,
					ip_address TEXT NOT NULL UNIQUE,
					hostname TEXT,
					is_active INTEGER DEFAULT 1,
					warmup_stage INTEGER DEFAULT 0,
					daily_send_limit INTEGER DEFAULT 100,
					today_sent INTEGER DEFAULT 0,
					reputation_score REAL DEFAULT 50,
					last_bounce_rate REAL DEFAULT 0,
					domain_id INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`, serial),
				"CREATE INDEX IF NOT EXISTS idx_proxies_active ON proxies(is_active)",
				"CREATE INDEX IF NOT EXISTS idx_sending_ips_active ON sending_ips(is_active)",
			}
			for _, stmt := range stmts {
				if _, err := db.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			for _, stmt := range []string{"DROP TABLE IF EXISTS proxies", "DROP TABLE IF EXISTS sending_ips"} {
				if _, err := db.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		Version:     5,
		Description: "Add complaint_logs table for FBL processing",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				fmt.Sprintf(`CREATE TABLE IF NOT EXISTS complaint_logs (
					id %s,
					email TEXT NOT NULL,
					feedback_type TEXT NOT NULL,
					provider TEXT,
					campaign_id INTEGER,
					received_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					source_ip TEXT,
					user_agent TEXT
				)`, serial),
				"CREATE INDEX IF NOT EXISTS idx_complaint_logs_email ON complaint_logs(email)",
				"CREATE INDEX IF NOT EXISTS idx_complaint_logs_campaign ON complaint_logs(campaign_id)",
				"CREATE INDEX IF NOT EXISTS idx_complaint_logs_received ON complaint_logs(received_at)",
			}
			for _, stmt := range stmts {
				if _, err := db.Exec(stmt); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			_, err := db.Exec("DROP TABLE IF EXISTS complaint_logs")
			return err
		},
	},
	{
		Version:     12,
		Description: "Add critical FK and query indexes for multi-tenancy and performance",
		Up: func(db *sql.DB, dialect string) error {
			indexes := []string{
				// Organization context — used in nearly every query
				"CREATE INDEX IF NOT EXISTS idx_campaigns_organization_id ON campaigns(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_domains_organization_id ON domains(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_suppressions_organization_id ON suppressions(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id)",
				// Composite: per-org suppression check (IsSuppressed hot path)
				"CREATE INDEX IF NOT EXISTS idx_suppressions_org_email ON suppressions(organization_id, email)",
				// Campaign FK lookups
				"CREATE INDEX IF NOT EXISTS idx_campaigns_sender_id ON campaigns(sender_id)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_campaign_id ON campaign_recipients(campaign_id)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_contact_id ON campaign_recipients(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_sent_at ON campaign_recipients(sent_at)",
				// Tracking events
				"CREATE INDEX IF NOT EXISTS idx_tracking_events_campaign_id ON tracking_events(campaign_id)",
				"CREATE INDEX IF NOT EXISTS idx_tracking_events_recipient_id ON tracking_events(recipient_id)",
				"CREATE INDEX IF NOT EXISTS idx_tracking_events_event_type ON tracking_events(event_type)",
				// Bounce events
				"CREATE INDEX IF NOT EXISTS idx_bounce_events_campaign_id ON bounce_events(campaign_id)",
				// Senders
				"CREATE INDEX IF NOT EXISTS idx_senders_domain_id ON senders(domain_id)",
				// Automation
				"CREATE INDEX IF NOT EXISTS idx_automation_runs_contact_id ON automation_run_v2s(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_automation_runs_status ON automation_run_v2s(status)",
				"CREATE INDEX IF NOT EXISTS idx_automation_runs_next_action ON automation_run_v2s(next_action_at)",
				// Sessions
				"CREATE INDEX IF NOT EXISTS idx_sessions_admin_id ON sessions(admin_id)",
				"CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)",
				// Org membership
				"CREATE INDEX IF NOT EXISTS idx_org_users_admin_id ON organization_users(admin_id)",
				"CREATE INDEX IF NOT EXISTS idx_org_users_org_id ON organization_users(organization_id)",
			}
			for _, idx := range indexes {
				if _, err := db.Exec(idx); err != nil {
					// Non-fatal: some indexes may already exist via GORM tags
					fmt.Printf("[migration 6] warning creating index: %v\n", err)
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			drops := []string{
				"DROP INDEX IF EXISTS idx_campaigns_organization_id",
				"DROP INDEX IF EXISTS idx_domains_organization_id",
				"DROP INDEX IF EXISTS idx_suppressions_organization_id",
				"DROP INDEX IF EXISTS idx_api_keys_organization_id",
				"DROP INDEX IF EXISTS idx_suppressions_org_email",
				"DROP INDEX IF EXISTS idx_campaigns_sender_id",
				"DROP INDEX IF EXISTS idx_campaign_recipients_campaign_id",
				"DROP INDEX IF EXISTS idx_campaign_recipients_contact_id",
				"DROP INDEX IF EXISTS idx_campaign_recipients_sent_at",
				"DROP INDEX IF EXISTS idx_tracking_events_campaign_id",
				"DROP INDEX IF EXISTS idx_tracking_events_recipient_id",
				"DROP INDEX IF EXISTS idx_tracking_events_event_type",
				"DROP INDEX IF EXISTS idx_bounce_events_campaign_id",
				"DROP INDEX IF EXISTS idx_senders_domain_id",
				"DROP INDEX IF EXISTS idx_automation_runs_contact_id",
				"DROP INDEX IF EXISTS idx_automation_runs_status",
				"DROP INDEX IF EXISTS idx_automation_runs_next_action",
				"DROP INDEX IF EXISTS idx_sessions_admin_id",
				"DROP INDEX IF EXISTS idx_sessions_expires_at",
				"DROP INDEX IF EXISTS idx_org_users_admin_id",
				"DROP INDEX IF EXISTS idx_org_users_org_id",
			}
			for _, d := range drops {
				db.Exec(d) //nolint:errcheck
			}
			return nil
		},
	},
	{
		Version:     16,
		Description: "Add AI bounce analysis fields",
		Up: func(db *sql.DB, dialect string) error {
			fields := []string{
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_category VARCHAR(50)",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_severity VARCHAR(20)",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_explanation TEXT",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_action TEXT",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_is_retryable BOOLEAN DEFAULT true",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_is_permanent_fail BOOLEAN DEFAULT false",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_email_quality VARCHAR(20)",
				"ALTER TABLE bounce_events ADD COLUMN IF NOT EXISTS ai_analyzed_at TIMESTAMP",
			}
			for _, f := range fields {
				if _, err := db.Exec(f); err != nil {
					logger.Warn("migration field may already exist", "field", f, "error", err)
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			drops := []string{
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_category",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_severity",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_explanation",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_action",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_is_retryable",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_is_permanent_fail",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_email_quality",
				"ALTER TABLE bounce_events DROP COLUMN IF EXISTS ai_analyzed_at",
			}
			for _, d := range drops {
				db.Exec(d) //nolint:errcheck
			}
			return nil
		},
	},
}

// RegisterMigration adds a new migration
func RegisterMigration(m Migration) {
	registeredMigrations = append(registeredMigrations, m)
}

// RunV2Migrations runs all pending migrations using a GORM DB connection
// This is a convenience wrapper for use with GORM's underlying *sql.DB
func RunV2Migrations(gormDB interface{}) error {
	// Try to get the underlying *sql.DB using GORM's method
	type sqlDBGetter interface {
		DB() (*sql.DB, error)
	}

	getter, ok := gormDB.(sqlDBGetter)
	if !ok {
		return fmt.Errorf("cannot get sql.DB from provided database connection")
	}

	sqlDB, err := getter.DB()
	if err != nil {
		return err
	}

	mgr := NewMigrationManager(sqlDB, "sqlite")
	return mgr.Migrate()
}
