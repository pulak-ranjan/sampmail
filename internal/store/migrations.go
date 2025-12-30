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
	Up          func(*sql.DB) error
	Down        func(*sql.DB) error
}

// MigrationManager handles database migrations
type MigrationManager struct {
	db         *sql.DB
	migrations []Migration
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sql.DB) *MigrationManager {
	return &MigrationManager{
		db:         db,
		migrations: registeredMigrations,
	}
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
		if err := migration.Up(m.db); err != nil {
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

	if err := migration.Down(m.db); err != nil {
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
		Up: func(db *sql.DB) error {
			// Initial schema is created by GORM AutoMigrate
			return nil
		},
		Down: func(db *sql.DB) error {
			// Cannot rollback initial schema
			return fmt.Errorf("cannot rollback initial schema")
		},
	},
	{
		Version:     2,
		Description: "Add indexes for performance",
		Up: func(db *sql.DB) error {
			indexes := []string{
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_status ON campaign_recipients(status)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_recipients_campaign_status ON campaign_recipients(campaign_id, status)",
				"CREATE INDEX IF NOT EXISTS idx_suppressions_email ON suppressions(email)",
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
		Down: func(db *sql.DB) error {
			indexes := []string{
				"DROP INDEX IF EXISTS idx_campaign_recipients_status",
				"DROP INDEX IF EXISTS idx_campaign_recipients_campaign_status",
				"DROP INDEX IF EXISTS idx_suppressions_email",
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
		Up: func(db *sql.DB) error {
			_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS audit_logs (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					actor TEXT NOT NULL,
					action TEXT NOT NULL,
					resource_type TEXT NOT NULL,
					resource_id TEXT,
					details TEXT,
					ip_address TEXT,
					user_agent TEXT
				);
				CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp);
				CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor);
				CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
			`)
			return err
		},
		Down: func(db *sql.DB) error {
			_, err := db.Exec("DROP TABLE IF EXISTS audit_logs")
			return err
		},
	},
	{
		Version:     4,
		Description: "Add proxies and sending_ips tables",
		Up: func(db *sql.DB) error {
			_, err := db.Exec(`
				CREATE TABLE IF NOT EXISTS proxies (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
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
				);

				CREATE TABLE IF NOT EXISTS sending_ips (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
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
				);

				CREATE INDEX IF NOT EXISTS idx_proxies_active ON proxies(is_active);
				CREATE INDEX IF NOT EXISTS idx_sending_ips_active ON sending_ips(is_active);
			`)
			return err
		},
		Down: func(db *sql.DB) error {
			_, err := db.Exec(`
				DROP TABLE IF EXISTS proxies;
				DROP TABLE IF EXISTS sending_ips;
			`)
			return err
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
	
	mgr := NewMigrationManager(sqlDB)
	return mgr.Migrate()
}


