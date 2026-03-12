package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TelegramBot represents a Telegram bot
type TelegramBot struct {
	token   string
	chatID string
	client *http.Client
	running bool
	mu     sync.RWMutex
}

// NewTelegramBot creates a new Telegram bot
func NewTelegramBot(token, chatID string) *TelegramBot {
	return &TelegramBot{
		token:   token,
		chatID: chatID,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Start starts the bot
func (b *TelegramBot) Start() {
	b.mu.Lock()
	b.running = true
	b.mu.Unlock()
	log.Printf("Telegram bot started (polling mode)")
}

// Stop stops the bot
func (b *TelegramBot) Stop() {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()
}

// SendMessage sends a message
func (b *TelegramBot) SendMessage(chatID int, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", b.token)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":      text,
		"parse_mode": "Markdown",
	}
	body, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
