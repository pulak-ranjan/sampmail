package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/pulak-ranjan/sampmail/internal/core"
	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// DiscordWebhookRequest represents an incoming Discord interaction
type DiscordWebhookRequest struct {
	Type int `json:"type"`
	Data struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	} `json:"data"`
	Message struct {
		ID        string `json:"id"`
		ChannelID string `json:"channel_id"`
		Author    struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"author"`
	} `json:"message"`
	Member struct {
		User struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
	} `json:"member"`
}

// DiscordMessage represents a Discord message
type DiscordMessage struct {
	Content string `json:"content"`
	TTS     bool   `json:"tts,omitempty"`
}

// handleDiscordWebhook processes incoming Discord interactions
func (s *Server) handleDiscordWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Warn("Failed to read Discord request body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var req DiscordWebhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		logger.Warn("Failed to parse Discord request", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Pong for interaction callback
	if req.Type == 1 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"type": 1}`))
		return
	}

	// Get the bot engine
	engine := core.GetBotEngine()
	if engine == nil {
		logger.Warn("Bot engine not available")
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	// Get user ID and message content
	var userID, messageContent string

	// Check if it's a slash command or message
	if len(body) > 0 {
		var msgData map[string]interface{}
		if err := json.Unmarshal(body, &msgData); err == nil {
			// Check for message content
			if data, ok := msgData["data"].(map[string]interface{}); ok {
				if options, ok := data["options"].([]interface{}); ok && len(options) > 0 {
					if opt, ok := options[0].(map[string]interface{}); ok {
						if value, ok := opt["value"].(string); ok {
							messageContent = value
						}
					}
				}
			}
			// Get user ID from member or message
			if member, ok := msgData["member"].(map[string]interface{}); ok {
				if user, ok := member["user"].(map[string]interface{}); ok {
					if id, ok := user["id"].(string); ok {
						userID = id
					}
				}
			}
		}
	}

	// If no content from command, we can't process
	if messageContent == "" {
		// Send a default response
		s.sendDiscordMessage(w, "Hi! I'm Anike. Use /anike to chat with me.")
		return
	}

	// Use Anike to process the message
	if engine.Anike == nil {
		s.sendDiscordMessage(w, "Anike is not initialized. Please restart the bot.")
		return
	}

	conversationID := int64(hashString(userID))
	response, err := engine.Anike.ProcessMessage(messageContent, conversationID)
	if err != nil {
		logger.Warn("Anike Discord handler error", "error", err)
		s.sendDiscordMessage(w, "I encountered an error. Please try again.")
		return
	}

	// Send response
	s.sendDiscordMessage(w, response)
}

// sendDiscordMessage sends a message to Discord
func (s *Server) sendDiscordMessage(w http.ResponseWriter, content string) {
	// Get settings for Discord channel
	settings, err := s.Store.GetSettings()
	if err != nil || settings == nil || settings.DiscordGuildID == "" {
		// Just return OK, Discord will timeout if no response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content": "Discord not configured"}`))
		return
	}

	// For Discord interactions, we respond with the message content
	// Discord expects a JSON response
	response := map[string]interface{}{
		"content": content,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// hashString creates a simple hash for string to use as ID
func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	return h
}
