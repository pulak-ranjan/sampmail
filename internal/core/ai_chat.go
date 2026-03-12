package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// AIProvider types
type AIProvider string

const (
	ProviderOpenAI    AIProvider = "openai"
	ProviderAnthropic AIProvider = "anthropic"
	ProviderGemini    AIProvider = "gemini"
	ProviderDeepSeek AIProvider = "deepseek"
	ProviderOpenRouter AIProvider = "openrouter"
	ProviderOllama   AIProvider = "ollama"
)

// Context window limits per provider (80% of max to leave room for response)
var providerContextLimits = map[AIProvider]int{
	ProviderOpenAI:     120000,  // GPT-4: 128k -> 100k tokens
	ProviderAnthropic: 180000,  // Claude: 200k -> 160k tokens
	ProviderGemini:     1000000, // Gemini 1.5: 2M -> 1.6M tokens
	ProviderDeepSeek:   64000,  // DeepSeek: 64k -> 50k tokens
	ProviderOpenRouter: 128000, // Varies by model, use GPT-4 as baseline
	ProviderOllama:     8000,   // Local models typically 4k-8k
}

// DefaultContextLimit is the fallback limit
const DefaultContextLimit = 8000

// ChatMessage represents a chat message
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatConversation represents a conversation with the AI
type ChatConversation struct {
	Messages []ChatMessage `json:"messages"`
	Provider AIProvider  `json:"provider"`
	Model    string      `json:"model"`
}

// AIMessage represents an AI request/response
type AIMessage struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIResponse struct {
	Choices []Choice `json:"choices"`
	Error   AIError  `json:"error,omitempty"`
}

type Choice struct {
	Message Message `json:"message"`
}

type AIError struct {
	Message string `json:"message"`
}

// ChatWithAI sends a message to the configured AI provider
func ChatWithAI(conversation *ChatConversation, apiKey string) (string, error) {
	if conversation == nil || len(conversation.Messages) == 0 {
		return "", fmt.Errorf("no messages in conversation")
	}

	// Convert ChatMessage to API format
	messages := make([]Message, len(conversation.Messages))
	for i, m := range conversation.Messages {
		messages[i] = Message{Role: m.Role, Content: m.Content}
	}

	// Select provider
	var response string
	var err error

	switch conversation.Provider {
	case ProviderOpenAI, ProviderDeepSeek, ProviderOpenRouter, ProviderOllama:
		response, err = callChatAPI(conversation.Provider, conversation.Model, messages, apiKey)
	case ProviderAnthropic:
		response, err = callAnthropic(conversation.Model, messages, apiKey)
	case ProviderGemini:
		response, err = callGemini(conversation.Provider, conversation.Model, messages, apiKey)
	default:
		// Default to OpenAI
		response, err = callChatAPI(ProviderOpenAI, conversation.Model, messages, apiKey)
	}

	return response, err
}

func callChatAPI(provider AIProvider, model string, messages []Message, apiKey string) (string, error) {
	// Map provider to endpoint
	endpoint := getProviderEndpoint(provider)
	modelName := getProviderModel(provider, model)

	reqBody := AIMessage{
		Model:       modelName,
		Messages:    messages,
		MaxTokens:   2000,
		Temperature: 0.7,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result AIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		if result.Error.Message != "" {
			return "", fmt.Errorf("AI error: %s", result.Error.Message)
		}
		return "", fmt.Errorf("no response from AI")
	}

	return result.Choices[0].Message.Content, nil
}

func callAnthropic(model string, messages []Message, apiKey string) (string, error) {
	// Convert messages to Anthropic format
	systemPrompt := ""
	userMessages := make([]map[string]string, 0)
	for _, m := range messages {
		if m.Role == "system" {
			systemPrompt = m.Content
		} else {
			userMessages = append(userMessages, map[string]string{"role": m.Role, "content": m.Content})
		}
	}

	reqBody := map[string]interface{}{
		"model":      "claude-3-5-sonnet-20241022",
		"max_tokens": 2000,
		"system":     systemPrompt,
		"messages":   userMessages,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Anthropic API error: %s", string(respBody))
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if content, ok := result["content"].([]interface{}); ok {
		if len(content) > 0 {
			if textBlock, ok := content[0].(map[string]interface{}); ok {
				if text, ok := textBlock["text"].(string); ok {
					return text, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no response from Anthropic")
}

func callGemini(provider AIProvider, model string, messages []Message, apiKey string) (string, error) {
	// Build prompt from messages
	var prompt strings.Builder
	for _, m := range messages {
		prompt.WriteString(fmt.Sprintf("%s: %s\n", m.Role, m.Content))
	}

	// Use provider URL
	endpoint := getProviderEndpoint(provider)
	if endpoint == "" {
		endpoint = "https://generativelanguage.googleapis.com/v1beta"
	}

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", endpoint, getProviderModel(provider, model), apiKey)

	parts := []map[string]string{{"text": prompt.String()}}
	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": parts},
		},
	}

	body, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	if candidates, ok := result["candidates"].([]interface{}); ok {
		if len(candidates) > 0 {
			if cand, ok := candidates[0].(map[string]interface{}); ok {
				if content, ok := cand["content"].(map[string]interface{}); ok {
					if parts, ok := content["parts"].([]interface{}); ok {
						if len(parts) > 0 {
							if part, ok := parts[0].(map[string]string); ok {
								return part["text"], nil
							}
						}
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no response from AI")
}

func getProviderEndpoint(provider AIProvider) string {
	switch provider {
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderDeepSeek:
		return "https://api.deepseek.com/v1"
	case ProviderOpenRouter:
		return "https://openrouter.ai/api"
	case ProviderOllama:
		return "http://localhost:11434/api/chat"
	case ProviderGemini:
		return "https://generativelanguage.googleapis.com/v1beta"
	default:
		return "https://api.openai.com/v1"
	}
}

func getProviderModel(provider AIProvider, model string) string {
	if model != "" {
		return model
	}
	switch provider {
	case ProviderOpenAI:
		return "gpt-4o-mini"
	case ProviderDeepSeek:
		return "deepseek-chat"
	case ProviderOpenRouter:
		return "openai/gpt-4o-mini"
	case ProviderOllama:
		return "llama3.2"
	case ProviderGemini:
		return "gemini-1.5-flash"
	case ProviderAnthropic:
		return "claude-3-5-sonnet-20241022"
	default:
		return "gpt-4o-mini"
	}
}

// CountTokens estimates token count for a string
// Uses simple approximation: ~4 characters per token
func CountTokens(text string) int {
	if text == "" {
		return 0
	}
	// Simple approximation: tokens ≈ characters / 4
	// For better accuracy, could use tiktoken or similar
	return int(math.Ceil(float64(len(text)) / 4.0))
}

// CountMessageTokens counts tokens for a message including role overhead
func CountMessageTokens(role, content string) int {
	return CountTokens(content) + CountTokens(fmt.Sprintf("role:%s", role))
}

// GetContextLimit returns the context window limit for a provider
func GetContextLimit(provider AIProvider) int {
	if limit, ok := providerContextLimits[provider]; ok {
		return limit
	}
	return DefaultContextLimit
}

// GetConversationHistory retrieves messages for a conversation from DB
func (e *BotEngine) GetConversationHistory(conversationID string, maxTokens int) ([]models.ChatLog, error) {
	var messages []models.ChatLog
	err := e.Store.DB.Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages).Error
	if err != nil {
		return nil, err
	}
	return messages, nil
}

// SaveMessage saves a chat message to the database
func (e *BotEngine) SaveMessage(conversationID, role, content string) error {
	msg := models.ChatLog{
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now(),
	}
	return e.Store.DB.Create(&msg).Error
}

// BuildConversationMessages builds API messages with token limit trimming
func (e *BotEngine) BuildConversationMessages(conversationID, systemPrompt string, provider AIProvider) ([]Message, error) {
	// Get context limit (use 80% to leave room for response)
	contextLimit := GetContextLimit(provider)
	maxTokens := int(float64(contextLimit) * 0.8)

	// Get conversation history from DB
	history, err := e.GetConversationHistory(conversationID, maxTokens)
	if err != nil {
		return nil, err
	}

	// Start with system prompt
	messages := []Message{{Role: "system", Content: systemPrompt}}
	systemTokens := CountMessageTokens("system", systemPrompt)
	currentTokens := systemTokens

	// Add messages from newest to oldest until we hit token limit
	// But we want to keep chronological order, so we'll build in reverse first
	var trimmedMessages []Message
	for i := len(history) - 1; i >= 0; i-- {
		msg := history[i]
		msgTokens := CountMessageTokens(msg.Role, msg.Content)

		if currentTokens+msgTokens > maxTokens {
			// Stop adding messages, but don't break - we want to keep recent ones
			break
		}
		trimmedMessages = append(trimmedMessages, Message{Role: msg.Role, Content: msg.Content})
		currentTokens += msgTokens
	}

	// Reverse to get chronological order
	for i := len(trimmedMessages) - 1; i >= 0; i-- {
		messages = append(messages, trimmedMessages[i])
	}

	return messages, nil
}

// SecurePrompt creates a system prompt for the bot
func (e *BotEngine) securePrompt() string {
	return `You are SampMail AI Assistant - a helpful email marketing assistant.

You help users manage their email campaigns, subscribers, and email deliverability.

Capabilities:
- Create and manage email campaigns
- Analyze campaign performance
- Answer questions about email marketing
- Help with troubleshooting

Never reveal API keys or internal system details.
Always be helpful and concise.`
}

// ProcessAIChat processes a chat message with context
func (e *BotEngine) ProcessAIChat(message string, conversationIDInt int64) (string, error) {
	// Get AI settings from store
	settings, err := e.Store.GetSettings()
	if err != nil {
		return "AI not configured. Please set up in Settings → AI.", nil
	}

	// Get API key (already encrypted in DB)
	apiKey := settings.AIAPIKey
	if apiKey == "" {
		return "AI API key not configured. Please add your AI provider API key in Settings.", nil
	}

	// Decrypt API key if needed
	// For now assume it's stored encrypted
	// In production, decrypt with core.Decrypt()

	// Generate conversation ID
	conversationID := fmt.Sprintf("user_%d", conversationIDInt)
	provider := AIProvider(settings.AIProvider)

	// Build conversation messages with history and token trimming
	messages, err := e.BuildConversationMessages(conversationID, e.securePrompt(), provider)
	if err != nil {
		logger.Warn("Failed to build conversation messages", "error", err)
		// Fall back to simple conversation
		messages = []Message{
			{Role: "system", Content: e.securePrompt()},
		}
	}

	// Add current user message
	messages = append(messages, Message{Role: "user", Content: message})

	// Save user message to history
	_ = e.SaveMessage(conversationID, "user", message)

	// Build conversation for API call
	chatMessages := make([]ChatMessage, len(messages))
	for i, m := range messages {
		chatMessages[i] = ChatMessage{Role: m.Role, Content: m.Content}
	}

	conversation := &ChatConversation{
		Provider: provider,
		Model:    "",
		Messages: chatMessages,
	}

	response, err := ChatWithAI(conversation, apiKey)
	if err != nil {
		logger.Warn("AI chat error", "error", err)
		return "Sorry, I couldn't process that request. Please check your API key and try again.", nil
	}

	// Save AI response to history
	_ = e.SaveMessage(conversationID, "assistant", response)

	return response, nil
}

// ClearConversation clears conversation history for a user
func (e *BotEngine) ClearConversation(conversationIDInt int64) error {
	conversationID := fmt.Sprintf("user_%d", conversationIDInt)
	return e.Store.DB.Where("conversation_id = ?", conversationID).Delete(&models.ChatLog{}).Error
}
