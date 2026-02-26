package core

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// =====================================
// RETRY LOGIC WITH EXPONENTIAL BACKOFF
// =====================================
// Implements intelligent retry for failed email sends with
// proper classification of temporary vs permanent failures.

// RetryPolicy defines how retries should be handled
type RetryPolicy struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	Multiplier      float64       `json:"multiplier"`
	Jitter          bool          `json:"jitter"`        // Add randomness to prevent thundering herd
	JitterPercent   float64       `json:"jitter_percent"` // 0.0 - 1.0
	RetryableCodes  []string      `json:"retryable_codes"`
	PermanentCodes  []string      `json:"permanent_codes"`
}

// DefaultRetryPolicy returns production-ready defaults
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:     5,
		InitialDelay:   1 * time.Minute,
		MaxDelay:       24 * time.Hour,
		Multiplier:     2.0,
		Jitter:         true,
		JitterPercent:  0.2, // ±20%
		RetryableCodes: []string{
			// Transient failures - should retry
			"421", // Service not available, try again later
			"450", // Mailbox busy, try again later
			"451", // Local error, try again later
			"452", // Insufficient storage, try again later
			"454", // TLS not available, try again later
			"423", // Mailbox full
			"422", // Mailbox full
		},
		PermanentCodes: []string{
			// Permanent failures - should NOT retry
			"550", // Mailbox unavailable
			"551", // User not local
			"552", // Mailbox full (permanent)
			"553", // Invalid mailbox name
			"554", // Transaction failed
			"511", // Bad destination mailbox
			"512", // Domain does not exist
			"513", // Address type invalid
			"5.1.1", // Bad destination mailbox address
			"5.1.2", // Bad destination system address
			"5.1.3", // Bad destination mailbox address syntax
			"5.1.4", // Destination mailbox address ambiguous
			"5.1.6", // Destination mailbox has moved
			"5.2.1", // Mailbox disabled
			"5.2.2", // Mailbox full
			"5.2.3", // Message too long
			"5.4.1", // No answer from host
			"5.4.4", // Unable to route
			"5.5.2", // Invalid command
			"5.7.1", // Delivery not authorized
			"5.7.26", // DKIM/SPF failure
		},
	}
}

// RetryManager handles retry logic
type RetryManager struct {
	policy RetryPolicy
}

// NewRetryManager creates a new retry manager
func NewRetryManager(policy RetryPolicy) *RetryManager {
	return &RetryManager{policy: policy}
}

// RetryResult indicates what action to take
type RetryResult struct {
	ShouldRetry bool
	IsPermanent bool
	NextRetryAt time.Time
	Reason      string
	Category    string // "transient", "permanent", "rate_limit", "connection", "auth", "unknown"
}

// ShouldRetry determines if an error should trigger a retry
func (rm *RetryManager) ShouldRetry(err error, attempt int) RetryResult {
	result := RetryResult{
		ShouldRetry: false,
		IsPermanent: false,
		Category:    "unknown",
	}

	if err == nil {
		return result
	}

	// Check if max retries exceeded
	if attempt >= rm.policy.MaxRetries {
		result.IsPermanent = true
		result.Reason = "max_retries_exceeded"
		return result
	}

	errStr := err.Error()

	// Extract SMTP error code if present
	smtpCode := extractSMTPCode(errStr)
	enhancedCode := extractEnhancedCode(errStr)

	// Check for permanent failure codes first
	for _, code := range rm.policy.PermanentCodes {
		if smtpCode == code || enhancedCode == code {
			result.IsPermanent = true
			result.ShouldRetry = false
			result.Category = "permanent"
			result.Reason = fmt.Sprintf("permanent_failure_code_%s", code)
			return result
		}
	}

	// Check for retryable codes
	for _, code := range rm.policy.RetryableCodes {
		if smtpCode == code || enhancedCode == code {
			result.ShouldRetry = true
			result.Category = "transient"
			result.Reason = fmt.Sprintf("transient_failure_code_%s", code)
			result.NextRetryAt = rm.calculateNextRetry(attempt)
			return result
		}
	}

	// Check for specific error patterns
	if isRateLimitError(errStr) {
		result.ShouldRetry = true
		result.Category = "rate_limit"
		result.Reason = "rate_limit_exceeded"
		// Use longer delay for rate limits
		result.NextRetryAt = time.Now().Add(rm.calculateDelay(attempt) * 2)
		return result
	}

	if isConnectionError(err) {
		result.ShouldRetry = true
		result.Category = "connection"
		result.Reason = "connection_failed"
		result.NextRetryAt = rm.calculateNextRetry(attempt)
		return result
	}

	if isAuthError(errStr) {
		result.IsPermanent = true
		result.Category = "auth"
		result.Reason = "authentication_failed"
		return result
	}

	if isGreylisting(errStr) {
		result.ShouldRetry = true
		result.Category = "transient"
		result.Reason = "greylisting"
		// Greylisting typically requires 15+ minutes
		result.NextRetryAt = time.Now().Add(15 * time.Minute)
		return result
	}

	if isDNSFailure(errStr) {
		result.ShouldRetry = true
		result.Category = "transient"
		result.Reason = "dns_failure"
		result.NextRetryAt = rm.calculateNextRetry(attempt)
		return result
	}

	if isTimeout(err) {
		result.ShouldRetry = true
		result.Category = "connection"
		result.Reason = "timeout"
		result.NextRetryAt = rm.calculateNextRetry(attempt)
		return result
	}

	// Default: check if it's an SMTP error with 4xx code
	if strings.HasPrefix(smtpCode, "4") {
		result.ShouldRetry = true
		result.Category = "transient"
		result.Reason = fmt.Sprintf("smtp_4xx_code_%s", smtpCode)
		result.NextRetryAt = rm.calculateNextRetry(attempt)
		return result
	}

	// Default: check if it's an SMTP error with 5xx code
	if strings.HasPrefix(smtpCode, "5") {
		result.IsPermanent = true
		result.Category = "permanent"
		result.Reason = fmt.Sprintf("smtp_5xx_code_%s", smtpCode)
		return result
	}

	// Unknown error - be conservative and don't retry
	result.Reason = "unknown_error"
	return result
}

// calculateNextRetry calculates the next retry time with exponential backoff
func (rm *RetryManager) calculateNextRetry(attempt int) time.Time {
	delay := rm.calculateDelay(attempt)
	return time.Now().Add(delay)
}

// calculateDelay calculates the delay duration for a given attempt
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: delay = initialDelay * (multiplier ^ attempt)
	delay := float64(rm.policy.InitialDelay)
	for i := 0; i < attempt; i++ {
		delay *= rm.policy.Multiplier
	}

	// Cap at max delay
	if delay > float64(rm.policy.MaxDelay) {
		delay = float64(rm.policy.MaxDelay)
	}

	// Add jitter if enabled
	if rm.policy.Jitter {
		jitterRange := delay * rm.policy.JitterPercent
		// Use attempt as seed for deterministic jitter within range
		jitter := (float64(attempt%100) / 100.0) * jitterRange * 2
		delay = delay - jitterRange + jitter
	}

	return time.Duration(delay)
}

// GetRetrySchedule returns the full retry schedule for display
func (rm *RetryManager) GetRetrySchedule() []time.Duration {
	schedule := make([]time.Duration, rm.policy.MaxRetries)
	for i := 0; i < rm.policy.MaxRetries; i++ {
		schedule[i] = rm.calculateDelay(i)
	}
	return schedule
}

// =====================================
// ERROR CLASSIFICATION HELPERS
// =====================================

var (
	// SMTP code extraction patterns for retry module
	retrySmtpCodeRe      = regexp.MustCompile(`(\d{3})\s`)
	retryEnhancedCodeRe  = regexp.MustCompile(`(\d\.\d\.\d)`)
	
	// Rate limit patterns
	rateLimitPatterns = []string{
		"rate limit",
		"too many",
		"throttl",
		"limit exceeded",
		"try again later",
		"slow down",
		"connection rate",
		"message rate",
	}
	
	// Connection error patterns
	connectionPatterns = []string{
		"connection refused",
		"connection reset",
		"connection closed",
		"broken pipe",
		"no route to host",
		"network is unreachable",
		"i/o timeout",
		"deadline exceeded",
	}
	
	// Authentication error patterns
	authPatterns = []string{
		"authentication failed",
		"auth failed",
		"invalid credentials",
		"login failed",
		"access denied",
		"535", // Authentication failure
		"5.7.8", // Authentication credentials invalid
		"5.7.0", // Authentication required
	}
	
	// Greylisting patterns
	greylistPatterns = []string{
		"greylist",
		"graylist",
		"try again later",
		"come back later",
		"temporarily rejected",
		"451 4.7.1",
	}
	
	// DNS failure patterns
	dnsPatterns = []string{
		"no such domain",
		"domain not found",
		"dns error",
		"mx lookup",
		"no mx record",
		"host not found",
		"name or service not known",
	}
)

// extractSMTPCode extracts the 3-digit SMTP code from an error
func extractSMTPCode(errStr string) string {
	matches := retrySmtpCodeRe.FindStringSubmatch(errStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractEnhancedCode extracts the enhanced status code (X.X.X) from an error
func extractEnhancedCode(errStr string) string {
	matches := retryEnhancedCodeRe.FindStringSubmatch(errStr)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// isRateLimitError checks if the error indicates rate limiting
func isRateLimitError(errStr string) bool {
	lower := strings.ToLower(errStr)
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isConnectionError checks if the error is a connection error
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	lower := strings.ToLower(errStr)
	
	for _, pattern := range connectionPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	
	// Check for net.Error
	if _, ok := err.(netError); ok {
		return true
	}
	
	// SMTP errors are handled by code parsing, not type assertion
	// The standard library smtp package doesn't export SMTPError
	
	return false
}

// isAuthError checks if the error is an authentication error
func isAuthError(errStr string) bool {
	lower := strings.ToLower(errStr)
	for _, pattern := range authPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// isGreylisting checks if the error indicates greylisting
func isGreylisting(errStr string) bool {
	lower := strings.ToLower(errStr)
	for _, pattern := range greylistPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isDNSFailure checks if the error indicates DNS failure
func isDNSFailure(errStr string) bool {
	lower := strings.ToLower(errStr)
	for _, pattern := range dnsPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// isTimeout checks if the error is a timeout
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	
	// Check for context timeout
	if err == context.DeadlineExceeded {
		return true
	}
	
	// Check for net timeout
	if netErr, ok := err.(netError); ok && netErr.Timeout() {
		return true
	}
	
	// Check string patterns
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline")
}

// netError interface for checking timeout errors
type netError interface {
	Error() string
	Timeout() bool
	Temporary() bool
}

// =====================================
// RETRY EXECUTOR
// =====================================

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// ExecuteWithRetry executes a function with retry logic
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, fn RetryableFunc) error {
	var lastErr error
	
	for attempt := 0; attempt <= rm.policy.MaxRetries; attempt++ {
		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Execute function
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		// Check if we should retry
		result := rm.ShouldRetry(err, attempt)
		
		logger.WithComponent("retry").Debug("retry check",
			"attempt", attempt,
			"should_retry", result.ShouldRetry,
			"is_permanent", result.IsPermanent,
			"reason", result.Reason,
			"category", result.Category)
		
		if result.IsPermanent || !result.ShouldRetry {
			return err
		}
		
		// Wait for next retry
		if !result.NextRetryAt.IsZero() {
			waitDuration := time.Until(result.NextRetryAt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitDuration):
				// Continue to next attempt
			}
		}
	}
	
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// =====================================
// GLOBAL RETRY MANAGER
// =====================================

var globalRetryManager *RetryManager

// InitRetryManager initializes the global retry manager
func InitRetryManager(policy RetryPolicy) {
	globalRetryManager = NewRetryManager(policy)
}

// GetRetryManager returns the global retry manager
func GetRetryManager() *RetryManager {
	if globalRetryManager == nil {
		globalRetryManager = NewRetryManager(DefaultRetryPolicy())
	}
	return globalRetryManager
}

// ShouldRetryJob determines if a job should be retried
func ShouldRetryJob(err error, attempt int) RetryResult {
	return GetRetryManager().ShouldRetry(err, attempt)
}

// ParseRetryDelay parses a delay string like "5m", "1h", etc.
func ParseRetryDelay(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("empty delay string")
	}
	
	// Try parsing as duration first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	
	// Try parsing as seconds
	if secs, err := strconv.Atoi(s); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	
	return 0, fmt.Errorf("invalid delay format: %s", s)
}
