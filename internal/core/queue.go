package core

// QueueStats stores summary of queue
type QueueStats struct {
	Total     int   `json:"total"`
	Queued    int   `json:"queued"`
	Deferred  int   `json:"deferred"`
	TotalSize int64 `json:"total_size"`
}

// GetQueueMessages reads the KumoMTA queue via HTTP API
// This replaces the dangerous filesystem-based implementation
func GetQueueMessages(limit int, domainFilter string) ([]QueueMessage, error) {
	client := GetKumoClient()
	return client.GetQueueMessages(limit, domainFilter)
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

// RetryQueueMessage retries a specific message
func RetryQueueMessage(id string) error {
	client := GetKumoClient()
	return client.RetryMessage(id)
}

// RetryAllDeferred retries all deferred messages
func RetryAllDeferred() error {
	client := GetKumoClient()
	return client.RetryAllDeferred()
}

// DeleteBounced removes all bounced messages from queue
func DeleteBounced() error {
	client := GetKumoClient()
	return client.DeleteBounced()
}

// GetDomainStats returns per-domain delivery statistics
func GetDomainStats() ([]DomainStats, error) {
	client := GetKumoClient()
	return client.GetDomainStats()
}

// GetKumoMTAStatus returns KumoMTA service status
func GetKumoMTAStatus() (*KumoMTAStatus, error) {
	client := GetKumoClient()
	return client.GetKumoMTAStatus()
}

// StartKumoMTA starts the KumoMTA service
func StartKumoMTA() error {
	client := GetKumoClient()
	return client.StartKumoMTA()
}

// StopKumoMTA stops the KumoMTA service
func StopKumoMTA() error {
	client := GetKumoClient()
	return client.StopKumoMTA()
}

// RestartKumoMTA restarts the KumoMTA service
func RestartKumoMTA() error {
	client := GetKumoClient()
	return client.RestartKumoMTA()
}

// ReloadKumoMTA reloads KumoMTA config
func ReloadKumoMTA() error {
	client := GetKumoClient()
	return client.ReloadKumoMTA()
}

// GetKumoLogs returns recent KumoMTA logs
func GetKumoLogs(lines int) ([]string, error) {
	client := GetKumoClient()
	return client.GetKumoLogs(lines)
}

// SendMailViaHTTP sends an email via KumoMTA HTTP API
func SendMailViaHTTP(req *KumoMailRequest) (string, error) {
	client := GetKumoClient()
	return client.SendMailViaHTTP(req)
}

// SendMailViaSMTP sends an email via direct SMTP (legacy)
func SendMailViaSMTP(smtpAddr, from, to, subject, body string) error {
	client := GetKumoClient()
	return client.SendMailViaSMTP(smtpAddr, from, to, subject, body)
}
