# SampMail Campaign Guide

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

Only campaigns in `draft` status can be deleted.

---

## Creating a Campaign

A campaign requires:

- **Name** — internal label, not visible to recipients.
- **Subject** — the email subject line. Supports personalization variables.
- **Body** — the HTML email content. Supports personalization variables.
- **Sender** — a configured sender identity (email address and IP).
- **Recipient source** — a contact list or a CSV upload.

Create a campaign via the API:

```
POST /api/v2/campaigns
```

```json
{
  "name": "March Newsletter",
  "subject": "What is new this month, {{first_name}}",
  "body": "<h1>Hello {{first_name}},</h1><p>Here is what is new...</p>",
  "sender_id": 3,
  "organization_id": 1
}
```

A campaign left without a `scheduled_at` value remains in draft status until
manually triggered.

---

## Recipient Lists

**Option 1: Contact list**

Create and manage contact lists in the Lists section. Assign the list to a campaign.
All active, non-suppressed contacts in the list become recipients.

**Option 2: CSV import**

Upload a CSV file directly to a list. Required column: `email`. Optional columns:
`first_name`, `last_name`, `phone`, `company`, and any custom field names defined
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

Custom fields defined on the contact list are available as `{{field_name}}`.

### Unsubscribe link

Every campaign email must include an unsubscribe mechanism. The `{{unsubscribe_link}}`
variable inserts a signed URL that takes the recipient to a confirmation page. Confirming
the unsubscribe adds the address to the suppression list.

An RFC 8058 one-click unsubscribe header is also added to every message automatically:

```
List-Unsubscribe: <https://mail.yourdomain.com/api/unsubscribe/{token}>
List-Unsubscribe-Post: List-Unsubscribe=One-Click
```

This allows mail clients that support one-click unsubscribe to process it without
requiring the recipient to visit the unsubscribe page.

---

## Scheduling

Set `scheduled_at` to an ISO 8601 timestamp to schedule a campaign:

```json
{
  "scheduled_at": "2025-03-15T09:00:00Z"
}
```

The background scheduler checks for scheduled campaigns every 5 minutes and starts
sending when the scheduled time has passed.

---

## Sending

When a campaign starts sending:

1. The campaign status is set to `sending`.
2. A pool of `SAMPMAIL_CAMPAIGN_WORKERS` goroutines (default: 50) is created.
3. Each worker claims the next pending recipient row.
4. For each recipient, the worker:
   - Checks the suppression list. Skips if suppressed.
   - Personalizes the subject and body.
   - Rewrites links to click-tracking URLs.
   - Injects the open tracking pixel.
   - Sends the email via the SMTP connection pool to KumoMTA.
   - Marks the recipient as `sent` or `failed`.
   - Updates the campaign's aggregate counters atomically.
5. When all recipients are processed, the campaign status is set to `completed`.

**Worker count and throughput:**

The default of 50 workers is appropriate for most deployments. Higher values increase
throughput but also increase the load on both SampMail and KumoMTA. The SMTP pool
maximum (`SAMPMAIL_SMTP_MAX_CONNS`) must be at least as large as the worker count.

**Resuming interrupted campaigns:**

If the process restarts while a campaign is sending, the scheduler detects campaigns
stuck in `sending` status on startup and resumes them automatically.

---

## Tracking Opens and Clicks

**Open tracking:**

Each email body includes a 1x1 transparent GIF loaded from:

```
https://mail.yourdomain.com/api/track/open/{recipientID}?sig={hmac}
```

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

```
https://mail.yourdomain.com/api/track/click/{recipientID}?url={encoded-url}&sig={hmac}
```

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

1. Clicking the `{{unsubscribe_link}}` in the email body.
2. Using the one-click unsubscribe mechanism in their mail client (RFC 8058).

On confirmation, the recipient's email address is added to the suppression list. Future
campaigns will not send to this address within the same organization.

Unsubscribes are visible in the Suppressions section of the UI with reason
`unsubscribe`.

---

## Bounce Handling

Bounces are processed by reading KumoMTA delivery logs every 10 minutes.

**Hard bounces (5xx SMTP codes):**

The recipient address does not exist or the domain does not accept mail. The address
is immediately added to the suppression list with reason `hard_bounce`. The campaign's
`total_bounced` counter is incremented.

**Soft bounces (4xx SMTP codes):**

Temporary failure: mailbox full, server temporarily unavailable. The address is not
immediately suppressed. After repeated soft bounces within a rolling window, the address
is added to the suppression list with reason `soft_bounce_threshold`.

**Spam complaints (FBL):**

When a recipient marks the email as spam and their ISP reports it via a feedback loop,
the address is added to the suppression list with reason `complaint`.

---

## Analytics

Campaign analytics are available at:

```
GET /api/v2/analytics/dashboard
GET /api/v2/analytics/deliverability
```

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
- Personalization in the subject (e.g. `{{first_name}}`) can improve open rates but
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
