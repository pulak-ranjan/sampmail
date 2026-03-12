package core

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// BotEngine manages all bots
type BotEngine struct {
	Store    *store.Store
	Commands map[string]*BotCommand
	wg       sync.WaitGroup
	Telegram *TelegramBot
	Discord  *DiscordBot
	Anike    *AnikeAgent
}

var botEngine *BotEngine

// GetBotEngine returns the bot engine
func GetBotEngine() *BotEngine {
	return botEngine
}

// SetBotEngine sets the bot engine instance
func SetBotEngine(engine *BotEngine) {
	if botEngine != nil {
		botEngine.wg.Wait()
	}
	botEngine = engine
}

// BotCommand represents a bot command
type BotCommand struct {
	Name        string
	Description string
	Handler    func(*BotContext, []string) (string, error)
}

// BotContext holds context for bot execution
type BotContext struct {
	Store          *store.Store
	TelegramChatID  string
	DiscordUserID  string
	Platform       string
}

// NewBotEngine creates a new bot engine
func NewBotEngine(st *store.Store) *BotEngine {
	engine := &BotEngine{
		Store:    st,
		Commands: make(map[string]*BotCommand),
	}

	// Register Anike actions
	RegisterAnikeActions()

	// Initialize Anike agent
	engine.Anike = NewAnikeAgent(engine)

	engine.registerCommands()
	return engine
}

// Start starts the bot engine
func (e *BotEngine) Start() error {
	_ = config.Get()
	settings, _ := e.Store.GetSettings()

	if settings != nil && settings.TelegramEnabled && settings.TelegramBotToken != "" {
		e.Telegram = NewTelegramBot(settings.TelegramBotToken, settings.TelegramChatID)
		e.Telegram.Start()
	}

	if settings != nil && settings.DiscordEnabled && settings.DiscordBotToken != "" {
		e.Discord = NewDiscordBot(settings.DiscordBotToken, settings.DiscordAppID, settings.DiscordPublicKey)
		e.Discord.Start()
	}

	logger.Info("Bot engine started")
	return nil
}

// Stop stops the bot engine
func (e *BotEngine) Stop() {
	if e.Telegram != nil {
		e.Telegram.Stop()
	}
	if e.Discord != nil {
		e.Discord.Stop()
	}
	e.wg.Wait()
	logger.Info("Bot engine stopped")
}

// registerCommands registers all bot commands
func (e *BotEngine) registerCommands() {
	e.Commands["/stats"] = &BotCommand{Name: "/stats", Description: "Get today's stats", Handler: e.handleStats}
	e.Commands["/campaigns"] = &BotCommand{Name: "/campaigns", Description: "List campaigns", Handler: e.handleCampaigns}
	e.Commands["/campaign"] = &BotCommand{Name: "/campaign", Description: "Campaign control", Handler: e.handleCampaign}
	e.Commands["/queue"] = &BotCommand{Name: "/queue", Description: "Queue status", Handler: e.handleQueue}
	e.Commands["/logs"] = &BotCommand{Name: "/logs", Description: "Recent logs", Handler: e.handleLogs}
	e.Commands["/alerts"] = &BotCommand{Name: "/alerts", Description: "Configure alerts", Handler: e.handleAlerts}
	e.Commands["/ai"] = &BotCommand{Name: "/ai", Description: "AI insights", Handler: e.handleAI}
	e.Commands["/anike"] = &BotCommand{Name: "/anike", Description: "Chat with Anike AI", Handler: e.handleAnike}
	e.Commands["/help"] = &BotCommand{Name: "/help", Description: "Help", Handler: e.handleHelp}
	e.Commands["/start"] = &BotCommand{Name: "/start", Description: "Welcome", Handler: e.handleHelp}
}

func (e *BotEngine) handleStats(ctx *BotContext, args []string) (string, error) {
	stats, _ := GetQueueStats()
	return fmt.Sprintf("[Stats]\n\nQueued: %d\nDeferred: %d", stats.Queued, stats.Deferred), nil
}

func (e *BotEngine) handleCampaigns(ctx *BotContext, args []string) (string, error) {
	var campaigns []models.Campaign
	e.Store.DB.Order("created_at desc").Limit(10).Find(&campaigns)
	if len(campaigns) == 0 {
		return "No campaigns found", nil
	}
	var result strings.Builder
	result.WriteString("[Campaigns]\n\n")
	for _, c := range campaigns {
		result.WriteString(fmt.Sprintf("• %s - %s (Sent: %d)\n", c.Name, c.Status, c.TotalSent))
	}
	return result.String(), nil
}

func (e *BotEngine) handleCampaign(ctx *BotContext, args []string) (string, error) {
	if len(args) < 2 {
		return "Usage: /campaign <start|pause|resume|status> <id>", nil
	}
	return "Campaign action: " + args[0], nil
}

func (e *BotEngine) handleQueue(ctx *BotContext, args []string) (string, error) {
	stats, _ := GetQueueStats()
	action := ""
	if len(args) > 0 && args[0] == "flush" {
		FlushQueue()
		action = "\nQueue flushed!"
	}
	return fmt.Sprintf("[Queue]\n\nTotal: %d\nQueued: %d\nDeferred: %d%s", stats.Total, stats.Queued, stats.Deferred, action), nil
}

func (e *BotEngine) handleLogs(ctx *BotContext, args []string) (string, error) {
	logs, _ := GetKumoLogs(10)
	if len(logs) == 0 {
		return "No logs available", nil
	}
	var result strings.Builder
	result.WriteString("[Logs]\n\n")
	for _, l := range logs[len(logs)-5:] {
		if len(l) > 80 {
			l = l[:80]
		}
		result.WriteString(l + "\n")
	}
	return result.String(), nil
}

func (e *BotEngine) handleAlerts(ctx *BotContext, args []string) (string, error) {
	if len(args) > 0 {
		return "Alerts " + args[0], nil
	}
	return "Alerts\n/alerts on - Enable\n/alerts off - Disable", nil
}

func (e *BotEngine) handleAI(ctx *BotContext, args []string) (string, error) {
	if len(args) == 0 {
		return "Usage: /ai <question>\n\nAsk me anything about your email campaigns!", nil
	}

	// Generate unique conversation ID based on platform and user
	var conversationID int64
	if ctx.TelegramChatID != "" {
		// Use hash of Telegram chat ID
		conversationID = int64(hashString(ctx.TelegramChatID))
	} else if ctx.DiscordUserID != "" {
		conversationID = int64(hashString(ctx.DiscordUserID))
	} else {
		conversationID = 0
	}

	message := strings.Join(args, " ")
	response, err := e.ProcessAIChat(message, conversationID)
	if err != nil {
		logger.Warn("AI chat handler error", "error", err)
		return "Sorry, I encountered an error. Please check your AI settings.", nil
	}

	// Format response nicely
	return response, nil
}

// handleAnike processes messages with Anike AI agent
func (e *BotEngine) handleAnike(ctx *BotContext, args []string) (string, error) {
	if e.Anike == nil {
		return "Anike is not initialized. Please restart the bot.", nil
	}

	// If no args, show welcome message
	if len(args) == 0 {
		return `Hi! I'm Anike, your AI assistant for SampMail.

I can help you with:
- Viewing stats and analytics
- Managing campaigns
- Checking subscribers
- Working with templates
- Answering questions about email marketing

Just chat with me naturally! For example:
- "show my campaigns"
- "how many subscribers do I have?"
- "check the queue status"
- "what's my open rate?"

I can also execute safe commands for you. What would you like to do?`, nil
	}

	// Generate unique conversation ID based on platform and user
	var conversationID int64
	if ctx.TelegramChatID != "" {
		conversationID = int64(hashString(ctx.TelegramChatID))
	} else if ctx.DiscordUserID != "" {
		conversationID = int64(hashString(ctx.DiscordUserID))
	} else {
		conversationID = 0
	}

	message := strings.Join(args, " ")
	response, err := e.Anike.ProcessMessage(message, conversationID)
	if err != nil {
		logger.Warn("Anike handler error", "error", err)
		return "I encountered an error. Please try again.", nil
	}

	return response, nil
}

// hashString creates a simple hash for string to use as ID
func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	return h
}

func (e *BotEngine) handleHelp(ctx *BotContext, args []string) (string, error) {
	return `[SampMail Bot]

/stats - Today's stats
/campaigns - List campaigns
/campaign <action> <id> - Control
/queue - Queue status
/queue flush - Flush queue
/logs - Recent logs
/alerts on/off - Toggle alerts
/ai <question> - AI insights
/help - This message`, nil
}

// RestartBotEngine restarts the bot engine
func RestartBotEngine() {
	if botEngine != nil {
		botEngine.Stop()
		botEngine.Start()
	}
	logger.Info("Bot engine restarted")
}
