package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// =====================================
// AI TEMPLATE BUILDER SERVICE
// =====================================

// AITemplateService generates email templates using AI
type AITemplateService struct {
	store       *store.Store
	httpClient  *http.Client
	rateLimiter *AIRateLimiter
}

// AIRateLimiter limits AI API calls to prevent billing bombs
type AIRateLimiter struct {
	mu           sync.Mutex
	requestCount map[uint]int       // user_id -> count
	resetTime    map[uint]time.Time // user_id -> reset time
	maxPerHour   int
	maxPerDay    int
}

// NewAIRateLimiter creates a rate limiter for AI endpoints
func NewAIRateLimiter(maxPerHour, maxPerDay int) *AIRateLimiter {
	return &AIRateLimiter{
		requestCount: make(map[uint]int),
		resetTime:    make(map[uint]time.Time),
		maxPerHour:   maxPerHour,
		maxPerDay:    maxPerDay,
	}
}

// Allow checks if a user can make an AI request
func (r *AIRateLimiter) Allow(userID uint) (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	resetTime, exists := r.resetTime[userID]

	// Reset if hour has passed
	if !exists || now.After(resetTime) {
		r.requestCount[userID] = 0
		r.resetTime[userID] = now.Add(time.Hour)
	}

	if r.requestCount[userID] >= r.maxPerHour {
		return false, fmt.Sprintf("AI rate limit exceeded. Max %d requests per hour.", r.maxPerHour)
	}

	r.requestCount[userID]++
	return true, ""
}

// NewAITemplateService creates a new AI template service
func NewAITemplateService(st *store.Store) *AITemplateService {
	return &AITemplateService{
		store:       st,
		httpClient:  &http.Client{Timeout: 60 * time.Second},
		rateLimiter: NewAIRateLimiter(20, 100), // 20/hour, 100/day
	}
}

// GenerateTemplateRequest holds the AI generation request
type GenerateTemplateRequest struct {
	Prompt          string         `json:"prompt"`
	Category        string         `json:"category"`
	Industry        string         `json:"industry"`
	Tone            string         `json:"tone"`
	IncludeElements []string       `json:"include_elements"`
	ColorScheme     string         `json:"color_scheme"`
	Brand           *BrandSettings `json:"brand,omitempty"`
	UserID          uint           `json:"-"` // Set by handler, not from JSON
}

// BrandSettings holds brand customization
type BrandSettings struct {
	CompanyName    string `json:"company_name"`
	LogoURL        string `json:"logo_url"`
	PrimaryColor   string `json:"primary_color"`
	SecondaryColor string `json:"secondary_color"`
	FontFamily     string `json:"font_family"`
}

// GenerateTemplateResponse holds the generated template
type GenerateTemplateResponse struct {
	Subject     string   `json:"subject"`
	PreviewText string   `json:"preview_text"`
	HTMLContent string   `json:"html_content"`
	MJMLSource  string   `json:"mjml_source"`
	Variables   []string `json:"variables"`
}

// sanitizeUserInput removes potential prompt injection attempts
func sanitizeUserInput(input string) string {
	// Remove common injection patterns
	dangerous := []string{
		"ignore previous",
		"ignore all",
		"disregard",
		"forget your instructions",
		"new instructions",
		"system prompt",
		"api key",
		"secret",
		"password",
		"credential",
		"<script",
		"javascript:",
		"{{",
		"}}",
	}

	lowered := strings.ToLower(input)
	for _, pattern := range dangerous {
		if strings.Contains(lowered, pattern) {
			// Replace the dangerous content
			re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(pattern))
			input = re.ReplaceAllString(input, "[filtered]")
		}
	}

	// Limit length
	if len(input) > 500 {
		input = input[:500]
	}

	// Remove any non-printable characters
	input = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, input)

	return strings.TrimSpace(input)
}

// validateCategory ensures category is from allowed list
func validateCategory(cat string) string {
	allowed := map[string]bool{
		"welcome": true, "newsletter": true, "promotional": true,
		"transactional": true, "reengagement": true, "announcement": true,
	}
	if allowed[strings.ToLower(cat)] {
		return strings.ToLower(cat)
	}
	return "newsletter"
}

// validateTone ensures tone is from allowed list
func validateTone(tone string) string {
	allowed := map[string]bool{
		"professional": true, "casual": true, "friendly": true,
		"formal": true, "playful": true,
	}
	if allowed[strings.ToLower(tone)] {
		return strings.ToLower(tone)
	}
	return "professional"
}

// GenerateTemplate generates an email template using AI
func (ats *AITemplateService) GenerateTemplate(ctx context.Context, req *GenerateTemplateRequest) (*GenerateTemplateResponse, error) {
	log := logger.WithComponent("ai_template")

	// Rate limit check
	if req.UserID > 0 {
		if allowed, reason := ats.rateLimiter.Allow(req.UserID); !allowed {
			return nil, fmt.Errorf(reason)
		}
	}

	// Validate and sanitize inputs
	req.Prompt = sanitizeUserInput(req.Prompt)
	req.Category = validateCategory(req.Category)
	req.Tone = validateTone(req.Tone)
	if req.Industry != "" {
		req.Industry = sanitizeUserInput(req.Industry)
	}
	if req.ColorScheme != "" {
		// Only allow hex colors
		if !regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`).MatchString(req.ColorScheme) {
			req.ColorScheme = "#3B82F6" // Default blue
		}
	}

	if req.Prompt == "" || req.Prompt == "[filtered]" {
		return nil, fmt.Errorf("invalid prompt")
	}

	var settings models.AppSettings
	if err := ats.store.DB.First(&settings).Error; err != nil {
		return nil, fmt.Errorf("failed to get settings: %w", err)
	}

	if settings.AIAPIKey == "" {
		return nil, fmt.Errorf("AI API key not configured")
	}

	systemPrompt := ats.buildSystemPrompt()
	userPrompt := ats.buildUserPrompt(req)

	var response *GenerateTemplateResponse
	var err error

	switch settings.AIProvider {
	case "openai":
		response, err = ats.callOpenAI(ctx, settings.AIAPIKey, systemPrompt, userPrompt)
	case "deepseek":
		response, err = ats.callDeepSeek(ctx, settings.AIAPIKey, systemPrompt, userPrompt)
	case "anthropic":
		response, err = ats.callAnthropic(ctx, settings.AIAPIKey, systemPrompt, userPrompt)
	default:
		response, err = ats.callOpenAI(ctx, settings.AIAPIKey, systemPrompt, userPrompt)
	}

	if err != nil {
		log.Error("AI generation failed", "error", err)
		return nil, err
	}

	// Post-process: sanitize output and extract variables
	response.HTMLContent = sanitizeHTMLOutput(response.HTMLContent)
	pe := NewPersonalizationEngine()
	response.Variables = pe.ExtractVariables(response.HTMLContent)

	return response, nil
}

// sanitizeHTMLOutput removes any dangerous content from generated HTML
func sanitizeHTMLOutput(html string) string {
	// Remove script tags
	re := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, "")

	// Remove event handlers
	re = regexp.MustCompile(`(?i)\s+on\w+="[^"]*"`)
	html = re.ReplaceAllString(html, "")

	// Remove javascript: URLs
	re = regexp.MustCompile(`(?i)javascript:`)
	html = re.ReplaceAllString(html, "")

	return html
}

// buildSystemPrompt creates the system prompt (FIXED: no user input here)
func (ats *AITemplateService) buildSystemPrompt() string {
	return `You are an email template designer. Create professional, responsive HTML email templates.

RULES:
1. Output ONLY valid JSON with these exact keys: subject, preview_text, html_content
2. Use {{variable_name}} syntax for personalization (first_name, last_name, company_name, unsubscribe_url)
3. Create mobile-responsive HTML using tables (max-width: 600px)
4. Include proper DOCTYPE and meta tags
5. Use inline CSS only
6. Include alt text for images
7. No external resources except placeholder images

OUTPUT FORMAT:
{
  "subject": "Subject line with {{first_name}}",
  "preview_text": "Preview text under 100 chars",
  "html_content": "<!DOCTYPE html>..."
}`
}

// buildUserPrompt creates the user prompt (sanitized input only)
func (ats *AITemplateService) buildUserPrompt(req *GenerateTemplateRequest) string {
	// Build structured request - user input is already sanitized
	parts := []string{
		fmt.Sprintf("Create a %s email template.", req.Category),
		fmt.Sprintf("Description: %s", req.Prompt),
		fmt.Sprintf("Tone: %s", req.Tone),
	}

	if req.Industry != "" {
		parts = append(parts, fmt.Sprintf("Industry: %s", req.Industry))
	}
	if req.ColorScheme != "" {
		parts = append(parts, fmt.Sprintf("Primary color: %s", req.ColorScheme))
	}
	if req.Brand != nil && req.Brand.CompanyName != "" {
		parts = append(parts, fmt.Sprintf("Company: %s", sanitizeUserInput(req.Brand.CompanyName)))
	}

	return strings.Join(parts, "\n")
}

func (ats *AITemplateService) callOpenAI(ctx context.Context, apiKey, systemPrompt, userPrompt string) (*GenerateTemplateResponse, error) {
	reqBody := map[string]interface{}{
		"model": "gpt-4-turbo-preview",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     0.7,
		"max_tokens":      4000,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := ats.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error: %s", string(respBody))
	}

	var openAIResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &openAIResp); err != nil {
		return nil, err
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI")
	}

	var result GenerateTemplateResponse
	if err := json.Unmarshal([]byte(openAIResp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return &result, nil
}

func (ats *AITemplateService) callDeepSeek(ctx context.Context, apiKey, systemPrompt, userPrompt string) (*GenerateTemplateResponse, error) {
	reqBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.7,
		"max_tokens":  4000,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepseek.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := ats.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek API error: %s", string(respBody))
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	content := extractJSON(apiResp.Choices[0].Message.Content)
	var result GenerateTemplateResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return &result, nil
}

func (ats *AITemplateService) callAnthropic(ctx context.Context, apiKey, systemPrompt, userPrompt string) (*GenerateTemplateResponse, error) {
	reqBody := map[string]interface{}{
		"model":      "claude-3-sonnet-20240229",
		"max_tokens": 4000,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := ats.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anthropic API error: %s", string(respBody))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("no response")
	}

	content := extractJSON(apiResp.Content[0].Text)
	var result GenerateTemplateResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse: %w", err)
	}

	return &result, nil
}

func extractJSON(content string) string {
	// Try to find JSON in code blocks
	if idx := strings.Index(content, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	// Try to find raw JSON object
	if idx := strings.Index(content, "{"); idx != -1 {
		depth := 0
		for i := idx; i < len(content); i++ {
			if content[i] == '{' {
				depth++
			} else if content[i] == '}' {
				depth--
				if depth == 0 {
					return content[idx : i+1]
				}
			}
		}
	}
	return content
}

// =====================================
// BUILT-IN TEMPLATES (unchanged)
// =====================================

type BuiltInTemplate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Category     string `json:"category"`
	Subcategory  string `json:"subcategory"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnail_url"`
}

func GetBuiltInTemplates() []BuiltInTemplate {
	return []BuiltInTemplate{
		{ID: "welcome-modern", Name: "Welcome - Modern", Category: "welcome", Description: "Clean modern welcome email"},
		{ID: "welcome-saas", Name: "Welcome - SaaS", Category: "welcome", Description: "SaaS onboarding welcome"},
		{ID: "welcome-ecommerce", Name: "Welcome - E-commerce", Category: "welcome", Description: "E-commerce welcome with discount"},
		{ID: "newsletter-minimal", Name: "Newsletter - Minimal", Category: "newsletter", Description: "Clean text-focused newsletter"},
		{ID: "newsletter-magazine", Name: "Newsletter - Magazine", Category: "newsletter", Description: "Multi-column magazine style"},
		{ID: "promo-sale", Name: "Sale Announcement", Category: "promotional", Description: "Bold sale announcement"},
		{ID: "promo-product", Name: "Product Launch", Category: "promotional", Description: "New product announcement"},
		{ID: "transact-receipt", Name: "Order Confirmation", Category: "transactional", Description: "Order receipt with items"},
		{ID: "transact-shipping", Name: "Shipping Update", Category: "transactional", Description: "Shipping notification"},
		{ID: "reengage-miss", Name: "We Miss You", Category: "reengagement", Description: "Win-back email"},
		{ID: "reengage-cart", Name: "Abandoned Cart", Category: "reengagement", Description: "Cart abandonment reminder"},
	}
}

func GetBuiltInTemplateContent(templateID string) (*GenerateTemplateResponse, error) {
	templates := map[string]*GenerateTemplateResponse{
		"welcome-modern": {
			Subject:     "Welcome to {{company_name}}, {{first_name}}!",
			PreviewText: "We're excited to have you on board",
			HTMLContent: WelcomeModernHTML,
			Variables:   []string{"first_name", "company_name", "unsubscribe_url"},
		},
		"welcome-saas": {
			Subject:     "Welcome aboard, {{first_name}}! Let's get started",
			PreviewText: "Your account is ready",
			HTMLContent: WelcomeSaaSHTML,
			Variables:   []string{"first_name", "company_name"},
		},
		"newsletter-minimal": {
			Subject:     "{{company_name}} Newsletter - {{current_month}}",
			PreviewText: "This month's updates",
			HTMLContent: NewsletterMinimalHTML,
			Variables:   []string{"first_name", "company_name", "current_month"},
		},
		"promo-sale": {
			Subject:     "{{first_name}}, Don't Miss Our Sale!",
			PreviewText: "Up to 50% off",
			HTMLContent: PromoSaleHTML,
			Variables:   []string{"first_name"},
		},
		"transact-receipt": {
			Subject:     "Order Confirmation #{{order_id}}",
			PreviewText: "Thank you for your purchase",
			HTMLContent: TransactReceiptHTML,
			Variables:   []string{"first_name", "order_id"},
		},
	}

	if tmpl, ok := templates[templateID]; ok {
		return tmpl, nil
	}
	return nil, fmt.Errorf("template not found: %s", templateID)
}

// HTML Templates (kept the same)
const WelcomeModernHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Welcome</title>
</head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background-color:#f4f4f5;">
  <table width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="background-color:#4F46E5;padding:40px 30px;text-align:center;">
              <h1 style="margin:0;color:#ffffff;font-size:28px;">Welcome to {{company_name}}!</h1>
            </td>
          </tr>
          <tr>
            <td style="padding:40px 30px;">
              <p style="font-size:18px;margin:0 0 20px;">Hi {{first_name|default:there}},</p>
              <p style="font-size:16px;line-height:1.6;color:#333333;margin:0 0 20px;">
                We're thrilled to have you join our community!
              </p>
              <p style="text-align:center;margin:30px 0;">
                <a href="#" style="display:inline-block;background-color:#4F46E5;color:#ffffff;padding:14px 30px;text-decoration:none;border-radius:8px;font-weight:bold;">Get Started</a>
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 30px;text-align:center;border-top:1px solid #e5e5e5;">
              <p style="font-size:12px;color:#666666;margin:0;">
                <a href="{{unsubscribe_url}}" style="color:#666666;">Unsubscribe</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const WelcomeSaaSHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background-color:#F3F4F6;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;border-radius:8px;">
          <tr>
            <td style="padding:40px 30px;">
              <h1 style="margin:0 0 20px;font-size:24px;color:#111827;">Welcome aboard, {{first_name}}!</h1>
              <p style="font-size:16px;line-height:1.6;color:#374151;">
                Your account is all set up. Here's how to get started.
              </p>
              <p style="text-align:center;">
                <a href="#" style="display:inline-block;background-color:#2563EB;color:#ffffff;padding:12px 24px;text-decoration:none;border-radius:6px;">Go to Dashboard →</a>
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 30px;text-align:center;border-top:1px solid #E5E7EB;">
              <p style="font-size:12px;color:#6B7280;margin:0;">
                <a href="{{unsubscribe_url}}" style="color:#6B7280;">Unsubscribe</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const NewsletterMinimalHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;font-family:Georgia,serif;background-color:#ffffff;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table width="600" cellpadding="0" cellspacing="0">
          <tr>
            <td style="text-align:center;padding-bottom:20px;border-bottom:1px solid #e5e5e5;">
              <h1 style="margin:0;font-size:24px;">{{company_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding:30px 0;">
              <p style="font-size:16px;line-height:1.8;color:#1a1a1a;">Hi {{first_name|default:Reader}},</p>
              <p style="font-size:16px;line-height:1.8;color:#1a1a1a;">
                Welcome to this month's newsletter.
              </p>
            </td>
          </tr>
          <tr>
            <td style="text-align:center;padding-top:20px;border-top:1px solid #e5e5e5;">
              <p style="font-size:12px;color:#666666;">
                <a href="{{unsubscribe_url}}" style="color:#666666;">Unsubscribe</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`

const PromoSaleHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background-color:#1F2937;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr>
      <td style="background-color:#DC2626;padding:15px;text-align:center;">
        <p style="margin:0;color:#ffffff;font-weight:bold;font-size:14px;">LIMITED TIME OFFER</p>
      </td>
    </tr>
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;">
          <tr>
            <td style="padding:40px 30px;text-align:center;">
              <h1 style="margin:0;font-size:48px;color:#DC2626;">UP TO 50% OFF</h1>
              <p style="font-size:20px;color:#333333;">Our biggest sale of the year!</p>
              <p style="font-size:16px;color:#666666;">Don't miss out, {{first_name}}!</p>
              <p style="margin:30px 0;">
                <a href="#" style="display:inline-block;background-color:#DC2626;color:#ffffff;padding:16px 40px;text-decoration:none;font-weight:bold;font-size:18px;">SHOP NOW →</a>
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
    <tr>
      <td style="text-align:center;padding:20px;">
        <a href="{{unsubscribe_url}}" style="color:#9CA3AF;font-size:12px;">Unsubscribe</a>
      </td>
    </tr>
  </table>
</body>
</html>`

const TransactReceiptHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body style="margin:0;padding:0;font-family:Arial,sans-serif;background-color:#f7f7f7;">
  <table width="100%" cellpadding="0" cellspacing="0">
    <tr>
      <td align="center" style="padding:40px 20px;">
        <table width="600" cellpadding="0" cellspacing="0" style="background-color:#ffffff;">
          <tr>
            <td style="padding:30px;text-align:center;">
              <p style="margin:0;color:#22C55E;font-size:12px;">ORDER CONFIRMED</p>
              <h1 style="margin:10px 0;font-size:24px;">Thank you for your order!</h1>
              <p style="margin:0;color:#666666;">Order #{{order_id}}</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 30px 30px;">
              <p style="margin:0 0 10px;">Hi {{first_name}},</p>
              <p style="margin:0;color:#666666;">We've received your order and it's being processed.</p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 30px;text-align:center;border-top:1px solid #e5e5e5;">
              <p style="font-size:12px;color:#666666;margin:0;">Questions? Reply to this email.</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`
