# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

---

## [0.2.1] - 2026-01-07

### Fixed

- **Critical:** Database driver now auto-detected from `DATABASE_URL` environment variable
- PostgreSQL connection works with standard `DATABASE_URL=postgres://user:pass@host:port/db?sslmode=disable` format
- No longer requires separate `SAMPMAIL_DB_DRIVER=postgres` setting

---

## [0.2.0] - 2026-01-07

### 🎉 Major Release: Multi-Tenancy & Admin Dashboard

This release introduces a complete separation between Admin and User roles, with dedicated dashboards for each.

### Added

**Multi-Tenancy System**
- Organization (Tenant) management for Super Admins
- Organization switcher in sidebar
- Org-scoped data isolation for campaigns, lists, subscribers

**Admin Dashboard**
- System-wide health monitoring (CPU, RAM)
- Service status display (KumoMTA, Dovecot, Fail2Ban)
- Open ports visualization
- Quick access links to admin tools

**User Dashboard**
- Campaign performance stats (sent, opens, clicks)
- Performance overview with trend indicators
- Recent activity feed
- Quick links to marketing features

**Service Manager**
- One-click install/start/stop/restart for KumoMTA, Dovecot, Reacher
- Real-time service status monitoring
- Service descriptions and documentation

**Navigation Overhaul**
- Split sidebar into "Admin Station" and "Org Workspace"
- Category headers for better organization
- Context-aware navigation based on role/org selection

### Changed

- Disabled public registration (Admins create users only)
- Smart Dashboard routing based on user role and org context
- Enhanced UI consistency across all pages

### Fixed

- `e.map is not a function` crash on Templates/Lists pages
- Light mode visibility issues (headers, text contrast)
- All emojis replaced with professional Lucide icons
- API error handling with Array.isArray() checks
- SSLPage missing Check import

### Security

- Public registration disabled by default
- Super Admin required to create organizations
- Organization-scoped API routes

---

## [0.1.12] - 2025-12-29

### Added

**Core Features**
- Email campaign management (create, schedule, send)
- Multi-domain support with automatic DKIM signing
- Subscriber list management with CSV import
- Email verification via [Reacher](https://reacher.email/) integration
- Real-time analytics and reporting
- Bounce processing (KumoMTA logs + Maildir)
- Suppression list management

**Infrastructure**
- [KumoMTA](https://kumomta.com/) configuration generation (Lua + TOML)
- IP warmup scheduling with automatic progression
- Queue monitoring and management
- DNS record generation (SPF, DKIM, DMARC)
- Backup and restore functionality

**Security**
- Two-factor authentication (TOTP)
- API key management
- Encrypted credential storage (AES-256-GCM)
- Rate limiting with proxy support
- HTML sanitization (XSS prevention)
- No shell command execution

**User Interface**
- Modern React frontend with Tailwind CSS
- Dark/light theme support
- AI assistant for server management
- Real-time notifications
- Mobile-responsive design

**API**
- RESTful API for all operations
- JWT authentication
- Webhook support for event notifications

### Security Hardening

- Eliminated all shell command execution
- Input validation with allowlists
- HTML sanitization using Bluemonday
- Removed root privilege requirements
- Path traversal prevention
- Lua injection prevention via templating

### Performance

- Concurrent campaign sending with worker pool (50 workers default)
- Batch database operations for imports
- Log parsing optimization with caching
- Connection pooling for SMTP

### Docker Support

- Multi-stage Dockerfile
- Docker Compose with Reacher included
- Environment-based configuration
- Graceful shutdown handling

---

## Version History

| Version | Release Date | Notes |
|---------|--------------|-------|
| 0.2.1 | 2026-01-07 | DATABASE_URL auto-detection fix |
| 0.2.0 | 2026-01-07 | Multi-tenancy, Admin/User Dashboards |
| 0.1.12 | 2025-12-29 | Initial release |

---

## Upgrade Notes

### Upgrading to 0.1.12

1. **Environment Variables Required**:
   ```bash
   export SAMPMAIL_SECRET="$(openssl rand -base64 32)"
   ```

2. **User Permissions**: Application runs as non-root
   ```bash
   useradd -r sampmail
   chown -R sampmail:sampmail /var/lib/sampmail
   ```

3. **Database Migration**: Automatic on first startup

4. **Reacher Integration**: Add to environment
   ```bash
   export REACHER_URL="http://localhost:8080"
   ```
