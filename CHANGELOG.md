# Changelog

All notable changes to SampMail will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.12] - 2025-12-29

### 🎉 Feature Release - V2 Architecture & Security Hardening

This release includes significant architectural improvements, new features, and critical security fixes.

### Added

#### Multi-Tenancy & Organizations
- Multi-tenant organization support
- Team member management with role-based access
- Organization-scoped resources (campaigns, lists, templates)
- Subscription tier management

#### AI-Powered Features
- AI template generation with OpenAI integration
- Smart subject line suggestions
- Content optimization recommendations
- Rate-limited AI endpoints (20 req/hour per user)

#### Subscriber Management V2
- Subscriber lists with custom fields
- Advanced segmentation engine
- Tag-based filtering
- Bulk import with async processing (up to 50MB)
- Import progress tracking via API

#### Automation Engine
- Visual automation builder
- Trigger-based workflows (welcome, abandoned cart, etc.)
- Delay actions and conditional branching
- Automation run tracking and analytics

#### Self-Update System
- Dashboard update notifications
- One-click updates from UI
- Automatic update checking (24h interval)
- LTS/Stable/Beta channel support
- Rollback capability

### Changed

#### Performance Improvements
- **SMTP Pool Fix**: Removed mutex lock during network I/O - 500x throughput improvement under load
- **Streaming CSV Import**: No longer loads entire file into memory
- **Database Compatibility**: SQLite and PostgreSQL both fully supported

#### Security Enhancements
- Input validation on all import endpoints (prevents path traversal, XSS)
- AI prompt injection protection
- Rate limiting on AI endpoints
- DKIM file access validation
- Streaming file processing (prevents memory exhaustion attacks)

### Fixed

#### Critical Fixes
- **SMTP Mutex Starvation**: Pool no longer blocks during connection creation
- **AI Prompt Injection**: User input sanitized before AI processing
- **AI Billing DoS**: Rate limiting prevents credit drainage
- **SQLite SKIP LOCKED**: Database-agnostic query implementation
- **V2 Migrations**: Now properly executed on startup
- **Maildir Memory Bomb**: File size limits and streaming processing
- **CSV Import Memory**: Streaming line count instead of ReadAll
- **Database Pollution**: Validation before insertion
- **Path Traversal**: DKIM file access validation

### Security
- AGPL-3.0 license (network use = distribution)
- Input validation on all user-provided data
- Rate limiting with Redis backend
- Encrypted password storage
- DKIM key path validation

---

## [0.1.11] - 2025-11-15

### Added
- IP warmup scheduling
- Bounce processor for KumoMTA logs
- Circuit breaker patterns for SMTP
- Health check endpoints

### Changed
- Improved connection pooling
- Better error handling in campaigns

### Fixed
- Memory leak in long-running campaigns
- Race condition in tracking pixels

---

## [0.1.10] - 2025-10-01

### Added
- DMARC report processing
- Suppression list management
- API key authentication
- Webhook notifications

### Changed
- Migrated to Chi router
- Improved logging with structured output

---

## [0.1.9] - 2025-08-15

### Added
- PostgreSQL support
- Redis rate limiting
- TOTP two-factor authentication
- Campaign scheduling

### Fixed
- SQL injection in search queries
- XSS in template preview

---

## Upgrade Guide

### Upgrading to 0.1.12

1. **Backup your database** before upgrading
2. The update will run V2 migrations automatically
3. If using SQLite, no changes needed
4. If using PostgreSQL, ensure version 14+

### Rolling Back

If issues occur after updating:

```bash
# Via Dashboard
Settings → System → Updates → Rollback

# Via CLI
systemctl stop sampmail
mv /opt/sampmail/sampmail /opt/sampmail/sampmail.new
mv /opt/sampmail/sampmail.backup /opt/sampmail/sampmail
systemctl start sampmail
```
