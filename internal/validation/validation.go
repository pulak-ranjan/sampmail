package validation

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator provides chainable validation methods
type Validator struct {
	errors []ValidationError
}

// New creates a new Validator
func New() *Validator {
	return &Validator{}
}

// Required checks that a value is not empty
func (v *Validator) Required(field, value string) *Validator {
	if strings.TrimSpace(value) == "" {
		v.errors = append(v.errors, ValidationError{field, "is required"})
	}
	return v
}

// MaxLength checks maximum string length
func (v *Validator) MaxLength(field, value string, max int) *Validator {
	if utf8.RuneCountInString(value) > max {
		v.errors = append(v.errors, ValidationError{field, fmt.Sprintf("must be at most %d characters", max)})
	}
	return v
}

// MinLength checks minimum string length
func (v *Validator) MinLength(field, value string, min int) *Validator {
	if utf8.RuneCountInString(value) < min {
		v.errors = append(v.errors, ValidationError{field, fmt.Sprintf("must be at least %d characters", min)})
	}
	return v
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Email validates email format
func (v *Validator) Email(field, value string) *Validator {
	if !emailRegex.MatchString(value) {
		v.errors = append(v.errors, ValidationError{field, "must be a valid email address"})
	}
	return v
}

// Sanitizers - using bluemonday for robust HTML sanitization

var (
	// StrictPolicy strips ALL HTML tags
	StrictPolicy = bluemonday.StrictPolicy()
	
	// UGCPolicy allows safe HTML (for user-generated content)
	UGCPolicy = bluemonday.UGCPolicy()
	
	// EmailPolicy allows safe HTML for email content
	EmailPolicy = createEmailPolicy()
)

// createEmailPolicy creates a policy suitable for email HTML
func createEmailPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	
	// Allow common email HTML elements
	p.AllowElements("p", "br", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowElements("strong", "b", "em", "i", "u", "s", "strike")
	p.AllowElements("ul", "ol", "li", "dl", "dt", "dd")
	p.AllowElements("table", "thead", "tbody", "tr", "th", "td", "caption")
	p.AllowElements("blockquote", "pre", "code")
	p.AllowElements("hr")
	
	// Allow images with src (for tracking pixels and content)
	p.AllowAttrs("src", "alt", "width", "height", "style").OnElements("img")
	
	// Allow links but require http/https/mailto
	p.AllowAttrs("href", "title", "target").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	
	// Allow common styling attributes
	p.AllowAttrs("style", "class", "id").Globally()
	p.AllowAttrs("align", "valign", "bgcolor", "border", "cellpadding", "cellspacing").OnElements("table", "td", "th", "tr")
	p.AllowAttrs("width", "height").OnElements("table", "td", "th", "img")
	
	// Allow data attributes for tracking
	p.AllowDataAttributes()
	
	return p
}

// SanitizeStrict removes ALL HTML, returns plain text
func SanitizeStrict(input string) string {
	return StrictPolicy.Sanitize(input)
}

// SanitizeUGC sanitizes user-generated content (comments, etc.)
func SanitizeUGC(input string) string {
	return UGCPolicy.Sanitize(input)
}

// SanitizeEmail sanitizes HTML for email content
func SanitizeEmail(input string) string {
	return EmailPolicy.Sanitize(input)
}

// NoScriptTags validates that content doesn't contain dangerous scripts
// Uses bluemonday for robust detection
func (v *Validator) NoScriptTags(field, value string) *Validator {
	// Sanitize and compare - if they differ significantly, there was dangerous content
	sanitized := StrictPolicy.Sanitize(value)
	
	// Check if significant content was removed (potential attack)
	// We use a simple heuristic: if the sanitized version is much shorter
	// and the original contained script-like patterns
	lowerValue := strings.ToLower(value)
	
	dangerousPatterns := []string{
		"<script",
		"javascript:",
		"vbscript:",
		"data:text/html",
		"data:application/x-javascript",
		"expression(",
		"url(javascript",
		"@import",
	}
	
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerValue, pattern) {
			v.errors = append(v.errors, ValidationError{field, "contains potentially dangerous content"})
			return v
		}
	}
	
	// Also check for event handlers
	eventHandlerRegex := regexp.MustCompile(`(?i)\s+on\w+\s*=`)
	if eventHandlerRegex.MatchString(value) {
		v.errors = append(v.errors, ValidationError{field, "contains potentially dangerous event handlers"})
		return v
	}
	
	// Check if content looks like it might have had malicious content stripped
	if len(value) > 100 && len(sanitized) < len(value)/2 {
		// Large content reduction might indicate stripped malicious content
		if strings.Contains(lowerValue, "<") && strings.Contains(lowerValue, ">") {
			v.errors = append(v.errors, ValidationError{field, "contains invalid HTML"})
			return v
		}
	}
	
	return v
}

// SafeHTML checks and returns sanitized HTML for email content
func (v *Validator) SafeHTML(field, value string, maxLen int) (string, *Validator) {
	if utf8.RuneCountInString(value) > maxLen {
		v.errors = append(v.errors, ValidationError{field, fmt.Sprintf("must be at most %d characters", maxLen)})
		return "", v
	}
	
	sanitized := SanitizeEmail(value)
	return sanitized, v
}

// Valid returns true if no validation errors occurred
func (v *Validator) Valid() bool {
	return len(v.errors) == 0
}

// Errors returns all validation errors
func (v *Validator) Errors() []ValidationError {
	return v.errors
}

// AddError adds a custom validation error
func (v *Validator) AddError(field, message string) {
	v.errors = append(v.errors, ValidationError{field, message})
}

// Domain validation

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.-]*[a-zA-Z0-9]\.[a-zA-Z]{2,}$`)

// Domain validates a domain name
func (v *Validator) Domain(field, value string) *Validator {
	if len(value) > 253 {
		v.errors = append(v.errors, ValidationError{field, "domain name too long"})
		return v
	}
	if !domainRegex.MatchString(value) {
		v.errors = append(v.errors, ValidationError{field, "invalid domain format"})
	}
	return v
}

// IP validation

var ipv4Regex = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)

// IPv4 validates an IPv4 address
func (v *Validator) IPv4(field, value string) *Validator {
	if !ipv4Regex.MatchString(value) {
		v.errors = append(v.errors, ValidationError{field, "invalid IPv4 address"})
		return v
	}
	
	// Also check octets are in valid range
	parts := strings.Split(value, ".")
	for _, part := range parts {
		var num int
		fmt.Sscanf(part, "%d", &num)
		if num < 0 || num > 255 {
			v.errors = append(v.errors, ValidationError{field, "IPv4 octet out of range"})
			return v
		}
	}
	
	return v
}

// Password validation

// Password validates password strength
func (v *Validator) Password(field, value string) *Validator {
	if len(value) < 8 {
		v.errors = append(v.errors, ValidationError{field, "must be at least 8 characters"})
	}
	
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(value)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(value)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(value)
	
	if !hasUpper || !hasLower || !hasDigit {
		v.errors = append(v.errors, ValidationError{field, "must contain uppercase, lowercase, and digits"})
	}
	
	return v
}

// URL validation

// URL validates a URL
func (v *Validator) URL(field, value string) *Validator {
	if value == "" {
		return v // Empty is OK, use Required() to check for presence
	}
	
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		v.errors = append(v.errors, ValidationError{field, "must start with http:// or https://"})
	}
	
	return v
}

// Alphanumeric validation

var alphanumericRegex = regexp.MustCompile(`^[a-zA-Z0-9]+$`)

// Alphanumeric validates that a value contains only letters and numbers
func (v *Validator) Alphanumeric(field, value string) *Validator {
	if !alphanumericRegex.MatchString(value) {
		v.errors = append(v.errors, ValidationError{field, "must contain only letters and numbers"})
	}
	return v
}

// InList validates that a value is in a list of allowed values
func (v *Validator) InList(field, value string, allowed []string) *Validator {
	for _, a := range allowed {
		if value == a {
			return v
		}
	}
	v.errors = append(v.errors, ValidationError{field, fmt.Sprintf("must be one of: %v", allowed)})
	return v
}
