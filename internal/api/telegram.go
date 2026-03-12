package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// TelegramUpdate represents an incoming Telegram update
type TelegramUpdate struct {
	UpdateID int `json:"update_id"`
	Message  struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID        int    `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name"`
			Username  string `json:"username"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

// handleTelegramWebhook processes incoming Telegram messages
func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	var update TelegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		logger.Warn("Failed to parse Telegram update", "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	if update.Message.Text == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Get the bot engine
	engine := core.GetBotEngine()
	if engine == nil {
		logger.Warn("Bot engine not available")
		writeJSON(w, http.StatusOK, map[string]string{"status": "bot not available"})
		return
	}

	// Create context
	ctx := &core.BotContext{
		Store:           s.Store,
		TelegramChatID:  string(rune(update.Message.Chat.ID)),
		Platform:        "telegram",
	}

	// Parse command and args
	text := update.Message.Text
	var command string
	var args []string

	if strings.HasPrefix(text, "/") {
		parts := strings.Fields(text)
		command = parts[0]
		if len(parts) > 1 {
			args = parts[1:]
		}
	} else {
		// Not a command - pass to Anike
		command = "/anike"
		args = strings.Fields(text)
	}

	// Execute command
	handler, ok := engine.Commands[command]
	if !ok {
		// Try Anike for unknown commands
		if handler, ok = engine.Commands["/anike"]; ok {
			args = strings.Fields(text)
		} else {
			sendTelegramMessage(ctx.TelegramChatID, "Unknown command. Type /help for available commands.")
			writeJSON(w, http.StatusOK, map[string]string{"status": "unknown command"})
			return
		}
	}

	response, err := handler.Handler(ctx, args)
	if err != nil {
		logger.Warn("Command execution failed", "command", command, "error", err)
		response = "An error occurred while processing your request."
	}

	// Send response back to Telegram
	sendTelegramMessage(ctx.TelegramChatID, response)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// sendTelegramMessage sends a message via Telegram
func sendTelegramMessage(chatID string, text string) {
	engine := core.GetBotEngine()
	if engine == nil || engine.Telegram == nil {
		return
	}

	var id int
	_, err := fmt.Sscanf(chatID, "%d", &id)
	if err != nil {
		return
	}

	engine.Telegram.SendMessage(id, text)
}
