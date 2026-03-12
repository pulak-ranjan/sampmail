package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// AnikeAgent represents the AI agent "Anike" that can chat and execute safe actions
type AnikeAgent struct {
	Engine *BotEngine
}

// AnikeAction represents an action that Anike can perform
type AnikeAction struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`       // "info", "campaign", "subscriber", "system"
	Execute     func(*BotEngine, map[string]interface{}) (string, error)
	Parameters  []ActionParameter
	Safe        bool     `json:"safe"`           // true = read-only, false = modifies data
	RequiresAuth bool    `json:"requires_auth"`  // needs admin auth
}

// ActionParameter defines a parameter for an action
type ActionParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "string", "int", "bool"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
}

// ActionLogEntry represents a logged action (immutable - Anike cannot delete)
type ActionLogEntry struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Actor       string    `json:"actor"`       // "Anike" or "user:<username>"
	Action      string    `json:"action"`      // action name
	Parameters  string    `json:"parameters"`  // JSON string of params
	Result      string    `json:"result"`      // result summary
	Status      string    `json:"status"`      // "success", "failed", "blocked"
	Timestamp   time.Time `json:"timestamp"`
	IPAddress   string    `json:"ip_address,omitempty"`
}

// SensitivePatterns are patterns that Anike will never execute
var SensitivePatterns = []string{
	"delete",
	"remove",
	"drop",
	"truncate",
	"rm -rf",
	"rmdir",
	"del /",
	"format",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"kill -9",
	"pkill",
	"chmod 777",
	"chown",
	"sudo",
	"su -",
	"passwd",
	"root",
	"--force",
	"-f",
	"eval",
	"exec(",
	"system(",
	"shell_exec",
	"<?php",
	"<script",
	"../",
	"..\\",
	"/etc/passwd",
	"/etc/shadow",
	"~/.ssh",
	".env",
	"password",
	"secret",
	"api_key",
	"apikey",
	"token",
}

// AnikeSystemPrompt returns Anike's system prompt
func AnikeSystemPrompt() string {
	return `You are Anike, an intelligent AI assistant for SampMail email marketing platform.

## Your Personality
- Friendly, helpful, and professional
- Proactive - you can offer to perform actions for users
- Curious - ask clarifying questions when needed
- Transparent about your capabilities and limitations

## Your Capabilities

### 1. Conversational Chat
You can have natural conversations about:
- Email marketing best practices
- Campaign performance and analytics
- Subscriber management
- Email deliverability
- Technical questions about the platform

### 2. Executing Actions (Safe Commands)
You can execute these SAFE actions for users:
- View stats and analytics
- List campaigns, subscribers, templates
- Check queue status
- Get system information
- Create drafts
- Run reports

### 3. Information Gathering
You can answer questions about:
- How to improve email deliverability
- Best practices for subject lines
- Subscriber engagement tips
- Campaign optimization
- Technical documentation

## IMPORTANT SAFETY RULES

 **NEVER execute these actions:**
- Any action involving deletion (except unsubscribing a single user who requests it)
- Any action with "rm", "delete", "drop", "truncate"
- Any shell commands or system commands
- Any action modifying system files
- Any action affecting other users without their request
- Any action involving payments, refunds, or financial data
- Any action bypassing authentication
- Any action you are unsure about - always ask for confirmation

 **Always ask for confirmation before executing:**
- Any action that modifies data
- Any action that affects campaigns
- Any bulk operations

 **Always safe to do:**
- View information
- List items
- Get statistics
- Answer questions
- Explain concepts

## How to Execute Actions

When a user asks you to do something, respond with:

[ACTION:action_name]
{"param1": "value1", "param2": "value2"}
[/ACTION]

For example, if user says "show my campaigns":
[ACTION:list_campaigns]
{}
[/ACTION]

If user asks "create a campaign called Newsletter":
[ACTION:create_campaign]
{"name": "Newsletter", "subject": "Your Newsletter Subject"}
[/ACTION]

## Response Format

When responding:
1. Be conversational and friendly
2. If you need to execute an action, use the ACTION format above
3. After executing, summarize what happened
4. If something goes wrong, explain the issue clearly
5. Always offer to help with anything else

Remember: You are helpful but cautious. It's better to ask than to assume!`
}

// IsActionSafe checks if an action is safe to execute
func IsActionSafe(actionName string, params map[string]interface{}) (bool, string) {
	actionName = strings.ToLower(actionName)

	// Check for sensitive patterns in action name
	for _, pattern := range SensitivePatterns {
		if strings.Contains(actionName, pattern) {
			return false, fmt.Sprintf("Action '%s' contains blocked pattern '%s'", actionName, pattern)
		}
	}

	// Check for sensitive patterns in parameters
	paramsJSON, _ := json.Marshal(params)
	paramsStr := strings.ToLower(string(paramsJSON))
	for _, pattern := range SensitivePatterns {
		if strings.Contains(paramsStr, pattern) {
			return false, fmt.Sprintf("Parameters contain blocked pattern '%s'", pattern)
		}
	}

	return true, ""
}

// NewAnikeAgent creates a new Anike agent
func NewAnikeAgent(engine *BotEngine) *AnikeAgent {
	return &AnikeAgent{Engine: engine}
}

// ProcessMessage processes a user message and returns Anike's response
func (a *AnikeAgent) ProcessMessage(message string, conversationID int64) (string, error) {
	// Log the user message
	a.logAction("user_message", map[string]interface{}{
		"message":         message,
		"conversation_id": conversationID,
	}, "received", "success")

	// Check if message contains an action request
	actionRequest, actionParams, hasAction := parseActionRequest(message)

	if hasAction {
		return a.executeAction(actionRequest, actionParams, conversationID)
	}

	// Regular chat - use the AI chat with Anike's personality
	return a.processChat(message, conversationID)
}

// parseActionRequest parses action requests from message
func parseActionRequest(message string) (actionName string, params map[string]interface{}, hasAction bool) {
	// Check for ACTION blocks
	if strings.Contains(message, "[ACTION:") && strings.Contains(message, "[/ACTION]") {
		start := strings.Index(message, "[ACTION:")
		end := strings.Index(message, "[/ACTION]")

		if start >= 0 && end > start {
			actionBlock := message[start+8 : end]
			lines := strings.SplitN(actionBlock, "\n", 2)
			actionName = strings.TrimSpace(lines[0])

			if len(lines) > 1 {
				paramsStr := strings.TrimSpace(lines[1])
				if paramsStr != "" && paramsStr != "{}" {
					json.Unmarshal([]byte(paramsStr), &params)
				}
			}

			return actionName, params, true
		}
	}

	// Check for natural language action hints
	lowerMsg := strings.ToLower(message)
	actionKeywords := map[string][]string{
		"list_campaigns":     {"list campaigns", "show campaigns", "get campaigns"},
		"get_stats":          {"stats", "statistics", "show stats"},
		"check_queue":        {"queue", "queue status", "check queue"},
		"list_subscribers":   {"subscribers", "list subscribers", "show subscribers"},
		"list_templates":     {"templates", "email templates"},
		"get_system_info":    {"system info", "system information", "server status"},
		"list_automations":   {"automations", "list automations", "show automations", "workflows"},
		"automation_stats":   {"automation stats", "automation statistics"},
		"list_lists":        {"lists", "subscriber lists", "show lists", "list lists"},
		"get_lists_count":   {"how many lists", "list count"},
	}

	for action, keywords := range actionKeywords {
		for _, kw := range keywords {
			if strings.Contains(lowerMsg, kw) && !strings.Contains(lowerMsg, "create") && !strings.Contains(lowerMsg, "delete") {
				return action, map[string]interface{}{}, true
			}
		}
	}

	return "", nil, false
}

// executeAction executes a requested action with safety checks
func (a *AnikeAgent) executeAction(actionName string, params map[string]interface{}, conversationID int64) (string, error) {
	// Security check
	safe, reason := IsActionSafe(actionName, params)
	if !safe {
		a.logAction(actionName, params, reason, "blocked")
		return fmt.Sprintf("⛔ I cannot execute this action: %s\n\nThis is a security measure to protect your data.", reason), nil
	}

	// Check if action exists
	action := GetAnikeAction(actionName)
	if action == nil {
		a.logAction(actionName, params, "action not found", "failed")
		return fmt.Sprintf(" I don't know how to perform '%s'. Here are things I can do:\n\n%s", actionName, ListAnikeActions()), nil
	}

	// Check if action is safe
	if !action.Safe {
		a.logAction(actionName, params, "unsafe action", "blocked")
		return " This action is marked as potentially unsafe. Please contact an administrator to perform this action.", nil
	}

	// Execute the action
	result, err := action.Execute(a.Engine, params)
	status := "success"
	if err != nil {
		status = "failed"
		logger.Warn("Anike action failed", "action", actionName, "error", err)
	}

	a.logAction(actionName, params, result, status)

	if err != nil {
		return fmt.Sprintf(" Action failed: %s", err.Error()), nil
	}

	return result, nil
}

// processChat handles regular chat using AI
func (a *AnikeAgent) processChat(message string, conversationID int64) (string, error) {
	// Use the existing AI chat with Anike's system prompt
	anikePrompt := `You are Anike, a friendly AI assistant for SampMail email marketing platform.

IMPORTANT: You are in CHAT MODE. Just have a natural conversation. Do NOT use [ACTION] blocks for regular conversation.

If user asks you to do something specific like "show my campaigns" or "check the queue", you CAN execute those actions. But for regular questions, just answer normally.

Be helpful, friendly, and concise. If you're unsure about something, ask for clarification.`

	// Build conversation with history
	conversation := &ChatConversation{
		Provider: ProviderOpenAI, // Use default, will be overridden by settings
		Messages: []ChatMessage{
			{Role: "system", Content: anikePrompt},
			{Role: "user", Content: message},
		},
	}

	// Get AI settings
	settings, err := a.Engine.Store.GetSettings()
	if err != nil || settings.AIAPIKey == "" {
		// Fallback to simple response
		return "I'm here to help! Configure AI in Settings to enable full conversation. Meanwhile, I can help you with:\n\n" + ListAnikeActions(), nil
	}

	// Add conversation history
	conversationIDStr := fmt.Sprintf("anike_%d", conversationID)
	history, _ := a.Engine.GetConversationHistory(conversationIDStr, 4000)
	for _, msg := range history {
		conversation.Messages = append(conversation.Messages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	response, err := ChatWithAI(conversation, settings.AIAPIKey)
	if err != nil {
		logger.Warn("Anike chat error", "error", err)
		return "I apologize, but I encountered an error processing your request. Please check the AI configuration.", nil
	}

	// Save to history
	_ = a.Engine.SaveMessage(conversationIDStr, "user", message)
	_ = a.Engine.SaveMessage(conversationIDStr, "assistant", response)

	return response, nil
}

// logAction logs an action to the database (Anike cannot delete this)
func (a *AnikeAgent) logAction(action string, params map[string]interface{}, result, status string) {
	paramsJSON, _ := json.Marshal(params)

	entry := models.ActionLog{
		Actor:      "Anike",
		Action:     action,
		Parameters: string(paramsJSON),
		Result:     result,
		Status:     status,
		Timestamp: time.Now(),
	}

	// Use raw SQL to ensure Anike cannot accidentally modify
	a.Engine.Store.DB.Exec(`
		INSERT INTO action_logs (actor, action, parameters, result, status, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
	`, entry.Actor, entry.Action, entry.Parameters, entry.Result, entry.Status, entry.Timestamp)

	logger.Info("Anike action logged", "action", action, "status", status)
}

// ListAnikeActions returns a formatted list of available actions
func ListAnikeActions() string {
	actions := GetAllAnikeActions()
	var sb strings.Builder

	sb.WriteString(" **Available Actions:**\n\n")

	categories := map[string][]*AnikeAction{}
	for _, action := range actions {
		categories[action.Category] = append(categories[action.Category], action)
	}

	for category, acts := range categories {
		sb.WriteString(fmt.Sprintf("**%s:**\n", strings.Title(category)))
		for _, act := range acts {
			icon := ""
			if !act.Safe {
				icon = ""
			}
			sb.WriteString(fmt.Sprintf("  %s `/%s` - %s\n", icon, act.Name, act.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n Just ask me naturally! For example: \"show my campaigns\" or \"check the queue\"")
	return sb.String()
}

// RegisterAnikeActions registers all available actions
func RegisterAnikeActions() {
	// Info actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "get_stats",
		Description: "View email statistics",
		Category:    "info",
		Execute:     actionGetStats,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "get_system_info",
		Description: "View system information",
		Category:    "info",
		Execute:     actionGetSystemInfo,
		Safe:        true,
	})

	// Campaign actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "list_campaigns",
		Description: "List all campaigns",
		Category:    "campaign",
		Execute:     actionListCampaigns,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "campaign_status",
		Description: "Get campaign status by ID",
		Category:    "campaign",
		Execute:     actionCampaignStatus,
		Parameters: []ActionParameter{
			{Name: "id", Type: "int", Description: "Campaign ID", Required: true},
		},
		Safe: true,
	})

	// Subscriber actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "list_subscribers",
		Description: "List subscribers (optionally filtered)",
		Category:    "subscriber",
		Execute:     actionListSubscribers,
		Parameters: []ActionParameter{
			{Name: "list_id", Type: "int", Description: "Filter by list ID", Required: false},
			{Name: "limit", Type: "int", Description: "Number to show", Required: false, Default: "10"},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "get_subscriber_count",
		Description: "Get total subscriber count",
		Category:    "subscriber",
		Execute:     actionGetSubscriberCount,
		Safe:        true,
	})

	// Queue actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "check_queue",
		Description: "Check email queue status",
		Category:    "system",
		Execute:     actionCheckQueue,
		Safe:        true,
	})

	// Template actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "list_templates",
		Description: "List email templates",
		Category:    "campaign",
		Execute:     actionListTemplates,
		Safe:        true,
	})

	// Automation actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "list_automations",
		Description: "List all automations",
		Category:    "automation",
		Execute:     actionListAutomations,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "automation_status",
		Description: "Get automation status by ID",
		Category:    "automation",
		Execute:     actionAutomationStatus,
		Parameters: []ActionParameter{
			{Name: "id", Type: "int", Description: "Automation ID", Required: true},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "automation_stats",
		Description: "Get automation statistics",
		Category:    "automation",
		Execute:     actionAutomationStats,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "start_automation",
		Description: "Activate an automation",
		Category:    "automation",
		Execute:     actionStartAutomation,
		Parameters: []ActionParameter{
			{Name: "id", Type: "int", Description: "Automation ID to activate", Required: true},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "stop_automation",
		Description: "Deactivate an automation",
		Category:    "automation",
		Execute:     actionStopAutomation,
		Parameters: []ActionParameter{
			{Name: "id", Type: "int", Description: "Automation ID to deactivate", Required: true},
		},
		Safe: true,
	})

	// List actions
	RegisterAnikeAction(&AnikeAction{
		Name:        "list_lists",
		Description: "List all subscriber lists",
		Category:    "list",
		Execute:     actionListSubscriberLists,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "create_list",
		Description: "Create a new subscriber list",
		Category:    "list",
		Execute:     actionCreateSubscriberList,
		Parameters: []ActionParameter{
			{Name: "name", Type: "string", Description: "List name", Required: true},
			{Name: "description", Type: "string", Description: "List description", Required: false},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "add_to_list",
		Description: "Add a subscriber to a list",
		Category:    "list",
		Execute:     actionAddToList,
		Parameters: []ActionParameter{
			{Name: "list", Type: "string", Description: "List name", Required: false},
			{Name: "list_id", Type: "int", Description: "List ID", Required: false},
			{Name: "email", Type: "string", Description: "Subscriber email", Required: true},
			{Name: "first_name", Type: "string", Description: "First name", Required: false},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "get_list_subscribers",
		Description: "Get subscribers in a specific list",
		Category:    "list",
		Execute:     actionGetListSubscribers,
		Parameters: []ActionParameter{
			{Name: "list", Type: "string", Description: "List name", Required: false},
			{Name: "list_id", Type: "int", Description: "List ID", Required: false},
		},
		Safe: true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "get_lists_count",
		Description: "Get statistics about subscriber lists",
		Category:    "list",
		Execute:     actionGetListsCount,
		Safe:        true,
	})

	RegisterAnikeAction(&AnikeAction{
		Name:        "send_test_email",
		Description: "Send a test email",
		Category:    "email",
		Execute:     actionSendTestEmail,
		Parameters: []ActionParameter{
			{Name: "sender", Type: "string", Description: "Sender email", Required: true},
			{Name: "recipient", Type: "string", Description: "Recipient email", Required: true},
			{Name: "subject", Type: "string", Description: "Email subject", Required: false},
		},
		Safe: true,
	})
}

// Action registry
var anikeActions = make(map[string]*AnikeAction)

// RegisterAnikeAction registers an action
func RegisterAnikeAction(action *AnikeAction) {
	anikeActions[action.Name] = action
}

// GetAnikeAction gets an action by name
func GetAnikeAction(name string) *AnikeAction {
	return anikeActions[name]
}

// GetAllAnikeActions gets all registered actions
func GetAllAnikeActions() []*AnikeAction {
	var actions []*AnikeAction
	for _, action := range anikeActions {
		actions = append(actions, action)
	}
	return actions
}
