package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

type AnalyticsHandlerV2 struct {
	Store *store.Store
}

func NewAnalyticsHandlerV2(st *store.Store) *AnalyticsHandlerV2 {
	return &AnalyticsHandlerV2{Store: st}
}

// GET /api/v2/analytics/dashboard
func (h *AnalyticsHandlerV2) DashboardTenant(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing organization context"})
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	since := time.Now().AddDate(0, 0, -days)

	var totalSubscribers int64
	h.Store.DB.Model(&models.ContactV2{}).Where("organization_id = ?", org.ID).Count(&totalSubscribers)

	var totalCampaigns int64
	h.Store.DB.Model(&models.CampaignV2{}).Where("organization_id = ?", org.ID).Count(&totalCampaigns)

	var totalSent int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.sent_at IS NOT NULL", org.ID).
		Count(&totalSent)

	var totalDomains int64
	h.Store.DB.Model(&models.Domain{}).Count(&totalDomains)

	var recentSent int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.sent_at >= ?", org.ID, since).
		Count(&recentSent)

	var recentUniqueOpens int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.first_open_at >= ?", org.ID, since).
		Count(&recentUniqueOpens)

	var recentUniqueClicks int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.first_click_at >= ?", org.ID, since).
		Count(&recentUniqueClicks)

	var recentBounces int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.sent_at >= ? AND cr.status = ?", org.ID, since, "bounced").
		Count(&recentBounces)

	openRate := float64(0)
	clickRate := float64(0)
	bounceRate := float64(0)

	if recentSent > 0 {
		openRate = float64(recentUniqueOpens) / float64(recentSent) * 100
		clickRate = float64(recentUniqueClicks) / float64(recentSent) * 100
		bounceRate = float64(recentBounces) / float64(recentSent) * 100
	}

	deliverabilityScore := 100.0
	if bounceRate > 0 {
		deliverabilityScore -= bounceRate * 2
	}
	if openRate < 10 {
		deliverabilityScore -= 10
	}
	if deliverabilityScore < 0 {
		deliverabilityScore = 0
	}

	dailyStats := h.getDailyStatsTenant(org.ID, days)

	var recentCampaigns []models.CampaignV2
	h.Store.DB.
		Where("organization_id = ?", org.ID).
		Order("created_at DESC").
		Limit(5).
		Find(&recentCampaigns)

	recentCampaignRows := make([]map[string]interface{}, 0, len(recentCampaigns))
	for _, c := range recentCampaigns {
		recentCampaignRows = append(recentCampaignRows, map[string]interface{}{
			"id":         c.ID,
			"name":       c.Name,
			"subject":    c.Subject,
			"status":     c.Status,
			"sent_count": c.TotalSent,
			"created_at": c.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"overview": map[string]interface{}{
			"total_subscribers": totalSubscribers,
			"total_campaigns":   totalCampaigns,
			"total_sent":        totalSent,
			"total_domains":     totalDomains,
		},
		"period": map[string]interface{}{
			"days":         days,
			"sent":         recentSent,
			"opens":        recentUniqueOpens,
			"clicks":       recentUniqueClicks,
			"bounces":      recentBounces,
			"unsubscribes": int64(0),
		},
		"rates": map[string]interface{}{
			"open_rate":   openRate,
			"click_rate":  clickRate,
			"bounce_rate": bounceRate,
		},
		"deliverability_score": deliverabilityScore,
		"daily_stats":          dailyStats,
		"recent_campaigns":     recentCampaignRows,
	})
}

func (h *AnalyticsHandlerV2) getDailyStatsTenant(orgID uint, days int) []map[string]interface{} {
	stats := make([]map[string]interface{}, 0)

	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var sent int64
		h.Store.DB.
			Table("campaign_recipient_v2s cr").
			Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
			Where("c.organization_id = ? AND cr.sent_at >= ? AND cr.sent_at < ?", orgID, startOfDay, endOfDay).
			Count(&sent)

		var opens int64
		h.Store.DB.
			Table("campaign_recipient_v2s cr").
			Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
			Where("c.organization_id = ? AND cr.first_open_at >= ? AND cr.first_open_at < ?", orgID, startOfDay, endOfDay).
			Count(&opens)

		var clicks int64
		h.Store.DB.
			Table("campaign_recipient_v2s cr").
			Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
			Where("c.organization_id = ? AND cr.first_click_at >= ? AND cr.first_click_at < ?", orgID, startOfDay, endOfDay).
			Count(&clicks)

		var bounces int64
		h.Store.DB.
			Table("campaign_recipient_v2s cr").
			Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
			Where("c.organization_id = ? AND cr.sent_at >= ? AND cr.sent_at < ? AND cr.status = ?", orgID, startOfDay, endOfDay, "bounced").
			Count(&bounces)

		stats = append(stats, map[string]interface{}{
			"date":    startOfDay.Format("2006-01-02"),
			"sent":    sent,
			"opens":   opens,
			"clicks":  clicks,
			"bounces": bounces,
		})
	}

	return stats
}

// GET /api/v2/analytics/deliverability
func (h *AnalyticsHandlerV2) DeliverabilityTenant(w http.ResponseWriter, r *http.Request) {
	org := getOrganizationFromContext(r.Context())
	if org == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing organization context"})
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	since := time.Now().AddDate(0, 0, -days)

	var totalSent int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.sent_at >= ?", org.ID, since).
		Count(&totalSent)

	var bounces int64
	h.Store.DB.
		Table("campaign_recipient_v2s cr").
		Joins("JOIN campaign_v2s c ON c.id = cr.campaign_id").
		Where("c.organization_id = ? AND cr.sent_at >= ? AND cr.status = ?", org.ID, since, "bounced").
		Count(&bounces)

	hardBounceRate := float64(0)
	if totalSent > 0 {
		hardBounceRate = float64(bounces) / float64(totalSent) * 100
	}

	healthStatus := "good"
	healthIssues := make([]string, 0)

	if hardBounceRate > 2 {
		healthStatus = "critical"
		healthIssues = append(healthIssues, "Hard bounce rate is too high (>2%)")
	} else if hardBounceRate > 1 {
		healthStatus = "warning"
		healthIssues = append(healthIssues, "Hard bounce rate is elevated (>1%)")
	}

	recommendations := make([]string, 0)
	if hardBounceRate > 1 {
		recommendations = append(recommendations, "Clean your email list to remove invalid addresses")
		recommendations = append(recommendations, "Enable email verification before sending")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"period": map[string]interface{}{
			"days":       days,
			"total_sent": totalSent,
		},
		"bounces": map[string]interface{}{
			"hard":       bounces,
			"soft":       int64(0),
			"complaints": int64(0),
		},
		"rates": map[string]interface{}{
			"hard_bounce_rate": hardBounceRate,
			"soft_bounce_rate": float64(0),
			"complaint_rate":   float64(0),
		},
		"suppression": map[string]interface{}{
			"total": int64(0),
		},
		"health": map[string]interface{}{
			"status": healthStatus,
			"issues": healthIssues,
		},
		"recommendations": recommendations,
	})
}

// GET /api/analytics/dashboard
func (h *AnalyticsHandlerV2) Dashboard(w http.ResponseWriter, r *http.Request) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	since := time.Now().AddDate(0, 0, -days)

	// Total counts
	var totalSubscribers int64
	h.Store.DB.Model(&models.Contact{}).Count(&totalSubscribers)

	var totalCampaigns int64
	h.Store.DB.Model(&models.Campaign{}).Count(&totalCampaigns)

	var totalSent int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("status = ?", "sent").Count(&totalSent)

	var totalDomains int64
	h.Store.DB.Model(&models.Domain{}).Count(&totalDomains)

	// Recent stats
	var recentSent int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("sent_at >= ? AND status = ?", since, "sent").Count(&recentSent)

	var recentOpens int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("event_type = ? AND created_at >= ?", "open", since).Count(&recentOpens)

	var recentClicks int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("event_type = ? AND created_at >= ?", "click", since).Count(&recentClicks)

	var recentBounces int64
	h.Store.DB.Model(&models.BounceEvent{}).Where("processed_at >= ?", since).Count(&recentBounces)

	var recentUnsubscribes int64
	h.Store.DB.Model(&models.Suppression{}).Where("reason = ? AND created_at >= ?", "unsubscribe", since).Count(&recentUnsubscribes)

	// Calculate rates
	openRate := float64(0)
	clickRate := float64(0)
	bounceRate := float64(0)

	if recentSent > 0 {
		openRate = float64(recentOpens) / float64(recentSent) * 100
		clickRate = float64(recentClicks) / float64(recentSent) * 100
		bounceRate = float64(recentBounces) / float64(recentSent) * 100
	}

	// Deliverability score (simple calculation)
	deliverabilityScore := 100.0
	if bounceRate > 0 {
		deliverabilityScore -= bounceRate * 2
	}
	if openRate < 10 {
		deliverabilityScore -= 10
	}
	if deliverabilityScore < 0 {
		deliverabilityScore = 0
	}

	// Daily stats for chart
	dailyStats := h.getDailyStats(days)

	// Recent campaigns
	var recentCampaigns []models.Campaign
	h.Store.DB.Order("created_at DESC").Limit(5).Find(&recentCampaigns)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"overview": map[string]interface{}{
			"total_subscribers": totalSubscribers,
			"total_campaigns":   totalCampaigns,
			"total_sent":        totalSent,
			"total_domains":     totalDomains,
		},
		"period": map[string]interface{}{
			"days":         days,
			"sent":         recentSent,
			"opens":        recentOpens,
			"clicks":       recentClicks,
			"bounces":      recentBounces,
			"unsubscribes": recentUnsubscribes,
		},
		"rates": map[string]interface{}{
			"open_rate":   openRate,
			"click_rate":  clickRate,
			"bounce_rate": bounceRate,
		},
		"deliverability_score": deliverabilityScore,
		"daily_stats":          dailyStats,
		"recent_campaigns":     recentCampaigns,
	})
}

// getDailyStats returns daily aggregated stats
func (h *AnalyticsHandlerV2) getDailyStats(days int) []map[string]interface{} {
	stats := make([]map[string]interface{}, 0)

	for i := days - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var sent int64
		h.Store.DB.Model(&models.CampaignRecipient{}).
			Where("sent_at >= ? AND sent_at < ? AND status = ?", startOfDay, endOfDay, "sent").
			Count(&sent)

		var opens int64
		h.Store.DB.Model(&models.TrackingEvent{}).
			Where("created_at >= ? AND created_at < ? AND event_type = ?", startOfDay, endOfDay, "open").
			Count(&opens)

		var clicks int64
		h.Store.DB.Model(&models.TrackingEvent{}).
			Where("created_at >= ? AND created_at < ? AND event_type = ?", startOfDay, endOfDay, "click").
			Count(&clicks)

		var bounces int64
		h.Store.DB.Model(&models.BounceEvent{}).
			Where("processed_at >= ? AND processed_at < ?", startOfDay, endOfDay).
			Count(&bounces)

		stats = append(stats, map[string]interface{}{
			"date":    startOfDay.Format("2006-01-02"),
			"sent":    sent,
			"opens":   opens,
			"clicks":  clicks,
			"bounces": bounces,
		})
	}

	return stats
}

// GET /api/analytics/campaigns/:id
func (h *AnalyticsHandlerV2) Campaign(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var campaign models.Campaign
	if err := h.Store.DB.First(&campaign, id).Error; err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "campaign not found"})
		return
	}

	// Get recipient counts by status
	var total int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ?", id).Count(&total)

	var sent int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ? AND status = ?", id, "sent").Count(&sent)

	var failed int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ? AND status = ?", id, "failed").Count(&failed)

	var skipped int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("campaign_id = ? AND status = ?", id, "skipped").Count(&skipped)

	// Get tracking stats
	var opens int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("campaign_id = ? AND event_type = ?", id, "open").Count(&opens)

	var uniqueOpens int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("campaign_id = ? AND event_type = ?", id, "open").Distinct("recipient_id").Count(&uniqueOpens)

	var clicks int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("campaign_id = ? AND event_type = ?", id, "click").Count(&clicks)

	var uniqueClicks int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("campaign_id = ? AND event_type = ?", id, "click").Distinct("recipient_id").Count(&uniqueClicks)

	// Get bounces for this campaign
	var bounces int64
	h.Store.DB.Model(&models.BounceEvent{}).Where("campaign_id = ?", id).Count(&bounces)

	// Get unsubscribes
	var unsubscribes int64
	h.Store.DB.Model(&models.Suppression{}).Where("source LIKE ?", "campaign:"+strconv.Itoa(id)+"%").Count(&unsubscribes)

	// Calculate rates
	openRate := float64(0)
	clickRate := float64(0)
	ctr := float64(0)

	if sent > 0 {
		openRate = float64(uniqueOpens) / float64(sent) * 100
		clickRate = float64(uniqueClicks) / float64(sent) * 100
	}
	if uniqueOpens > 0 {
		ctr = float64(uniqueClicks) / float64(uniqueOpens) * 100
	}

	// Get hourly stats
	hourlyStats := h.getHourlyCampaignStats(id)

	// Get top clicked links
	topLinks := h.getTopClickedLinks(id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"campaign": campaign,
		"delivery": map[string]interface{}{
			"total":   total,
			"sent":    sent,
			"failed":  failed,
			"skipped": skipped,
		},
		"engagement": map[string]interface{}{
			"opens":         opens,
			"unique_opens":  uniqueOpens,
			"clicks":        clicks,
			"unique_clicks": uniqueClicks,
			"bounces":       bounces,
			"unsubscribes":  unsubscribes,
		},
		"rates": map[string]interface{}{
			"open_rate":  openRate,
			"click_rate": clickRate,
			"ctr":        ctr,
		},
		"hourly_stats": hourlyStats,
		"top_links":    topLinks,
	})
}

func (h *AnalyticsHandlerV2) getHourlyCampaignStats(campaignID int) []map[string]interface{} {
	stats := make([]map[string]interface{}, 0)

	// Get campaign start time
	var campaign models.Campaign
	if err := h.Store.DB.First(&campaign, campaignID).Error; err != nil {
		return stats
	}

	startTime := campaign.CreatedAt
	if campaign.ScheduledAt != nil {
		startTime = *campaign.ScheduledAt
	}

	// Get 48 hours of data
	for i := 0; i < 48; i++ {
		hourStart := startTime.Add(time.Duration(i) * time.Hour)
		hourEnd := hourStart.Add(time.Hour)

		var opens int64
		h.Store.DB.Model(&models.TrackingEvent{}).
			Where("campaign_id = ? AND event_type = ? AND created_at >= ? AND created_at < ?",
				campaignID, "open", hourStart, hourEnd).
			Count(&opens)

		var clicks int64
		h.Store.DB.Model(&models.TrackingEvent{}).
			Where("campaign_id = ? AND event_type = ? AND created_at >= ? AND created_at < ?",
				campaignID, "click", hourStart, hourEnd).
			Count(&clicks)

		if opens > 0 || clicks > 0 {
			stats = append(stats, map[string]interface{}{
				"hour":   hourStart.Format("2006-01-02 15:00"),
				"opens":  opens,
				"clicks": clicks,
			})
		}
	}

	return stats
}

func (h *AnalyticsHandlerV2) getTopClickedLinks(campaignID int) []map[string]interface{} {
	type LinkStat struct {
		URL   string
		Count int64
	}

	var results []LinkStat
	h.Store.DB.Model(&models.TrackingEvent{}).
		Select("url, COUNT(*) as count").
		Where("campaign_id = ? AND event_type = ? AND url != ''", campaignID, "click").
		Group("url").
		Order("count DESC").
		Limit(10).
		Scan(&results)

	links := make([]map[string]interface{}, 0)
	for _, r := range results {
		links = append(links, map[string]interface{}{
			"url":    r.URL,
			"clicks": r.Count,
		})
	}

	return links
}

// GET /api/analytics/domains
// Refactored to use 4 bulk queries instead of O(N) per-domain queries.
func (h *AnalyticsHandlerV2) Domains(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	// 1. Load all domains
	var domains []models.Domain
	if err := h.Store.DB.Find(&domains).Error; err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load domains"})
		return
	}
	if len(domains) == 0 {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	domainIDs := make([]uint, len(domains))
	for i, d := range domains {
		domainIDs[i] = d.ID
	}

	// 2. Bulk-load all senders for these domains in one query
	var allSenders []models.Sender
	h.Store.DB.Where("domain_id IN ?", domainIDs).Find(&allSenders)

	// Map: domainID → []senderID
	domainToSenders := make(map[uint][]uint)
	for _, s := range allSenders {
		domainToSenders[s.DomainID] = append(domainToSenders[s.DomainID], s.ID)
	}

	// Collect all senderIDs
	var allSenderIDs []uint
	for _, ids := range domainToSenders {
		allSenderIDs = append(allSenderIDs, ids...)
	}

	// 3. Bulk-load all campaigns for these senders in one query
	var allCampaigns []models.Campaign
	if len(allSenderIDs) > 0 {
		h.Store.DB.Where("sender_id IN ?", allSenderIDs).Find(&allCampaigns)
	}

	// Map: senderID → domainID (via allSenders), campaignID → domainID
	senderToDomain := make(map[uint]uint)
	for _, s := range allSenders {
		senderToDomain[s.ID] = s.DomainID
	}
	campaignToDomain := make(map[uint]uint)
	var allCampaignIDs []uint
	for _, c := range allCampaigns {
		domainID := senderToDomain[c.SenderID]
		campaignToDomain[c.ID] = domainID
		allCampaignIDs = append(allCampaignIDs, c.ID)
	}

	since := time.Now().AddDate(0, 0, -days)

	// 4a. Bulk count sent recipients grouped by campaign_id
	type campaignCount struct {
		CampaignID uint
		Count      int64
	}
	var sentRows []campaignCount
	if len(allCampaignIDs) > 0 {
		h.Store.DB.Model(&models.CampaignRecipient{}).
			Select("campaign_id, count(*) as count").
			Where("campaign_id IN ? AND sent_at >= ?", allCampaignIDs, since).
			Group("campaign_id").Scan(&sentRows)
	}
	sentByCampaign := make(map[uint]int64)
	for _, row := range sentRows {
		sentByCampaign[row.CampaignID] = row.Count
	}

	// 4b. Bulk count opens grouped by campaign_id
	var openRows []campaignCount
	if len(allCampaignIDs) > 0 {
		h.Store.DB.Model(&models.TrackingEvent{}).
			Select("campaign_id, count(*) as count").
			Where("campaign_id IN ? AND event_type = ? AND created_at >= ?", allCampaignIDs, "open", since).
			Group("campaign_id").Scan(&openRows)
	}
	opensByCampaign := make(map[uint]int64)
	for _, row := range openRows {
		opensByCampaign[row.CampaignID] = row.Count
	}

	// 4c. Bulk count bounces + complaints grouped by campaign_id
	type bounceCount struct {
		CampaignID  uint
		BounceType  string
		Count       int64
	}
	var bounceRows []bounceCount
	if len(allCampaignIDs) > 0 {
		h.Store.DB.Model(&models.BounceEvent{}).
			Select("campaign_id, bounce_type, count(*) as count").
			Where("campaign_id IN ? AND processed_at >= ?", allCampaignIDs, since).
			Group("campaign_id, bounce_type").Scan(&bounceRows)
	}
	bouncesByCampaign := make(map[uint]int64)
	complaintsByCampaign := make(map[uint]int64)
	for _, row := range bounceRows {
		if row.BounceType == "complaint" {
			complaintsByCampaign[row.CampaignID] += row.Count
		} else {
			bouncesByCampaign[row.CampaignID] += row.Count
		}
	}

	// 5. Aggregate per-domain stats
	type domainAgg struct {
		sent       int64
		opens      int64
		bounces    int64
		complaints int64
	}
	domainStats := make(map[uint]*domainAgg)
	for _, d := range domains {
		domainStats[d.ID] = &domainAgg{}
	}
	for campaignID, domainID := range campaignToDomain {
		agg := domainStats[domainID]
		if agg == nil {
			continue
		}
		agg.sent += sentByCampaign[campaignID]
		agg.opens += opensByCampaign[campaignID]
		agg.bounces += bouncesByCampaign[campaignID]
		agg.complaints += complaintsByCampaign[campaignID]
	}

	// 6. Build response
	domainByID := make(map[uint]models.Domain, len(domains))
	for _, d := range domains {
		domainByID[d.ID] = d
	}

	results := make([]map[string]interface{}, 0, len(domains))
	for _, d := range domains {
		agg := domainStats[d.ID]
		score := 100
		if agg.sent > 0 {
			bounceRate := float64(agg.bounces) / float64(agg.sent) * 100
			complaintRate := float64(agg.complaints) / float64(agg.sent) * 100
			if bounceRate > 5 {
				score -= 30
			} else if bounceRate > 2 {
				score -= 15
			}
			if complaintRate > 0.1 {
				score -= 40
			} else if complaintRate > 0.05 {
				score -= 20
			}
		}
		if score < 0 {
			score = 0
		}
		results = append(results, map[string]interface{}{
			"domain":           d.Name,
			"domain_id":        d.ID,
			"sent":             agg.sent,
			"opens":            agg.opens,
			"bounces":          agg.bounces,
			"complaints":       agg.complaints,
			"reputation_score": score,
		})
	}

	writeJSON(w, http.StatusOK, results)
}

// GET /api/analytics/deliverability
func (h *AnalyticsHandlerV2) Deliverability(w http.ResponseWriter, r *http.Request) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		days, _ = strconv.Atoi(d)
	}

	since := time.Now().AddDate(0, 0, -days)

	var totalSent int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("sent_at >= ?", since).Count(&totalSent)

	var hardBounces int64
	h.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ? AND processed_at >= ?", "hard", since).Count(&hardBounces)

	var softBounces int64
	h.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ? AND processed_at >= ?", "soft", since).Count(&softBounces)

	var complaints int64
	h.Store.DB.Model(&models.BounceEvent{}).Where("bounce_type = ? AND processed_at >= ?", "complaint", since).Count(&complaints)

	var suppressionCount int64
	h.Store.DB.Model(&models.Suppression{}).Count(&suppressionCount)

	// Calculate rates
	hardBounceRate := float64(0)
	softBounceRate := float64(0)
	complaintRate := float64(0)

	if totalSent > 0 {
		hardBounceRate = float64(hardBounces) / float64(totalSent) * 100
		softBounceRate = float64(softBounces) / float64(totalSent) * 100
		complaintRate = float64(complaints) / float64(totalSent) * 100
	}

	// Health indicators
	healthStatus := "good"
	healthIssues := make([]string, 0)

	if hardBounceRate > 2 {
		healthStatus = "critical"
		healthIssues = append(healthIssues, "Hard bounce rate is too high (>2%)")
	} else if hardBounceRate > 1 {
		healthStatus = "warning"
		healthIssues = append(healthIssues, "Hard bounce rate is elevated (>1%)")
	}

	if complaintRate > 0.1 {
		healthStatus = "critical"
		healthIssues = append(healthIssues, "Complaint rate is too high (>0.1%)")
	} else if complaintRate > 0.05 {
		if healthStatus != "critical" {
			healthStatus = "warning"
		}
		healthIssues = append(healthIssues, "Complaint rate is elevated (>0.05%)")
	}

	// Recommendations
	recommendations := make([]string, 0)
	if hardBounceRate > 1 {
		recommendations = append(recommendations, "Clean your email list to remove invalid addresses")
		recommendations = append(recommendations, "Enable email verification before sending")
	}
	if complaintRate > 0.05 {
		recommendations = append(recommendations, "Review your email content for spam triggers")
		recommendations = append(recommendations, "Ensure subscribers have clearly opted in")
		recommendations = append(recommendations, "Add easy unsubscribe links")
	}
	if softBounceRate > 5 {
		recommendations = append(recommendations, "Check if you're sending too fast")
		recommendations = append(recommendations, "Review your warmup schedule")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"period": map[string]interface{}{
			"days":       days,
			"total_sent": totalSent,
		},
		"bounces": map[string]interface{}{
			"hard":       hardBounces,
			"soft":       softBounces,
			"complaints": complaints,
		},
		"rates": map[string]interface{}{
			"hard_bounce_rate": hardBounceRate,
			"soft_bounce_rate": softBounceRate,
			"complaint_rate":   complaintRate,
		},
		"suppression": map[string]interface{}{
			"total": suppressionCount,
		},
		"health": map[string]interface{}{
			"status": healthStatus,
			"issues": healthIssues,
		},
		"recommendations": recommendations,
	})
}

// GET /api/analytics/realtime
func (h *AnalyticsHandlerV2) Realtime(w http.ResponseWriter, r *http.Request) {
	// Get stats from last 5 minutes
	since := time.Now().Add(-5 * time.Minute)

	var sending int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("sent_at >= ? AND status = ?", since, "sent").Count(&sending)

	var opens int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("event_type = ? AND created_at >= ?", "open", since).Count(&opens)

	var clicks int64
	h.Store.DB.Model(&models.TrackingEvent{}).Where("event_type = ? AND created_at >= ?", "click", since).Count(&clicks)

	// Active campaigns (status = sending)
	var activeCampaigns int64
	h.Store.DB.Model(&models.Campaign{}).Where("status = ?", "sending").Count(&activeCampaigns)

	// Queue depth (pending recipients)
	var queueDepth int64
	h.Store.DB.Model(&models.CampaignRecipient{}).Where("status = ?", "pending").Count(&queueDepth)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"timestamp":        time.Now().Format(time.RFC3339),
		"sending_rate":     sending, // per 5 min
		"opens_rate":       opens,
		"clicks_rate":      clicks,
		"active_campaigns": activeCampaigns,
		"queue_depth":      queueDepth,
	})
}
