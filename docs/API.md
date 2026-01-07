# API Reference

SampMail provides a RESTful API for all operations. All endpoints require authentication unless otherwise noted.

## Base URL

```
http://localhost:9000/api
```

## Authentication

### Login

```http
POST /api/auth/login
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "your-password"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": 1,
    "email": "admin@example.com",
    "is_admin": true,
    "two_factor_enabled": false
  }
}
```

### Using the Token

Include the token in all subsequent requests:

```http
Authorization: Bearer <token>
```

### Register (Deprecated in v0.2.0+)

> **Note:** Public registration is disabled in v0.2.0+. Super Admins create users via CLI:
> ```bash
> ./sampmail user create email@example.com "password" --role super_admin
> ```

```http
POST /api/auth/register
Content-Type: application/json

{
  "email": "admin@example.com",
  "password": "secure-password-123"
}
```

### Two-Factor Authentication

**Setup 2FA:**
```http
POST /api/auth/setup-2fa

Response:
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code": "data:image/png;base64,..."
}
```

**Enable 2FA:**
```http
POST /api/auth/enable-2fa
{
  "code": "123456"
}
```

**Verify 2FA (after login):**
```http
POST /api/auth/verify-2fa
{
  "code": "123456",
  "temp_token": "temporary-token-from-login"
}
```

---

## Domains

### List Domains

```http
GET /api/domains
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "example.com",
    "mail_host": "mail.example.com",
    "bounce_host": "bounce.example.com",
    "dmarc_policy": "quarantine",
    "dmarc_percentage": 100,
    "created_at": "2025-12-29T10:30:00Z"
  }
]
```

### Create Domain

```http
POST /api/domains
Content-Type: application/json

{
  "name": "example.com",
  "mail_host": "mail.example.com",
  "bounce_host": "bounce.example.com"
}
```

### Get Domain

```http
GET /api/domains/{id}
```

### Update Domain

```http
PUT /api/domains/{id}
Content-Type: application/json

{
  "dmarc_policy": "reject",
  "dmarc_percentage": 100
}
```

### Delete Domain

```http
DELETE /api/domains/{id}
```

---

## Senders

### List Senders for Domain

```http
GET /api/domains/{domainID}/senders
```

**Response:**
```json
[
  {
    "id": 1,
    "domain_id": 1,
    "local_part": "newsletter",
    "email": "newsletter@example.com",
    "ip": "192.168.1.100",
    "created_at": "2025-12-29T10:30:00Z"
  }
]
```

### Create Sender

```http
POST /api/domains/{domainID}/senders
Content-Type: application/json

{
  "local_part": "newsletter",
  "ip": "192.168.1.100",
  "smtp_password": "optional-smtp-auth-password"
}
```

### Setup Sender (Generate DKIM)

```http
POST /api/domains/{domainID}/senders/{id}/setup
```

---

## Campaigns

### List Campaigns

```http
GET /api/campaigns?status=draft&page=1&limit=20
```

**Query Parameters:**
- `status`: filter by status (draft, scheduled, sending, completed, failed)
- `page`: page number (default: 1)
- `limit`: items per page (default: 20)

**Response:**
```json
{
  "campaigns": [
    {
      "id": 1,
      "name": "Weekly Newsletter",
      "subject": "This Week's Updates",
      "status": "draft",
      "sender_id": 1,
      "list_id": 1,
      "total_sent": 0,
      "total_failed": 0,
      "created_at": "2025-12-29T10:30:00Z"
    }
  ],
  "total": 45,
  "page": 1,
  "limit": 20
}
```

### Create Campaign

```http
POST /api/campaigns
Content-Type: application/json

{
  "name": "Weekly Newsletter",
  "subject": "This Week's Updates",
  "body": "<html><body>Hello {{first_name}}!</body></html>",
  "sender_id": 1,
  "list_id": 1
}
```

### Update Campaign

```http
PUT /api/campaigns/{id}
Content-Type: application/json

{
  "subject": "Updated Subject Line",
  "body": "<html>...</html>"
}
```

### Send Campaign

```http
POST /api/campaigns/{id}/send
```

### Schedule Campaign

```http
POST /api/campaigns/{id}/schedule
Content-Type: application/json

{
  "scheduled_for": "2025-12-30T09:00:00Z"
}
```

### Get Campaign Statistics

```http
GET /api/campaigns/{id}/stats
```

**Response:**
```json
{
  "total_recipients": 10000,
  "sent": 9500,
  "failed": 100,
  "skipped": 400,
  "opens": 3200,
  "unique_opens": 2800,
  "clicks": 450,
  "unique_clicks": 380,
  "bounces": 50,
  "unsubscribes": 15
}
```

---

## Templates

### List Templates

```http
GET /api/templates?category=newsletter&search=welcome
```

### Create Template

```http
POST /api/templates
Content-Type: application/json

{
  "name": "Welcome Email",
  "subject": "Welcome to {{company_name}}!",
  "html_content": "<html>...</html>",
  "text_content": "Plain text version...",
  "category": "welcome"
}
```

### Preview Template

```http
POST /api/templates/{id}/preview
Content-Type: application/json

{
  "first_name": "John",
  "last_name": "Doe",
  "company_name": "Acme Inc"
}
```

**Response:**
```json
{
  "html": "<html>rendered content...</html>",
  "subject": "Welcome to Acme Inc!"
}
```

### Clone Template

```http
POST /api/templates/{id}/clone
```

---

## Contacts & Lists

### List Contact Lists

```http
GET /api/lists
```

### Create List

```http
POST /api/lists
Content-Type: application/json

{
  "name": "Newsletter Subscribers",
  "description": "Main newsletter list"
}
```

### Import Contacts (CSV)

```http
POST /api/lists/{id}/import
Content-Type: multipart/form-data

file: <CSV file>
```

**CSV Format:**
```csv
email,first_name,last_name
john@example.com,John,Doe
jane@example.com,Jane,Smith
```

### Get List Contacts

```http
GET /api/lists/{id}/contacts?page=1&limit=50
```

---

## Tags & Segments

### List Tags

```http
GET /api/tags
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "VIP",
    "color": "#FF5733",
    "subscriber_count": 150
  }
]
```

### Create Tag

```http
POST /api/tags
Content-Type: application/json

{
  "name": "VIP",
  "color": "#FF5733"
}
```

### Add Tags to Subscriber

```http
POST /api/subscribers/{id}/tags
Content-Type: application/json

{
  "tag_ids": [1, 2, 3]
}
```

### Create Segment

```http
POST /api/segments
Content-Type: application/json

{
  "name": "Active VIPs",
  "description": "VIP customers who opened email in last 30 days",
  "conditions": [
    {
      "field": "tag",
      "operator": "equals",
      "value": "VIP"
    },
    {
      "field": "last_open",
      "operator": "within_days",
      "value": "30",
      "combiner": "and"
    }
  ]
}
```

### Get Segment Subscribers

```http
GET /api/segments/{id}/subscribers?page=1&limit=50
```

---

## Email Verification

### Verify Single Email

```http
POST /api/contacts/verify
Content-Type: application/json

{
  "email": "test@example.com"
}
```

**Response:**
```json
{
  "email": "test@example.com",
  "is_valid": true,
  "mx_found": true,
  "smtp_check": "deliverable",
  "is_disposable": false,
  "is_role": false,
  "suggestion": null
}
```

### Verify Batch

```http
POST /api/contacts/verify-batch
Content-Type: application/json

{
  "emails": [
    "test1@example.com",
    "test2@example.com"
  ]
}
```

### Clean List

```http
POST /api/lists/{id}/clean
```

---

## Analytics

### Dashboard Overview

```http
GET /api/analytics/dashboard?days=7
```

**Response:**
```json
{
  "overview": {
    "total_subscribers": 50000,
    "total_campaigns": 120,
    "total_sent": 1500000,
    "total_domains": 5
  },
  "period": {
    "days": 7,
    "sent": 25000,
    "opens": 8500,
    "clicks": 1200,
    "bounces": 150,
    "unsubscribes": 45
  },
  "rates": {
    "open_rate": 34.0,
    "click_rate": 4.8,
    "bounce_rate": 0.6
  },
  "deliverability_score": 94,
  "daily_stats": [...]
}
```

### Campaign Analytics

```http
GET /api/analytics/campaigns/{id}
```

### Deliverability Report

```http
GET /api/analytics/deliverability?days=30
```

---

## Suppressions

### List Suppressions

```http
GET /api/suppressions?reason=hard_bounce&page=1
```

### Add Suppression

```http
POST /api/suppressions
Content-Type: application/json

{
  "email": "bounce@example.com",
  "reason": "hard_bounce",
  "source": "manual"
}
```

### Remove Suppression

```http
DELETE /api/suppressions/{id}
```

### Bulk Check

```http
POST /api/suppressions/check
Content-Type: application/json

{
  "emails": ["test1@example.com", "test2@example.com"]
}
```

---
## Proxy Management

### List Proxies

```http
GET /api/proxies
```

**Response:**
```json
[
  {
    "id": 1,
    "host": "1.2.3.4",
    "port": 1080,
    "protocol": "socks5",
    "username": "user",
    "enabled": true,
    "failure_count": 0
  }
]
```

### Create Proxy

```http
POST /api/proxies
Content-Type: application/json

{
  "host": "1.2.3.4",
  "port": 1080,
  "protocol": "socks5",
  "username": "optional_user",
  "password": "optional_password"
}
```

### Update Proxy

```http
PUT /api/proxies/{id}
Content-Type: application/json

{
  "enabled": false,
  "port": 1081
}
```

### Test Proxy

```http
POST /api/proxies/{id}/test
```

### Delete Proxy

```http
DELETE /api/proxies/{id}
```

---

## System

### Health Check

```http
GET /api/system/health
```

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": "72h15m30s",
  "database": "connected",
  "kumomta": "running"
}
```

### Queue Status

```http
GET /api/queue/stats
```

### DNS Records

```http
GET /api/dns/{domainID}
```

**Response:**
```json
{
  "spf": {
    "name": "example.com",
    "type": "TXT",
    "value": "v=spf1 ip4:192.168.1.100 -all"
  },
  "dkim": {
    "name": "newsletter._domainkey.example.com",
    "type": "TXT",
    "value": "v=DKIM1; k=rsa; p=MIIBIjAN..."
  },
  "dmarc": {
    "name": "_dmarc.example.com",
    "type": "TXT",
    "value": "v=DMARC1; p=quarantine; pct=100"
  }
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "error": "Error message here",
  "code": "ERROR_CODE",
  "details": {}
}
```

### HTTP Status Codes

| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Created |
| 400 | Bad Request - Invalid input |
| 401 | Unauthorized - Invalid/missing token |
| 403 | Forbidden - Insufficient permissions |
| 404 | Not Found |
| 409 | Conflict - Resource already exists |
| 422 | Unprocessable Entity - Validation error |
| 429 | Too Many Requests - Rate limited |
| 500 | Internal Server Error |

---

## Rate Limits

| Endpoint | Limit |
|----------|-------|
| Authentication | 5 req/sec |
| Email Verification | 2 req/sec |
| General API | 100 req/sec |

Rate limit headers are included in responses:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705312800
```

---

## Webhooks (Outgoing)

Configure webhook endpoints in Settings to receive real-time events:

### Event Types

- `campaign.started`
- `campaign.completed`
- `email.sent`
- `email.opened`
- `email.clicked`
- `email.bounced`
- `email.unsubscribed`
- `email.complained`

### Payload Format

```json
{
  "event": "email.opened",
  "timestamp": "2025-12-29T10:30:00Z",
  "data": {
    "campaign_id": 1,
    "recipient_id": 12345,
    "email": "user@example.com"
  }
}
```

### Webhook Signature

Verify authenticity using HMAC-SHA256:

```
X-Webhook-Signature: sha256=abc123...
```

```python
import hmac
import hashlib

def verify_signature(payload, signature, secret):
    expected = hmac.new(
        secret.encode(),
        payload.encode(),
        hashlib.sha256
    ).hexdigest()
    return hmac.compare_digest(f"sha256={expected}", signature)
```

---

## Internal Policy API (KumoMTA Integration)

> **Note**: These endpoints are restricted to localhost only. They are designed for KumoMTA Lua scripts to query dynamic policies.

### Get Sending IPs

```http
GET /api/internal/policy/sending-ips
```

**Response:**
```json
[
  {
    "ip_address": "192.168.1.100",
    "hostname": "mail1.example.com",
    "warmup_stage": 3,
    "daily_send_limit": 5000,
    "today_sent": 1234,
    "reputation_score": 95.5,
    "is_active": true
  }
]
```

### Get Domains

```http
GET /api/internal/policy/domains
```

**Response:**
```json
[
  {
    "name": "example.com",
    "dkim_selector": "default",
    "warmup_enabled": true,
    "rate_limit": "500/h",
    "max_per_hour": 500
  }
]
```

### Get Sender Rate

```http
GET /api/internal/policy/sender-rate?email=sender@example.com
```

**Response:**
```json
{
  "email": "sender@example.com",
  "rate_limit": "100/h",
  "warmup_day": 5,
  "max_per_hour": 100,
  "egress_pool": "example.com__sender",
  "is_active": true
}
```

### Log Delivery Event

```http
POST /api/internal/policy/log-event
Content-Type: application/json

{
  "message_id": "abc123",
  "recipient": "user@example.com",
  "sender": "sender@example.com",
  "domain": "example.com",
  "event_type": "delivered",
  "status_code": "250",
  "diagnostic": "OK",
  "timestamp": 1704499200
}
```

### Check Suppression

```http
GET /api/internal/policy/suppression-check?email=user@example.com
```

**Response:**
```json
{
  "email": "user@example.com",
  "suppressed": true,
  "reason": "hard_bounce"
}
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SAMPMAIL_SECRET` | (required) | App secret key (min 32 chars) |
| `SAMPMAIL_LISTEN_ADDR` | `127.0.0.1:9000` | API listen address |
| `SAMPMAIL_DB_DRIVER` | `sqlite` | Database driver (sqlite, postgres) |
| `SAMPMAIL_PG_HOST` | `localhost` | PostgreSQL host |
| `SAMPMAIL_PG_PORT` | `5432` | PostgreSQL port |
| `SAMPMAIL_PG_USER` | - | PostgreSQL username |
| `SAMPMAIL_PG_PASSWORD` | - | PostgreSQL password |
| `SAMPMAIL_PG_DATABASE` | - | PostgreSQL database name |
| `SAMPMAIL_REDIS_ADDR` | - | Redis address (optional, for scaling) |
| `SAMPMAIL_KUMO_API_URL` | `http://127.0.0.1:8000` | KumoMTA HTTP API URL |
| `REACHER_URL` | `http://reacher:8080` | Email verification service URL |

