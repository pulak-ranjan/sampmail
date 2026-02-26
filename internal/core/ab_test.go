package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/store"
	"gorm.io/gorm"
)

// =====================================
// A/B TESTING FOR CAMPAIGNS
// =====================================
// Provides split testing for email campaigns:
// - Multiple variants (A, B, C...)
// - Traffic splitting
// - Statistical significance calculation
// - Automatic winner selection

// ABTestStatus defines the status of an A/B test
type ABTestStatus string

const (
	ABTestStatusDraft     ABTestStatus = "draft"
	ABTestStatusRunning   ABTestStatus = "running"
	ABTestStatusPaused    ABTestStatus = "paused"
	ABTestStatusCompleted ABTestStatus = "completed"
)

// ABTestMetric defines the metric to optimize
type ABTestMetric string

const (
	ABTestMetricOpens       ABTestMetric = "opens"
	ABTestMetricClicks      ABTestMetric = "clicks"
	ABTestMetricConversions ABTestMetric = "conversions"
	ABTestMetricRevenue     ABTestMetric = "revenue"
)

// ABVariant represents a single variant in an A/B test
type ABVariant struct {
	ID           uint                   `json:"id" gorm:"primaryKey"`
	ABTestID     uint                   `json:"ab_test_id" gorm:"index"`
	Name         string                 `json:"name"`                           // "A", "B", "Control", etc.
	Subject      string                 `json:"subject"`                        // Email subject line
	FromName     string                 `json:"from_name"`                      // Sender display name
	TemplateID   *uint                  `json:"template_id"`                    // Optional different template
	ContentDelta map[string]interface{} `json:"content_delta" gorm:"type:text"` // Content changes

	// Stats
	Sent         int     `json:"sent"`
	Opens        int     `json:"opens"`
	Clicks       int     `json:"clicks"`
	Unsubscribes int     `json:"unsubscribes"`
	Revenue      float64 `json:"revenue"`

	// Calculated metrics
	OpenRate       float64 `json:"open_rate" gorm:"-"`
	ClickRate      float64 `json:"click_rate" gorm:"-"`
	UnsubRate      float64 `json:"unsub_rate" gorm:"-"`
	ConversionRate float64 `json:"conversion_rate" gorm:"-"`

	CreatedAt time.Time `json:"created_at"`
}

// ABTest represents an A/B test for a campaign
type ABTest struct {
	ID             uint         `json:"id" gorm:"primaryKey"`
	OrganizationID uint         `json:"organization_id" gorm:"index"`
	CampaignID     uint         `json:"campaign_id" gorm:"index"`
	Name           string       `json:"name"`
	Status         ABTestStatus `json:"status"`

	// Configuration
	Variants        []ABVariant  `json:"variants" gorm:"foreignKey:ABTestID"`
	TrafficSplit    []int        `json:"traffic_split" gorm:"type:text"` // e.g., [50, 50] or [33, 33, 34]
	WinnerMetric    ABTestMetric `json:"winner_metric"`
	ConfidenceLevel float64      `json:"confidence_level"` // 0.90, 0.95, 0.99
	MinSampleSize   int          `json:"min_sample_size"`  // Minimum per variant

	// Timing
	TestDuration time.Duration `json:"test_duration"`
	StartedAt    *time.Time    `json:"started_at"`
	EndsAt       *time.Time    `json:"ends_at"`
	CompletedAt  *time.Time    `json:"completed_at"`

	// Results
	WinnerID       *uint   `json:"winner_id"`
	WinnerSelected bool    `json:"winner_selected"`
	StatisticalSig float64 `json:"statistical_significance"` // p-value

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ABTestResult represents the result of an A/B test
type ABTestResult struct {
	TestID          uint              `json:"test_id"`
	Status          ABTestStatus      `json:"status"`
	WinnerID        *uint             `json:"winner_id"`
	WinnerName      string            `json:"winner_name"`
	StatisticalSig  float64           `json:"statistical_significance"`
	ConfidenceLevel float64           `json:"confidence_level"`
	IsSignificant   bool              `json:"is_significant"`
	VariantResults  []ABVariantResult `json:"variant_results"`
	Recommendation  string            `json:"recommendation"`
	TotalSent       int               `json:"total_sent"`
	TestDuration    time.Duration     `json:"test_duration"`
}

// ABVariantResult represents results for a single variant
type ABVariantResult struct {
	VariantID       uint    `json:"variant_id"`
	Name            string  `json:"name"`
	Sent            int     `json:"sent"`
	Opens           int     `json:"opens"`
	Clicks          int     `json:"clicks"`
	OpenRate        float64 `json:"open_rate"`
	ClickRate       float64 `json:"click_rate"`
	ClickToOpenRate float64 `json:"ctor"` // Clicks / Opens
	Confidence      float64 `json:"confidence"`
	IsWinner        bool    `json:"is_winner"`
	Lift            float64 `json:"lift"` // Improvement over control
}

// ABTestManager manages A/B tests
type ABTestManager struct {
	store       *store.Store
	activeTests map[uint]*ABTest // test_id -> test
	mu          sync.RWMutex
}

var globalABTestManager *ABTestManager

// InitABTestManager initializes the global A/B test manager
func InitABTestManager(st *store.Store) *ABTestManager {
	globalABTestManager = &ABTestManager{
		store:       st,
		activeTests: make(map[uint]*ABTest),
	}
	return globalABTestManager
}

// GetABTestManager returns the global A/B test manager
func GetABTestManager() *ABTestManager {
	return globalABTestManager
}

// CreateTest creates a new A/B test
func (m *ABTestManager) CreateTest(ctx context.Context, test *ABTest) error {
	if len(test.Variants) < 2 {
		return fmt.Errorf("A/B test requires at least 2 variants")
	}

	if len(test.TrafficSplit) != len(test.Variants) {
		return fmt.Errorf("traffic split must match number of variants")
	}

	// Validate traffic split sums to 100
	total := 0
	for _, split := range test.TrafficSplit {
		total += split
	}
	if total != 100 {
		return fmt.Errorf("traffic split must sum to 100, got %d", total)
	}

	// Set defaults
	if test.Status == "" {
		test.Status = ABTestStatusDraft
	}
	if test.WinnerMetric == "" {
		test.WinnerMetric = ABTestMetricOpens
	}
	if test.ConfidenceLevel == 0 {
		test.ConfidenceLevel = 0.95
	}
	if test.MinSampleSize == 0 {
		test.MinSampleSize = 1000
	}

	test.CreatedAt = time.Now()
	test.UpdatedAt = time.Now()

	// Save to database
	if err := m.store.DB.Create(test).Error; err != nil {
		return fmt.Errorf("failed to create A/B test: %w", err)
	}

	logger.WithComponent("ab_test").Info("created A/B test",
		"test_id", test.ID,
		"campaign_id", test.CampaignID,
		"variants", len(test.Variants))

	return nil
}

// StartTest starts an A/B test
func (m *ABTestManager) StartTest(ctx context.Context, testID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var test ABTest
	if err := m.store.DB.Preload("Variants").First(&test, testID).Error; err != nil {
		return fmt.Errorf("test not found: %w", err)
	}

	if test.Status != ABTestStatusDraft {
		return fmt.Errorf("test is not in draft status")
	}

	now := time.Now()
	test.Status = ABTestStatusRunning
	test.StartedAt = &now

	if test.TestDuration > 0 {
		endTime := now.Add(test.TestDuration)
		test.EndsAt = &endTime
	}

	if err := m.store.DB.Save(&test).Error; err != nil {
		return err
	}

	m.activeTests[testID] = &test

	logger.WithComponent("ab_test").Info("started A/B test",
		"test_id", testID,
		"duration", test.TestDuration)

	return nil
}

// GetVariantForRecipient determines which variant a recipient should receive
func (m *ABTestManager) GetVariantForRecipient(testID uint, recipientID uint) (*ABVariant, error) {
	m.mu.RLock()
	test, exists := m.activeTests[testID]
	m.mu.RUnlock()

	if !exists {
		// Load from database
		var abTest ABTest
		if err := m.store.DB.Preload("Variants").First(&abTest, testID).Error; err != nil {
			return nil, fmt.Errorf("test not found")
		}
		if abTest.Status != ABTestStatusRunning {
			return nil, fmt.Errorf("test is not running")
		}
		test = &abTest
	}

	// Deterministic assignment based on recipient ID
	// This ensures the same recipient always gets the same variant
	recipientHash := int(recipientID % 100)
	cumulative := 0
	for i, split := range test.TrafficSplit {
		cumulative += split
		if recipientHash < cumulative {
			if i < len(test.Variants) {
				return &test.Variants[i], nil
			}
		}
	}

	// Fallback to first variant
	return &test.Variants[0], nil
}

// RecordMetric records a metric for a variant
func (m *ABTestManager) RecordMetric(ctx context.Context, testID, variantID uint, metric ABTestMetric, value float64) error {
	return m.store.DB.Transaction(func(tx *gorm.DB) error {
		var variant ABVariant
		if err := tx.First(&variant, variantID).Error; err != nil {
			return err
		}

		switch metric {
		case ABTestMetricOpens:
			variant.Opens++
		case ABTestMetricClicks:
			variant.Clicks++
		case ABTestMetricRevenue:
			variant.Revenue += value
		}

		return tx.Save(&variant).Error
	})
}

// RecordSent records that a variant was sent
func (m *ABTestManager) RecordSent(ctx context.Context, variantID uint) error {
	return m.store.DB.Model(&ABVariant{}).
		Where("id = ?", variantID).
		UpdateColumn("sent", gorm.Expr("sent + 1")).Error
}

// CalculateResults calculates the results of an A/B test
func (m *ABTestManager) CalculateResults(ctx context.Context, testID uint) (*ABTestResult, error) {
	var test ABTest
	if err := m.store.DB.Preload("Variants").First(&test, testID).Error; err != nil {
		return nil, fmt.Errorf("test not found: %w", err)
	}

	result := &ABTestResult{
		TestID:          testID,
		Status:          test.Status,
		ConfidenceLevel: test.ConfidenceLevel,
		VariantResults:  make([]ABVariantResult, 0, len(test.Variants)),
	}

	// Calculate metrics for each variant
	var controlRate float64
	for i, variant := range test.Variants {
		vr := ABVariantResult{
			VariantID: variant.ID,
			Name:      variant.Name,
			Sent:      variant.Sent,
			Opens:     variant.Opens,
			Clicks:    variant.Clicks,
		}

		if variant.Sent > 0 {
			vr.OpenRate = float64(variant.Opens) / float64(variant.Sent)
			vr.ClickRate = float64(variant.Clicks) / float64(variant.Sent)
		}

		if variant.Opens > 0 {
			vr.ClickToOpenRate = float64(variant.Clicks) / float64(variant.Opens)
		}

		// First variant is control
		if i == 0 {
			controlRate = vr.OpenRate
		} else if controlRate > 0 {
			vr.Lift = (vr.OpenRate - controlRate) / controlRate * 100
		}

		result.VariantResults = append(result.VariantResults, vr)
		result.TotalSent += variant.Sent
	}

	// Calculate statistical significance using Chi-Square test
	if len(test.Variants) >= 2 {
		pValue := m.calculatePValue(test.Variants[0], test.Variants[1], test.WinnerMetric)
		result.StatisticalSig = pValue
		result.IsSignificant = pValue < (1 - test.ConfidenceLevel)
	}

	// Determine winner
	if result.IsSignificant || test.Status == ABTestStatusCompleted {
		winner := m.determineWinner(test.Variants, test.WinnerMetric)
		if winner != nil {
			result.WinnerID = &winner.ID
			result.WinnerName = winner.Name
			for i := range result.VariantResults {
				if result.VariantResults[i].VariantID == winner.ID {
					result.VariantResults[i].IsWinner = true
				}
			}
		}
	}

	// Generate recommendation
	result.Recommendation = m.generateRecommendation(result)

	return result, nil
}

// calculatePValue calculates the p-value using Chi-Square test
func (m *ABTestManager) calculatePValue(v1, v2 ABVariant, metric ABTestMetric) float64 {
	var successes1, successes2 int

	switch metric {
	case ABTestMetricOpens:
		successes1 = v1.Opens
		successes2 = v2.Opens
	case ABTestMetricClicks:
		successes1 = v1.Clicks
		successes2 = v2.Clicks
	default:
		successes1 = v1.Opens
		successes2 = v2.Opens
	}

	// Simple Chi-Square calculation
	// For production, use a proper statistics library
	total1 := v1.Sent
	total2 := v2.Sent

	if total1 == 0 || total2 == 0 {
		return 1.0
	}

	p1 := float64(successes1) / float64(total1)
	p2 := float64(successes2) / float64(total2)
	pooled := float64(successes1+successes2) / float64(total1+total2)

	if pooled == 0 || pooled == 1 {
		return 1.0
	}

	// Z-score calculation
	se := math.Sqrt(pooled * (1 - pooled) * (1/float64(total1) + 1/float64(total2)))
	if se == 0 {
		return 1.0
	}

	z := (p1 - p2) / se

	// Convert z-score to p-value (two-tailed)
	// Using approximation for normal distribution
	pValue := 2 * (1 - normalCDF(math.Abs(z)))

	return pValue
}

// normalCDF approximates the cumulative distribution function for normal distribution
func normalCDF(x float64) float64 {
	// Approximation using error function
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	sign := 1.0
	if x < 0 {
		sign = -1.0
	}
	x = math.Abs(x) / math.Sqrt(2)

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return 0.5 * (1.0 + sign*y)
}

// determineWinner determines the winning variant
func (m *ABTestManager) determineWinner(variants []ABVariant, metric ABTestMetric) *ABVariant {
	if len(variants) == 0 {
		return nil
	}

	var winner *ABVariant
	var bestRate float64

	for i := range variants {
		v := &variants[i]
		if v.Sent == 0 {
			continue
		}

		var rate float64
		switch metric {
		case ABTestMetricOpens:
			rate = float64(v.Opens) / float64(v.Sent)
		case ABTestMetricClicks:
			rate = float64(v.Clicks) / float64(v.Sent)
		case ABTestMetricConversions:
			rate = float64(v.Clicks) / float64(v.Opens) // CTR as proxy
		default:
			rate = float64(v.Opens) / float64(v.Sent)
		}

		if rate > bestRate {
			bestRate = rate
			winner = v
		}
	}

	return winner
}

// generateRecommendation generates a recommendation based on results
func (m *ABTestManager) generateRecommendation(result *ABTestResult) string {
	if result.Status == ABTestStatusRunning {
		if result.IsSignificant {
			return fmt.Sprintf("Statistical significance reached! Winner: %s with %.1f%% confidence. Consider ending the test early.",
				result.WinnerName, (1-result.StatisticalSig)*100)
		}
		return "Test is still running. Continue collecting data for statistical significance."
	}

	if result.IsSignificant {
		return fmt.Sprintf("Test complete. Winner: %s. Recommend using this variant for future campaigns.",
			result.WinnerName)
	}

	return "No statistically significant winner found. Consider running the test longer or with more recipients."
}

// SelectWinner manually selects a winner for a test
func (m *ABTestManager) SelectWinner(ctx context.Context, testID, variantID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var test ABTest
	if err := m.store.DB.First(&test, testID).Error; err != nil {
		return fmt.Errorf("test not found")
	}

	now := time.Now()
	test.WinnerID = &variantID
	test.WinnerSelected = true
	test.Status = ABTestStatusCompleted
	test.CompletedAt = &now

	if err := m.store.DB.Save(&test).Error; err != nil {
		return err
	}

	delete(m.activeTests, testID)

	logger.WithComponent("ab_test").Info("selected winner for A/B test",
		"test_id", testID,
		"winner_id", variantID)

	return nil
}

// CheckAndAutoSelectWinner checks if any tests should auto-select a winner
func (m *ABTestManager) CheckAndAutoSelectWinner(ctx context.Context) error {
	m.mu.RLock()
	tests := make([]*ABTest, 0, len(m.activeTests))
	for _, test := range m.activeTests {
		tests = append(tests, test)
	}
	m.mu.RUnlock()

	for _, test := range tests {
		// Check if test has ended
		if test.EndsAt != nil && time.Now().After(*test.EndsAt) {
			result, err := m.CalculateResults(ctx, test.ID)
			if err != nil {
				continue
			}

			// Auto-select winner if significant
			if result.IsSignificant && result.WinnerID != nil {
				m.SelectWinner(ctx, test.ID, *result.WinnerID)
			}
		}
	}

	return nil
}

// GetTest retrieves an A/B test by ID
func (m *ABTestManager) GetTest(ctx context.Context, testID uint) (*ABTest, error) {
	var test ABTest
	if err := m.store.DB.Preload("Variants").First(&test, testID).Error; err != nil {
		return nil, err
	}
	return &test, nil
}

// ListTests lists all A/B tests for an organization
func (m *ABTestManager) ListTests(ctx context.Context, orgID uint) ([]ABTest, error) {
	var tests []ABTest
	query := m.store.DB.Preload("Variants")
	if orgID > 0 {
		query = query.Where("organization_id = ?", orgID)
	}
	if err := query.Order("created_at DESC").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

// DeleteTest deletes an A/B test
func (m *ABTestManager) DeleteTest(ctx context.Context, testID uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.activeTests, testID)

	return m.store.DB.Transaction(func(tx *gorm.DB) error {
		// Delete variants first
		if err := tx.Where("ab_test_id = ?", testID).Delete(&ABVariant{}).Error; err != nil {
			return err
		}
		// Delete test
		return tx.Delete(&ABTest{}, testID).Error
	})
}
