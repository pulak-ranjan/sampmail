# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Template library with built-in email templates
- Tags and segments for subscriber management
- Enhanced analytics dashboard with deliverability scoring

### Changed
- Improved CSV import performance with batch inserts

### Fixed
- Rate limiting now correctly identifies client IP behind proxies

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
