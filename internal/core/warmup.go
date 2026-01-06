package core

import (
	"fmt"
	"log"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// Schedules mapping: Day Number (1-based) -> KumoMTA Rate String
// Day 1 starts low. If a sender is on Day 5, they get that rate.
var WarmupSchedules = map[string][]string{
	"conservative": {
		"10/hr", "20/hr", "40/hr", "80/hr", "150/hr", "300/hr", "600/hr", "1000/hr", "2000/hr", "4000/hr",
	},
	"standard": {
		"25/hr", "50/hr", "100/hr", "200/hr", "400/hr", "800/hr", "1600/hr", "3200/hr", "6400/hr", "12000/hr",
	},
	"aggressive": {
		"50/hr", "100/hr", "250/hr", "500/hr", "1000/hr", "2500/hr", "5000/hr", "10000/hr", "20000/hr",
	},
}

// WarmupConfig holds thresholds for the "brake check"
type WarmupConfig struct {
	MaxBounceRate   float64 // Default: 3% - stop advancement if exceeded
	MaxDeferralRate float64 // Default: 10% - stop advancement if exceeded (soft errors)
	RollbackOnError bool    // If true, roll back warmup day on very poor performance
}

// DefaultWarmupConfig returns safe defaults based on industry standards
func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{
		MaxBounceRate:   3.0,   // 3% bounce = hold
		MaxDeferralRate: 10.0,  // 10% deferral = hold
		RollbackOnError: false, // Conservative: don't rollback, just hold
	}
}

// ProcessDailyWarmup checks all senders and advances their schedule if 24h passed
// IMPORTANT: Now includes "brake check" - won't advance if bounce/error rates are too high
func ProcessDailyWarmup(st *store.Store) error {
	return ProcessDailyWarmupWithConfig(st, DefaultWarmupConfig())
}

// ProcessDailyWarmupWithConfig allows custom thresholds
func ProcessDailyWarmupWithConfig(st *store.Store, cfg WarmupConfig) error {
	domains, err := st.ListDomains()
	if err != nil {
		return err
	}

	updatesMade := false

	for _, d := range domains {
		// Get yesterday's stats from KumoMTA logs for this domain
		domainStats, err := GetDomainStatsFromLogs(d.Name, 1)
		var yesterdayStats *DayStats
		if err == nil && len(domainStats) > 0 {
			yesterdayStats = &domainStats[0]
		}

		for _, s := range d.Senders {
			if !s.WarmupEnabled {
				continue
			}

			// Initialize timestamp if missing (first run)
			if s.WarmupLastUpdate.IsZero() {
				s.WarmupLastUpdate = time.Now()
				// Save initial state without advancing day
				if err := st.UpdateSender(&s); err != nil {
					log.Printf("[Warmup] DB Error init %s: %v", s.Email, err)
				}
				continue
			}

			// Check if 24 hours have passed since last increment
			if time.Since(s.WarmupLastUpdate) < 24*time.Hour {
				continue
			}

			// Get the plan (default to standard if missing)
			planName := s.WarmupPlan
			if planName == "" {
				planName = "standard"
			}

			plan, exists := WarmupSchedules[planName]
			if !exists {
				plan = WarmupSchedules["standard"]
			}

			// ===== BRAKE CHECK: Performance-based warmup =====
			// Don't advance if yesterday's performance was poor
			shouldAdvance := true
			brakeReason := ""

			if yesterdayStats != nil && yesterdayStats.Sent > 0 {
				// Calculate rates from KumoMTA log stats
				bounceRate := float64(yesterdayStats.Bounced) / float64(yesterdayStats.Sent) * 100
				deferralRate := float64(yesterdayStats.Deferred) / float64(yesterdayStats.Sent) * 100

				log.Printf("[Warmup] Stats for %s: Sent=%d, Bounced=%d (%.1f%%), Deferred=%d (%.1f%%)",
					d.Name, yesterdayStats.Sent, yesterdayStats.Bounced, bounceRate,
					yesterdayStats.Deferred, deferralRate)

				if bounceRate > cfg.MaxBounceRate {
					shouldAdvance = false
					brakeReason = fmt.Sprintf("bounce rate %.1f%% exceeds limit %.1f%%", bounceRate, cfg.MaxBounceRate)
				}
				if deferralRate > cfg.MaxDeferralRate {
					shouldAdvance = false
					brakeReason = fmt.Sprintf("deferral rate %.1f%% exceeds limit %.1f%%", deferralRate, cfg.MaxDeferralRate)
				}
			}

			// Check if we should Advance, Hold, or Rollback
			if s.WarmupDay >= len(plan) {
				log.Printf("[Warmup] ✓ Sender %s completed %s plan. Disabling warmup limits.", s.Email, planName)
				s.WarmupEnabled = false // Done! They run unlimited now.
			} else if !shouldAdvance {
				// BRAKE ENGAGED: Hold at current day due to poor performance
				log.Printf("[Warmup] ⚠️ HOLDING %s at Day %d - %s", s.Email, s.WarmupDay, brakeReason)

				// Optional: Rollback for severe issues (bounce rate > 10%)
				if cfg.RollbackOnError && s.WarmupDay > 1 && yesterdayStats != nil {
					bounceRate := float64(yesterdayStats.Bounced) / float64(yesterdayStats.Sent) * 100
					if bounceRate > 10.0 {
						s.WarmupDay--
						log.Printf("[Warmup] ⚠️ ROLLED BACK %s to Day %d (severe bounce rate)", s.Email, s.WarmupDay)
					}
				}

				// Still update timestamp so we check again in 24h
				s.WarmupLastUpdate = time.Now()
			} else {
				// Performance is good, advance schedule
				s.WarmupDay++
				s.WarmupLastUpdate = time.Now()
				log.Printf("[Warmup] ✓ Bumped %s to Day %d (%s)", s.Email, s.WarmupDay, plan[s.WarmupDay-1])
			}

			// Save progress
			if err := st.UpdateSender(&s); err != nil {
				log.Printf("[Warmup] DB Error saving %s: %v", s.Email, err)
			}
			updatesMade = true
		}
	}

	// If any rates changed, we MUST regenerate Kumo config to apply them
	if updatesMade {
		log.Println("[Warmup] Applying new rate limits to KumoMTA...")
		snap, err := LoadSnapshot(st)
		if err == nil {
			if _, err := ApplyKumoConfig(snap); err != nil {
				return fmt.Errorf("failed to apply warmup config: %v", err)
			}
		}
	}

	return nil
}

// GetSenderRate returns the current KumoMTA rate string (e.g. "100/hr") for a sender
// This is called by configgen.go
func GetSenderRate(s models.Sender) string {
	if !s.WarmupEnabled {
		return "" // Empty means no limit
	}

	planName := s.WarmupPlan
	if planName == "" {
		planName = "standard"
	}

	plan, ok := WarmupSchedules[planName]
	if !ok {
		return ""
	}

	// Calculate array index (Day 1 = index 0)
	dayIndex := s.WarmupDay - 1

	// Safety bounds
	if dayIndex < 0 {
		dayIndex = 0
	}
	if dayIndex >= len(plan) {
		return ""
	} // Should have been disabled, but safe fallback

	return plan[dayIndex]
}
