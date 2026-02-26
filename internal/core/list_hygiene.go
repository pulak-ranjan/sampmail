package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// LIST HYGIENE & ENGAGEMENT SCORING
// =====================================
// Tracks subscriber engagement and manages list hygiene
// to maintain sender reputation and deliverability

// EngagementScore represents a contact's engagement level
type EngagementScore struct {
	ContactID       uint      `json:"contact_id"`
	Email           string    `json:"email"`
	Score           float64   `json:"score"`           // 0.0 - 1.0
	Level           string    `json:"level"`           // engaged, at_risk, inactive, dormant
	LastOpen        *time.Time `json:"last_open"`
	LastClick       *time.Time `json:"last_click"`
	LastPurchase    *time.Time `json:"last_purchase"`
	TotalOpens      int       `json:"total_opens"`
	TotalClicks     int       `json:"total_clicks"`
	DaysSinceOpen   int       `json:"days_since_open"`
	DaysSinceClick  int       `json:"days_since_click"`
	OpenRate        float64   `json:"open_rate"`       // Opens / Emails Received
	ClickRate       float64   `json:"click_rate"`      // Clicks / Emails Received
	EngagementTrend string    `json:"engagement_trend"` // improving, declining, stable
}

// EngagementThresholds defines thresholds for engagement levels
type EngagementThresholds struct {
	EngagedDays      int     // Days since last engagement to be "engaged"
	AtRiskDays       int     // Days since last engagement to be "at risk"
	InactiveDays     int     // Days since last engagement to be "inactive"
	DormantDays      int     // Days since last engagement to be "dormant"
	MinOpenRate      float64 // Minimum open rate for engaged
	MinClickRate     float64 // Minimum click rate for engaged
	SuppressionDays  int     // Days before auto-suppression
}

// DefaultEngagementThresholds returns production-ready defaults
func DefaultEngagementThresholds() EngagementThresholds {
	return EngagementThresholds{
		EngagedDays:     30,   // Engaged in last 30 days
		AtRiskDays:      90,   // No engagement in 90 days
		InactiveDays:    180,  // No engagement in 6 months
		DormantDays:     365,  // No engagement in 12 months
		MinOpenRate:     0.10, // 10% minimum open rate
		MinClickRate:    0.02, // 2% minimum click rate
		SuppressionDays: 365,  // Auto-suppress after 12 months
	}
}

// ListHygieneManager manages list hygiene operations
type ListHygieneManager struct {
	store      *store.Store
	thresholds EngagementThresholds
	mu         sync.RWMutex
}

// NewListHygieneManager creates a new list hygiene manager
func NewListHygieneManager(st *store.Store) *ListHygieneManager {
	return &ListHygieneManager{
		store:      st,
		thresholds: DefaultEngagementThresholds(),
	}
}

// SetThresholds sets custom engagement thresholds
func (lhm *ListHygieneManager) SetThresholds(t EngagementThresholds) {
	lhm.mu.Lock()
	defer lhm.mu.Unlock()
	lhm.thresholds = t
}

// CalculateEngagementScore calculates engagement score for a contact
func (lhm *ListHygieneManager) CalculateEngagementScore(contact *models.ContactV2) *EngagementScore {
	score := &EngagementScore{
		ContactID:    contact.ID,
		Email:        contact.Email,
		TotalOpens:   contact.TotalOpens,
		TotalClicks:  contact.TotalClicks,
		LastOpen:     contact.LastOpenedAt,
		LastClick:    contact.LastClickedAt,
	}
	
	now := time.Now()
	
	// Calculate days since engagement
	if contact.LastOpenedAt != nil {
		score.DaysSinceOpen = int(now.Sub(*contact.LastOpenedAt).Hours() / 24)
	} else {
		score.DaysSinceOpen = 9999 // Never opened
	}
	
	if contact.LastClickedAt != nil {
		score.DaysSinceClick = int(now.Sub(*contact.LastClickedAt).Hours() / 24)
	} else {
		score.DaysSinceClick = 9999 // Never clicked
	}
	
	// Calculate rates (using total opens/clicks as proxy since TotalEmailsReceived doesn't exist)
	totalEmails := contact.TotalOpens + contact.TotalClicks
	if totalEmails > 0 {
		score.OpenRate = float64(contact.TotalOpens) / float64(totalEmails)
		score.ClickRate = float64(contact.TotalClicks) / float64(totalEmails)
	}
	
	// Calculate engagement score (0.0 - 1.0)
	score.Score = lhm.computeScore(contact, score)
	
	// Determine engagement level
	score.Level = lhm.determineLevel(score)
	
	// Determine trend
	score.EngagementTrend = lhm.determineTrend(contact)
	
	return score
}

// computeScore calculates the numeric engagement score
func (lhm *ListHygieneManager) computeScore(contact *models.ContactV2, es *EngagementScore) float64 {
	var score float64
	
	// 1. Recency of last open (0-0.3 points)
	if contact.LastOpenedAt != nil {
		daysSince := time.Since(*contact.LastOpenedAt).Hours() / 24
		if daysSince <= 7 {
			score += 0.30 // Opened in last week
		} else if daysSince <= 30 {
			score += 0.25 // Opened in last month
		} else if daysSince <= 90 {
			score += 0.15 // Opened in last quarter
		} else if daysSince <= 180 {
			score += 0.05 // Opened in last 6 months
		}
	}
	
	// 2. Recency of last click (0-0.25 points)
	if contact.LastClickedAt != nil {
		daysSince := time.Since(*contact.LastClickedAt).Hours() / 24
		if daysSince <= 7 {
			score += 0.25 // Clicked in last week
		} else if daysSince <= 30 {
			score += 0.20 // Clicked in last month
		} else if daysSince <= 90 {
			score += 0.10 // Clicked in last quarter
		} else if daysSince <= 180 {
			score += 0.05 // Clicked in last 6 months
		}
	}
	
	// 3. Open rate (0-0.20 points)
	if es.OpenRate >= 0.50 {
		score += 0.20 // 50%+ open rate
	} else if es.OpenRate >= 0.30 {
		score += 0.15 // 30%+ open rate
	} else if es.OpenRate >= 0.10 {
		score += 0.10 // 10%+ open rate
	} else if es.OpenRate >= 0.05 {
		score += 0.05 // 5%+ open rate
	}
	
	// 4. Click rate (0-0.15 points)
	if es.ClickRate >= 0.10 {
		score += 0.15 // 10%+ click rate
	} else if es.ClickRate >= 0.05 {
		score += 0.10 // 5%+ click rate
	} else if es.ClickRate >= 0.02 {
		score += 0.05 // 2%+ click rate
	}
	
	// 5. Frequency of engagement (0-0.10 points)
	if contact.TotalOpens >= 10 {
		score += 0.10
	} else if contact.TotalOpens >= 5 {
		score += 0.07
	} else if contact.TotalOpens >= 2 {
		score += 0.03
	}
	
	// Clamp to 0-1
	return math.Max(0, math.Min(1, score))
}

// determineLevel determines the engagement level
func (lhm *ListHygieneManager) determineLevel(score *EngagementScore) string {
	lhm.mu.RLock()
	thresholds := lhm.thresholds
	lhm.mu.RUnlock()
	
	// Check days since last engagement
	minDays := score.DaysSinceOpen
	if score.DaysSinceClick < minDays {
		minDays = score.DaysSinceClick
	}
	
	switch {
	case minDays <= thresholds.EngagedDays && score.Score >= 0.3:
		return "engaged"
	case minDays <= thresholds.AtRiskDays:
		return "at_risk"
	case minDays <= thresholds.InactiveDays:
		return "inactive"
	default:
		return "dormant"
	}
}

// determineTrend determines if engagement is improving or declining
func (lhm *ListHygieneManager) determineTrend(contact *models.ContactV2) string {
	// Compare recent activity to historical
	// This is simplified - in production you'd track activity over time
	
	if contact.TotalOpens == 0 {
		return "no_activity"
	}
	
	// If last open was recent but total opens are low, improving
	if contact.LastOpenedAt != nil {
		daysSince := time.Since(*contact.LastOpenedAt).Hours() / 24
		if daysSince <= 14 && contact.TotalOpens <= 3 {
			return "improving"
		}
		if daysSince >= 60 {
			return "declining"
		}
	}
	
	return "stable"
}

// =====================================
// LIST HYGIENE OPERATIONS
// =====================================

// HygieneReport represents a list hygiene report
type HygieneReport struct {
	TotalContacts      int                      `json:"total_contacts"`
	EngagedCount       int                      `json:"engaged_count"`
	AtRiskCount        int                      `json:"at_risk_count"`
	InactiveCount      int                      `json:"inactive_count"`
	DormantCount       int                      `json:"dormant_count"`
	AverageScore       float64                  `json:"average_score"`
	ToSuppress         []string                 `json:"to_suppress,omitempty"`
	Recommendations    []string                 `json:"recommendations"`
	SegmentBreakdown   map[string]int           `json:"segment_breakdown"`
}

// RunHygieneCheck performs a full list hygiene check
func (lhm *ListHygieneManager) RunHygieneCheck(ctx context.Context, orgID uint) (*HygieneReport, error) {
	log := logger.WithComponent("list_hygiene")
	
	report := &HygieneReport{
		SegmentBreakdown: make(map[string]int),
		Recommendations:  []string{},
	}
	
	// Get all contacts for organization
	var contacts []models.ContactV2
	query := lhm.store.DB.Model(&models.ContactV2{})
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	if err := query.Find(&contacts).Error; err != nil {
		return nil, err
	}
	
	report.TotalContacts = len(contacts)
	
	var totalScore float64
	scores := make([]*EngagementScore, 0, len(contacts))
	
	for _, contact := range contacts {
		score := lhm.CalculateEngagementScore(&contact)
		scores = append(scores, score)
		totalScore += score.Score
		
		// Count by level
		switch score.Level {
		case "engaged":
			report.EngagedCount++
		case "at_risk":
			report.AtRiskCount++
		case "inactive":
			report.InactiveCount++
		case "dormant":
			report.DormantCount++
		}
		
		// Check for suppression candidates
		lhm.mu.RLock()
		thresholds := lhm.thresholds
		lhm.mu.RUnlock()
		
		if score.DaysSinceOpen >= thresholds.SuppressionDays {
			report.ToSuppress = append(report.ToSuppress, contact.Email)
		}
	}
	
	// Calculate average score
	if len(contacts) > 0 {
		report.AverageScore = totalScore / float64(len(contacts))
	}
	
	// Generate recommendations
	report.Recommendations = lhm.generateRecommendations(report)
	
	log.Info("list hygiene check completed",
		"total", report.TotalContacts,
		"engaged", report.EngagedCount,
		"at_risk", report.AtRiskCount,
		"inactive", report.InactiveCount,
		"dormant", report.DormantCount,
		"to_suppress", len(report.ToSuppress))
	
	return report, nil
}

// generateRecommendations generates hygiene recommendations
func (lhm *ListHygieneManager) generateRecommendations(report *HygieneReport) []string {
	var recommendations []string
	
	// Check engagement ratio
	engagedRatio := float64(report.EngagedCount) / float64(report.TotalContacts)
	if engagedRatio < 0.20 {
		recommendations = append(recommendations,
			"Low engagement rate (<20%). Consider re-engagement campaign or list cleaning.")
	}
	
	// Check at-risk ratio
	atRiskRatio := float64(report.AtRiskCount) / float64(report.TotalContacts)
	if atRiskRatio > 0.30 {
		recommendations = append(recommendations,
			"High at-risk population (>30%). Send targeted re-engagement emails.")
	}
	
	// Check dormant
	if report.DormantCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("%d dormant subscribers should be suppressed.", report.DormantCount))
	}
	
	// Check suppression candidates
	if len(report.ToSuppress) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("%d subscribers inactive for 12+ months - recommend suppression.", len(report.ToSuppress)))
	}
	
	return recommendations
}

// SuppressInactive suppresses contacts that have been inactive too long
func (lhm *ListHygieneManager) SuppressInactive(ctx context.Context, orgID uint, dryRun bool) (int, error) {
	log := logger.WithComponent("list_hygiene")
	
	lhm.mu.RLock()
	thresholds := lhm.thresholds
	lhm.mu.RUnlock()
	
	cutoff := time.Now().AddDate(0, 0, -thresholds.SuppressionDays)
	
	// Find inactive contacts
	var contacts []models.ContactV2
	query := lhm.store.DB.Model(&models.ContactV2{}).
		Where("last_open_at < ? OR last_open_at IS NULL", cutoff)
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	if err := query.Find(&contacts).Error; err != nil {
		return 0, err
	}
	
	if dryRun {
		log.Info("dry run: would suppress contacts", "count", len(contacts))
		return len(contacts), nil
	}
	
	suppressed := 0
	for _, contact := range contacts {
		// Add to suppression list
		reason := "inactive_" + fmt.Sprintf("%d_days", thresholds.SuppressionDays)
		
		// orgID=0: list hygiene suppressions apply system-wide
		if err := lhm.store.AddSuppression(0, contact.Email, reason, "list_hygiene"); err != nil {
			log.Warn("failed to suppress contact", "email", contact.Email, "error", err)
			continue
		}
		
		// Update contact status
		lhm.store.DB.Model(&contact).Update("status", "suppressed")
		suppressed++
	}
	
	log.Info("suppressed inactive contacts", "count", suppressed)
	return suppressed, nil
}

// =====================================
// RE-ENGAGEMENT CAMPAIGNS
// =====================================

// ReEngagementConfig configures a re-engagement campaign
type ReEngagementConfig struct {
	InactiveDays     int      `json:"inactive_days"`     // Days since last engagement
	MaxEmails        int      `json:"max_emails"`        // Max emails to send
	Subject          string   `json:"subject"`           // Email subject
	Body             string   `json:"body"`              // Email body
	FromEmail        string   `json:"from_email"`        // Sender email
	FinalSuppression bool     `json:"final_suppression"` // Suppress if no engagement after
}

// CreateReEngagementSegment creates a segment for re-engagement
func (lhm *ListHygieneManager) CreateReEngagementSegment(orgID uint, inactiveDays int) ([]models.ContactV2, error) {
	cutoff := time.Now().AddDate(0, 0, -inactiveDays)
	
	var contacts []models.ContactV2
	query := lhm.store.DB.Model(&models.ContactV2{}).
		Where("last_open_at >= ? AND last_open_at < ?", 
			time.Now().AddDate(0, 0, -inactiveDays*2), // Not completely dormant
			cutoff).
		Where("status = ?", "active")
	
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	
	if err := query.Find(&contacts).Error; err != nil {
		return nil, err
	}
	
	return contacts, nil
}

// =====================================
// ENGAGEMENT TRACKING
// =====================================

// TrackOpen records an email open for engagement tracking
func (lhm *ListHygieneManager) TrackOpen(contactID uint) error {
	now := time.Now()
	updates := map[string]interface{}{
		"last_open_at":     now,
		"total_opens":      lhm.store.DB.Raw("total_opens + 1"),
		"last_engaged_at":  now,
	}
	
	return lhm.store.DB.Model(&models.ContactV2{}).
		Where("id = ?", contactID).
		Updates(updates).Error
}

// TrackClick records a click for engagement tracking
func (lhm *ListHygieneManager) TrackClick(contactID uint) error {
	now := time.Now()
	updates := map[string]interface{}{
		"last_click_at":    now,
		"total_clicks":     lhm.store.DB.Raw("total_clicks + 1"),
		"last_engaged_at":  now,
	}
	
	return lhm.store.DB.Model(&models.ContactV2{}).
		Where("id = ?", contactID).
		Updates(updates).Error
}

// TrackEmailSent records an email sent for engagement tracking
func (lhm *ListHygieneManager) TrackEmailSent(contactID uint) error {
	return lhm.store.DB.Model(&models.ContactV2{}).
		Where("id = ?", contactID).
		UpdateColumn("total_emails_received", lhm.store.DB.Raw("total_emails_received + 1")).Error
}

// =====================================
// GLOBAL INSTANCE
// =====================================

var globalListHygiene *ListHygieneManager

// InitListHygiene initializes the global list hygiene manager
func InitListHygiene(st *store.Store) {
	globalListHygiene = NewListHygieneManager(st)
}

// GetListHygiene returns the global list hygiene manager
func GetListHygiene() *ListHygieneManager {
	if globalListHygiene == nil {
		globalListHygiene = NewListHygieneManager(nil)
	}
	return globalListHygiene
}

// GetEngagementScore calculates engagement score for a contact
func GetEngagementScore(contact *models.ContactV2) *EngagementScore {
	return GetListHygiene().CalculateEngagementScore(contact)
}
