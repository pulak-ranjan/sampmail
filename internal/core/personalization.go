package core

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/models"
)

// =====================================
// TEMPLATE VARIABLE SYSTEM
// =====================================

// PersonalizationEngine handles template variable replacement
type PersonalizationEngine struct {
	// Built-in variables
	builtInVars map[string]func(ctx *PersonalizationContext) string
	
	// Custom formatters
	formatters map[string]Formatter
}

// PersonalizationContext holds data for variable replacement
type PersonalizationContext struct {
	Contact      *models.ContactV2
	Campaign     *models.CampaignV2
	Organization *models.Organization
	CustomData   map[string]interface{}
	
	// Runtime data
	UnsubscribeURL string
	TrackingPixel  string
	WebViewURL     string
	CurrentDate    time.Time
}

// Formatter is a function that formats a value
type Formatter func(value interface{}, args ...string) string

// NewPersonalizationEngine creates a new personalization engine
func NewPersonalizationEngine() *PersonalizationEngine {
	pe := &PersonalizationEngine{
		builtInVars: make(map[string]func(ctx *PersonalizationContext) string),
		formatters:  make(map[string]Formatter),
	}
	
	// Register built-in variables
	pe.registerBuiltInVars()
	pe.registerFormatters()
	
	return pe
}

// registerBuiltInVars registers all built-in template variables
func (pe *PersonalizationEngine) registerBuiltInVars() {
	// Contact fields
	pe.builtInVars["email"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.Email
		}
		return ""
	}
	pe.builtInVars["first_name"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.FirstName
		}
		return ""
	}
	pe.builtInVars["last_name"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.LastName
		}
		return ""
	}
	pe.builtInVars["full_name"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			name := strings.TrimSpace(ctx.Contact.FirstName + " " + ctx.Contact.LastName)
			if name == "" {
				return ctx.Contact.Email
			}
			return name
		}
		return ""
	}
	pe.builtInVars["name"] = pe.builtInVars["full_name"] // Alias
	
	pe.builtInVars["company"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.Company
		}
		return ""
	}
	pe.builtInVars["job_title"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.JobTitle
		}
		return ""
	}
	pe.builtInVars["phone"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.Phone
		}
		return ""
	}
	
	// Address fields
	pe.builtInVars["city"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.City
		}
		return ""
	}
	pe.builtInVars["state"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.State
		}
		return ""
	}
	pe.builtInVars["country"] = func(ctx *PersonalizationContext) string {
		if ctx.Contact != nil {
			return ctx.Contact.Country
		}
		return ""
	}
	
	// Organization/Sender fields
	pe.builtInVars["company_name"] = func(ctx *PersonalizationContext) string {
		if ctx.Organization != nil {
			return ctx.Organization.Name
		}
		return ""
	}
	pe.builtInVars["sender_name"] = func(ctx *PersonalizationContext) string {
		// From campaign sender
		return ""
	}
	
	// Campaign fields
	pe.builtInVars["subject"] = func(ctx *PersonalizationContext) string {
		if ctx.Campaign != nil {
			return ctx.Campaign.Subject
		}
		return ""
	}
	
	// System variables
	pe.builtInVars["unsubscribe_url"] = func(ctx *PersonalizationContext) string {
		return ctx.UnsubscribeURL
	}
	pe.builtInVars["unsubscribe_link"] = func(ctx *PersonalizationContext) string {
		return fmt.Sprintf(`<a href="%s">Unsubscribe</a>`, ctx.UnsubscribeURL)
	}
	pe.builtInVars["tracking_pixel"] = func(ctx *PersonalizationContext) string {
		return ctx.TrackingPixel
	}
	pe.builtInVars["webview_url"] = func(ctx *PersonalizationContext) string {
		return ctx.WebViewURL
	}
	pe.builtInVars["view_in_browser"] = func(ctx *PersonalizationContext) string {
		return fmt.Sprintf(`<a href="%s">View in browser</a>`, ctx.WebViewURL)
	}
	
	// Date/Time
	pe.builtInVars["current_date"] = func(ctx *PersonalizationContext) string {
		return ctx.CurrentDate.Format("January 2, 2006")
	}
	pe.builtInVars["current_year"] = func(ctx *PersonalizationContext) string {
		return ctx.CurrentDate.Format("2006")
	}
	pe.builtInVars["current_month"] = func(ctx *PersonalizationContext) string {
		return ctx.CurrentDate.Format("January")
	}
	pe.builtInVars["current_day"] = func(ctx *PersonalizationContext) string {
		return ctx.CurrentDate.Format("Monday")
	}
}

// registerFormatters registers value formatters
func (pe *PersonalizationEngine) registerFormatters() {
	// String formatters
	pe.formatters["upper"] = func(v interface{}, args ...string) string {
		return strings.ToUpper(fmt.Sprint(v))
	}
	pe.formatters["lower"] = func(v interface{}, args ...string) string {
		return strings.ToLower(fmt.Sprint(v))
	}
	pe.formatters["title"] = func(v interface{}, args ...string) string {
		return strings.Title(strings.ToLower(fmt.Sprint(v)))
	}
	pe.formatters["capitalize"] = func(v interface{}, args ...string) string {
		s := fmt.Sprint(v)
		if len(s) == 0 {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
	pe.formatters["truncate"] = func(v interface{}, args ...string) string {
		s := fmt.Sprint(v)
		maxLen := 50
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &maxLen)
		}
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen-3] + "..."
	}
	
	// Default value
	pe.formatters["default"] = func(v interface{}, args ...string) string {
		s := fmt.Sprint(v)
		if s == "" && len(args) > 0 {
			return args[0]
		}
		return s
	}
}

// Variable patterns
var (
	// Matches {{variable}}, {{variable|filter}}, {{variable|filter:arg}}
	varPattern = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)*)\s*(?:\|\s*([a-zA-Z_]+)(?::([^}]+))?)?\s*\}\}`)
	
	// Legacy pattern: {variable}
	legacyPattern = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_]*)\}`)
	
	// Conditional pattern: {{#if variable}}...{{/if}}
	conditionalPattern = regexp.MustCompile(`\{\{#if\s+([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}(.*?)\{\{/if\}\}`)
	
	// Else pattern: {{#if variable}}...{{else}}...{{/if}}
	elsePattern = regexp.MustCompile(`\{\{#if\s+([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}(.*?)\{\{else\}\}(.*?)\{\{/if\}\}`)
	
	// Loop pattern: {{#each items}}...{{/each}}
	loopPattern = regexp.MustCompile(`\{\{#each\s+([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}(.*?)\{\{/each\}\}`)
)

// Render processes a template with personalization context
func (pe *PersonalizationEngine) Render(content string, ctx *PersonalizationContext) (string, error) {
	if ctx.CurrentDate.IsZero() {
		ctx.CurrentDate = time.Now()
	}
	
	result := content
	
	// Process conditionals with else first
	result = elsePattern.ReplaceAllStringFunc(result, func(match string) string {
		groups := elsePattern.FindStringSubmatch(match)
		if len(groups) < 4 {
			return match
		}
		varName := groups[1]
		trueContent := groups[2]
		falseContent := groups[3]
		
		value := pe.getVariableValue(varName, ctx)
		if pe.isTruthy(value) {
			return trueContent
		}
		return falseContent
	})
	
	// Process simple conditionals
	result = conditionalPattern.ReplaceAllStringFunc(result, func(match string) string {
		groups := conditionalPattern.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}
		varName := groups[1]
		innerContent := groups[2]
		
		value := pe.getVariableValue(varName, ctx)
		if pe.isTruthy(value) {
			return innerContent
		}
		return ""
	})
	
	// Process new-style variables {{variable}}
	result = varPattern.ReplaceAllStringFunc(result, func(match string) string {
		groups := varPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		
		varName := groups[1]
		filter := ""
		filterArg := ""
		if len(groups) > 2 {
			filter = groups[2]
		}
		if len(groups) > 3 {
			filterArg = groups[3]
		}
		
		value := pe.getVariableValue(varName, ctx)
		
		// Apply filter if specified
		if filter != "" {
			if formatter, ok := pe.formatters[filter]; ok {
				var args []string
				if filterArg != "" {
					args = []string{filterArg}
				}
				return formatter(value, args...)
			}
		}
		
		return fmt.Sprint(value)
	})
	
	// Process legacy variables {variable}
	result = legacyPattern.ReplaceAllStringFunc(result, func(match string) string {
		groups := legacyPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		varName := groups[1]
		value := pe.getVariableValue(varName, ctx)
		return fmt.Sprint(value)
	})
	
	return result, nil
}

// getVariableValue retrieves a variable value from context
func (pe *PersonalizationEngine) getVariableValue(varName string, ctx *PersonalizationContext) interface{} {
	// Check built-in variables first
	if fn, ok := pe.builtInVars[varName]; ok {
		return fn(ctx)
	}
	
	// Check custom fields
	if ctx.Contact != nil && ctx.Contact.CustomFields != nil {
		if val, ok := ctx.Contact.CustomFields[varName]; ok {
			return val
		}
	}
	
	// Check custom data
	if ctx.CustomData != nil {
		// Support dot notation: custom.field
		parts := strings.Split(varName, ".")
		current := ctx.CustomData
		for i, part := range parts {
			if val, ok := current[part]; ok {
				if i == len(parts)-1 {
					return val
				}
				if nested, ok := val.(map[string]interface{}); ok {
					current = nested
				} else {
					break
				}
			}
		}
	}
	
	return ""
}

// isTruthy checks if a value is truthy
func (pe *PersonalizationEngine) isTruthy(value interface{}) bool {
	if value == nil {
		return false
	}
	
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != ""
	case int, int64, float64:
		return v != 0
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return fmt.Sprint(v) != ""
	}
}

// ExtractVariables finds all variables used in a template
func (pe *PersonalizationEngine) ExtractVariables(content string) []string {
	seen := make(map[string]bool)
	var vars []string
	
	// Extract from new-style
	matches := varPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	
	// Extract from legacy
	matches = legacyPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	
	// Extract from conditionals
	matches = conditionalPattern.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		if len(m) > 1 && !seen[m[1]] {
			seen[m[1]] = true
			vars = append(vars, m[1])
		}
	}
	
	return vars
}

// ValidateTemplate checks if a template is valid
func (pe *PersonalizationEngine) ValidateTemplate(content string) error {
	// Check for unclosed tags
	opens := strings.Count(content, "{{#if")
	closes := strings.Count(content, "{{/if}}")
	if opens != closes {
		return fmt.Errorf("unclosed {{#if}} tags: %d opens, %d closes", opens, closes)
	}
	
	opens = strings.Count(content, "{{#each")
	closes = strings.Count(content, "{{/each}}")
	if opens != closes {
		return fmt.Errorf("unclosed {{#each}} tags: %d opens, %d closes", opens, closes)
	}
	
	// Try to render with empty context
	_, err := pe.Render(content, &PersonalizationContext{})
	return err
}

// PreviewWithSampleData renders with sample data for preview
func (pe *PersonalizationEngine) PreviewWithSampleData(content string) (string, error) {
	ctx := &PersonalizationContext{
		Contact: &models.ContactV2{
			Email:     "john.doe@example.com",
			FirstName: "John",
			LastName:  "Doe",
			Company:   "Acme Inc",
			JobTitle:  "Marketing Manager",
			City:      "New York",
			State:     "NY",
			Country:   "USA",
		},
		Organization: &models.Organization{
			Name: "Your Company",
		},
		UnsubscribeURL: "https://example.com/unsubscribe/sample",
		WebViewURL:     "https://example.com/view/sample",
		TrackingPixel:  `<img src="https://example.com/track/sample" width="1" height="1" />`,
		CurrentDate:    time.Now(),
	}
	
	return pe.Render(content, ctx)
}

// =====================================
// TEMPLATE RENDERING SERVICE
// =====================================

// TemplateRenderService handles template compilation and rendering
type TemplateRenderService struct {
	personalization *PersonalizationEngine
}

// NewTemplateRenderService creates a new render service
func NewTemplateRenderService() *TemplateRenderService {
	return &TemplateRenderService{
		personalization: NewPersonalizationEngine(),
	}
}

// RenderCampaignEmail renders a complete email for a recipient
func (trs *TemplateRenderService) RenderCampaignEmail(
	campaign *models.CampaignV2,
	recipient *models.CampaignRecipientV2,
	contact *models.ContactV2,
	org *models.Organization,
	baseURL string,
) (*RenderedEmail, error) {
	
	// Build context
	ctx := &PersonalizationContext{
		Contact:      contact,
		Campaign:     campaign,
		Organization: org,
		CurrentDate:  time.Now(),
	}
	
	// Add personalization data from recipient
	if recipient.PersonalizationData != nil {
		ctx.CustomData = recipient.PersonalizationData
	}
	
	// Generate tracking URLs
	ctx.UnsubscribeURL = fmt.Sprintf("%s/api/v2/unsubscribe/%d/%d", baseURL, campaign.ID, recipient.ID)
	ctx.WebViewURL = fmt.Sprintf("%s/api/v2/view/%d/%d", baseURL, campaign.ID, recipient.ID)
	ctx.TrackingPixel = fmt.Sprintf(`<img src="%s/api/v2/track/open/%d/%d" alt="" width="1" height="1" style="display:none" />`, 
		baseURL, campaign.ID, recipient.ID)
	
	// Render subject
	subject, err := trs.personalization.Render(campaign.Subject, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render subject: %w", err)
	}
	
	// Render preview text
	previewText, _ := trs.personalization.Render(campaign.PreviewText, ctx)
	
	// Render HTML body
	htmlBody, err := trs.personalization.Render(campaign.HTMLContent, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to render HTML: %w", err)
	}
	
	// Add tracking pixel before </body>
	htmlBody = strings.Replace(htmlBody, "</body>", ctx.TrackingPixel+"</body>", 1)
	
	// Rewrite links for click tracking
	htmlBody = trs.rewriteLinksForTracking(htmlBody, baseURL, campaign.ID, recipient.ID)
	
	// Render plain text
	textBody, _ := trs.personalization.Render(campaign.TextContent, ctx)
	
	return &RenderedEmail{
		Subject:     subject,
		PreviewText: previewText,
		HTMLBody:    htmlBody,
		TextBody:    textBody,
	}, nil
}

// RenderedEmail contains the final rendered email content
type RenderedEmail struct {
	Subject     string
	PreviewText string
	HTMLBody    string
	TextBody    string
}

// rewriteLinksForTracking wraps all links with click tracking
func (trs *TemplateRenderService) rewriteLinksForTracking(html string, baseURL string, campaignID, recipientID uint) string {
	linkPattern := regexp.MustCompile(`href=["']([^"']+)["']`)
	
	return linkPattern.ReplaceAllStringFunc(html, func(match string) string {
		groups := linkPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		
		originalURL := groups[1]
		
		// Skip unsubscribe and special links
		if strings.Contains(originalURL, "unsubscribe") ||
			strings.HasPrefix(originalURL, "#") ||
			strings.HasPrefix(originalURL, "mailto:") ||
			strings.HasPrefix(originalURL, "tel:") {
			return match
		}
		
		// Create tracking URL
		trackingURL := fmt.Sprintf("%s/api/v2/track/click/%d/%d?url=%s",
			baseURL, campaignID, recipientID, template.URLQueryEscaper(originalURL))
		
		return fmt.Sprintf(`href="%s"`, trackingURL)
	})
}

// =====================================
// AVAILABLE VARIABLES DOCUMENTATION
// =====================================

// GetAvailableVariables returns documentation of all available variables
func GetAvailableVariables() []VariableDoc {
	return []VariableDoc{
		// Contact
		{Name: "email", Category: "Contact", Description: "Contact's email address"},
		{Name: "first_name", Category: "Contact", Description: "Contact's first name"},
		{Name: "last_name", Category: "Contact", Description: "Contact's last name"},
		{Name: "full_name", Category: "Contact", Description: "Contact's full name (first + last)"},
		{Name: "name", Category: "Contact", Description: "Alias for full_name"},
		{Name: "company", Category: "Contact", Description: "Contact's company name"},
		{Name: "job_title", Category: "Contact", Description: "Contact's job title"},
		{Name: "phone", Category: "Contact", Description: "Contact's phone number"},
		{Name: "city", Category: "Contact", Description: "Contact's city"},
		{Name: "state", Category: "Contact", Description: "Contact's state/province"},
		{Name: "country", Category: "Contact", Description: "Contact's country"},
		
		// Organization
		{Name: "company_name", Category: "Organization", Description: "Your company/organization name"},
		
		// System
		{Name: "unsubscribe_url", Category: "System", Description: "URL to unsubscribe"},
		{Name: "unsubscribe_link", Category: "System", Description: "HTML link to unsubscribe"},
		{Name: "webview_url", Category: "System", Description: "URL to view email in browser"},
		{Name: "view_in_browser", Category: "System", Description: "HTML link to view in browser"},
		
		// Date/Time
		{Name: "current_date", Category: "Date", Description: "Current date (January 2, 2006)"},
		{Name: "current_year", Category: "Date", Description: "Current year (2006)"},
		{Name: "current_month", Category: "Date", Description: "Current month name"},
		{Name: "current_day", Category: "Date", Description: "Current day name"},
	}
}

// VariableDoc documents a template variable
type VariableDoc struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Example     string `json:"example,omitempty"`
}

// GetAvailableFilters returns documentation of available filters
func GetAvailableFilters() []FilterDoc {
	return []FilterDoc{
		{Name: "upper", Description: "Convert to uppercase", Example: "{{name|upper}}"},
		{Name: "lower", Description: "Convert to lowercase", Example: "{{name|lower}}"},
		{Name: "title", Description: "Convert to title case", Example: "{{name|title}}"},
		{Name: "capitalize", Description: "Capitalize first letter", Example: "{{name|capitalize}}"},
		{Name: "truncate", Description: "Truncate to length", Example: "{{description|truncate:50}}"},
		{Name: "default", Description: "Default value if empty", Example: "{{name|default:Friend}}"},
	}
}

// FilterDoc documents a template filter
type FilterDoc struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

// Global personalization engine instance
var globalPersonalizationEngine = NewPersonalizationEngine()

// RenderTemplate is a convenience function
func RenderTemplate(content string, ctx *PersonalizationContext) (string, error) {
	return globalPersonalizationEngine.Render(content, ctx)
}

// PersonalizeEmail is used by the campaign sender
func PersonalizeEmail(body string, contact *models.ContactV2, customData map[string]interface{}) (string, error) {
	ctx := &PersonalizationContext{
		Contact:     contact,
		CustomData:  customData,
		CurrentDate: time.Now(),
	}
	return globalPersonalizationEngine.Render(body, ctx)
}
