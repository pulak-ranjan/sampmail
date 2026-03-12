package core

import (
	"fmt"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
)

// actionGetStats returns email statistics
func actionGetStats(engine *BotEngine, params map[string]interface{}) (string, error) {
	var stats models.EmailStats
	err := engine.Store.DB.Order("id DESC").First(&stats).Error
	if err != nil {
		return "No stats available yet", nil
	}

	today := time.Now().Format("2006-01-02")

	return fmt.Sprintf(`[Email Statistics - %s]

**Sent Today:** %d
**Delivered:** %d
**Bounced:** %d
**Deferred:** %d
`, today, stats.Sent, stats.Delivered, stats.Bounced, stats.Deferred), nil
}

// actionGetSystemInfo returns system information
func actionGetSystemInfo(engine *BotEngine, params map[string]interface{}) (string, error) {
	var settings models.AppSettings
	err := engine.Store.DB.First(&settings).Error
	if err != nil {
		return "", fmt.Errorf("failed to get settings: %w", err)
	}

	return fmt.Sprintf(`[System Information]

Hostname: %s
Server IP: %s

AI Provider: %s (configured: %s)

Webhook: %s
Bounce Alert: %.1f%%

**Verification:**
- Catch-all: %s
- Reacher Only: %s
`, settings.MainHostname, settings.MainServerIP,
		settings.AIProvider, boolToYesNo(settings.AIAPIKey != ""),
		boolToEnabled(settings.WebhookEnabled),
		settings.BounceAlertPct,
		boolToYesNo(settings.EnableCatchall),
		boolToYesNo(settings.UseReacherOnly)), nil
}

// actionListCampaigns returns list of campaigns
func actionListCampaigns(engine *BotEngine, params map[string]interface{}) (string, error) {
	var campaigns []models.Campaign
	err := engine.Store.DB.Order("created_at DESC").Limit(10).Find(&campaigns).Error
	if err != nil {
		return "", fmt.Errorf("failed to get campaigns: %w", err)
	}

	if len(campaigns) == 0 {
		return "📭 No campaigns found. Create one to get started!", nil
	}

	var sb strings.Builder
	sb.WriteString("**,Recent Campaigns:**\n\n")

	for _, c := range campaigns {
		statusIcon := getCampaignStatusIcon(c.Status)
		sb.WriteString(fmt.Sprintf("%s **%s**\n", statusIcon, c.Name))
		sb.WriteString(fmt.Sprintf("   Subject: %s\n", c.Subject))
		sb.WriteString(fmt.Sprintf("   Status: %s | Sent: %d\n\n", c.Status, c.TotalSent))
	}

	return sb.String(), nil
}

// actionCampaignStatus returns status of a specific campaign
func actionCampaignStatus(engine *BotEngine, params map[string]interface{}) (string, error) {
	id, ok := params["id"].(float64)
	if !ok {
		return "Please provide a campaign ID", nil
	}

	var campaign models.Campaign
	err := engine.Store.DB.First(&campaign, uint(id)).Error
	if err != nil {
		return fmt.Sprintf("Campaign #%d not found", int(id)), nil
	}

	return fmt.Sprintf(`**,Campaign: %s**

**Subject:** %s
**Status:** %s
**Created:** %s

**Statistics:**
- Sent: %d
- Opens: %d
- Clicks: %d
- Failed: %d
`, campaign.Name,
		campaign.Subject, campaign.Status,
		campaign.CreatedAt.Format("2006-01-02 15:04"),
		campaign.TotalSent, campaign.TotalOpens, campaign.TotalClicks, campaign.TotalFailed), nil
}

// actionListSubscribers returns list of subscribers
func actionListSubscribers(engine *BotEngine, params map[string]interface{}) (string, error) {
	limit := 10
	if l, ok := params["limit"].(float64); ok {
		limit = int(l)
	}

	var contacts []models.ContactV2
	err := engine.Store.DB.Order("created_at DESC").Limit(limit).Find(&contacts).Error
	if err != nil {
		return "", fmt.Errorf("failed to get subscribers: %w", err)
	}

	if len(contacts) == 0 {
		return " No subscribers found. Add some subscribers to get started!", nil
	}

	var sb strings.Builder
	sb.WriteString(" **Recent Subscribers:**\n\n")

	for _, c := range contacts {
		statusIcon := ""
		if c.Status == "pending" {
			statusIcon = ""
		}
		sb.WriteString(fmt.Sprintf("%s %s <%s>\n", statusIcon, c.FirstName, c.Email))
	}

	var count int64
	engine.Store.DB.Model(&models.ContactV2{}).Count(&count)
	sb.WriteString(fmt.Sprintf("\n**Total: %d subscribers**", count))

	return sb.String(), nil
}

// actionGetSubscriberCount returns total subscriber count
func actionGetSubscriberCount(engine *BotEngine, params map[string]interface{}) (string, error) {
	var count int64
	err := engine.Store.DB.Model(&models.ContactV2{}).Count(&count).Error
	if err != nil {
		return "", fmt.Errorf("failed to get count: %w", err)
	}

	return fmt.Sprintf(" **Total Subscribers: %d**", count), nil
}

// actionCheckQueue returns queue status
func actionCheckQueue(engine *BotEngine, params map[string]interface{}) (string, error) {
	stats, err := GetQueueStats()
	if err != nil {
		return "Unable to get queue status. Is KumoMTA running?", nil
	}

	if stats.Queued == 0 {
		return "**Queue is empty!** No emails waiting to be sent.", nil
	}

	return fmt.Sprintf(` **Queue Status**

 **Emails queued:** %d
 **Deferred:** %d
 **Total:** %d

The queue will process automatically.
`, stats.Queued, stats.Deferred, stats.Total), nil
}

// actionListTemplates returns list of email templates
func actionListTemplates(engine *BotEngine, params map[string]interface{}) (string, error) {
	var templates []models.Template
	err := engine.Store.DB.Order("created_at DESC").Limit(10).Find(&templates).Error
	if err != nil {
		return "", fmt.Errorf("failed to get templates: %w", err)
	}

	if len(templates) == 0 {
		return " No templates found. Create one to get started!", nil
	}

	var sb strings.Builder
	sb.WriteString(" **Email Templates:**\n\n")

	for _, t := range templates {
		sb.WriteString(fmt.Sprintf(" **%s**\n", t.Name))
		sb.WriteString(fmt.Sprintf("   Category: %s | Created: %s\n\n", t.Category, t.CreatedAt.Format("2006-01-02")))
	}

	return sb.String(), nil
}

// Helper functions
func boolToYesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func boolToEnabled(b bool) string {
	if b {
		return "Enabled"
	}
	return "Disabled"
}

func getCampaignStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "sending":
		return ""
	case "completed", "sent":
		return ""
	case "draft":
		return ""
	case "scheduled":
		return ""
	case "paused":
		return ""
	case "failed":
		return ""
	default:
		return ""
	}
}

// ===========================================
// AUTOMATION ACTIONS
// ===========================================

// actionListAutomations returns list of automations
func actionListAutomations(engine *BotEngine, params map[string]interface{}) (string, error) {
	var automations []models.AutomationV2
	err := engine.Store.DB.Order("created_at DESC").Limit(10).Find(&automations).Error
	if err != nil {
		return "", fmt.Errorf("failed to get automations: %w", err)
	}

	if len(automations) == 0 {
		return " No automations found. Create one in the automation builder!", nil
	}

	var sb strings.Builder
	sb.WriteString(" **Automations:**\n\n")

	for _, a := range automations {
		statusIcon := getAutomationStatusIcon(a.Status)
		sb.WriteString(fmt.Sprintf("%s **%s**\n", statusIcon, a.Name))
		sb.WriteString(fmt.Sprintf("   Trigger: %s\n", a.TriggerType))
		sb.WriteString(fmt.Sprintf("   Status: %s | Entered: %d | Completed: %d\n\n",
			a.Status, a.TotalEntered, a.TotalCompleted))
	}

	return sb.String(), nil
}

// actionAutomationStatus returns status of a specific automation
func actionAutomationStatus(engine *BotEngine, params map[string]interface{}) (string, error) {
	id, ok := params["id"].(float64)
	if !ok {
		return "Please provide an automation ID", nil
	}

	var automation models.AutomationV2
	err := engine.Store.DB.First(&automation, uint(id)).Error
	if err != nil {
		return fmt.Sprintf("Automation #%d not found", int(id)), nil
	}

	// Get recent runs
	var runs []models.AutomationRunV2
	engine.Store.DB.Where("automation_id = ?", uint(id)).
		Order("entered_at DESC").Limit(5).Find(&runs)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(` **Automation: %s**

**Trigger:** %s
**Status:** %s
**Created:** %s

**Statistics:**
- Total Entered: %d
- Total Completed: %d
- Conversion Rate: %.1f%%

`, automation.Name, automation.TriggerType, automation.Status,
		automation.CreatedAt.Format("2006-01-02 15:04"),
		automation.TotalEntered, automation.TotalCompleted, automation.ConversionRate))

	if len(runs) > 0 {
		sb.WriteString("**Recent Runs:**\n")
		for _, r := range runs {
			sb.WriteString(fmt.Sprintf("• %s - %s\n", r.EnteredAt.Format("15:04"), r.Status))
		}
	}

	return sb.String(), nil
}

// actionAutomationStats returns automation statistics
func actionAutomationStats(engine *BotEngine, params map[string]interface{}) (string, error) {
	var total, active, paused int64
	engine.Store.DB.Model(&models.AutomationV2{}).Count(&total)
	engine.Store.DB.Model(&models.AutomationV2{}).Where("status = ?", "active").Count(&active)
	engine.Store.DB.Model(&models.AutomationV2{}).Where("status = ?", "paused").Count(&paused)

	var runsActive, runsCompleted, runsFailed int64
	engine.Store.DB.Model(&models.AutomationRunV2{}).Where("status = ?", "active").Count(&runsActive)
	engine.Store.DB.Model(&models.AutomationRunV2{}).Where("status = ?", "completed").Count(&runsCompleted)
	engine.Store.DB.Model(&models.AutomationRunV2{}).Where("status = ?", "failed").Count(&runsFailed)

	return fmt.Sprintf(` **Automation Statistics**

**Automations:**
- Total: %d
- Active: %d
- Paused: %d

**Runs:**
- Active: %d
- Completed: %d
- Failed: %d
`, total, active, paused, runsActive, runsCompleted, runsFailed), nil
}

// actionStartAutomation activates an automation
func actionStartAutomation(engine *BotEngine, params map[string]interface{}) (string, error) {
	id, ok := params["id"].(float64)
	if !ok {
		return "Please provide an automation ID to start", nil
	}

	err := engine.Store.DB.Model(&models.AutomationV2{}).
		Where("id = ?", uint(id)).
		Update("status", "active").Error
	if err != nil {
		return "", fmt.Errorf("failed to start automation: %w", err)
	}

	// Reload automation to register it
	var automation models.AutomationV2
	if err := engine.Store.DB.First(&automation, uint(id)).Error; err == nil {
		// Register with engine if available
		if engine.Anike != nil {
			// Could reload automations here
		}
	}

	return fmt.Sprintf("**Automation #%d is now active!**\n\nIt will trigger when: %s", int(id), automation.TriggerType), nil
}

// actionStopAutomation deactivates an automation
func actionStopAutomation(engine *BotEngine, params map[string]interface{}) (string, error) {
	id, ok := params["id"].(float64)
	if !ok {
		return "Please provide an automation ID to stop", nil
	}

	err := engine.Store.DB.Model(&models.AutomationV2{}).
		Where("id = ?", uint(id)).
		Update("status", "paused").Error
	if err != nil {
		return "", fmt.Errorf("failed to stop automation: %w", err)
	}

	return fmt.Sprintf(" **Automation #%d is now paused.**\n\nIt will not trigger until activated again.", int(id)), nil
}

// Helper function
func getAutomationStatusIcon(status string) string {
	switch strings.ToLower(status) {
	case "active":
		return ""
	case "paused":
		return ""
	case "draft":
		return ""
	case "archived":
		return ""
	default:
		return ""
	}
}

// ===========================================
// LIST ACTIONS
// ===========================================

// actionListSubscriberLists returns list of subscriber lists
func actionListSubscriberLists(engine *BotEngine, params map[string]interface{}) (string, error) {
	var lists []models.SubscriberList
	err := engine.Store.DB.Order("created_at DESC").Limit(10).Find(&lists).Error
	if err != nil {
		return "", fmt.Errorf("failed to get lists: %w", err)
	}

	if len(lists) == 0 {
		return " No subscriber lists found. Create one to organize your contacts!", nil
	}

	var sb strings.Builder
	sb.WriteString(" **Subscriber Lists:**\n\n")

	for _, l := range lists {
		var count int64
		engine.Store.DB.Model(&models.ListSubscriber{}).Where("list_id = ?", l.ID).Count(&count)
		sb.WriteString(fmt.Sprintf(" **%s**\n", l.Name))
		sb.WriteString(fmt.Sprintf("   Description: %s\n", l.Description))
		sb.WriteString(fmt.Sprintf("   Subscribers: %d | Created: %s\n\n", count, l.CreatedAt.Format("2006-01-02")))
	}

	return sb.String(), nil
}

// actionCreateSubscriberList creates a new subscriber list
func actionCreateSubscriberList(engine *BotEngine, params map[string]interface{}) (string, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return "Please provide a list name. Example: create list name=Newsletter", nil
	}

	description := ""
	if desc, ok := params["description"].(string); ok {
		description = desc
	}

	newList := models.SubscriberList{
		Name:        name,
		Description: description,
		Type:        "static",
	}

	if err := engine.Store.DB.Create(&newList).Error; err != nil {
		return "", fmt.Errorf("failed to create list: %w", err)
	}

	return fmt.Sprintf("**Created new list: %s**\n\nList ID: %d\nDescription: %s", newList.Name, newList.ID, newList.Description), nil
}

// actionAddToList adds a subscriber to a list
func actionAddToList(engine *BotEngine, params map[string]interface{}) (string, error) {
	listID, ok := params["list_id"].(float64)
	if !ok {
		// Try to find by name
		listName, ok := params["list"].(string)
		if !ok {
			return "Please provide a list_id or list name. Example: add to list list=Newsletter email=user@example.com", nil
		}

		var list models.SubscriberList
		if err := engine.Store.DB.Where("name = ?", listName).First(&list).Error; err != nil {
			return fmt.Sprintf("List '%s' not found. Use 'show lists' to see available lists.", listName), nil
		}
		listID = float64(list.ID)
	}

	email, ok := params["email"].(string)
	if !ok || email == "" {
		return "Please provide an email address. Example: add to list list_id=1 email=user@example.com", nil
	}

	// Check if contact exists, if not create
	var contact models.ContactV2
	err := engine.Store.DB.Where("email = ?", email).First(&contact).Error
	if err != nil {
		// Create new contact
		contact = models.ContactV2{
			Email:   email,
			Status:  "active",
		}
		if firstName, ok := params["first_name"].(string); ok {
			contact.FirstName = firstName
		}
		if err := engine.Store.DB.Create(&contact).Error; err != nil {
			return "", fmt.Errorf("failed to create contact: %w", err)
		}
	}

	// Add to list
	listSub := models.ListSubscriber{
		ListID:     uint(listID),
		ContactID:  contact.ID,
		Status:     "active",
		SubscribedAt: time.Now(),
	}

	if err := engine.Store.DB.Create(&listSub).Error; err != nil {
		// Check if already in list
		var existing models.ListSubscriber
		if err := engine.Store.DB.Where("list_id = ? AND contact_id = ?", listID, contact.ID).First(&existing).Error; err == nil {
			return fmt.Sprintf(" %s is already in this list!", email), nil
		}
		return "", fmt.Errorf("failed to add to list: %w", err)
	}

	return fmt.Sprintf("**Added %s to the list!**\n\nContact ID: %d\nList ID: %d", email, contact.ID, uint(listID)), nil
}

// actionGetListSubscribers gets subscribers from a specific list
func actionGetListSubscribers(engine *BotEngine, params map[string]interface{}) (string, error) {
	listID, ok := params["list_id"].(float64)
	if !ok {
		listName, ok := params["list"].(string)
		if !ok {
			return "Please provide a list_id or list name. Example: get list_subscribers list=Newsletter", nil
		}

		var list models.SubscriberList
		if err := engine.Store.DB.Where("name = ?", listName).First(&list).Error; err != nil {
			return fmt.Sprintf("List '%s' not found. Use 'show lists' to see available lists.", listName), nil
		}
		listID = float64(list.ID)
	}

	// Get subscribers with contact info using join
	type ListSubWithContact struct {
		ID        uint
		ListID    uint
		ContactID uint
		Status    string
		Email     string
		FirstName string
	}

	var subs []ListSubWithContact
	err := engine.Store.DB.Table("list_subscribers").
		Select("list_subscribers.id, list_subscribers.list_id, list_subscribers.contact_id, list_subscribers.status, contact_v2.email, contact_v2.first_name").
		Joins("JOIN contact_v2 ON contact_v2.id = list_subscribers.contact_id").
		Where("list_subscribers.list_id = ?", uint(listID)).
		Find(&subs).Error
	if err != nil {
		return "", fmt.Errorf("failed to get subscribers: %w", err)
	}

	if len(subs) == 0 {
		return "This list has no subscribers yet.", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(" **Subscribers in List #%d:**\n\n", uint(listID)))

	for _, s := range subs {
		statusIcon := ""
		if s.Status == "unsubscribed" {
			statusIcon = ""
		}
		sb.WriteString(fmt.Sprintf("%s %s <%s>\n", statusIcon, s.FirstName, s.Email))
	}

	return sb.String(), nil
}

// ===========================================
// EMAIL ACTIONS
// ===========================================

// actionSendTestEmail sends a test email
func actionSendTestEmail(engine *BotEngine, params map[string]interface{}) (string, error) {
	sender, ok := params["sender"].(string)
	if !ok || sender == "" {
		return "Please provide a sender email. Example: send test sender=you@domain.com recipient=test@example.com subject=Test", nil
	}

	recipient, ok := params["recipient"].(string)
	if !ok || recipient == "" {
		return "Please provide a recipient email. Example: send test sender=you@domain.com recipient=test@example.com subject=Test", nil
	}

	subject := "Test Email"
	if s, ok := params["subject"].(string); ok {
		subject = s
	}

	// This action returns instructions since we need to call the API
	return fmt.Sprintf(`**,Test Email Ready**

From: %s
To: %s
Subject: %s

To send this test email, I'll need to call the API. This action requires a confirmation for security reasons.

Would you like me to send this test email?`, sender, recipient, subject), nil
}

// actionGetListsCount returns count of lists
func actionGetListsCount(engine *BotEngine, params map[string]interface{}) (string, error) {
	var count int64
	engine.Store.DB.Model(&models.SubscriberList{}).Count(&count)

	var subCount int64
	engine.Store.DB.Model(&models.ListSubscriber{}).Count(&subCount)

	return fmt.Sprintf(` **List Statistics**

**Lists:** %d total
**Total Subscriptions:** %d

Organize your subscribers into lists for better targeting!`, count, subCount), nil
}
