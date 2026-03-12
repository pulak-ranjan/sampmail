package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// KumoClient provides safe access to KumoMTA HTTP API
// This replaces dangerous filesystem access to /var/spool/kumomta
type KumoClient struct {
	baseURL    string
	httpClient *http.Client
}

// QueueMessage represents a message in the KumoMTA queue
type QueueMessage struct {
	ID          string    `json:"id"`
	Queue       string    `json:"queue"`
	Domain      string    `json:"domain"`
	From        string    `json:"from"`
	To          string    `json:"to"`
	Subject     string    `json:"subject"`
	Status      string    `json:"status"`
	RetryCount  int       `json:"retry_count"`
	NextRetry   time.Time `json:"next_retry"`
	CreatedAt   time.Time `json:"created_at"`
	Error       string    `json:"error,omitempty"`
	Size        int64     `json:"size"`
}

// DomainStats represents delivery stats per domain
type DomainStats struct {
	Domain         string `json:"domain"`
	Sent           int64  `json:"sent"`
	Delivered      int64  `json:"delivered"`
	Bounced        int64  `json:"bounced"`
	Deferred       int64  `json:"deferred"`
	Filtered       int64  `json:"filtered"`
	DeliveryRate   float64 `json:"delivery_rate"`
	BounceRate     float64 `json:"bounce_rate"`
}

// KumoMTAStatus represents the running status of KumoMTA
type KumoMTAStatus struct {
	Running      bool   `json:"running"`
	Version      string `json:"version"`
	Uptime       string `json:"uptime"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryMB     float64 `json:"memory_mb"`
	Health       string  `json:"health"`
}

// Singleton instance
var kumoClient *KumoClient

// GetKumoClient returns the singleton KumoMTA client
func GetKumoClient() *KumoClient {
	if kumoClient == nil {
		cfg := config.Get()
		kumoClient = NewKumoClient(cfg.KumoAPIURL)
	}
	return kumoClient
}

// NewKumoClient creates a new KumoMTA API client
func NewKumoClient(baseURL string) *KumoClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &KumoClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// KumoMetricsResponse represents the JSON metrics from KumoMTA
type KumoMetricsResponse struct {
	// Prometheus-style metrics as key-value pairs
	Metrics map[string]interface{} `json:"metrics"`
}

// GetQueueStats fetches queue statistics from KumoMTA HTTP API
// Uses /metrics.json endpoint instead of reading filesystem
func (k *KumoClient) GetQueueStats() (*QueueStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First try /metrics.json for structured data
	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics.json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		logger.Warn("KumoMTA API unavailable, returning empty stats", "error", err)
		// Return empty stats instead of error (graceful degradation)
		return &QueueStats{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fall back to parsing Prometheus text format
		return k.parsePrometheusMetrics()
	}

	var metricsResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metricsResp); err != nil {
		logger.Warn("Failed to parse KumoMTA metrics JSON", "error", err)
		return &QueueStats{}, nil
	}

	return k.extractQueueStats(metricsResp), nil
}

// parsePrometheusMetrics fetches and parses /metrics in Prometheus format
func (k *KumoClient) parsePrometheusMetrics() (*QueueStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return &QueueStats{}, nil
	}
	defer resp.Body.Close()

	// Parse Prometheus text format
	// Example: scheduled_count{queue="default"} 42
	stats := &QueueStats{}

	var body strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	// Parse key metrics using regex
	scheduledRe := regexp.MustCompile(`scheduled_count\{[^}]*\}\s+(\d+)`)
	if matches := scheduledRe.FindAllStringSubmatch(body.String(), -1); len(matches) > 0 {
		for _, m := range matches {
			if val, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				stats.Total += int(val)
				stats.Queued += int(val)
			}
		}
	}

	// Parse deferred count
	deferredRe := regexp.MustCompile(`ready_count\{[^}]*\}\s+(\d+)`)
	if matches := deferredRe.FindAllStringSubmatch(body.String(), -1); len(matches) > 0 {
		for _, m := range matches {
			if val, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				stats.Deferred += int(val)
			}
		}
	}

	return stats, nil
}

// extractQueueStats extracts queue statistics from JSON metrics
func (k *KumoClient) extractQueueStats(metrics map[string]interface{}) *QueueStats {
	stats := &QueueStats{}

	// Navigate the metrics structure
	// KumoMTA returns metrics in various formats depending on version
	for key, value := range metrics {
		switch {
		case strings.Contains(key, "scheduled_count"):
			if v, ok := toInt64(value); ok {
				stats.Total += int(v)
				stats.Queued += int(v)
			}
		case strings.Contains(key, "ready_count"):
			if v, ok := toInt64(value); ok {
				stats.Deferred += int(v)
			}
		case strings.Contains(key, "total_messages_received"):
			// Could be used for additional stats
		}
	}

	return stats
}

// GetQueueMessages fetches queue message list via KumoMTA API
// Uses /queue/list endpoint for queue management
func (k *KumoClient) GetQueueMessages(limit int, domainFilter string) ([]QueueMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Build URL with query params
	url := k.baseURL + "/queue/list?limit=" + strconv.Itoa(limit)
	if domainFilter != "" {
		url += "&domain=" + domainFilter
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		logger.Warn("KumoMTA queue API unavailable", "error", err)
		return []QueueMessage{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Try alternative: parse from queue text endpoint
		return k.getQueueMessagesFromText(limit, domainFilter)
	}

	var messages []QueueMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		logger.Warn("Failed to parse queue messages", "error", err)
		return []QueueMessage{}, nil
	}

	return messages, nil
}

// getQueueMessagesFromText parses queue from text format fallback
func (k *KumoClient) getQueueMessagesFromText(limit int, domainFilter string) ([]QueueMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/queue/text", nil)
	if err != nil {
		return []QueueMessage{}, nil
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return []QueueMessage{}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return k.parseQueueText(string(body), limit, domainFilter)
}

// parseQueueText parses queue messages from text format
func (k *KumoClient) parseQueueText(body string, limit int, domainFilter string) ([]QueueMessage, error) {
	var messages []QueueMessage
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		if limit > 0 && len(messages) >= limit {
			break
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse format: id|queue|domain|from|to|status|retries
		parts := strings.Split(line, "|")
		if len(parts) >= 6 {
			msg := QueueMessage{
				ID:         parts[0],
				Queue:      parts[1],
				Domain:     parts[2],
				From:       parts[3],
				To:         parts[4],
				Status:     parts[5],
				RetryCount: 0,
			}
			if len(parts) > 6 {
				if rc, err := strconv.Atoi(parts[6]); err == nil {
					msg.RetryCount = rc
				}
			}

			if domainFilter == "" || strings.Contains(msg.Domain, domainFilter) {
				messages = append(messages, msg)
			}
		}
	}

	return messages, nil
}

// FlushQueue triggers queue processing - attempts to flush all deferred messages
func (k *KumoClient) FlushQueue() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// POST to /queue/flush endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", k.baseURL+"/queue/flush", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("flush failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("flush returned status %d", resp.StatusCode)
	}

	logger.Info("KumoMTA queue flushed successfully")
	return nil
}

// RetryMessage retries a specific message by ID
func (k *KumoClient) RetryMessage(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", k.baseURL+"/queue/retry/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("retry message failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("retry returned status %d", resp.StatusCode)
	}

	logger.Info("Retried message", "id", id)
	return nil
}

// RetryAllDeferred retries all deferred messages
func (k *KumoClient) RetryAllDeferred() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", k.baseURL+"/queue/retry/all", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("retry all failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Info("Retried all deferred messages")
	return nil
}

// DeleteMessage removes a message from the queue
func (k *KumoClient) DeleteMessage(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "DELETE", k.baseURL+"/queue/message/"+id, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		// Fallback: try POST to delete
		req, err := http.NewRequestWithContext(ctx, "POST", k.baseURL+"/queue/delete/"+id, nil)
		if err != nil {
			return fmt.Errorf("delete message failed: %w", err)
		}
		resp, err = k.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("delete message failed: %w", err)
		}
	}
	defer resp.Body.Close()

	logger.Info("Deleted message from queue", "id", id)
	return nil
}

// DeleteBounced removes all bounced messages from the queue
func (k *KumoClient) DeleteBounced() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", k.baseURL+"/queue/delete/bounced", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete bounced failed: %w", err)
	}
	defer resp.Body.Close()

	logger.Info("Deleted bounced messages from queue")
	return nil
}

// GetDomainStats fetches per-domain delivery statistics
func (k *KumoClient) GetDomainStats() ([]DomainStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/domain/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		logger.Warn("KumoMTA domain stats unavailable", "error", err)
		return []DomainStats{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fallback: parse from metrics
		return k.getDomainStatsFromMetrics()
	}

	var stats []DomainStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		logger.Warn("Failed to parse domain stats", "error", err)
		return []DomainStats{}, nil
	}

	return stats, nil
}

// getDomainStatsFromMetrics extracts domain stats from Prometheus metrics
func (k *KumoClient) getDomainStatsFromMetrics() ([]DomainStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err != nil {
		return []DomainStats{}, nil
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return []DomainStats{}, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	return k.parseDomainMetrics(string(body)), nil
}

// parseDomainMetrics extracts domain-level metrics
func (k *KumoClient) parseDomainMetrics(body string) []DomainStats {
	domainStats := make(map[string]*DomainStats)

	// Parse sent messages per domain
	sentRe := regexp.MustCompile(`kumo_domain_messages_sent_total\{domain="([^"]+)"\}\s+(\d+)`)
	for _, m := range sentRe.FindAllStringSubmatch(body, -1) {
		domain := m[1]
		count, _ := strconv.ParseInt(m[2], 10, 64)
		if ds, ok := domainStats[domain]; ok {
			ds.Sent = count
		} else {
			domainStats[domain] = &DomainStats{Domain: domain, Sent: count}
		}
	}

	// Parse bounces per domain
	bounceRe := regexp.MustCompile(`kumo_domain_bounces_total\{domain="([^"]+)"\}\s+(\d+)`)
	for _, m := range bounceRe.FindAllStringSubmatch(body, -1) {
		domain := m[1]
		count, _ := strconv.ParseInt(m[2], 10, 64)
		if ds, ok := domainStats[domain]; ok {
			ds.Bounced = count
		} else {
			domainStats[domain] = &DomainStats{Domain: domain, Bounced: count}
		}
	}

	// Calculate rates
	var result []DomainStats
	for _, ds := range domainStats {
		if ds.Sent > 0 {
			ds.DeliveryRate = float64(ds.Sent-ds.Bounced) / float64(ds.Sent) * 100
			ds.BounceRate = float64(ds.Bounced) / float64(ds.Sent) * 100
		}
		result = append(result, *ds)
	}

	return result
}

// GetKumoMTAStatus returns the running status of KumoMTA service
func (k *KumoClient) GetKumoMTAStatus() (*KumoMTAStatus, error) {
	status := &KumoMTAStatus{
		Running: false,
		Health:  "unhealthy",
	}

	// Check if KumoMTA HTTP API is responding
	if err := k.HealthCheck(); err != nil {
		status.Health = "unreachable"
		return status, nil
	}

	status.Running = true
	status.Health = "healthy"

	// Try to get version from metrics
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err == nil {
		resp, err := k.httpClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				versionRe := regexp.MustCompile(`kumo_version\{[^}]*\}\s+"([^"]+)"`)
				if matches := versionRe.FindStringSubmatch(string(body)); len(matches) > 1 {
					status.Version = matches[1]
				}
			}
		}
	}

	return status, nil
}

// StartKumoMTA starts the KumoMTA service via systemctl
func (k *KumoClient) StartKumoMTA() error {
	cmd := exec.Command("systemctl", "start", "kumo-mta")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to start KumoMTA", "error", err, "output", string(output))
		return fmt.Errorf("failed to start KumoMTA: %w", err)
	}
	logger.Info("KumoMTA service started")
	return nil
}

// StopKumoMTA stops the KumoMTA service via systemctl
func (k *KumoClient) StopKumoMTA() error {
	cmd := exec.Command("systemctl", "stop", "kumo-mta")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to stop KumoMTA", "error", err, "output", string(output))
		return fmt.Errorf("failed to stop KumoMTA: %w", err)
	}
	logger.Info("KumoMTA service stopped")
	return nil
}

// RestartKumoMTA restarts the KumoMTA service via systemctl
func (k *KumoClient) RestartKumoMTA() error {
	cmd := exec.Command("systemctl", "restart", "kumo-mta")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to restart KumoMTA", "error", err, "output", string(output))
		return fmt.Errorf("failed to restart KumoMTA: %w", err)
	}
	logger.Info("KumoMTA service restarted")
	return nil
}

// ReloadKumoMTA reloads KumoMTA config without restarting
func (k *KumoClient) ReloadKumoMTA() error {
	cmd := exec.Command("systemctl", "reload", "kumo-mta")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to reload KumoMTA", "error", err, "output", string(output))
		return fmt.Errorf("failed to reload KumoMTA: %w", err)
	}
	logger.Info("KumoMTA config reloaded")
	return nil
}

// GetKumoLogs returns recent KumoMTA logs
func (k *KumoClient) GetKumoLogs(lines int) ([]string, error) {
	if lines <= 0 {
		lines = 50
	}
	if lines > 500 {
		lines = 500
	}

	cmd := exec.Command("journalctl", "-u", "kumo-mta", "-n", strconv.Itoa(lines), "--no-pager")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn("Failed to get KumoMTA logs", "error", err)
		return []string{}, nil
	}

	logs := strings.Split(string(output), "\n")
	return logs, nil
}

// KumoMailRequest represents an email to be sent via KumoMTA HTTP API
type KumoMailRequest struct {
	From    string            `json:"from"`
	To      string            `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html,omitempty"`
	Text    string            `json:"text,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// KumoMailResponse represents the response from KumoMTA HTTP API
type KumoMailResponse struct {
	QueueID string `json:"queue_id"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
}

// SendMailViaHTTP sends an email via KumoMTA HTTP API
func (k *KumoClient) SendMailViaHTTP(req *KumoMailRequest) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// KumoMTA HTTP API endpoint for mail submission
	url := k.baseURL + "/mail"

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := k.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("KumoMTA returned status %d: %s", resp.StatusCode, string(body))
	}

	var mailResp KumoMailResponse
	if err := json.Unmarshal(body, &mailResp); err != nil {
		// If we can't parse JSON, check for queue ID in response
		return string(body), nil
	}

	if mailResp.Error != "" {
		return "", fmt.Errorf("KumoMTA error: %s", mailResp.Error)
	}

	return mailResp.QueueID, nil
}

// SendMailViaSMTP sends an email via direct SMTP (legacy method)
func (k *KumoClient) SendMailViaSMTP(smtpAddr, from, to, subject, body string) error {
	// Simple SMTP send for fallback
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		from, to, subject, body)

	err := smtp.SendMail(smtpAddr, nil, from, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("SMTP send failed: %w", err)
	}

	return nil
}

// HealthCheck verifies KumoMTA is responding
func (k *KumoClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err != nil {
		return err
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("KumoMTA unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("KumoMTA returned status %d", resp.StatusCode)
	}

	return nil
}

// Helper function to convert interface{} to int64
func toInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case string:
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}
