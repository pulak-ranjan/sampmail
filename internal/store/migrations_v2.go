package store

import (
	"database/sql"
)

// RegisterV2Migrations registers all V2 migrations.
// These migrations use the dialect-aware signature func(*sql.DB, string) error.
func RegisterV2Migrations() {
	// Migration 5: Organizations (Multi-tenancy)
	RegisterMigration(Migration{
		Version:     5,
		Description: "Add organizations for multi-tenancy",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS organizations (
					id ` + serial + `,
					name TEXT NOT NULL,
					slug TEXT UNIQUE NOT NULL,
					plan TEXT DEFAULT 'free',
					max_users INTEGER DEFAULT 5,
					max_contacts INTEGER DEFAULT 1000,
					max_emails INTEGER DEFAULT 10000,
					logo_url TEXT,
					primary_color TEXT DEFAULT '#3B82F6',
					settings TEXT,
					contacts_used INTEGER DEFAULT 0,
					emails_sent INTEGER DEFAULT 0,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS organization_users (
					id ` + serial + `,
					organization_id INTEGER NOT NULL,
					admin_id INTEGER NOT NULL,
					role TEXT DEFAULT 'viewer',
					permissions TEXT,
					invited_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					joined_at TIMESTAMP,
					FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE,
					FOREIGN KEY (admin_id) REFERENCES admin_users(id) ON DELETE CASCADE,
					UNIQUE(organization_id, admin_id)
				)`,
				"CREATE INDEX IF NOT EXISTS idx_org_users_org ON organization_users(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_org_users_admin ON organization_users(admin_id)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			for _, s := range []string{
				"DROP TABLE IF EXISTS organization_users",
				"DROP TABLE IF EXISTS organizations",
			} {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
	})

	// Migration 6: Enhanced Contacts (V2)
	RegisterMigration(Migration{
		Version:     6,
		Description: "Add enhanced contacts with more fields",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS contact_v2s (
					id ` + serial + `,
					organization_id INTEGER,
					list_id INTEGER,
					email TEXT NOT NULL,
					first_name TEXT,
					last_name TEXT,
					phone TEXT,
					company TEXT,
					job_title TEXT,
					address1 TEXT,
					address2 TEXT,
					city TEXT,
					state TEXT,
					postal_code TEXT,
					country TEXT,
					custom_fields TEXT,
					lead_score INTEGER DEFAULT 0,
					total_opens INTEGER DEFAULT 0,
					total_clicks INTEGER DEFAULT 0,
					last_opened_at TIMESTAMP,
					last_clicked_at TIMESTAMP,
					status TEXT DEFAULT 'active',
					subscribed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					unsubscribed_at TIMESTAMP,
					is_verified INTEGER DEFAULT 0,
					risk_score INTEGER DEFAULT 0,
					timezone TEXT,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				"CREATE INDEX IF NOT EXISTS idx_contact_v2_org ON contact_v2s(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_contact_v2_list ON contact_v2s(list_id)",
				"CREATE INDEX IF NOT EXISTS idx_contact_v2_email ON contact_v2s(email)",
				"CREATE INDEX IF NOT EXISTS idx_contact_v2_status ON contact_v2s(status)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			_, err := db.Exec("DROP TABLE IF EXISTS contact_v2s")
			return err
		},
	})

	// Migration 7: Email Templates (V2)
	RegisterMigration(Migration{
		Version:     7,
		Description: "Add enhanced email templates with builder support",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS email_templates (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					subject TEXT,
					preview_text TEXT,
					builder_json TEXT,
					html_content TEXT,
					text_content TEXT,
					category TEXT DEFAULT 'custom',
					tags TEXT,
					thumbnail_url TEXT,
					is_public INTEGER DEFAULT 0,
					version INTEGER DEFAULT 1,
					created_by INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				"CREATE INDEX IF NOT EXISTS idx_email_templates_org ON email_templates(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_email_templates_category ON email_templates(category)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			_, err := db.Exec("DROP TABLE IF EXISTS email_templates")
			return err
		},
	})

	// Migration 8: Campaigns V2
	RegisterMigration(Migration{
		Version:     8,
		Description: "Add enhanced campaigns with automation support",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS campaign_v2s (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					subject TEXT,
					preview_text TEXT,
					from_name TEXT,
					from_email TEXT,
					reply_to TEXT,
					template_id INTEGER,
					list_ids TEXT,
					segment_ids TEXT,
					excluded_list_ids TEXT,
					status TEXT DEFAULT 'draft',
					type TEXT DEFAULT 'regular',
					scheduled_at TIMESTAMP,
					sent_at TIMESTAMP,
					completed_at TIMESTAMP,
					total_recipients INTEGER DEFAULT 0,
					sent_count INTEGER DEFAULT 0,
					delivered_count INTEGER DEFAULT 0,
					opened_count INTEGER DEFAULT 0,
					clicked_count INTEGER DEFAULT 0,
					bounced_count INTEGER DEFAULT 0,
					unsubscribed_count INTEGER DEFAULT 0,
					complained_count INTEGER DEFAULT 0,
					ab_test_config TEXT,
					winning_variant TEXT,
					send_config TEXT,
					tracking_config TEXT,
					created_by INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				"CREATE INDEX IF NOT EXISTS idx_campaign_v2_org ON campaign_v2s(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_v2_status ON campaign_v2s(status)",
				"CREATE INDEX IF NOT EXISTS idx_campaign_v2_scheduled ON campaign_v2s(scheduled_at)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			_, err := db.Exec("DROP TABLE IF EXISTS campaign_v2s")
			return err
		},
	})

	// Migration 9: Automation Workflows
	RegisterMigration(Migration{
		Version:     9,
		Description: "Add automation workflows for email sequences",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS automation_workflows (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					description TEXT,
					trigger_type TEXT NOT NULL,
					trigger_config TEXT,
					is_active INTEGER DEFAULT 0,
					status TEXT DEFAULT 'draft',
					entry_count INTEGER DEFAULT 0,
					completion_count INTEGER DEFAULT 0,
					created_by INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS automation_steps (
					id ` + serial + `,
					workflow_id INTEGER NOT NULL,
					step_order INTEGER NOT NULL,
					step_type TEXT NOT NULL,
					config TEXT,
					delay_value INTEGER DEFAULT 0,
					delay_unit TEXT DEFAULT 'hours',
					condition_config TEXT,
					template_id INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					FOREIGN KEY (workflow_id) REFERENCES automation_workflows(id) ON DELETE CASCADE
				)`,
				`CREATE TABLE IF NOT EXISTS automation_enrollments (
					id ` + serial + `,
					workflow_id INTEGER NOT NULL,
					contact_id INTEGER NOT NULL,
					current_step INTEGER DEFAULT 0,
					status TEXT DEFAULT 'active',
					enrolled_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					next_step_at TIMESTAMP,
					completed_at TIMESTAMP,
					FOREIGN KEY (workflow_id) REFERENCES automation_workflows(id) ON DELETE CASCADE
				)`,
				"CREATE INDEX IF NOT EXISTS idx_automation_org ON automation_workflows(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_automation_steps_workflow ON automation_steps(workflow_id)",
				"CREATE INDEX IF NOT EXISTS idx_automation_enrollments_workflow ON automation_enrollments(workflow_id)",
				"CREATE INDEX IF NOT EXISTS idx_automation_enrollments_contact ON automation_enrollments(contact_id)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			for _, s := range []string{
				"DROP TABLE IF EXISTS automation_enrollments",
				"DROP TABLE IF EXISTS automation_steps",
				"DROP TABLE IF EXISTS automation_workflows",
			} {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
	})

	// Migration 10: Lists V2 with Tags and Segments
	RegisterMigration(Migration{
		Version:     10,
		Description: "Add enhanced lists with tags and segments",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS list_v2s (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					description TEXT,
					type TEXT DEFAULT 'standard',
					double_optin INTEGER DEFAULT 0,
					welcome_email_id INTEGER,
					default_from_name TEXT,
					default_from_email TEXT,
					subscriber_count INTEGER DEFAULT 0,
					unsubscribe_count INTEGER DEFAULT 0,
					created_by INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS tags (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					color TEXT DEFAULT '#6B7280',
					description TEXT,
					contact_count INTEGER DEFAULT 0,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS contact_tags (
					id ` + serial + `,
					contact_id INTEGER NOT NULL,
					tag_id INTEGER NOT NULL,
					added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					UNIQUE(contact_id, tag_id)
				)`,
				`CREATE TABLE IF NOT EXISTS segments (
					id ` + serial + `,
					organization_id INTEGER,
					name TEXT NOT NULL,
					description TEXT,
					conditions TEXT,
					match_type TEXT DEFAULT 'all',
					is_dynamic INTEGER DEFAULT 1,
					contact_count INTEGER DEFAULT 0,
					last_calculated_at TIMESTAMP,
					created_by INTEGER,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				"CREATE INDEX IF NOT EXISTS idx_list_v2_org ON list_v2s(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_tags_org ON tags(organization_id)",
				"CREATE INDEX IF NOT EXISTS idx_contact_tags_contact ON contact_tags(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_contact_tags_tag ON contact_tags(tag_id)",
				"CREATE INDEX IF NOT EXISTS idx_segments_org ON segments(organization_id)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			for _, s := range []string{
				"DROP TABLE IF EXISTS segments",
				"DROP TABLE IF EXISTS contact_tags",
				"DROP TABLE IF EXISTS tags",
				"DROP TABLE IF EXISTS list_v2s",
			} {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
	})

	// Migration 11: Subscriber Activity Tracking
	RegisterMigration(Migration{
		Version:     11,
		Description: "Add subscriber activity tracking",
		Up: func(db *sql.DB, dialect string) error {
			serial := "INTEGER PRIMARY KEY AUTOINCREMENT"
			if dialect == "postgres" {
				serial = "BIGSERIAL PRIMARY KEY"
			}
			stmts := []string{
				`CREATE TABLE IF NOT EXISTS subscriber_activities (
					id ` + serial + `,
					contact_id INTEGER NOT NULL,
					campaign_id INTEGER,
					activity_type TEXT NOT NULL,
					metadata TEXT,
					ip_address TEXT,
					user_agent TEXT,
					location TEXT,
					created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`,
				`CREATE TABLE IF NOT EXISTS email_events (
					id ` + serial + `,
					campaign_id INTEGER,
					contact_id INTEGER,
					message_id TEXT,
					event_type TEXT NOT NULL,
					timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
					metadata TEXT,
					processed INTEGER DEFAULT 0
				)`,
				"CREATE INDEX IF NOT EXISTS idx_subscriber_activity_contact ON subscriber_activities(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_subscriber_activity_type ON subscriber_activities(activity_type)",
				"CREATE INDEX IF NOT EXISTS idx_subscriber_activity_created ON subscriber_activities(created_at)",
				"CREATE INDEX IF NOT EXISTS idx_email_events_campaign ON email_events(campaign_id)",
				"CREATE INDEX IF NOT EXISTS idx_email_events_contact ON email_events(contact_id)",
				"CREATE INDEX IF NOT EXISTS idx_email_events_type ON email_events(event_type)",
			}
			for _, s := range stmts {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(db *sql.DB, dialect string) error {
			for _, s := range []string{
				"DROP TABLE IF EXISTS email_events",
				"DROP TABLE IF EXISTS subscriber_activities",
			} {
				if _, err := db.Exec(s); err != nil {
					return err
				}
			}
			return nil
		},
	})
}

// init registers V2 migrations at startup
func init() {
	RegisterV2Migrations()
}
