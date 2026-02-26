package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// =====================================
// WEBSOCKET SUPPORT FOR REAL-TIME UPDATES
// =====================================
// Provides real-time updates for:
// - Campaign progress
// - Queue status changes
// - Bounce notifications
// - Automation run status
// - System alerts

// wsAllowedOrigins is set by NewServer to validate WebSocket origins.
// Falls back to localhost-only if not set.
var wsAllowedOrigins func(origin string) bool

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return false
		}
		// Always allow localhost in development
		if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
			return true
		}
		if wsAllowedOrigins != nil {
			return wsAllowedOrigins(origin)
		}
		return false
	},
}

// MessageType defines the type of WebSocket message
type MessageType string

const (
	MessageTypeCampaignProgress MessageType = "campaign_progress"
	MessageTypeQueueStatus      MessageType = "queue_status"
	MessageTypeBounceAlert      MessageType = "bounce_alert"
	MessageTypeAutomationStatus MessageType = "automation_status"
	MessageTypeSystemAlert      MessageType = "system_alert"
	MessageTypeStatsUpdate      MessageType = "stats_update"
	MessageTypePing             MessageType = "ping"
	MessageTypePong             MessageType = "pong"
)

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type      MessageType            `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// WSClient represents a connected WebSocket client
type WSClient struct {
	conn           *websocket.Conn
	userID         uint
	organizationID uint
	send           chan *WSMessage
	subscriptions  map[MessageType]bool
}

// WSHub maintains the set of active clients and broadcasts messages
type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan *WSBroadcast
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex

	// Organization-specific channels
	orgClients map[uint]map[*WSClient]bool
}

// WSBroadcast represents a message to broadcast
type WSBroadcast struct {
	Message        *WSMessage
	OrganizationID uint    // 0 = broadcast to all
	UserID         uint    // 0 = broadcast to all in org
	MessageType    MessageType // Filter by subscription
}

// wsMaxConnections is the maximum number of concurrent WebSocket connections.
// Exceeding this prevents goroutine exhaustion under connection-flood attacks.
const wsMaxConnections = 1000

// Global WebSocket hub
var wsHub *WSHub

// InitWebSocket initializes the WebSocket hub
func InitWebSocket() *WSHub {
	wsHub = &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan *WSBroadcast, 1000),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		orgClients: make(map[uint]map[*WSClient]bool),
	}
	go wsHub.run()
	return wsHub
}

// GetWSHub returns the global WebSocket hub
func GetWSHub() *WSHub {
	return wsHub
}

// run starts the WebSocket hub
func (h *WSHub) run() {
	log := logger.WithComponent("websocket")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if client.organizationID > 0 {
				if h.orgClients[client.organizationID] == nil {
					h.orgClients[client.organizationID] = make(map[*WSClient]bool)
				}
				h.orgClients[client.organizationID][client] = true
			}
			h.mu.Unlock()
			log.Debug("client connected", "user_id", client.userID, "org_id", client.organizationID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if client.organizationID > 0 && h.orgClients[client.organizationID] != nil {
					delete(h.orgClients[client.organizationID], client)
				}
				close(client.send)
			}
			h.mu.Unlock()
			log.Debug("client disconnected", "user_id", client.userID)

		case broadcast := <-h.broadcast:
			h.mu.RLock()
			var recipients []*WSClient

			if broadcast.OrganizationID > 0 {
				// Send to specific organization
				if orgClients, ok := h.orgClients[broadcast.OrganizationID]; ok {
					for client := range orgClients {
						if broadcast.UserID == 0 || client.userID == broadcast.UserID {
							if broadcast.MessageType == "" || client.subscriptions[broadcast.MessageType] {
								recipients = append(recipients, client)
							}
						}
					}
				}
			} else {
				// Broadcast to all
				for client := range h.clients {
					if broadcast.MessageType == "" || client.subscriptions[broadcast.MessageType] {
						recipients = append(recipients, client)
					}
				}
			}
			h.mu.RUnlock()

			// Send to recipients
			for _, client := range recipients {
				select {
				case client.send <- broadcast.Message:
				default:
					// Client buffer full, skip
				}
			}
		}
	}
}

// Broadcast sends a message to connected clients
func (h *WSHub) Broadcast(msg *WSMessage, orgID, userID uint, msgType MessageType) {
	h.broadcast <- &WSBroadcast{
		Message:        msg,
		OrganizationID: orgID,
		UserID:         userID,
		MessageType:    msgType,
	}
}

// BroadcastCampaignProgress broadcasts campaign progress update
func (h *WSHub) BroadcastCampaignProgress(orgID uint, campaignID uint, sent, total int, status string) {
	msg := &WSMessage{
		Type: MessageTypeCampaignProgress,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"campaign_id": campaignID,
			"sent":        sent,
			"total":       total,
			"status":      status,
			"progress":    float64(sent) / float64(total) * 100,
		},
	}
	h.Broadcast(msg, orgID, 0, MessageTypeCampaignProgress)
}

// BroadcastBounceAlert broadcasts a bounce alert
func (h *WSHub) BroadcastBounceAlert(orgID uint, email, bounceType, reason string, campaignID uint) {
	msg := &WSMessage{
		Type: MessageTypeBounceAlert,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"email":       email,
			"bounce_type": bounceType,
			"reason":      reason,
			"campaign_id": campaignID,
		},
	}
	h.Broadcast(msg, orgID, 0, MessageTypeBounceAlert)
}

// BroadcastSystemAlert broadcasts a system alert
func (h *WSHub) BroadcastSystemAlert(orgID uint, level, message string, details map[string]interface{}) {
	msg := &WSMessage{
		Type: MessageTypeSystemAlert,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"level":   level, // "info", "warning", "error", "critical"
			"message": message,
			"details": details,
		},
	}
	h.Broadcast(msg, orgID, 0, MessageTypeSystemAlert)
}

// BroadcastQueueStatus broadcasts queue status update
func (h *WSHub) BroadcastQueueStatus(orgID uint, pending, processing int) {
	msg := &WSMessage{
		Type: MessageTypeQueueStatus,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"pending":    pending,
			"processing": processing,
		},
	}
	h.Broadcast(msg, orgID, 0, MessageTypeQueueStatus)
}

// HandleWebSocket handles WebSocket connection requests
func HandleWebSocket(w http.ResponseWriter, r *http.Request, userID, orgID uint) {
	if wsHub == nil {
		http.Error(w, "WebSocket not initialized", http.StatusServiceUnavailable)
		return
	}

	// Enforce connection limit to prevent goroutine exhaustion
	wsHub.mu.RLock()
	connCount := len(wsHub.clients)
	wsHub.mu.RUnlock()
	if connCount >= wsMaxConnections {
		http.Error(w, "Too many connections", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithComponent("websocket").Error("failed to upgrade connection", "error", err)
		return
	}

	client := &WSClient{
		conn:           conn,
		userID:         userID,
		organizationID: orgID,
		send:           make(chan *WSMessage, 256),
		subscriptions: map[MessageType]bool{
			MessageTypeCampaignProgress: true,
			MessageTypeQueueStatus:      true,
			MessageTypeBounceAlert:      true,
			MessageTypeAutomationStatus: true,
			MessageTypeSystemAlert:      true,
			MessageTypeStatsUpdate:      true,
		},
	}

	wsHub.register <- client

	// Start read and write goroutines. Both defer unregister+close on exit,
	// ensuring the client is cleaned up from the hub regardless of how they exit.
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *WSClient) readPump() {
	defer func() {
		wsHub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		// Parse message for subscription changes
		var msg struct {
			Type    string   `json:"type"`
			Subscribe []string `json:"subscribe,omitempty"`
			Unsubscribe []string `json:"unsubscribe,omitempty"`
		}
		if err := json.Unmarshal(message, &msg); err == nil {
			if msg.Type == "subscribe" {
				for _, t := range msg.Subscribe {
					c.subscriptions[MessageType(t)] = true
				}
				for _, t := range msg.Unsubscribe {
					delete(c.subscriptions, MessageType(t))
				}
			}
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(message)
			if err != nil {
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// GetConnectedClients returns the number of connected clients
func (h *WSHub) GetConnectedClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetOrganizationClients returns the number of connected clients for an organization
func (h *WSHub) GetOrganizationClients(orgID uint) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.orgClients[orgID]; ok {
		return len(clients)
	}
	return 0
}

// =====================================
// WEBSOCKET HELPER FUNCTIONS
// =====================================

// NotifyCampaignStarted notifies clients that a campaign started
func NotifyCampaignStarted(orgID uint, campaignID uint, totalRecipients int) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastCampaignProgress(orgID, campaignID, 0, totalRecipients, "started")
}

// NotifyCampaignProgress notifies clients of campaign progress
func NotifyCampaignProgress(orgID uint, campaignID uint, sent, total int) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastCampaignProgress(orgID, campaignID, sent, total, "sending")
}

// NotifyCampaignCompleted notifies clients that a campaign completed
func NotifyCampaignCompleted(orgID uint, campaignID uint, sent, total int) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastCampaignProgress(orgID, campaignID, sent, total, "completed")
}

// NotifyBounce notifies clients of a bounce
func NotifyBounce(orgID uint, email, bounceType, reason string, campaignID uint) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastBounceAlert(orgID, email, bounceType, reason, campaignID)
}

// NotifySystemAlert notifies clients of a system alert
func NotifySystemAlert(orgID uint, level, message string, details map[string]interface{}) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastSystemAlert(orgID, level, message, details)
}

// NotifyQueueUpdate notifies clients of queue status
func NotifyQueueUpdate(orgID uint, pending, processing int) {
	if wsHub == nil {
		return
	}
	wsHub.BroadcastQueueStatus(orgID, pending, processing)
}
