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

// DiscordBot represents a Discord bot
type DiscordBot struct {
	token  string
	appID string
	pubKey string
	client *http.Client
	running bool
	mu     sync.RWMutex
}

// NewDiscordBot creates a new Discord bot
func NewDiscordBot(token, appID, pubKey string) *DiscordBot {
	return &DiscordBot{
		token:  token,
		appID: appID,
		pubKey: pubKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Start starts the bot
func (d *DiscordBot) Start() error {
	d.mu.Lock()
	d.running = true
	d.mu.Unlock()
	log.Printf("Discord bot started with app ID: %s", d.appID)
	return nil
}

// Stop stops the bot
func (d *DiscordBot) Stop() {
	d.mu.Lock()
	d.running = false
	d.mu.Unlock()
}

// SendMessage sends a message
func (d *DiscordBot) SendMessage(channelID, content string) error {
	url := fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", channelID)
	body, _ := json.Marshal(map[string]string{"content": content})
	req, _ := http.NewRequest("POST", url, strings.NewReader(string(body)))
	req.Header.Set("Authorization", "Bot "+d.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
