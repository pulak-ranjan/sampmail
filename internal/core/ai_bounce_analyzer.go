package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// =====================================
// AI BOUNCE ANALYZER (Ollama + Qwen)
// =====================================

// OllamaClient connects to local Ollama API
type OllamaClient struct {
	baseURL string
	client  *http.Client
	model   string
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL string, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "qwen2.5:0.5b" // Lightweight model, use qwen2.5:0.8b for better results
	}
	return &OllamaClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		model: model,
	}
}

// OllamaRequest represents a request to Ollama
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Format string `json:"format,omitempty"`
}

// OllamaResponse represents a response from Ollama
type OllamaResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// BounceAnalysis represents AI-analyzed bounce information
type BounceAnalysis struct {
	Category        string `json:"category"`          // e.g., "mailbox_issue", "policy_violation", "technical_issue"
	Severity        string `json:"severity"`          // "critical", "warning", "info"
	Explanation     string `json:"explanation"`       // Human-readable explanation
	Action          string `json:"action"`            // Suggested action
	IsRetryable     bool   `json:"is_retryable"`      // Can we retry sending?
	IsPermanentFail bool   `json:"is_permanent_fail"` // Permanent failure?
	EmailQuality    string `json:"email_quality"`     // "valid", "risky", "invalid"
}

// AnalyzeBounce uses AI to analyze a bounce message
func (o *OllamaClient) AnalyzeBounce(bounce *models.BounceEvent) (*BounceAnalysis, error) {
	// Build prompt for the AI
	prompt := buildBouncePrompt(bounce)

	// Prepare request
	reqBody := OllamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
		Format: "json",
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", o.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result OllamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Parse the JSON from AI response
	return parseBounceAnalysis(result.Response)
}

// buildBouncePrompt creates a detailed prompt for bounce analysis
func buildBouncePrompt(bounce *models.BounceEvent) string {
	return fmt.Sprintf(`You are an email deliverability expert. Analyze this bounce message and provide a detailed JSON response.

Bounce Information:
- Email: %s
- Bounce Type: %s
- SMTP Code: %s
- Diagnostic Code: %s
- Raw Message: %s

Respond with ONLY valid JSON (no markdown, no explanation) in this exact format:
{
    "category": "one of: mailbox_issue, policy_violation, reputation_issue, technical_issue, routing_issue, unknown",
    "severity": "one of: critical, warning, info",
    "explanation": "2-3 sentence explanation of what happened in plain English",
    "action": "specific recommended action to take (e.g., 'Remove from list', 'Retry later', 'Check DNS')",
    "is_retryable": true or false,
    "is_permanent_fail": true or false,
    "email_quality": "one of: valid, risky, invalid"
}`, bounce.Email, bounce.BounceType, bounce.BounceCode, bounce.DiagnosticCode, truncateForAI(bounce.RawMessage, 2000))
}

// parseBounceAnalysis parses AI response into struct
func parseBounceAnalysis(response string) (*BounceAnalysis, error) {
	// Clean response - remove any markdown formatting
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var analysis BounceAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		logger.Warn("failed to parse AI bounce analysis", "response", response, "error", err)
		// Return a safe default
		return &BounceAnalysis{
			Category:    "unknown",
			Severity:    "info",
			Explanation: "Unable to analyze bounce message",
			Action:      "Manual review required",
			IsRetryable: true,
		}, nil
	}

	// Validate required fields
	if analysis.Category == "" {
		analysis.Category = "unknown"
	}
	if analysis.Severity == "" {
		analysis.Severity = "info"
	}
	if analysis.EmailQuality == "" {
		analysis.EmailQuality = "risky"
	}

	return &analysis, nil
}

// truncateForAI truncates text to fit within AI context limits
func truncateForAI(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// CheckOllamaHealth verifies Ollama is running and accessible
func (o *OllamaClient) CheckOllamaHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// GetAvailableModels returns list of available Ollama models
func (o *OllamaClient) GetAvailableModels() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get models: status %d", resp.StatusCode)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, len(result.Models))
	for i, m := range result.Models {
		models[i] = m.Name
	}

	return models, nil
}
