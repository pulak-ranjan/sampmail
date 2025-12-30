// Package logger provides structured logging using Go's slog package
package logger

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"
)

var (
	defaultLogger *slog.Logger
)

func init() {
	// Create a JSON handler for production
	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize source to be more readable
			if a.Key == slog.SourceKey {
				if source, ok := a.Value.Any().(*slog.Source); ok {
					a.Value = slog.StringValue(source.File + ":" + string(rune(source.Line)))
				}
			}
			return a
		},
	}

	// Use JSON in production, text in development
	var handler slog.Handler
	if os.Getenv("SAMPMAIL_ENV") == "development" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// Logger returns the default logger
func Logger() *slog.Logger {
	return defaultLogger
}

// WithComponent returns a logger with component context
func WithComponent(component string) *slog.Logger {
	return defaultLogger.With("component", component)
}

// Info logs an info message
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}

// Fatal logs an error and exits
func Fatal(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
	os.Exit(1)
}

// WithError adds error context to the logger
func WithError(err error) *slog.Logger {
	return defaultLogger.With("error", err.Error())
}

// WithContext returns a logger with request context (for tracing)
func WithContext(ctx context.Context) *slog.Logger {
	// Add request ID if available
	if reqID := ctx.Value("request_id"); reqID != nil {
		return defaultLogger.With("request_id", reqID)
	}
	return defaultLogger
}

// RequestLogger creates a logger for HTTP request logging
func RequestLogger(method, path, remoteIP string) *slog.Logger {
	return defaultLogger.With(
		"method", method,
		"path", path,
		"remote_ip", remoteIP,
	)
}

// CampaignLogger creates a logger for campaign operations
func CampaignLogger(campaignID uint) *slog.Logger {
	return WithComponent("campaign").With("campaign_id", campaignID)
}

// BounceLogger creates a logger for bounce processing
func BounceLogger() *slog.Logger {
	return WithComponent("bounce_processor")
}

// AutomationLogger creates a logger for automation workflows
func AutomationLogger(workflowID uint) *slog.Logger {
	return WithComponent("automation").With("workflow_id", workflowID)
}

// SecurityLogger creates a logger for security events
func SecurityLogger() *slog.Logger {
	return WithComponent("security")
}

// LogPanic recovers from a panic and logs it properly
func LogPanic(component string) {
	if r := recover(); r != nil {
		// Get stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stack := string(buf[:n])

		WithComponent(component).Error("panic recovered",
			"panic", r,
			"stack", stack,
		)
	}
}

// Timed returns a function to log operation duration
func Timed(operation string) func() {
	start := time.Now()
	return func() {
		defaultLogger.Info("operation completed",
			"operation", operation,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Audit logs an audit event (for compliance)
func Audit(action, actor, resource string, details map[string]any) {
	attrs := []any{
		"type", "audit",
		"action", action,
		"actor", actor,
		"resource", resource,
		"timestamp", time.Now().UTC().Format(time.RFC3339),
	}

	for k, v := range details {
		attrs = append(attrs, k, v)
	}

	defaultLogger.Info("audit event", attrs...)
}
