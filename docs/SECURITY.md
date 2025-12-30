# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

**DO NOT** open a public GitHub issue for security vulnerabilities.

Instead, please email us at: **cloudnesh@gmail.com**

Include:
1. Description of the vulnerability
2. Steps to reproduce
3. Potential impact
4. Suggested fix (if any)

### What to Expect

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 1 week
- **Resolution Timeline**: Depends on severity
  - Critical: 24-72 hours
  - High: 1-2 weeks
  - Medium: 2-4 weeks
  - Low: Next release

### Disclosure Policy

- We will work with you to understand and resolve the issue
- We will credit you in the security advisory (unless you prefer anonymity)
- We ask that you do not disclose publicly until we've released a fix

---

## Security Features

### Authentication & Authorization

- **Password Hashing**: bcrypt with cost factor 12
- **Session Management**: JWT tokens with configurable expiry
- **Two-Factor Authentication**: TOTP-based (RFC 6238)
- **Rate Limiting**: Per-IP rate limiting on auth endpoints

### Data Protection

- **Encryption at Rest**: AES-256-GCM for sensitive data
  - API keys
  - SMTP passwords
  - Webhook secrets
- **Encryption in Transit**: TLS 1.2+ required for production
- **Database**: SQLite with restricted file permissions

### Input Validation

- **Strict Allowlists**: All user input validated
- **HTML Sanitization**: Bluemonday library for XSS prevention
- **SQL Injection**: Parameterized queries via GORM
- **Path Traversal**: Validated file paths with prefix checks

### No Shell Execution

The application does not execute shell commands. All system operations use:
- Native Go libraries for DNS lookups
- Database queries for log access
- HTTP APIs for service status
- Go standard library for file operations

### Network Security

- **Proxy-Aware Rate Limiting**: Respects X-Forwarded-For from trusted proxies
- **CORS**: Configurable allowed origins
- **CSRF Protection**: Token-based protection
- **IP Blocking**: Firewall integration (firewalld/iptables)

---

## Security Configuration

### Required Settings

```bash
# Strong secret key (minimum 32 characters)
export SAMPMAIL_SECRET="$(openssl rand -base64 32)"
```

### Production Recommendations

```bash
# Bind to localhost, use reverse proxy for TLS
SAMPMAIL_LISTEN_ADDR=127.0.0.1:9000

# Trust proxy headers only from known proxies
SAMPMAIL_TRUST_PROXY=true
SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32,10.0.0.0/8

# Enable firewall integration
SAMPMAIL_FIREWALL_ENABLED=true
```

### File Permissions

```bash
# Database file
chmod 600 /var/lib/sampmail/panel.db
chown sampmail:sampmail /var/lib/sampmail/panel.db

# DKIM keys
chmod 600 /opt/kumomta/etc/dkim/*/*.key
chmod 644 /opt/kumomta/etc/dkim/*/*.pub

# Environment file
chmod 600 /etc/sampmail.env
```

---

## Security Checklist

### Deployment

- [ ] Strong `SAMPMAIL_SECRET` set (32+ chars, random)
- [ ] TLS configured on reverse proxy
- [ ] Database file permissions restricted
- [ ] Application running as non-root user
- [ ] Firewall rules in place
- [ ] Rate limiting enabled
- [ ] Log monitoring configured

### Operations

- [ ] Regular security updates applied
- [ ] Database backups encrypted
- [ ] Access logs reviewed
- [ ] Failed login attempts monitored
- [ ] API keys rotated periodically

---

## Security Audit Log

The application logs security-relevant events:

```json
{
  "level": "info",
  "component": "security",
  "action": "login_success",
  "user": "admin@example.com",
  "ip": "192.168.1.100",
  "timestamp": "2025-12-29T10:30:00Z"
}
```

**Logged Events:**
- Login attempts (success/failure)
- 2FA setup/verification
- API key creation/revocation
- IP blocks
- Configuration changes
- Admin actions

---

## Vulnerability History

| Date | Severity | Description | Fixed In |
|------|----------|-------------|----------|
| - | - | No vulnerabilities reported yet | - |

---

## Third-Party Dependencies

We regularly audit dependencies for known vulnerabilities:

```bash
# Go dependencies
go list -json -m all | nancy sleuth

# NPM dependencies
cd web && npm audit
```

**Key Dependencies:**
- `golang.org/x/crypto` - Cryptographic functions
- `github.com/microcosm-cc/bluemonday` - HTML sanitization
- `gorm.io/gorm` - SQL ORM with parameterized queries
