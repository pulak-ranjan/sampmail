import { writeFileSync } from 'fs';
const base = 'D:/Mail Sender/sampmail/sampmail/docs/';

// ─────────────────────────────────────────────────────────────────────────────
// ARCHITECTURE.md
// ─────────────────────────────────────────────────────────────────────────────
writeFileSync(base + 'ARCHITECTURE.md', `# SampMail Architecture

This document describes how SampMail is structured internally: how the code is
organized, how data flows through the system, and how each major subsystem works.

---

## Table of Contents

- [Repository Layout](#repository-layout)
- [Process Architecture](#process-architecture)
- [Request Lifecycle](#request-lifecycle)
- [Database Layer](#database-layer)
- [Campaign Pipeline](#campaign-pipeline)
- [Tracking System](#tracking-system)
- [Suppression and Unsubscribe](#suppression-and-unsubscribe)
- [Email Verification](#email-verification)
- [Bounce Processing](#bounce-processing)
- [Automation Engine](#automation-engine)
- [Webhook System](#webhook-system)
- [Multi-Tenancy](#multi-tenancy)
- [SMTP Connection Pool](#smtp-connection-pool)
- [Rate Limiting](#rate-limiting)
- [Background Scheduler](#background-scheduler)
- [Secret Derivation](#secret-derivation)

---

## Repository Layout

\`\`\`
sampmail/
  cmd/
    server/
      main.go          Entry point. Initialises everything and starts the HTTP server.
  internal/
    api/               HTTP handlers. One file per domain (auth.go, campaigns.go, etc.)
    config/            Configuration struct and environment variable parsing.
    core/              Business logic: campaign sending, bounce processing, automation.
    logger/            Structured logger wrapper.
    middleware/        HTTP middleware: auth, rate limiting, CORS, request ID.
    middleware/custom/ Redis-backed and in-memory rate limiters.
    models/            GORM model structs. models.go (v1), models_v2.go (v2/multi-tenant).
    store/             Database access layer. db.go is the primary interface.
  web/
    src/               React frontend source.
    dist/              Compiled frontend (embedded in the binary at build time).
  scripts/
    install.sh         Universal Linux installer.
    build.sh           Multi-platform build script.
  docs/                Documentation files.
  Dockerfile           Multi-stage build for production container.
  docker-compose.yml   Full stack: app, postgres, redis, reacher, kumomta.
\`\`\`

---

## Process Architecture

A single SampMail process handles all responsibilities:

\`\`\`
Process: sampmail
  HTTP server          Handles all API and frontend requests.
  Background scheduler Runs periodic tasks (bounce processing, warmup, health checks).
  Automation engine    Processes automation workflows and delayed actions.
  SMTP pool            Maintains persistent connections to KumoMTA.
\`\`\`

There are no separate worker processes. Concurrency is managed with Go goroutines.
The campaign sending pool uses a configurable number of goroutines (default: 50).

External dependencies:

\`\`\`
KumoMTA     Accepts email over SMTP submission (port 587) and delivers it.
PostgreSQL  Primary data store for production.
Redis       Distributed rate limiting. Falls back to in-memory if unavailable.
Reacher     Email address verification via HTTP API.
\`\`\`

---

## Request Lifecycle

Every incoming HTTP request follows this path:

\`\`\`
Client request
  -> CORS middleware          Checks allowed origins.
  -> Request ID middleware    Attaches a unique ID for log correlation.
  -> Rate limiter             Redis-backed or in-memory per-IP limits.
  -> Auth middleware          Validates session token from the Authorization header.
  -> Organization middleware  Extracts and validates org context (V2 routes only).
  -> Handler                  Executes business logic, queries the database.
  -> Response                 JSON response or redirect.
\`\`\`

The router is \`go-chi/chi\`. All API routes are under \`/api/\`. The frontend SPA is
served from \`/\` as a catch-all that returns \`index.html\`.

---

## Database Layer

The store interface is in \`internal/store/db.go\`. It wraps GORM and exposes named
methods (e.g. \`GetSettings()\`, \`AddSuppression(orgID, email, reason, source)\`).

**Database drivers:**

- SQLite via \`gorm.io/driver/sqlite\` — for development. The file is stored at
  \`SAMPMAIL_DATA_DIR/sampmail.db\`.
- PostgreSQL via \`gorm.io/driver/postgres\` — for production.

**Schema management:**

GORM's \`AutoMigrate\` runs on every startup and creates or alters tables to match the
model struct definitions. This is safe to run repeatedly on an existing database.

A secondary version-based migration system in \`migrations_v2.go\` handles migrations
that require dialect-specific SQL (SQLite vs PostgreSQL). Each migration has a version
number and a \`func(*sql.DB, string) error\` signature where the second argument is the
detected dialect.

**Connection pooling:**

- Maximum open connections: \`SAMPMAIL_DB_MAX_OPEN_CONNS\` (default 100)
- Minimum idle connections: \`SAMPMAIL_DB_MAX_IDLE_CONNS\` (default 10)
- Connection maximum lifetime: 1 hour
- Connection maximum idle time: 10 minutes

**Atomic operations:**

The \`AtomicOps\` struct wraps operations that must be race-free under concurrent load,
specifically campaign open and click tracking counters. These use SQL \`UPDATE ... SET
counter = counter + 1\` rather than read-modify-write cycles.

---

## Campaign Pipeline

**States:** draft > scheduled > sending > completed (or failed)

**Sending flow:**

1. Operator creates a campaign with a sender, subject, HTML body, and recipient list.
2. Campaign is moved to \`sending\` status (manually or by the scheduler at the
   scheduled time).
3. The campaign service creates a worker pool of \`SAMPMAIL_CAMPAIGN_WORKERS\` goroutines.
4. Each goroutine picks the next pending \`CampaignRecipient\` row.
5. For each recipient:
   - Check the suppression list. Skip if suppressed.
   - Personalize the subject and body with the contact's field values.
   - Rewrite all links to click-tracking redirect URLs.
   - Inject a 1x1 tracking pixel in the email body.
   - Generate a signed unsubscribe token.
   - Acquire an SMTP connection from the pool.
   - Send the email via \`DATA\` command.
   - Mark the recipient row as \`sent\` or \`failed\`.
   - Increment campaign-level counters atomically.
6. When all recipients are processed, set campaign status to \`completed\`.

**Base URL derivation for tracking links:**

Priority order when building tracking and unsubscribe URLs:

1. Verified custom tracking domain configured for the organization.
2. \`AppSettings.MainHostname\` stored in the database.
3. \`http://localhost:9000\` as a development fallback.

The \`SAMPMAIL_BASE_URL\` environment variable seeds \`AppSettings.MainHostname\` on first
startup, so tracking links work immediately without manual UI configuration.

**Personalization variables:**

- \`{{first_name}}\`, \`{{last_name}}\`, \`{{email}}\` — contact fields.
- \`{{unsubscribe_link}}\` — the one-click unsubscribe URL.
- Custom fields defined on the contact list.

---

## Tracking System

**Open tracking:**

Each email body includes an \`<img>\` tag pointing to:

\`\`\`
GET /api/track/open/{recipientID}?sig={hmac}
\`\`\`

The handler returns a 1x1 transparent GIF immediately. In the background it:

- Marks the recipient row as opened (atomic, first-open only).
- Increments \`Campaign.TotalOpens\` atomically.
- Increments \`Contact.TotalOpens\`.
- Records a \`TrackingEvent\` with the IP address and User-Agent.

**Click tracking:**

All links in the campaign body are rewritten to:

\`\`\`
GET /api/track/click/{recipientID}?url={base64url}&sig={hmac}
\`\`\`

The handler validates the HMAC signature. An invalid or missing signature returns 403.
If valid, the handler records the click event and issues an HTTP 302 redirect to the
original URL.

**HMAC signatures:**

Both tracking endpoints validate an HMAC-SHA256 signature computed over the recipient
ID and URL using a key derived from \`SAMPMAIL_SECRET\`. This prevents recipients from
manipulating tracking URLs to generate false analytics.

---

## Suppression and Unsubscribe

**Suppression list:**

The \`Suppression\` table stores email addresses that must not receive email. Each row
has an \`organization_id\` scope. Passing \`orgID=0\` means system-wide.

Reasons: \`hard_bounce\`, \`soft_bounce_threshold\`, \`unsubscribe\`, \`complaint\`, \`manual\`.

Addresses are added to the suppression list by:

- The bounce processor (hard bounces and repeated soft bounces).
- The FBL processor (spam complaints from ISP feedback loops).
- The unsubscribe handler (when a recipient clicks unsubscribe).
- The list hygiene manager (risky or invalid addresses found during verification).
- Operators manually via the UI or API.

**Suppression checks occur at two points:**

1. Import time — suppressed addresses are skipped when importing recipients.
2. Send time — each recipient is checked before the email is handed to the SMTP pool.

**Unsubscribe flow:**

Each email includes a signed unsubscribe token in the headers
(\`List-Unsubscribe\`) and in the body. The token links to a specific
\`CampaignRecipient\` record.

\`\`\`
GET  /api/unsubscribe/{token}   Returns the unsubscribe confirmation page.
POST /api/unsubscribe/{token}   Confirms the unsubscribe.
\`\`\`

On confirmation, the recipient's email is added to the suppression list with reason
\`unsubscribe\` scoped to the campaign's organization ID.

---

## Email Verification

SampMail supports three verification strategies, tried in order:

1. **Reacher HTTP API** — POST to \`REACHER_URL/v0/check_email\`. Returns a detailed
   result including SMTP deliverability, catchall status, disposable domain detection,
   and a risk score.

2. **Reacher binary** — If \`REACHER_BIN_PATH\` is set, the binary is executed as a
   subprocess. This avoids port binding conflicts on servers where 25 is restricted.

3. **Local SMTP verification** — Direct MX lookup and SMTP probe from the server. Less
   accurate for major providers that block probes.

**Result classification:**

- \`safe\` — address is deliverable.
- \`risky\` — deliverable but a role address, catchall, or low reputation domain.
- \`invalid\` — address does not exist or domain does not accept mail.
- \`unknown\` — could not determine (e.g. timeout, connection refused).

Addresses classified as \`invalid\` are automatically added to the suppression list
during list cleaning.

---

## Bounce Processing

The \`BounceProcessor\` runs every 10 minutes as a background task.

**Log parsing:**

KumoMTA writes delivery events (bounces, deferrals, complaints) to its log directory.
The bounce processor reads these log files, parses each JSON delivery record, and
extracts the recipient address, bounce code, and reason.

**Classification:**

- 5xx SMTP codes: hard bounce. Address added to suppression list immediately.
- 4xx SMTP codes: soft bounce. Counter incremented. After a threshold, added to
  suppression list.
- Spam complaints: added to suppression list with reason \`complaint\`.

**FBL (Feedback Loop):**

ISPs send abuse reports via email when recipients mark messages as spam. The
\`fbl_processor.go\` parses these ARF-format reports and adds the original recipient
to the suppression list.

**Processed files:**

Log files that have been fully processed are recorded in the \`ProcessedLogFile\` table
to prevent double-processing on restart.

---

## Automation Engine

The automation engine executes visual workflows triggered by contact events.

**Workflow definition:**

A workflow (\`AutomationV2\`) stores its definition as React Flow nodes and edges in
JSON columns. Each node has a type (send_email, wait, condition, tag, webhook) and
a configuration block.

**Trigger types:**

- \`subscribe\` — fires when a contact is added to a list.
- \`tag_added\` — fires when a tag is applied to a contact.
- \`email_opened\` — fires when a contact opens a campaign email.
- Custom triggers via webhook.

**Execution:**

When a trigger fires, an \`AutomationRun\` row is created linking the workflow to the
contact. A goroutine picks up the run, executes the first node, and writes the result.
Nodes that require waiting (delay nodes) store a \`NextActionAt\` timestamp. The
background scheduler picks up runs whose \`NextActionAt\` has passed and resumes them.

**Personalization:**

The \`PersonalizationEngine\` substitutes \`{{variable}}\` placeholders in email subjects
and bodies using the contact's stored fields.

---

## Webhook System

Webhooks deliver event notifications to external HTTP endpoints.

**Events:**

- Campaign completed.
- Bounce rate exceeded the configured alert threshold.
- Daily summary report.

**Delivery:**

When an event fires, SampMail POSTs a JSON payload to the configured \`WebhookURL\`.
The result (status code, response body) is stored in \`WebhookLog\` for inspection.

**V2 webhooks** are scoped per organization. Each organization can configure multiple
webhook endpoints with independent event filtering.

---

## Multi-Tenancy

SampMail supports multiple organizations sharing a single installation.

**Data isolation:**

Every model that belongs to a tenant has an \`organization_id\` column. All queries in
V2 handlers are scoped by this column. The organization middleware extracts the
\`org_id\` from the request context and verifies the authenticated user is a member.

**Roles within an organization:**

| Role | Permissions |
|---|---|
| owner | Full control. Can manage members and billing. |
| admin | Manage all resources within the organization. |
| editor | Create and edit campaigns, lists, templates. Cannot manage members. |
| viewer | Read-only access to analytics and campaign results. |

**Superadmin:**

Users with \`IsSuperAdmin = true\` can manage all organizations and users system-wide.
Superadmin routes are under \`/api/v2/admin/\` and \`/api/admin/\`.

**Per-organization domain configuration:**

Each organization can configure a custom tracking domain and a custom unsubscribe
domain (\`OrganizationDomainConfig\`). Verified custom domains take priority over the
global \`AppSettings.MainHostname\` when building tracking and unsubscribe URLs.

---

## SMTP Connection Pool

The pool is initialized at startup and maintained for the lifetime of the process.

**Configuration:**

- Min connections: \`SAMPMAIL_SMTP_MIN_CONNS\` (default 5) — kept alive permanently.
- Max connections: \`SAMPMAIL_SMTP_MAX_CONNS\` (default 100) — hard cap.
- Connection timeout: \`SAMPMAIL_SMTP_CONN_TIMEOUT\` (default 10s).
- Idle expiry: 5 minutes.

**Health checking:**

A background goroutine periodically sends NOOP commands to idle connections and
removes any that have closed. This prevents stale connections from being handed to
campaign workers.

**Circuit breaker:**

SMTP operations are wrapped in a circuit breaker. If too many consecutive failures
occur (e.g. KumoMTA is down), the breaker opens and immediately rejects new send
attempts until KumoMTA recovers.

---

## Rate Limiting

Rate limiting is applied at multiple layers.

**Per-IP limits on public endpoints:**

The auth endpoints (login, register, 2FA) are limited by IP address using sliding
window counters. When Redis is available, counters are stored in Redis to share state
across multiple instances. When Redis is unavailable, in-memory counters are used as
a secure fallback (fail-secure, not fail-open).

**2FA lockout:**

After a configurable number of failed TOTP attempts, the account is locked for a
period. Lockout state is stored in Redis with an in-memory fallback.

**Domain rate limiting:**

The domain limiter throttles outbound sending per domain to prevent exceeding
per-domain sending thresholds that could harm deliverability.

---

## Background Scheduler

The scheduler runs in a dedicated goroutine started at application startup.

**Schedule:**

| Interval | Tasks |
|---|---|
| Every 5 minutes | IP warmup processing, start scheduled campaigns |
| Every 10 minutes | Bounce log processing, maildir processing |
| Every 30 seconds | Automation delayed action processing |
| Every 1 hour | Blacklist checks, bounce rate alerts |
| Every 24 hours | Daily summary webhook, security audit, configuration backup |

**Graceful shutdown:**

When a shutdown signal (SIGTERM or SIGINT) is received, the scheduler is cancelled via
a context. The main goroutine waits for the scheduler to finish before exiting.

---

## Secret Derivation

SampMail derives all cryptographic keys from a single master secret (\`SAMPMAIL_SECRET\`)
using PBKDF2 with SHA-256 and 100,000 iterations.

**Derived keys:**

| Key | Salt | Purpose |
|---|---|---|
| Encryption key | \`sampmail-encryption-key-v1\` | AES-256 encryption of stored secrets |
| Session key | \`sampmail-session-key-v1\` | HMAC-SHA256 of session tokens before storage |

**Session token storage:**

Session tokens are generated as 32 random bytes (encoded as 64 hex characters). The
token is hashed with HMAC-SHA256 using the derived session key before being stored in
the \`auth_sessions\` table. The raw token is never persisted, so a database breach
does not expose valid session tokens.

**Password hashing:**

Passwords are hashed with bcrypt at the cost factor configured by
\`SAMPMAIL_BCRYPT_COST\` (default 12). The hash is stored in the \`admin_users\` table.
Passwords are never logged or included in API responses.
`);
console.log('ARCHITECTURE.md written');

// ─────────────────────────────────────────────────────────────────────────────
// API.md
// ─────────────────────────────────────────────────────────────────────────────
writeFileSync(base + 'API.md', `# SampMail API Reference

All API communication uses JSON. All endpoints require an \`Authorization\` header
with a valid session token unless stated otherwise.

\`\`\`
Authorization: Bearer <session-token>
\`\`\`

The base URL depends on your deployment. Examples in this document use:

\`\`\`
http://localhost:9000
\`\`\`

---

## Table of Contents

- [Authentication](#authentication)
- [Dashboard](#dashboard)
- [Settings](#settings)
- [Domains and Senders](#domains-and-senders)
- [Campaigns V2](#campaigns-v2)
- [Contact Lists V2](#contact-lists-v2)
- [Contacts and Import](#contacts-and-import)
- [Templates V2](#templates-v2)
- [Suppressions V2](#suppressions-v2)
- [Automations V2](#automations-v2)
- [Webhooks V2](#webhooks-v2)
- [Analytics V2](#analytics-v2)
- [Organizations (Superadmin)](#organizations-superadmin)
- [Users (Superadmin)](#users-superadmin)
- [System and Health](#system-and-health)
- [Tracking Endpoints](#tracking-endpoints)
- [Error Format](#error-format)

---

## Authentication

### Register

\`\`\`
POST /api/auth/register
\`\`\`

Body:

\`\`\`json
{
  "email": "admin@example.com",
  "password": "minimum8chars1"
}
\`\`\`

The first registered user is automatically assigned the superadmin role.

Response:

\`\`\`json
{
  "token": "<session-token>",
  "user": {
    "id": 1,
    "email": "admin@example.com",
    "is_super_admin": true
  }
}
\`\`\`

---

### Login

\`\`\`
POST /api/auth/login
\`\`\`

Body:

\`\`\`json
{
  "email": "admin@example.com",
  "password": "your-password"
}
\`\`\`

If 2FA is enabled, the response contains a \`requires_2fa: true\` field and a temporary
token. Use that token to call \`POST /api/auth/verify-2fa\`.

Response (no 2FA):

\`\`\`json
{
  "token": "<session-token>",
  "user": { "id": 1, "email": "admin@example.com", "is_super_admin": false }
}
\`\`\`

---

### Verify 2FA

\`\`\`
POST /api/auth/verify-2fa
\`\`\`

Rate-limited. Locked after repeated failures.

Body:

\`\`\`json
{
  "token": "<temp-token-from-login>",
  "code": "123456"
}
\`\`\`

Response: same as login without 2FA.

---

### Get current user

\`\`\`
GET /api/auth/me
\`\`\`

Returns the authenticated user's profile.

---

### Logout

\`\`\`
POST /api/auth/logout
\`\`\`

Invalidates the current session token.

---

### List sessions

\`\`\`
GET /api/auth/sessions
\`\`\`

Returns all active sessions for the current user, including IP address and User-Agent.

---

### Setup 2FA

\`\`\`
POST /api/auth/setup-2fa
\`\`\`

Returns a TOTP secret and a \`otpauth://\` URI for QR code generation. The secret is
not yet active until \`enable-2fa\` is called with a valid code.

---

### Enable 2FA

\`\`\`
POST /api/auth/enable-2fa
\`\`\`

Body:

\`\`\`json
{ "code": "123456" }
\`\`\`

Activates 2FA for the current user after verifying the code.

---

### Disable 2FA

\`\`\`
POST /api/auth/disable-2fa
\`\`\`

Body:

\`\`\`json
{ "code": "123456" }
\`\`\`

---

## Dashboard

### Get dashboard statistics

\`\`\`
GET /api/dashboard/stats
\`\`\`

Returns aggregate counts: total contacts, campaigns, sent emails, open rate,
bounce rate, and recent campaign summaries.

---

## Settings

### Get app settings

\`\`\`
GET /api/settings
\`\`\`

Returns the global AppSettings record including the configured hostname, SMTP settings,
AI provider, and webhook configuration.

---

### Update app settings

\`\`\`
POST /api/settings
\`\`\`

Body fields (all optional, sends only what you want to change):

\`\`\`json
{
  "main_hostname": "mail.yourdomain.com",
  "main_server_ip": "203.0.113.42",
  "webhook_url": "https://hooks.example.com/sampmail",
  "webhook_enabled": true,
  "bounce_alert_pct": 5
}
\`\`\`

---

## Domains and Senders

### List domains

\`\`\`
GET /api/domains
\`\`\`

### Create domain

\`\`\`
POST /api/domains
\`\`\`

Body:

\`\`\`json
{
  "name": "example.com",
  "mail_host": "mail.example.com",
  "bounce_host": "bounce.example.com"
}
\`\`\`

### Get / Update / Delete domain

\`\`\`
GET    /api/domains/{id}
PUT    /api/domains/{id}
DELETE /api/domains/{id}
\`\`\`

### List senders for a domain

\`\`\`
GET /api/domains/{domainID}/senders
\`\`\`

### Create sender

\`\`\`
POST /api/domains/{domainID}/senders
\`\`\`

Body:

\`\`\`json
{
  "local_part": "newsletter",
  "ip": "203.0.113.42"
}
\`\`\`

The resulting sender email address is \`local_part@domain.name\`.

### Get / Update / Delete sender

\`\`\`
GET    /api/senders/{id}
PUT    /api/senders/{id}
DELETE /api/senders/{id}
\`\`\`

### Run sender setup (DKIM generation)

\`\`\`
POST /api/domains/{domainID}/senders/{id}/setup
\`\`\`

Generates a DKIM key pair, writes the private key to the KumoMTA DKIM directory, and
returns the public key DNS record to publish.

---

## Campaigns V2

All campaign endpoints are scoped to an organization. Include the organization ID in
requests where required.

### List campaigns

\`\`\`
GET /api/v2/campaigns
\`\`\`

Query parameters: \`page\`, \`per_page\`, \`status\` (draft|scheduled|sending|completed|failed).

### Campaign statistics summary

\`\`\`
GET /api/v2/campaigns/stats
\`\`\`

Returns aggregated totals across all campaigns: total sent, total opens, total clicks,
average open rate, average click rate.

### Create campaign

\`\`\`
POST /api/v2/campaigns
\`\`\`

Body:

\`\`\`json
{
  "name": "March Newsletter",
  "subject": "What is new in March",
  "body": "<h1>Hello {{first_name}}</h1>...",
  "sender_id": 3,
  "organization_id": 1,
  "scheduled_at": "2025-03-01T09:00:00Z"
}
\`\`\`

Leave \`scheduled_at\` empty to keep the campaign as a draft.

### Get / Update / Delete campaign

\`\`\`
GET    /api/v2/campaigns/{id}
PUT    /api/v2/campaigns/{id}
DELETE /api/v2/campaigns/{id}
\`\`\`

Deletion is only permitted for campaigns in \`draft\` status.

### Preview campaign

\`\`\`
POST /api/v2/campaigns/{id}/preview
\`\`\`

Body:

\`\`\`json
{ "email": "test@example.com" }
\`\`\`

Sends a rendered preview to the specified address using the first contact in the
recipient list for variable substitution.

---

## Contact Lists V2

### List all subscriber lists

\`\`\`
GET /api/v2/lists
\`\`\`

### Create list

\`\`\`
POST /api/v2/lists
\`\`\`

Body:

\`\`\`json
{
  "name": "Newsletter Subscribers",
  "description": "Main newsletter list",
  "organization_id": 1
}
\`\`\`

### Get / Update / Delete list

\`\`\`
GET    /api/v2/lists/{id}
PUT    /api/v2/lists/{id}
DELETE /api/v2/lists/{id}
\`\`\`

### List subscribers in a list

\`\`\`
GET /api/v2/lists/{id}/subscribers
\`\`\`

Query parameters: \`page\`, \`per_page\`, \`status\` (active|unsubscribed|bounced).

### Add subscriber to a list

\`\`\`
POST /api/v2/lists/{id}/subscribers
\`\`\`

Body:

\`\`\`json
{
  "email": "contact@example.com",
  "first_name": "Jane",
  "last_name": "Smith"
}
\`\`\`

### Remove subscriber from list

\`\`\`
DELETE /api/v2/lists/{id}/subscribers/{contactId}
\`\`\`

### Import subscribers

\`\`\`
POST /api/v2/lists/{id}/import
Content-Type: multipart/form-data
\`\`\`

Upload a CSV file. Required columns: \`email\`. Optional: \`first_name\`, \`last_name\`,
\`phone\`, \`company\`, and any custom field names.

Maximum file size: 50 MB.

### Export subscribers

\`\`\`
GET /api/v2/lists/{id}/export
\`\`\`

Returns a CSV download of all active subscribers in the list.

### Get list statistics

\`\`\`
GET /api/v2/lists/{id}/stats
\`\`\`

Returns subscriber counts broken down by status.

---

## Contacts and Import

### Verify a single email address

\`\`\`
POST /api/contacts/verify
\`\`\`

Rate-limited.

Body:

\`\`\`json
{ "email": "test@example.com" }
\`\`\`

Response:

\`\`\`json
{
  "email": "test@example.com",
  "is_reachable": "safe",
  "risk_score": 12,
  "is_disposable": false,
  "is_role_account": false,
  "is_catch_all": false
}
\`\`\`

### Batch verify emails

\`\`\`
POST /api/contacts/verify-batch
\`\`\`

Body:

\`\`\`json
{ "emails": ["a@example.com", "b@example.com"] }
\`\`\`

### Clean a list (verify all contacts)

\`\`\`
POST /api/lists/{id}/clean
\`\`\`

Triggers verification of all unverified contacts in the list. Runs asynchronously.
Invalid addresses are added to the suppression list automatically.

### CSV import (synchronous, up to 1 MB)

\`\`\`
POST /api/import/csv
Content-Type: multipart/form-data
\`\`\`

### CSV import (asynchronous, up to 50 MB)

\`\`\`
POST /api/import/csv/async
Content-Type: multipart/form-data
\`\`\`

Returns a job ID.

### Check import status

\`\`\`
GET /api/import/status/{jobId}
\`\`\`

Returns progress: total rows, imported, skipped (invalid or suppressed), errors.

---

## Templates V2

### List templates

\`\`\`
GET /api/v2/templates
\`\`\`

### Create template

\`\`\`
POST /api/v2/templates
\`\`\`

Body:

\`\`\`json
{
  "name": "Welcome Email",
  "subject": "Welcome to {{company_name}}",
  "html_content": "<html>...</html>",
  "organization_id": 1
}
\`\`\`

### Get / Update / Delete template

\`\`\`
GET    /api/v2/templates/{id}
PUT    /api/v2/templates/{id}
DELETE /api/v2/templates/{id}
\`\`\`

### Built-in template library

\`\`\`
GET /api/v2/templates/library
GET /api/v2/templates/library/{templateId}
\`\`\`

Returns the set of built-in starter templates that ship with SampMail.

### Preview template

\`\`\`
POST /api/v2/templates/{id}/preview
\`\`\`

Body:

\`\`\`json
{
  "variables": { "first_name": "Jane", "company_name": "Acme" }
}
\`\`\`

---

## Suppressions V2

### List suppressions

\`\`\`
GET /api/v2/suppressions
\`\`\`

Query parameters: \`page\`, \`per_page\`, \`reason\`.

### Add suppression

\`\`\`
POST /api/v2/suppressions
\`\`\`

Body:

\`\`\`json
{
  "email": "user@example.com",
  "reason": "manual",
  "organization_id": 1
}
\`\`\`

### Remove suppression

\`\`\`
DELETE /api/v2/suppressions/{id}
\`\`\`

---

## Automations V2

### List automations

\`\`\`
GET /api/v2/automations
\`\`\`

### Create automation

\`\`\`
POST /api/v2/automations
\`\`\`

Body:

\`\`\`json
{
  "name": "Welcome Series",
  "trigger_type": "subscribe",
  "trigger_config": { "list_id": 5 },
  "organization_id": 1,
  "nodes": [...],
  "edges": [...]
}
\`\`\`

The \`nodes\` and \`edges\` fields use the React Flow JSON format as produced by the
visual automation builder in the UI.

### Get / Update / Delete automation

\`\`\`
GET    /api/v2/automations/{id}
PUT    /api/v2/automations/{id}
DELETE /api/v2/automations/{id}
\`\`\`

### Activate / Pause automation

\`\`\`
POST /api/v2/automations/{id}/activate
POST /api/v2/automations/{id}/pause
\`\`\`

### Get automation statistics

\`\`\`
GET /api/v2/automations/{id}/stats
\`\`\`

Returns counts: contacts entered, currently active, completed, failed.

### Get automation runs

\`\`\`
GET /api/v2/automations/{id}/runs
\`\`\`

Returns individual contact execution records with status and current node.

---

## Webhooks V2

### List webhooks

\`\`\`
GET /api/v2/webhooks
\`\`\`

### Create webhook

\`\`\`
POST /api/v2/webhooks
\`\`\`

Body:

\`\`\`json
{
  "url": "https://hooks.example.com/endpoint",
  "is_active": true,
  "organization_id": 1
}
\`\`\`

### Update / Delete webhook

\`\`\`
PUT    /api/v2/webhooks/{id}
DELETE /api/v2/webhooks/{id}
\`\`\`

### Test webhook URL

\`\`\`
POST /api/v2/webhooks/test
\`\`\`

Body:

\`\`\`json
{ "url": "https://hooks.example.com/endpoint" }
\`\`\`

Sends a test payload and returns the HTTP response code and body.

---

## Analytics V2

### Dashboard analytics

\`\`\`
GET /api/v2/analytics/dashboard
\`\`\`

Returns time-series data for sends, opens, clicks, bounces, and complaints over the
last 30 days, scoped to the organization.

### Deliverability metrics

\`\`\`
GET /api/v2/analytics/deliverability
\`\`\`

Returns per-domain deliverability statistics: sent count, open rate, bounce rate,
complaint rate.

---

## Organizations (Superadmin)

Requires \`is_super_admin: true\`.

### List all organizations

\`\`\`
GET /api/v2/admin/organizations
\`\`\`

### Create organization

\`\`\`
POST /api/v2/admin/organizations
\`\`\`

Body:

\`\`\`json
{
  "name": "Acme Corp",
  "slug": "acme",
  "plan": "pro"
}
\`\`\`

### Delete organization

\`\`\`
DELETE /api/v2/admin/organizations/{id}
\`\`\`

### Get organization members

\`\`\`
GET /api/v2/admin/organizations/{id}/members
\`\`\`

---

## Users (Superadmin)

### List users

\`\`\`
GET /api/admin/users
\`\`\`

### Create user

\`\`\`
POST /api/admin/users
\`\`\`

Body:

\`\`\`json
{
  "email": "user@example.com",
  "password": "secure-password",
  "is_super_admin": false
}
\`\`\`

### Update / Delete user

\`\`\`
PUT    /api/admin/users/{id}
DELETE /api/admin/users/{id}
\`\`\`

### Assign user to organization

\`\`\`
POST /api/admin/users/{id}/organizations
\`\`\`

Body:

\`\`\`json
{
  "organization_id": 2,
  "role": "admin"
}
\`\`\`

Valid roles: \`owner\`, \`admin\`, \`editor\`, \`viewer\`.

### Remove user from organization

\`\`\`
DELETE /api/admin/users/{id}/organizations/{org_id}
\`\`\`

---

## System and Health

### Health endpoints (no authentication required)

\`\`\`
GET /health         Full health status of all subsystems.
GET /health/live    Kubernetes liveness probe. Returns 200 if the process is running.
GET /health/ready   Kubernetes readiness probe. Returns 200 when the database is connected.
\`\`\`

### Detailed system health

\`\`\`
GET /api/system/health
\`\`\`

Returns status of: database, SMTP pool, Redis, Reacher, KumoMTA.

### Version information

\`\`\`
GET /api/version
\`\`\`

Response:

\`\`\`json
{
  "version": "0.1.12",
  "build_time": "2025-01-15T10:30:00Z",
  "git_commit": "a1b2c3d"
}
\`\`\`

### Prometheus metrics

\`\`\`
GET /metrics
\`\`\`

Standard Prometheus text format. Scraped by Prometheus or compatible systems.

---

## Tracking Endpoints

These endpoints are public (no authentication) and rate-limited.

### Track email open

\`\`\`
GET /api/track/open/{recipientID}?sig={hmac}
\`\`\`

Returns a 1x1 transparent GIF. The HMAC signature is validated. An invalid or missing
signature returns 403 without recording the event.

### Track link click

\`\`\`
GET /api/track/click/{recipientID}?url={base64-encoded-url}&sig={hmac}
\`\`\`

Validates the HMAC signature, records the click event, then issues a 302 redirect to
the original URL. An invalid signature returns 403.

### Unsubscribe page

\`\`\`
GET  /api/unsubscribe/{token}   Returns the unsubscribe confirmation page.
POST /api/unsubscribe/{token}   Confirms the unsubscribe and adds to suppression list.
\`\`\`

---

## Error Format

All errors return a JSON body with an \`error\` field:

\`\`\`json
{
  "error": "invalid credentials"
}
\`\`\`

**HTTP status codes used:**

| Code | Meaning |
|---|---|
| 200 | Success |
| 201 | Created |
| 204 | Success, no content (used for DELETE) |
| 400 | Bad request — invalid input |
| 401 | Unauthorized — missing or invalid session token |
| 403 | Forbidden — authenticated but not permitted |
| 404 | Not found |
| 409 | Conflict — e.g. duplicate email |
| 422 | Unprocessable entity — validation failed |
| 429 | Too many requests — rate limited |
| 500 | Internal server error |
`);
console.log('API.md written');

// ─────────────────────────────────────────────────────────────────────────────
// SECURITY.md
// ─────────────────────────────────────────────────────────────────────────────
writeFileSync(base + 'SECURITY.md', `# SampMail Security Reference

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

All cryptographic keys are derived from a single master secret (\`SAMPMAIL_SECRET\`)
using PBKDF2-SHA256 with 100,000 iterations.

**Key derivation:**

| Derived key | Salt used | Purpose |
|---|---|---|
| Encryption key (32 bytes) | sampmail-encryption-key-v1 | AES-256 for encrypting stored secrets |
| Session key (32 bytes) | sampmail-session-key-v1 | HMAC-SHA256 for session token verification |

**Requirements for SAMPMAIL_SECRET:**

- Minimum 32 characters.
- Must be cryptographically random. Generate with: \`openssl rand -base64 32\`
- Never reuse across installations.
- Never change after first run. Changing it invalidates all active sessions and any
  data that was encrypted with the derived encryption key.
- Stored in \`/etc/sampmail.env\` with permissions 600, owned by root.

The install script generates this value automatically.

---

## Authentication

SampMail uses session tokens, not JWTs.

**Login flow:**

1. User submits email and password.
2. The bcrypt hash stored in the database is compared to the submitted password.
3. If correct and 2FA is not enabled, a 32-byte random session token is generated.
4. The token is HMAC-SHA256 hashed using the derived session key.
5. The hash is stored in the \`auth_sessions\` table with an expiry timestamp.
6. The raw token is returned to the client and used in subsequent \`Authorization\` headers.

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

1. \`POST /api/auth/setup-2fa\` — returns a TOTP secret and an \`otpauth://\` URI.
2. User scans the QR code in an authenticator app.
3. \`POST /api/auth/enable-2fa\` — user submits a code to confirm enrollment.

**Login with 2FA enabled:**

1. \`POST /api/auth/login\` returns \`requires_2fa: true\` and a short-lived temporary token.
2. \`POST /api/auth/verify-2fa\` validates the TOTP code. On success, a full session token
   is issued.

**Lockout:**

After repeated failed TOTP attempts, the account is locked for a cooldown period.
Lockout state is tracked in Redis. When Redis is unavailable, an in-memory fallback
provides the same protection (fail-secure, not fail-open). The in-memory state resets
on process restart, which is acceptable since the lockout window is short.

**Rate limiting on 2FA setup endpoints:**

The \`/api/auth/setup-2fa\`, \`/api/auth/enable-2fa\`, and \`/api/auth/disable-2fa\`
endpoints are rate-limited to prevent abuse.

---

## Session Handling

Sessions are stored in the \`auth_sessions\` table.

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

- Default cost factor: 12 (configurable via \`SAMPMAIL_BCRYPT_COST\`).
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

The \`POST /api/contacts/verify\` endpoint is individually rate-limited to prevent use
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
- Request body sizes are limited to \`SAMPMAIL_MAX_REQUEST_BODY\` (default 10 MB), with
  an exception for asynchronous CSV imports (50 MB).
- Pagination parameters are clamped to prevent excessive database queries.

---

## SQL Injection Prevention

All database queries use GORM with parameterized queries. Raw SQL is avoided.

In the one case where a column name is programmatically chosen (bounce counter
increment), an explicit whitelist is used:

\`\`\`go
var column string
if bounceType == "soft" {
    column = "total_soft_bounced"
} else {
    column = "total_bounced"
}
db.Model(&Campaign{}).Where("id = ?", campaignID).UpdateColumn(column, gorm.Expr(column+" + 1"))
\`\`\`

User-supplied strings are never interpolated directly into SQL.

---

## XSS Prevention

The email template builder renders HTML content using \`dangerouslySetInnerHTML\`.
All HTML is sanitized with DOMPurify before rendering:

\`\`\`jsx
dangerouslySetInnerHTML={{
  __html: DOMPurify.sanitize(content.html || '', { USE_PROFILES: { html: true } })
}}
\`\`\`

API responses are JSON. The frontend does not render raw API response strings as HTML
outside of controlled, sanitized contexts.

---

## Open Redirect Prevention

The click tracking endpoint (\`/api/track/click/{id}\`) redirects to user-supplied URLs.
Two protections are applied:

1. **HMAC signature validation** — the URL and recipient ID are signed at email
   generation time. Requests with invalid or missing signatures receive a 403 response
   and are not redirected.

2. **URL scheme validation** — only \`http://\` and \`https://\` schemes are allowed as
   redirect targets. \`javascript:\`, \`data:\`, and other schemes are rejected with 403.

---

## HMAC-Signed Tracking Links

Every tracking URL (open pixel and click redirect) includes an HMAC-SHA256 signature
computed over the recipient ID and, for click links, the target URL. The key is derived
from \`SAMPMAIL_SECRET\`.

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

\`\`\`go
const wsMaxConnections = 1000
\`\`\`

Requests that exceed this limit receive a 503 response. This prevents resource
exhaustion from clients that open many WebSocket connections.

---

## TLS and HTTPS

SampMail supports TLS in two configurations:

**Option 1: Nginx termination (recommended)**

Nginx handles TLS and proxies plaintext HTTP to SampMail on port 9000. Set
\`SAMPMAIL_TRUST_PROXY=true\` and \`SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32\` so that
SampMail correctly identifies client IP addresses from the \`X-Forwarded-For\` header.

**Option 2: Direct TLS**

Set \`SAMPMAIL_TLS_ENABLED=true\`, \`SAMPMAIL_TLS_CERT\`, and \`SAMPMAIL_TLS_KEY\`.
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

The process runs as the \`sampmail\` system user with no login shell and no home
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

- [ ] \`SAMPMAIL_SECRET\` is at least 32 characters and was generated with \`openssl rand -base64 32\`
- [ ] \`/etc/sampmail.env\` has permissions 600 and is owned by root
- [ ] \`SAMPMAIL_ENV=production\` is set
- [ ] PostgreSQL is used instead of SQLite
- [ ] Nginx is configured as a reverse proxy with a valid TLS certificate
- [ ] \`SAMPMAIL_BASE_URL\` is set to the HTTPS domain
- [ ] \`SAMPMAIL_TRUST_PROXY=true\` and \`SAMPMAIL_TRUSTED_PROXY_CIDR\` are set correctly
- [ ] Port 25 is not exposed publicly
- [ ] Ports 5432, 6379, 8080, and 8000 are firewalled from external access
- [ ] Redis is configured with a password if accessible from a network
- [ ] \`SAMPMAIL_BCRYPT_COST\` is at least 12
- [ ] 2FA is enabled for all admin accounts
- [ ] DKIM, SPF, and DMARC DNS records are published for all sending domains

---

## Reporting Vulnerabilities

To report a security vulnerability, send a message to the maintainers directly rather
than opening a public GitHub issue. Include a description of the vulnerability, steps
to reproduce, and an assessment of the potential impact.

Do not publish vulnerability details publicly until a fix has been released and
sufficient time has passed for users to upgrade.
`);
console.log('SECURITY.md written');

// ─────────────────────────────────────────────────────────────────────────────
// CAMPAIGNS.md
// ─────────────────────────────────────────────────────────────────────────────
writeFileSync(base + 'CAMPAIGNS.md', `# SampMail Campaign Guide

This document explains how campaigns work from creation through delivery, how tracking
operates, and how to interpret campaign results.

---

## Table of Contents

- [Campaign Lifecycle](#campaign-lifecycle)
- [Creating a Campaign](#creating-a-campaign)
- [Recipient Lists](#recipient-lists)
- [Email Personalization](#email-personalization)
- [Scheduling](#scheduling)
- [Sending](#sending)
- [Tracking Opens and Clicks](#tracking-opens-and-clicks)
- [Unsubscribes](#unsubscribes)
- [Bounce Handling](#bounce-handling)
- [Analytics](#analytics)
- [Best Practices](#best-practices)

---

## Campaign Lifecycle

A campaign moves through the following states:

| State | Meaning |
|---|---|
| draft | Created and editable. Not yet scheduled or sent. |
| scheduled | Scheduled for a future date and time. The sending starts automatically. |
| sending | Actively being sent. Workers are processing recipients. |
| completed | All recipients have been processed (sent or failed). |
| failed | The campaign encountered a fatal error before completing. |

Only campaigns in \`draft\` status can be deleted.

---

## Creating a Campaign

A campaign requires:

- **Name** — internal label, not visible to recipients.
- **Subject** — the email subject line. Supports personalization variables.
- **Body** — the HTML email content. Supports personalization variables.
- **Sender** — a configured sender identity (email address and IP).
- **Recipient source** — a contact list or a CSV upload.

Create a campaign via the API:

\`\`\`
POST /api/v2/campaigns
\`\`\`

\`\`\`json
{
  "name": "March Newsletter",
  "subject": "What is new this month, {{first_name}}",
  "body": "<h1>Hello {{first_name}},</h1><p>Here is what is new...</p>",
  "sender_id": 3,
  "organization_id": 1
}
\`\`\`

A campaign left without a \`scheduled_at\` value remains in draft status until
manually triggered.

---

## Recipient Lists

**Option 1: Contact list**

Create and manage contact lists in the Lists section. Assign the list to a campaign.
All active, non-suppressed contacts in the list become recipients.

**Option 2: CSV import**

Upload a CSV file directly to a list. Required column: \`email\`. Optional columns:
\`first_name\`, \`last_name\`, \`phone\`, \`company\`, and any custom field names defined
on the list.

Suppressed addresses are automatically excluded at import time. Addresses that were
added to the suppression list after the import are excluded again at send time.

**List cleaning:**

Before sending to a large list, use the Clean List feature to verify all addresses.
Verification uses Reacher to check deliverability. Invalid addresses are suppressed
automatically. This step significantly reduces bounce rates.

---

## Email Personalization

Variables in the subject and body are replaced with contact-specific values at send time.

### Built-in variables

| Variable | Replaced with |
|---|---|
| {{first_name}} | Contact first name |
| {{last_name}} | Contact last name |
| {{email}} | Contact email address |
| {{unsubscribe_link}} | One-click unsubscribe URL |

### Custom variables

Custom fields defined on the contact list are available as \`{{field_name}}\`.

### Unsubscribe link

Every campaign email must include an unsubscribe mechanism. The \`{{unsubscribe_link}}\`
variable inserts a signed URL that takes the recipient to a confirmation page. Confirming
the unsubscribe adds the address to the suppression list.

An RFC 8058 one-click unsubscribe header is also added to every message automatically:

\`\`\`
List-Unsubscribe: <https://mail.yourdomain.com/api/unsubscribe/{token}>
List-Unsubscribe-Post: List-Unsubscribe=One-Click
\`\`\`

This allows mail clients that support one-click unsubscribe to process it without
requiring the recipient to visit the unsubscribe page.

---

## Scheduling

Set \`scheduled_at\` to an ISO 8601 timestamp to schedule a campaign:

\`\`\`json
{
  "scheduled_at": "2025-03-15T09:00:00Z"
}
\`\`\`

The background scheduler checks for scheduled campaigns every 5 minutes and starts
sending when the scheduled time has passed.

---

## Sending

When a campaign starts sending:

1. The campaign status is set to \`sending\`.
2. A pool of \`SAMPMAIL_CAMPAIGN_WORKERS\` goroutines (default: 50) is created.
3. Each worker claims the next pending recipient row.
4. For each recipient, the worker:
   - Checks the suppression list. Skips if suppressed.
   - Personalizes the subject and body.
   - Rewrites links to click-tracking URLs.
   - Injects the open tracking pixel.
   - Sends the email via the SMTP connection pool to KumoMTA.
   - Marks the recipient as \`sent\` or \`failed\`.
   - Updates the campaign's aggregate counters atomically.
5. When all recipients are processed, the campaign status is set to \`completed\`.

**Worker count and throughput:**

The default of 50 workers is appropriate for most deployments. Higher values increase
throughput but also increase the load on both SampMail and KumoMTA. The SMTP pool
maximum (\`SAMPMAIL_SMTP_MAX_CONNS\`) must be at least as large as the worker count.

**Resuming interrupted campaigns:**

If the process restarts while a campaign is sending, the scheduler detects campaigns
stuck in \`sending\` status on startup and resumes them automatically.

---

## Tracking Opens and Clicks

**Open tracking:**

Each email body includes a 1x1 transparent GIF loaded from:

\`\`\`
https://mail.yourdomain.com/api/track/open/{recipientID}?sig={hmac}
\`\`\`

When a recipient's email client loads this image, the server records the open event.
The HMAC signature is validated. Requests with invalid or missing signatures return 403
without recording any event.

Open tracking records:

- Whether the recipient opened the email (first open only for unique count).
- The total number of opens per recipient.
- The IP address and User-Agent of the device that loaded the image.
- The timestamp.

Note: Recipients who have image loading disabled in their mail client will not trigger
open tracking events even if they read the email.

**Click tracking:**

All links in the campaign body are automatically rewritten to:

\`\`\`
https://mail.yourdomain.com/api/track/click/{recipientID}?url={encoded-url}&sig={hmac}
\`\`\`

When a recipient clicks a link, the server:

1. Validates the HMAC signature. Returns 403 if invalid.
2. Records the click event.
3. Issues a 302 redirect to the original URL.

The total click count and unique click count are tracked separately.

**Accuracy:**

Tracking accuracy depends on the recipient's email client and network. Corporate
email security scanners may load tracking pixels and follow links automatically,
which can inflate open and click counts. This is a limitation of email tracking
technology generally, not specific to SampMail.

---

## Unsubscribes

Every campaign email includes a signed unsubscribe token. The recipient can unsubscribe
via:

1. Clicking the \`{{unsubscribe_link}}\` in the email body.
2. Using the one-click unsubscribe mechanism in their mail client (RFC 8058).

On confirmation, the recipient's email address is added to the suppression list. Future
campaigns will not send to this address within the same organization.

Unsubscribes are visible in the Suppressions section of the UI with reason
\`unsubscribe\`.

---

## Bounce Handling

Bounces are processed by reading KumoMTA delivery logs every 10 minutes.

**Hard bounces (5xx SMTP codes):**

The recipient address does not exist or the domain does not accept mail. The address
is immediately added to the suppression list with reason \`hard_bounce\`. The campaign's
\`total_bounced\` counter is incremented.

**Soft bounces (4xx SMTP codes):**

Temporary failure: mailbox full, server temporarily unavailable. The address is not
immediately suppressed. After repeated soft bounces within a rolling window, the address
is added to the suppression list with reason \`soft_bounce_threshold\`.

**Spam complaints (FBL):**

When a recipient marks the email as spam and their ISP reports it via a feedback loop,
the address is added to the suppression list with reason \`complaint\`.

---

## Analytics

Campaign analytics are available at:

\`\`\`
GET /api/v2/analytics/dashboard
GET /api/v2/analytics/deliverability
\`\`\`

**Metrics tracked per campaign:**

| Metric | Description |
|---|---|
| Total sent | Number of emails handed to KumoMTA |
| Total opens | Total open events across all recipients |
| Unique opens | Recipients who opened at least once |
| Total clicks | Total click events across all links |
| Unique clicks | Recipients who clicked at least once |
| Open rate | Unique opens / total sent |
| Click rate | Unique clicks / total sent |
| Bounce rate | Bounced / total sent |
| Complaint rate | Complaints / total sent |

**Industry benchmarks for context:**

- Open rate above 20% is considered good for cold or marketing email.
- Bounce rate above 5% is a warning sign that list quality needs attention.
- Complaint rate above 0.1% (1 in 1000) can affect deliverability at major ISPs.

---

## Best Practices

**Before sending:**

- Clean the contact list with email verification to remove invalid addresses.
- Verify that DKIM, SPF, and DMARC DNS records are published for the sending domain.
- Send a test email to a real inbox and check how it renders.
- Check the suppression list to confirm no recently unsubscribed addresses remain in
  the recipient list.

**Subject lines:**

- Keep subject lines under 50 characters for best display across clients.
- Avoid words associated with spam filters: free, guaranteed, act now, urgent.
- Personalization in the subject (e.g. \`{{first_name}}\`) can improve open rates but
  must be used carefully — missing values produce awkward results.

**Sending volume:**

- New IP addresses and domains should use the warmup feature before sending to large
  lists. Sending large volumes from a cold IP triggers spam filters.
- Monitor bounce and complaint rates after each campaign. If either rises above
  acceptable thresholds, pause sending and investigate list quality.

**Timing:**

Use the scheduling feature to send at times appropriate for the recipient's timezone.
Campaigns sent at 3 AM local time typically have lower open rates.

**Suppression list hygiene:**

Do not remove addresses from the suppression list unless you have confirmed the issue
is resolved (e.g. a hard bounce address that has since been corrected by the contact).
Ignoring unsubscribes or re-adding bounced addresses is a violation of anti-spam
regulations in most jurisdictions.
`);
console.log('CAMPAIGNS.md written');

// ─────────────────────────────────────────────────────────────────────────────
// CONFIGURATION.md (exhaustive variable reference)
// ─────────────────────────────────────────────────────────────────────────────
writeFileSync(base + 'CONFIGURATION.md', `# SampMail Configuration Reference

SampMail is configured entirely through environment variables. There is no configuration
file format other than the \`.env\`-style key-value files read by the systemd service
unit and Docker Compose.

For the systemd installation, variables are loaded from \`/etc/sampmail.env\`.
For Docker Compose, they are loaded from the \`.env\` file in the Compose project
directory.

---

## How variables are resolved

When the process starts, \`config.Get()\` is called exactly once and the result is
cached for the lifetime of the process. Variables are read with the following priority:

1. Explicit environment variable set in the shell or EnvironmentFile.
2. Default value defined in \`internal/config/config.go\`.

There is no live reload. To apply a configuration change, restart the process.

---

## SAMPMAIL_BASE_URL — how it works

This variable is the most important one to get right for a working installation.

**What it controls:**

Every tracking link, click redirect URL, and unsubscribe URL embedded in campaign
emails is built from this value. If it is wrong, clicks and opens will not be tracked,
and the unsubscribe page will not be reachable.

**Auto-detection:**

If \`SAMPMAIL_BASE_URL\` is not set, the server derives a fallback from
\`SAMPMAIL_LISTEN_ADDR\`:

- \`0.0.0.0:9000\` becomes \`http://localhost:9000\`
- \`[::]:9000\` becomes \`http://localhost:9000\`
- Any other address is used as-is with the \`http://\` scheme.

If \`SAMPMAIL_TLS_ENABLED=true\`, the scheme becomes \`https://\`.

**Auto-seeding into the database:**

On first startup, the hostname extracted from \`SAMPMAIL_BASE_URL\` is written to the
\`app_settings\` table as \`main_hostname\`. Subsequent restarts only update the stored
value if \`main_hostname\` is currently empty. This means:

- A fresh installation works immediately with no UI configuration.
- After the first startup, operators can update \`main_hostname\` in the UI without
  it being overwritten on the next restart.
- To force an update from the environment variable, clear \`main_hostname\` in the UI
  settings and restart.

---

## SAMPMAIL_SECRET — key derivation

The master secret is used as input to PBKDF2-SHA256 with 100,000 iterations to derive
two 32-byte keys:

| Key | Salt |
|---|---|
| Encryption key | sampmail-encryption-key-v1 |
| Session key | sampmail-session-key-v1 |

These keys are used for AES-256 encryption of stored credentials and for HMAC-SHA256
signing of session tokens and tracking links respectively.

Changing this value after first use is a destructive operation. All active sessions
become invalid, and any data encrypted with the derived encryption key becomes
unreadable.

---

## Full variable reference

### Application identity

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_SECRET | Required | Master secret. Minimum 32 characters. |
| SAMPMAIL_BASE_URL | Derived from SAMPMAIL_LISTEN_ADDR | Public-facing base URL for tracking and unsubscribe links. Example: https://mail.yourdomain.com |
| SAMPMAIL_ENV | development | Set to production for JSON logging and production behavior. |
| SAMPMAIL_VERSION | dev | Version string. Set at build time. |

### HTTP server

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LISTEN_ADDR | 127.0.0.1:9000 | Host and port the HTTP server listens on. Use 0.0.0.0:9000 to accept connections on all interfaces. |
| SAMPMAIL_REQUEST_TIMEOUT | 30s | Maximum time allowed to process a single request. |
| SAMPMAIL_MAX_REQUEST_BODY | 10485760 | Maximum request body size in bytes. The async import endpoint uses a separate 50 MB limit. |
| SAMPMAIL_SHUTDOWN_TIMEOUT | 30s | Grace period for in-flight requests when a shutdown signal is received. |

### TLS (optional — use Nginx instead)

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_TLS_ENABLED | false | Enable TLS on the application HTTP server directly. |
| SAMPMAIL_TLS_CERT | | Path to the PEM-format TLS certificate file. Required when TLS is enabled. |
| SAMPMAIL_TLS_KEY | | Path to the PEM-format TLS private key file. Required when TLS is enabled. |

### Proxy

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_TRUST_PROXY | false | Trust X-Forwarded-For and X-Forwarded-Proto headers from the upstream proxy. Must be set to true when running behind Nginx or another reverse proxy. |
| SAMPMAIL_TRUSTED_PROXY_CIDR | | Comma-separated list of CIDR ranges from which proxy headers are trusted. Example: 127.0.0.1/32,10.0.0.0/8 |

### Database

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DB_DRIVER | sqlite | Database engine. Use postgres for production. |
| DATABASE_URL | | Full PostgreSQL connection string. Parsed automatically when set. Takes precedence over individual SAMPMAIL_PG_* variables. Example: postgres://user:pass@host:5432/db?sslmode=disable |
| SAMPMAIL_PG_HOST | localhost | PostgreSQL server hostname. |
| SAMPMAIL_PG_PORT | 5432 | PostgreSQL server port. |
| SAMPMAIL_PG_USER | | PostgreSQL username. Required when using the postgres driver. |
| SAMPMAIL_PG_PASSWORD | | PostgreSQL password. Required when using the postgres driver. |
| SAMPMAIL_PG_DATABASE | | PostgreSQL database name. Required when using the postgres driver. |
| SAMPMAIL_PG_SSLMODE | require | PostgreSQL SSL mode: require, disable, or verify-full. Use disable only on localhost connections. |
| SAMPMAIL_DB_MAX_OPEN_CONNS | 100 | Maximum number of open connections in the database connection pool. |
| SAMPMAIL_DB_MAX_IDLE_CONNS | 10 | Number of idle connections kept alive in the pool at all times. |

### Redis

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_REDIS_ADDR | | Redis server address, e.g. redis:6379 or 127.0.0.1:6379. When not set, all rate limiting uses in-memory counters. |
| SAMPMAIL_REDIS_PASSWORD | | Redis AUTH password. |
| SAMPMAIL_REDIS_DB | 0 | Redis logical database number (0-15). |

### SMTP and KumoMTA

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_SMTP_ADDR | kumomta:587 | KumoMTA SMTP submission address (host:port). |
| SAMPMAIL_SMTP_MAX_CONNS | 100 | Maximum number of connections in the SMTP pool. Must be at least as large as SAMPMAIL_CAMPAIGN_WORKERS. |
| SAMPMAIL_SMTP_MIN_CONNS | 5 | Number of connections kept alive in the pool at all times. |
| SAMPMAIL_SMTP_CONN_TIMEOUT | 10s | Timeout for establishing a new SMTP connection. |
| SAMPMAIL_KUMO_DIR | /opt/kumomta | KumoMTA installation root. Used for DKIM key management. |
| SAMPMAIL_LOG_DIR | /var/log/kumomta | Directory where KumoMTA writes delivery logs. Read by the bounce processor. |
| SAMPMAIL_SPOOL_DIR | /var/spool/kumomta | KumoMTA spool directory. |
| SAMPMAIL_KUMO_API_URL | http://127.0.0.1:8000 | KumoMTA HTTP management API URL. |

### Email verification

| Variable | Default | Description |
|---|---|---|
| REACHER_URL | http://reacher:8080 | Reacher service base URL. |
| REACHER_API_KEY | | API key for Reacher Cloud. Leave empty for self-hosted Reacher. |

### Campaigns

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_CAMPAIGN_WORKERS | 50 | Number of goroutines sending emails concurrently per campaign. Higher values increase throughput but also CPU and SMTP pool usage. |
| SAMPMAIL_IMPORT_BATCH_SIZE | 500 | Number of rows processed per database transaction during CSV import. Lower values use less memory. |

### Security

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_BCRYPT_COST | 12 | bcrypt cost factor for password hashing. Each increment doubles the computation time. Values 12 to 14 are appropriate for production. |
| SAMPMAIL_FIREWALL_ENABLED | true | Automatically block IP addresses that exhibit attack patterns such as repeated auth failures. |

### File paths

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_DATA_DIR | /var/lib/sampmail | Root directory for the SQLite database file and configuration backups. |
| SAMPMAIL_KUMO_DIR | /opt/kumomta | See SMTP and KumoMTA section above. |
| SAMPMAIL_HOME_DIR | /home | System home directory. Used internally for certain path operations. |

### Logging

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_LOG_LEVEL | info | Log verbosity: debug, info, warn, or error. Use debug only for troubleshooting — it is very verbose. |
| SAMPMAIL_LOG_JSON | true when SAMPMAIL_ENV=production | Emit structured JSON log lines. Set to false for human-readable output during development. |

### Monitoring

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_METRICS_ENABLED | true | Expose a Prometheus-compatible metrics endpoint. |
| SAMPMAIL_METRICS_PATH | /metrics | URL path for the Prometheus scrape endpoint. |

### Backups

| Variable | Default | Description |
|---|---|---|
| SAMPMAIL_BACKUP_RETENTION | 3 | Maximum number of configuration backup copies retained in the data directory. Older copies are deleted automatically. |

---

## Example: minimal production configuration

\`\`\`bash
# /etc/sampmail.env
# Permissions: 600 — readable only by root and the sampmail group

SAMPMAIL_SECRET=YourMinimum32CharacterSecretHere1234
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

SAMPMAIL_LISTEN_ADDR=0.0.0.0:9000
SAMPMAIL_ENV=production

SAMPMAIL_DB_DRIVER=postgres
SAMPMAIL_PG_HOST=127.0.0.1
SAMPMAIL_PG_USER=sampmail
SAMPMAIL_PG_PASSWORD=YourDatabasePassword
SAMPMAIL_PG_DATABASE=sampmail
SAMPMAIL_PG_SSLMODE=disable

SAMPMAIL_TRUST_PROXY=true
SAMPMAIL_TRUSTED_PROXY_CIDR=127.0.0.1/32

SAMPMAIL_SMTP_ADDR=127.0.0.1:587
REACHER_URL=http://127.0.0.1:8080

SAMPMAIL_LOG_JSON=true
SAMPMAIL_LOG_LEVEL=info
\`\`\`

---

## Example: Docker Compose .env

\`\`\`bash
# .env — do not commit to version control

SAMPMAIL_SECRET=YourMinimum32CharacterSecretHere1234
POSTGRES_PASSWORD=YourDatabasePassword
SAMPMAIL_BASE_URL=https://mail.yourdomain.com

SAMPMAIL_DB_DRIVER=postgres
POSTGRES_USER=sampmail
POSTGRES_DB=sampmail
SAMPMAIL_REDIS_ADDR=redis:6379
\`\`\`

---

## Verifying the configuration

To confirm the application starts with a valid configuration:

\`\`\`bash
systemctl start sampmail
journalctl -u sampmail -n 30 --no-pager
\`\`\`

The startup log shows:

- The resolved listen address.
- The database driver and connection target.
- Whether Redis was successfully connected.
- The SMTP pool status.
- The Reacher URL.
- The auto-seeded hostname (on first run).

If any required variable is missing or invalid, the process exits immediately with a
descriptive error message before attempting any network connections.
`);
console.log('CONFIGURATION.md written');
