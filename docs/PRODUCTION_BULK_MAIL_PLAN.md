# Production Bulk Mailing Implementation Plan

## Overview

This document outlines the implementation plan to transform SampMail from a basic email system into a production-ready bulk mailing solution. The plan addresses all critical gaps identified in the code review.

## Current State Assessment

### What's Working ✅
- SMTP connection pooling with circuit breaker
- Basic bounce processing and parsing
- IP warmup schedules (conservative, standard, aggressive)
- Suppression list management
- Email personalization engine
- Open/click tracking
- KumoMTA integration
- Multi-tenancy support

### Critical Gaps ❌
1. No persistent job queue (in-memory only)
2. No retry logic with exponential backoff
3. No per-domain rate limiting
4. Warmup schedules not enforced during sending
5. No FBL (Feedback Loop) processing
6. Bounce classification not acted upon
7. No pre-send DNS validation
8. No list hygiene/engagement scoring
9. No RFC 8058 one-click unsubscribe

---

## Implementation Plan

### Phase 1: Core Infrastructure (P0 - Critical)

#### 1.1 Persistent Job Queue with Redis

**File:** `internal/core/job_queue.go` (NEW)

**Purpose:** Replace in-memory channel with durable Redis-backed queue

**Components:**
```go
type JobQueue struct {
    redis      *redis.Client
    queueName  string
    processing string // processing queue name for reliability
}

type EmailJob struct {
    ID           string
    CampaignID   uint
    RecipientID  uint
    Email        string
    Subject      string
    Body         string
    SenderID     uint
    RetryCount   int
    MaxRetries   int
    NextRetryAt  time.Time
    CreatedAt    time.Time
    Priority     int
}
```

**Features:**
- LPUSH/RPOP for FIFO processing
- Backup queue for in-flight jobs
- Automatic job recovery on restart
- Priority queue support
- Job timeout and requeue

**Integration Points:**
- `campaign.go:StartCampaign()` → Push jobs to queue
- `campaign.go:sendWorker()` → Pull jobs from queue
- Server startup → Recover processing jobs

---

#### 1.2 Retry Logic with Exponential Backoff

**File:** `internal/core/retry.go` (NEW)

**Purpose:** Implement intelligent retry for failed emails

**Components:**
```go
type RetryPolicy struct {
    MaxRetries     int
    InitialDelay   time.Duration
    MaxDelay       time.Duration
    Multiplier     float64
    RetryableCodes []string // 421, 450, 451, 452, etc.
}

var DefaultRetryPolicy = RetryPolicy{
    MaxRetries:     5,
    InitialDelay:   1 * time.Minute,
    MaxDelay:       24 * time.Hour,
    Multiplier:     2.0,
    RetryableCodes: []string{"421", "450", "451", "452", "454"},
}
```

**Retry Schedule:**
- Attempt 1: Immediate
- Attempt 2: 1 minute
- Attempt 3: 5 minutes
- Attempt 4: 15 minutes
- Attempt 5: 1 hour
- Attempt 6: 4 hours
- Attempt 7: 24 hours (final)

**Integration Points:**
- `campaign.go:sendWorker()` → Check retry policy on error
- `job_queue.go` → Schedule retry with delay
- `bounce_processor.go` → Classify retryable vs permanent

---

#### 1.3 Per-Domain Rate Limiting

**File:** `internal/core/domain_limiter.go` (NEW)

**Purpose:** Enforce provider-specific rate limits

**Components:**
```go
type DomainLimiter struct {
    redis    *redis.Client
    limits   map[string]DomainLimit
    mu       sync.RWMutex
}

type DomainLimit struct {
    Domain       string
    MaxPerHour   int
    MaxPerDay    int
    CurrentHour  int
    CurrentDay   int
    WarmupFactor float64 // 0.0 - 1.0
}

// Provider-specific defaults
var ProviderLimits = map[string]DomainLimit{
    "gmail.com":     {MaxPerHour: 500, MaxPerDay: 5000},
    "googlemail.com": {MaxPerHour: 500, MaxPerDay: 5000},
    "outlook.com":   {MaxPerHour: 30, MaxPerDay: 1000},
    "hotmail.com":   {MaxPerHour: 30, MaxPerDay: 1000},
    "yahoo.com":     {MaxPerHour: 100, MaxPerDay: 2000},
    "aol.com":       {MaxPerHour: 100, MaxPerDay: 2000},
    "_default":      {MaxPerHour: 1000, MaxPerDay: 10000},
}
```

**Features:**
- Token bucket algorithm per domain
- Redis-based distributed counting
- Automatic warmup factor application
- Real-time limit adjustment

**Integration Points:**
- `campaign.go:StartCampaign()` → Check limits before send
- `job_queue.go` → Delay jobs when limit reached
- `warmup.go` → Adjust limits based on warmup day

---

### Phase 2: Sending Intelligence (P1 - High)

#### 2.1 Warmup Integration into Campaigns

**File:** `internal/core/campaign.go` (MODIFY)

**Changes:**
```go
// In StartCampaign, before sending:
func (cs *CampaignService) StartCampaign(c *models.Campaign) error {
    // ... existing code ...
    
    // NEW: Check warmup status and apply rate limit
    if sender.WarmupEnabled {
        warmupRate := GetWarmupRateForSender(sender)
        domainLimiter.SetWarmupFactor(sender.Domain, warmupRate)
    }
    
    // NEW: Throttle sending based on warmup
    ticker := time.NewTicker(calculateThrottleInterval(sender))
    defer ticker.Stop()
    
    // ... rest of sending logic ...
}

func calculateThrottleInterval(sender *models.Sender) time.Duration {
    if !sender.WarmupEnabled {
        return 0 // No throttling
    }
    
    plan := WarmupSchedules[sender.WarmupPlan]
    day := sender.WarmupDay
    if day >= len(plan) {
        day = len(plan) - 1
    }
    
    rateStr := plan[day]
    rate := parseRateString(rateStr) // e.g., "100/hr" → 100
    return time.Hour / time.Duration(rate)
}
```

---

#### 2.2 FBL (Feedback Loop) Processing

**File:** `internal/core/fbl_processor.go` (NEW)

**Purpose:** Process spam complaints from feedback loops

**Components:**
```go
type FBLProcessor struct {
    store     *store.Store
    imapConn  *imap.Client
}

type FBLReport struct {
    OriginalRecipient string
    OriginalMessageID string
    FeedbackType      string // abuse, fraud, not-spam, opt-out, other
    OriginalSubject   string
    ReceivedDate      time.Time
    ReportingMTA      string
}

func (f *FBLProcessor) ProcessFBLReport(report *FBLReport) error {
    // 1. Add to suppression list (type: complaint)
    // 2. Log complaint for analytics
    // 3. Update sender reputation score
    // 4. Alert if complaint rate > 0.1%
}
```

**Integration Points:**
- IMAP mailbox monitoring for abuse@
- ARF (Abuse Report Format) parsing
- Suppression list sync
- Reputation dashboard

---

#### 2.3 Bounce Classification Actions

**File:** `internal/core/bounce_processor.go` (MODIFY)

**Changes:**
```go
func (bp *BounceProcessor) ProcessBounce(info *BounceInfo) error {
    switch info.BounceType {
    case "hard":
        // Add to permanent suppression
        bp.Store.AddSuppression(&models.Suppression{
            Email:       info.OriginalRecipient,
            Reason:      "hard_bounce",
            BounceCode:  info.BounceCode,
            Source:      "bounce_processor",
        })
        
        // Update contact status
        bp.Store.DB.Model(&models.ContactV2{}).
            Where("email = ?", info.OriginalRecipient).
            Update("status", "bounced")
            
    case "soft":
        // Schedule for retry (handled by retry policy)
        // Log for monitoring
        bp.logSoftBounce(info)
        
    case "complaint":
        // Add to suppression
        bp.Store.AddSuppression(&models.Suppression{
            Email:       info.OriginalRecipient,
            Reason:      "complaint",
            Source:      "fbl",
        })
        
        // Decrement sender reputation
        bp.updateSenderReputation(info.SenderID, -10)
    }
    
    return nil
}
```

---

### Phase 3: Pre-Send Validation (P2 - Medium)

#### 3.1 Pre-Send DNS Validation

**File:** `internal/core/validation.go` (MODIFY)

**Changes:**
```go
func ValidateSenderReady(sender *models.Sender) ([]ValidationError, error) {
    var errors []ValidationError
    
    // 1. Check SPF
    spfRecord, err := lookupSPF(sender.Domain)
    if err != nil || !isValidSPF(spfRecord, sender.IP) {
        errors = append(errors, ValidationError{
            Field:   "spf",
            Message: "SPF record missing or invalid",
        })
    }
    
    // 2. Check DKIM
    dkimRecord, err := lookupDKIM(sender.Domain, sender.DKIMSelector)
    if err != nil || !isValidDKIM(dkimRecord, sender.DKIMPublicKey) {
        errors = append(errors, ValidationError{
            Field:   "dkim",
            Message: "DKIM record missing or key mismatch",
        })
    }
    
    // 3. Check DMARC
    dmarcRecord, err := lookupDMARC(sender.Domain)
    if err != nil || dmarcRecord.Policy == "none" {
        errors = append(errors, ValidationError{
            Field:   "dmarc",
            Message: "DMARC policy should be at least 'quarantine'",
        })
    }
    
    // 4. Check MX
    mxRecords, err := lookupMX(sender.Domain)
    if err != nil || len(mxRecords) == 0 {
        errors = append(errors, ValidationError{
            Field:   "mx",
            Message: "No MX records found for domain",
        })
    }
    
    // 5. Check IP reputation (optional)
    reputation, _ := checkIPReputation(sender.IP)
    if reputation < 50 {
        errors = append(errors, ValidationError{
            Field:   "ip_reputation",
            Message: fmt.Sprintf("IP reputation is low: %d/100", reputation),
        })
    }
    
    return errors, nil
}
```

**Integration Points:**
- `campaign.go:StartCampaign()` → Validate before sending
- API endpoint for manual validation
- Dashboard warning indicators

---

#### 3.2 List Hygiene & Engagement Scoring

**File:** `internal/core/list_hygiene.go` (NEW)

**Purpose:** Track engagement and auto-suppress inactive subscribers

**Components:**
```go
type EngagementScore struct {
    ContactID       uint
    LastOpen        *time.Time
    LastClick       *time.Time
    LastPurchase    *time.Time
    TotalOpens      int
    TotalClicks     int
    EngagementScore float64 // 0.0 - 1.0
    Status          string  // active, at-risk, inactive
}

func CalculateEngagementScore(contact *models.ContactV2) float64 {
    score := 0.0
    
    // Recent engagement (last 30 days)
    if contact.LastOpenAt != nil && time.Since(*contact.LastOpenAt) < 30*24*time.Hour {
        score += 0.4
    }
    
    // Click engagement (high value)
    if contact.LastClickAt != nil && time.Since(*contact.LastClickAt) < 30*24*time.Hour {
        score += 0.3
    }
    
    // Frequency
    if contact.TotalOpens > 5 {
        score += 0.2
    }
    
    // Recency decay
    if contact.LastOpenAt != nil {
        daysSinceOpen := time.Since(*contact.LastOpenAt).Hours() / 24
        score -= daysSinceOpen * 0.01
    }
    
    return math.Max(0, math.Min(1, score))
}

func ProcessInactiveSubscribers(st *store.Store) error {
    // Find contacts with no engagement in 6+ months
    cutoff := time.Now().AddDate(0, -6, 0)
    
    var inactiveContacts []models.ContactV2
    st.DB.Where("last_open_at < ? OR last_open_at IS NULL", cutoff).
        Find(&inactiveContacts)
    
    for _, contact := range inactiveContacts {
        // Option 1: Auto-suppress
        st.AddSuppression(&models.Suppression{
            Email:  contact.Email,
            Reason: "inactive_6_months",
            Source: "list_hygiene",
        })
        
        // Option 2: Mark as inactive (softer approach)
        st.DB.Model(&contact).Update("status", "inactive")
    }
    
    return nil
}
```

---

#### 3.3 RFC 8058 One-Click Unsubscribe

**File:** `internal/api/unsubscribe.go` (MODIFY)

**Changes:**
```go
// Add List-Unsubscribe-Post header support
func BuildUnsubscribeHeaders(sender *models.Sender, recipient string, campaignID uint) string {
    unsubscribeURL := fmt.Sprintf("%s/unsubscribe/one-click?email=%s&campaign=%d&sig=%s",
        sender.BaseURL,
        url.QueryEscape(recipient),
        campaignID,
        SignUnsubscribe(recipient, campaignID),
    )
    
    return fmt.Sprintf(
        "List-Unsubscribe: <%s>\r\n"+
        "List-Unsubscribe-Post: List-Unsubscribe=One-Click\r\n",
        unsubscribeURL,
    )
}

// Handle one-click unsubscribe POST
func (h *Handler) HandleOneClickUnsubscribe(w http.ResponseWriter, r *http.Request) {
    // Gmail, Outlook send POST to List-Unsubscribe URL
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", 405)
        return
    }
    
    email := r.FormValue("email")
    campaignID := r.FormValue("campaign")
    sig := r.FormValue("sig")
    
    // Verify signature
    if !VerifyUnsubscribeSig(email, campaignID, sig) {
        http.Error(w, "Invalid signature", 400)
        return
    }
    
    // Add to suppression
    h.Store.AddSuppression(&models.Suppression{
        Email:  email,
        Reason: "one_click_unsubscribe",
        Source: "rfc8058",
    })
    
    // Return 200 OK (required by RFC 8058)
    w.WriteHeader(http.StatusOK)
}
```

---

## File Changes Summary

### New Files to Create
| File | Purpose |
|------|---------|
| `internal/core/job_queue.go` | Redis-backed persistent job queue |
| `internal/core/retry.go` | Retry policy with exponential backoff |
| `internal/core/domain_limiter.go` | Per-domain rate limiting |
| `internal/core/fbl_processor.go` | Feedback loop processing |
| `internal/core/list_hygiene.go` | Engagement scoring and list hygiene |

### Files to Modify
| File | Changes |
|------|---------|
| `internal/core/campaign.go` | Integrate queue, warmup, rate limiting |
| `internal/core/bounce_processor.go` | Add bounce classification actions |
| `internal/core/validation.go` | Add pre-send DNS validation |
| `internal/api/unsubscribe.go` | Add RFC 8058 support |
| `internal/config/config.go` | Add Redis queue config |
| `go.mod` | Add Redis dependency |

---

## Implementation Order

1. **Week 1: Core Infrastructure**
   - Day 1-2: Job queue with Redis
   - Day 3: Retry logic
   - Day 4-5: Per-domain rate limiting

2. **Week 2: Sending Intelligence**
   - Day 1-2: Warmup integration
   - Day 3-4: FBL processing
   - Day 5: Bounce classification actions

3. **Week 3: Validation & Hygiene**
   - Day 1-2: Pre-send DNS validation
   - Day 3-4: List hygiene & engagement
   - Day 5: RFC 8058 unsubscribe

---

## Testing Strategy

### Unit Tests
- Job queue operations (push, pop, timeout, recovery)
- Retry policy calculations
- Rate limiter token bucket
- Bounce classification logic
- Engagement score calculations

### Integration Tests
- Full campaign send with warmup
- Rate limit enforcement across domains
- Bounce → suppression flow
- FBL → suppression flow
- One-click unsubscribe flow

### Load Tests
- 100K emails with rate limiting
- Concurrent campaigns
- Queue recovery after crash
- Redis failover

---

## Monitoring & Alerts

### Key Metrics
- Queue depth (pending jobs)
- Send rate per domain
- Bounce rate per campaign
- Complaint rate per sender
- Retry queue size
- Warmup progress

### Alerts
- Bounce rate > 5%
- Complaint rate > 0.1%
- Queue depth > 10,000
- Retry failures > 100
- DNS validation failures

---

## Dependencies

### New Dependencies
```
github.com/redis/go-redis/v9  # Redis client
github.com/emersion/go-imap   # FBL IMAP processing
```

### Infrastructure
- Redis 7.x (for job queue)
- IMAP access to abuse@ mailbox

---

## Rollback Plan

Each component is designed to be independently deployable and reversible:

1. **Job Queue**: Feature flag to switch between Redis and in-memory
2. **Rate Limiting**: Can be disabled per sender
3. **Warmup**: Can be disabled per sender
4. **FBL**: Can be disabled globally
5. **List Hygiene**: Manual trigger, not automatic

---

## Success Criteria

- [ ] Zero email loss on server restart
- [ ] 95%+ retry success rate for temporary failures
- [ ] Zero rate limit violations
- [ ] < 2% bounce rate on warmed IPs
- [ ] < 0.05% complaint rate
- [ ] 100% DNS validation before send
- [ ] RFC 8058 compliant unsubscribe
