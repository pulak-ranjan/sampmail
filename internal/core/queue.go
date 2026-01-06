package core

import (
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// QueueStats stores summary of queue
type QueueStats struct {
	Total     int   `json:"total"`
	Queued    int   `json:"queued"`
	Deferred  int   `json:"deferred"`
	TotalSize int64 `json:"total_size"`
}

// GetQueueMessages reads the KumoMTA queue via HTTP API
// This replaces the dangerous filesystem-based implementation
func GetQueueMessages(limit int) ([]models.QueueMessage, error) {
	client := GetKumoClient()
	return client.GetQueueMessages(limit)
}

// GetQueueStats returns summary of queue via KumoMTA HTTP API
func GetQueueStats() (*QueueStats, error) {
	client := GetKumoClient()
	return client.GetQueueStats()
}

// DeleteQueueMessage removes a message from the queue
func DeleteQueueMessage(id string) error {
	client := GetKumoClient()
	return client.DeleteMessage(id)
}

// FlushQueue attempts to retry all deferred messages
func FlushQueue() error {
	client := GetKumoClient()
	return client.FlushQueue()
}
