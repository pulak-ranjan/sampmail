# SampMail Security Reference

This document explains the security mechanisms built into SampMail, how to
configure them correctly, and how to report vulnerabilities.

---

## Table of Contents

- [Secret Management](#secret-management)
- [Authentication](#authentication)
- [Two-Factor Authentication](#two-factor-authentication)
- [Session Handling](#session-handling)
- [Password Storage](#password-storage)
- [Rate Limiting](#rate-limiting)
- [Input Validation](#input-validation)
- [SQL Injection Prevention](#sql-injection-prevention)
- [XSS Prevention](#xss-prevention)
- [Open Redirect Prevention](#open-redirect-prevention)
- [HMAC-Signed Tracking Links](#hmac-signed-tracking-links)
- [Suppression List Integrity](#suppression-list-integrity)
- [WebSocket Connection Limits](#websocket-connection-limits)
- [TLS and HTTPS](#tls-and-https)
- [Systemd Hardening](#systemd-hardening)
- [Network Exposure](#network-exposure)
- [Security Checklist](#security-checklist)
- [Reporting Vulnerabilities](#reporting-vulnerabilities)

---

## Secret Management

All cryptographic keys are derived from a single master secret (`SAMPMAIL_SECRET`)
using PBKDF2-SHA256 with 100,000 iterations.

**Key derivation:**

| Derived key | Salt used | Purpose |
|---|---|---|
| Encryption key (32 bytes) | sampmail-encryption-key-v1 | AES-256 for encrypting stored secrets |
| Session key (32 bytes) | sampmail-session-key-v1 | HMAC-SHA256 for session token verification |

**Requirements for SAMPMAIL_SECRET:**

- Minimum 32 characters.
- Must be cryptographically random. Generate with: `openssl rand -base64 32`
- Never reuse across installations.
- Never change after first run. Changing it invalidates all active sessions and any
  data that was encrypted with the derived encryption key.
- Stored in `/etc/sampmail.env` with permissions 600, owned by root.

The install script generates this value automatically.

---

## Authentication

SampMail uses session tokens, not JWTs.

**Login flow:**

1. User submits email and password.
2. The bcrypt hash stored in the database is compared to the submitted password.
3. If correct and 2FA is not enabled, a 32-byte random session token is generated.
4. The token is HMAC-SHA256 hashed using the derived session key.
5. The hash is stored in the `auth_sessions` table with an expiry timestamp.
6. The raw token is returned to the client and used in subsequent `Authorization` headers.

The raw token is never persisted anywhere. A database breach exposes only the hashes,
which cannot be used to authenticate without the derived session key, which requires
the master secret.

**Session limits:**

A maximum of 3 concurrent sessions are allowed per user. Creating a new session when
the limit is reached invalidates the oldest session.

---

## Two-Factor Authentication

TOTP-based 2FA is available for all user accounts.

**Enrollment flow:**

1. `POST /api/auth/setup-2fa` — returns a TOTP secret and an `otpauth://` URI.
2. User scans the QR code in an authenticator app.
3. `POST /api/auth/enable-2fa` — user submits a code to confirm enrollment.

**Login with 2FA enabled:**

1. `POST /api/auth/login` returns `requires_2fa: true` and a short-lived temporary token.
2. `POST /api/auth/verify-2fa` validates the TOTP code. On success, a full session token
   is issued.

**Lockout:**

After repeated failed TOTP attempts, the account is locked for a cooldown period.
Lockout state is tracked in Redis. When Redis is unavailable, an in-memory fallback
provides the same protection (fail-secure, not fail-open). The in-memory state resets
on process restart, which is acceptable since the lockout window is short.

**Rate limiting on 2FA setup endpoints:**

The `/api/auth/setup-2fa`, `/api/auth/enable-2fa`, and `/api/auth/disable-2fa`
endpoints are rate-limited to prevent abuse.

---

## Session Handling

Sessions are stored in the `auth_sessions` table.

| Field | Description |
|---|---|
| token | HMAC-SHA256 hash of the raw session token |
| admin_id | The user this session belongs to |
| expires_at | Session expiry timestamp |
| device_ip | IP address at login time |
| user_agent | Browser or client identifier |

Sessions are validated on every authenticated request by hashing the submitted token
and comparing it to the stored hash.

Logout deletes the session row. There is no token revocation mechanism other than
deletion because the tokens are not self-contained (unlike JWTs).

---

## Password Storage

Passwords are hashed using bcrypt before storage.

- Default cost factor: 12 (configurable via `SAMPMAIL_BCRYPT_COST`).
- Recommended range for production: 12 to 14.
- At cost 12, a single hash takes approximately 250 ms, making brute force attacks
  impractical at scale.
- Passwords are never logged, returned in API responses, or stored in plaintext anywhere.

---

## Rate Limiting

Rate limiting is applied at multiple layers.

**Authentication endpoints:**

Login, registration, and 2FA verification are rate-limited per IP address using a
sliding window algorithm. Limits are stored in Redis when available, in-memory
otherwise.

**General API:**

A per-IP rate limit applies to all API endpoints to prevent abuse.

**Email verification:**

The `POST /api/contacts/verify` endpoint is individually rate-limited to prevent use
as a bulk verification proxy.

**Redis availability:**

When Redis is unavailable, all rate limiting falls back to in-memory counters. These
counters do not share state across multiple instances and reset on restart. This is
intentional — the alternative (failing open with no rate limiting) would be worse.

---

## Input Validation

All user-supplied input is validated before use.

- Email addresses are validated for format before being stored or used.
- Password complexity is enforced: minimum 8 characters, must contain at least one
  letter and one digit.
- Request body sizes are limited to `SAMPMAIL_MAX_REQUEST_BODY` (default 10 MB), with
  an exception for asynchronous CSV imports (50 MB).
- Pagination parameters are clamped to prevent excessive database queries.

---

## SQL Injection Prevention

All database queries use GORM with parameterized queries. Raw SQL is avoided.

In the one case where a column name is programmatically chosen (bounce counter
increment), an explicit whitelist is used:

```go
var column string
if bounceType == "soft" {
    column = "total_soft_bounced"
} else {
    column = "total_bounced"
}
db.Model(&Campaign{}).Where("id = ?", campaignID).UpdateColumn(column, gorm.Expr(column+" + 1"))
```

User-supplied strings are never interpolated directly into SQL.

---

## XSS Prevention

The email template builder renders HTML content using `dangerouslySetInnerHTML`.
All HTML is sanitized with DOMPurify before rendering:

```jsx
dangerouslySetInnerHTML={{
  __html: DOMPurify.sanitize(content.html || '', { USE_PROFILES: { html: true } })
}}
```

API responses are JSON. The frontend does not render raw API response strings as HTML
outside of controlled, sanitized contexts.

---

## Open Redirect Prevention

The click tracking endpoint (`/api/track/click/{id}`) redirects to user-supplied URLs.
Two protections are applied:

1. **HMAC signature validation** — the URL and recipient ID are signed at email
   generation time. Requests with invalid or missing signatures receive a 403 response
   and are not redirected.

2. **URL scheme validation** — only `http://` and `https://` schemes are allowed as
   redirect targets. `javascript:`, `data:`, and other schemes are rejected with 403.

---

## HMAC-Signed Tracking Links

Every tracking URL (open pixel and click redirect) includes an HMAC-SHA256 signature
computed over the recipient ID and, for click links, the target URL. The key is derived
from `SAMPMAIL_SECRET`.

This prevents:

- Fabrication of tracking events by external parties.
- Manipulation of click redirect targets.
- Enumeration of recipient IDs.

---

## Suppression List Integrity

The suppression list is the primary mechanism for ensuring unsubscribe requests and
bounce disposals are honored.

Checks are performed at two points:

1. **Import time** — suppressed addresses are excluded when a CSV is imported.
2. **Send time** — each recipient is checked individually before the email is handed
   to the SMTP pool.

This dual-check design ensures that addresses added to the suppression list after an
import are still excluded when the campaign actually sends.

---

## WebSocket Connection Limits

The WebSocket handler enforces a maximum of 1,000 concurrent connections:

```go
const wsMaxConnections = 1000
```

Requests that exceed this limit receive a 503 response. This prevents resource
exhaustion from clients that open many WebSocket connections.

---

## TLS and HTTPS

SampMail supports TLS in two configurations:

**Option 1: Nginx termination (recommended)**

Nginx handles TLS and proxies plaintext HTTP to SampMail on port 9000. Set
`SAMPMAIL_TRUST_PROXY=true` and `SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32` so that
SampMail correctly identifies client IP addresses from the `X-Forwarded-For` header.

**Option 2: Direct TLS**

Set `SAMPMAIL_TLS_ENABLED=true`, `SAMPMAIL_TLS_CERT`, and `SAMPMAIL_TLS_KEY`.
SampMail will use TLS 1.2 or later with a restricted cipher suite:

- TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
- TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
- TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305
- TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305
- TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
- TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256

---

## Systemd Hardening

The systemd service unit applies the following hardening options:

| Option | Effect |
|---|---|
| NoNewPrivileges=true | Prevents the process from gaining elevated privileges via setuid |
| ProtectSystem=strict | Mounts the system directories read-only |
| ProtectHome=true | Prevents access to user home directories |
| ReadWritePaths= | Restricts write access to only the data and log directories |
| PrivateTmp=true | Gives the process its own private /tmp namespace |

The process runs as the `sampmail` system user with no login shell and no home
directory write access outside of the designated data directory.

---

## Network Exposure

**Ports that must be accessible:**

| Port | Service | Exposure |
|---|---|---|
| 9000 | SampMail HTTP | Public (via Nginx on 443) |
| 443 | Nginx HTTPS | Public |
| 587 | KumoMTA submission | Public (for authenticated sending) |

**Ports that must not be exposed publicly:**

| Port | Service | Reason |
|---|---|---|
| 25 | SMTP | Prevents use as open relay |
| 5432 | PostgreSQL | Database should only be reachable from localhost |
| 6379 | Redis | No authentication required in default config |
| 8080 | Reacher | Internal verification service only |
| 8000 | KumoMTA API | Internal management API only |

---

## Security Checklist

Before putting a SampMail instance into production:

- [ ] `SAMPMAIL_SECRET` is at least 32 characters and was generated with `openssl rand -base64 32`
- [ ] `/etc/sampmail.env` has permissions 600 and is owned by root
- [ ] `SAMPMAIL_ENV=production` is set
- [ ] PostgreSQL is used instead of SQLite
- [ ] Nginx is configured as a reverse proxy with a valid TLS certificate
- [ ] `SAMPMAIL_BASE_URL` is set to the HTTPS domain
- [ ] `SAMPMAIL_TRUST_PROXY=true` and `SAMPMAIL_TRUSTED_PROXY_CIDR` are set correctly
- [ ] Port 25 is not exposed publicly
- [ ] Ports 5432, 6379, 8080, and 8000 are firewalled from external access
- [ ] Redis is configured with a password if accessible from a network
- [ ] `SAMPMAIL_BCRYPT_COST` is at least 12
- [ ] 2FA is enabled for all admin accounts
- [ ] DKIM, SPF, and DMARC DNS records are published for all sending domains

---

## Reporting Vulnerabilities

To report a security vulnerability, send a message to the maintainers directly rather
than opening a public GitHub issue. Include a description of the vulnerability, steps
to reproduce, and an assessment of the potential impact.

Do not publish vulnerability details publicly until a fix has been released and
sufficient time has passed for users to upgrade.
