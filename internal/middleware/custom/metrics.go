package custom

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP Metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sampmail_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	httpRequestsInFlight = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sampmail_http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
	)

	// Campaign Metrics
	campaignEmailsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_campaign_emails_sent_total",
			Help: "Total number of campaign emails sent",
		},
		[]string{"campaign_id", "status"},
	)

	campaignDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sampmail_campaign_duration_seconds",
			Help:    "Campaign processing duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800, 3600},
		},
		[]string{"campaign_id"},
	)

	// Database Metrics
	dbQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sampmail_db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	dbConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sampmail_db_connections_open",
			Help: "Number of open database connections",
		},
	)

	dbConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "sampmail_db_connections_idle",
			Help: "Number of idle database connections",
		},
	)

	// Queue Metrics
	queueSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sampmail_queue_size",
			Help: "Current size of various queues",
		},
		[]string{"queue_name"},
	)

	// Bounce Metrics
	bounceEventsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_bounce_events_total",
			Help: "Total number of bounce events",
		},
		[]string{"type", "domain"},
	)

	// Warmup Metrics
	warmupEmailsSent = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_warmup_emails_sent_total",
			Help: "Total warmup emails sent",
		},
		[]string{"domain", "day"},
	)

	// Rate Limiting Metrics
	rateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_rate_limit_hits_total",
			Help: "Number of rate limit hits",
		},
		[]string{"limiter", "ip"},
	)

	// External Service Metrics
	externalServiceRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sampmail_external_service_requests_total",
			Help: "Total requests to external services",
		},
		[]string{"service", "status"},
	)

	externalServiceDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sampmail_external_service_duration_seconds",
			Help:    "External service request duration",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"service"},
	)
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware records HTTP metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics endpoint itself
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start).Seconds()

		// Normalize path to avoid high cardinality
		path := normalizePath(r.URL.Path)

		httpRequestsTotal.WithLabelValues(
			r.Method,
			path,
			strconv.Itoa(wrapped.statusCode),
		).Inc()

		httpRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// normalizePath removes IDs from paths to reduce cardinality
func normalizePath(path string) string {
	// Simple normalization - in production use a proper router-aware solution
	// Replace numeric IDs with :id
	normalized := path

	// Common patterns
	patterns := map[string]string{
		"/api/domains/":    "/api/domains/:id",
		"/api/campaigns/":  "/api/campaigns/:id",
		"/api/templates/":  "/api/templates/:id",
		"/api/contacts/":   "/api/contacts/:id",
		"/api/track/open/": "/api/track/open/:id",
		"/api/track/click/":"/api/track/click/:id",
	}

	for prefix, replacement := range patterns {
		if len(path) > len(prefix) && path[:len(prefix)] == prefix {
			return replacement
		}
	}

	return normalized
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordCampaignEmail records a campaign email metric
func RecordCampaignEmail(campaignID string, status string) {
	campaignEmailsSent.WithLabelValues(campaignID, status).Inc()
}

// RecordCampaignDuration records campaign processing duration
func RecordCampaignDuration(campaignID string, duration time.Duration) {
	campaignDuration.WithLabelValues(campaignID).Observe(duration.Seconds())
}

// RecordDBQuery records a database query metric
func RecordDBQuery(operation string, duration time.Duration) {
	dbQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// UpdateDBConnections updates database connection metrics
func UpdateDBConnections(open, idle int) {
	dbConnectionsOpen.Set(float64(open))
	dbConnectionsIdle.Set(float64(idle))
}

// RecordBounceEvent records a bounce event
func RecordBounceEvent(bounceType, domain string) {
	bounceEventsTotal.WithLabelValues(bounceType, domain).Inc()
}

// RecordWarmupEmail records a warmup email sent
func RecordWarmupEmail(domain string, day int) {
	warmupEmailsSent.WithLabelValues(domain, strconv.Itoa(day)).Inc()
}

// RecordRateLimitHit records a rate limit hit
func RecordRateLimitHit(limiter, ip string) {
	rateLimitHits.WithLabelValues(limiter, ip).Inc()
}

// RecordExternalServiceRequest records external service metrics
func RecordExternalServiceRequest(service, status string, duration time.Duration) {
	externalServiceRequests.WithLabelValues(service, status).Inc()
	externalServiceDuration.WithLabelValues(service).Observe(duration.Seconds())
}

// SetQueueSize sets the current queue size
func SetQueueSize(queueName string, size int) {
	queueSize.WithLabelValues(queueName).Set(float64(size))
}
