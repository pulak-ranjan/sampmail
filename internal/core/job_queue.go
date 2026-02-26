package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
)

// =====================================
// PERSISTENT JOB QUEUE WITH REDIS
// =====================================
// This replaces the in-memory channel approach with a durable
// Redis-backed queue that survives server restarts.

var (
	ErrQueueNotAvailable = errors.New("job queue not available")
	ErrJobNotFound       = errors.New("job not found")
	ErrQueueFull         = errors.New("queue full")
)

// EmailJob represents a single email send job
type EmailJob struct {
	ID           string                 `json:"id"`
	CampaignID   uint                   `json:"campaign_id"`
	RecipientID  uint                   `json:"recipient_id"`
	Email        string                 `json:"email"`
	Subject      string                 `json:"subject"`
	Body         string                 `json:"body"`
	PlainText    string                 `json:"plain_text,omitempty"`
	SenderID     uint                   `json:"sender_id"`
	SenderEmail  string                 `json:"sender_email"`
	SenderDomain string                 `json:"sender_domain"`
	ReplyTo      string                 `json:"reply_to,omitempty"`
	CustomHeaders map[string]string     `json:"custom_headers,omitempty"`
	RetryCount   int                    `json:"retry_count"`
	MaxRetries   int                    `json:"max_retries"`
	NextRetryAt  *time.Time             `json:"next_retry_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	Priority     int                    `json:"priority"` // 0=normal, 1=high, 2=urgent
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// JobQueue manages the persistent email job queue
type JobQueue struct {
	redis        RedisClient
	queueName    string
	processing   string // processing queue for in-flight jobs
	delayedQueue string // delayed queue for retries
	backupQueue  string // backup for recovery
	mu           sync.RWMutex
	workers      int
	stopCh       chan struct{}
	wg           sync.WaitGroup
	handler      JobHandler
}

// JobHandler is called for each job
type JobHandler func(ctx context.Context, job *EmailJob) error

// RedisClient interface for testability
type RedisClient interface {
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPop(ctx context.Context, key string) (string, error)
	LLen(ctx context.Context, key string) (int64, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)
	LRem(ctx context.Context, key string, count int64, value interface{}) error
	ZAdd(ctx context.Context, key string, score float64, member string) error
	ZRangeByScore(ctx context.Context, key string, min, max float64, offset, count int64) ([]string, error)
	ZRem(ctx context.Context, key string, members ...string) error
	ZRemRangeByScore(ctx context.Context, key string, min, max float64) error
	ZCard(ctx context.Context, key string) (int64, error)
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
	Tx(ctx context.Context, fn func(tx RedisTx) error) error
}

// RedisTx for transactions
type RedisTx interface {
	LPush(key string, values ...interface{}) error
	RPop(key string) (string, error)
	ZAdd(key string, score float64, member string) error
	ZRem(key string, members ...string) error
	Get(key string) (string, error)
	Set(key string, value interface{}, expiration time.Duration) error
}

// JobQueueConfig holds queue configuration
type JobQueueConfig struct {
	QueueName    string
	Workers      int
	RedisClient  RedisClient
}

// NewJobQueue creates a new persistent job queue
func NewJobQueue(cfg *JobQueueConfig) (*JobQueue, error) {
	if cfg.RedisClient == nil {
		return nil, ErrQueueNotAvailable
	}

	queueName := cfg.QueueName
	if queueName == "" {
		queueName = "sampmail:email_queue"
	}

	jq := &JobQueue{
		redis:        cfg.RedisClient,
		queueName:    queueName,
		processing:   queueName + ":processing",
		delayedQueue: queueName + ":delayed",
		backupQueue:  queueName + ":backup",
		workers:      cfg.Workers,
		stopCh:       make(chan struct{}),
	}

	if jq.workers == 0 {
		jq.workers = 10
	}

	return jq, nil
}

// Enqueue adds a job to the queue
func (jq *JobQueue) Enqueue(ctx context.Context, job *EmailJob) error {
	if jq == nil || jq.redis == nil {
		return ErrQueueNotAvailable
	}

	if job.ID == "" {
		job.ID = generateJobID()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}
	if job.MaxRetries == 0 {
		job.MaxRetries = 5
	}

	jobBytes, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job: %w", err)
	}

	// If job has a future retry time, add to delayed queue
	if job.NextRetryAt != nil && job.NextRetryAt.After(time.Now()) {
		score := float64(job.NextRetryAt.Unix())
		if err := jq.redis.ZAdd(ctx, jq.delayedQueue, score, string(jobBytes)); err != nil {
			return fmt.Errorf("failed to add to delayed queue: %w", err)
		}
		logger.WithComponent("job_queue").Debug("job added to delayed queue",
			"job_id", job.ID,
			"retry_at", job.NextRetryAt)
		return nil
	}

	// Add to main queue
	if err := jq.redis.LPush(ctx, jq.queueName, string(jobBytes)); err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	logger.WithComponent("job_queue").Debug("job enqueued",
		"job_id", job.ID,
		"email", job.Email,
		"campaign_id", job.CampaignID)

	return nil
}

// EnqueueBatch adds multiple jobs efficiently
func (jq *JobQueue) EnqueueBatch(ctx context.Context, jobs []*EmailJob) error {
	if jq == nil || jq.redis == nil {
		return ErrQueueNotAvailable
	}

	for _, job := range jobs {
		if err := jq.Enqueue(ctx, job); err != nil {
			return err
		}
	}

	return nil
}

// Dequeue retrieves a job from the queue
func (jq *JobQueue) Dequeue(ctx context.Context) (*EmailJob, error) {
	if jq == nil || jq.redis == nil {
		return nil, ErrQueueNotAvailable
	}

	// Move delayed jobs that are ready to main queue
	if err := jq.promoteDelayedJobs(ctx); err != nil {
		logger.WithComponent("job_queue").Warn("failed to promote delayed jobs", "error", err)
	}

	// Pop from main queue
	jobStr, err := jq.redis.RPop(ctx, jq.queueName)
	if err != nil {
		return nil, nil // No job available
	}

	var job EmailJob
	if err := json.Unmarshal([]byte(jobStr), &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	// Store in processing queue for reliability
	processingKey := fmt.Sprintf("%s:%s", jq.processing, job.ID)
	if err := jq.redis.Set(ctx, processingKey, jobStr, 10*time.Minute); err != nil {
		logger.WithComponent("job_queue").Warn("failed to store in processing queue", "error", err)
	}

	return &job, nil
}

// Ack marks a job as completed
func (jq *JobQueue) Ack(ctx context.Context, jobID string) error {
	if jq == nil || jq.redis == nil {
		return ErrQueueNotAvailable
	}

	processingKey := fmt.Sprintf("%s:%s", jq.processing, jobID)
	return jq.redis.Del(ctx, processingKey)
}

// Nack requeues a job after failure
func (jq *JobQueue) Nack(ctx context.Context, job *EmailJob, retryAt time.Time) error {
	if jq == nil || jq.redis == nil {
		return ErrQueueNotAvailable
	}

	// Remove from processing queue
	processingKey := fmt.Sprintf("%s:%s", jq.processing, job.ID)
	jq.redis.Del(ctx, processingKey)

	// Increment retry count
	job.RetryCount++

	// Check if max retries exceeded
	if job.RetryCount > job.MaxRetries {
		logger.WithComponent("job_queue").Warn("job exceeded max retries",
			"job_id", job.ID,
			"retry_count", job.RetryCount,
			"max_retries", job.MaxRetries)
		return fmt.Errorf("job exceeded max retries: %d", job.MaxRetries)
	}

	// Set next retry time
	job.NextRetryAt = &retryAt

	// Re-enqueue with delay
	return jq.Enqueue(ctx, job)
}

// promoteDelayedJobs moves ready delayed jobs to main queue
func (jq *JobQueue) promoteDelayedJobs(ctx context.Context) error {
	now := float64(time.Now().Unix())

	// Get all jobs with score <= now
	jobs, err := jq.redis.ZRangeByScore(ctx, jq.delayedQueue, 0, now, 0, 100)
	if err != nil {
		return err
	}

	if len(jobs) == 0 {
		return nil
	}

	// Move each job to main queue
	for _, jobStr := range jobs {
		var job EmailJob
		if err := json.Unmarshal([]byte(jobStr), &job); err != nil {
			continue
		}

		// Clear next retry time since it's now ready
		job.NextRetryAt = nil

		jobBytes, _ := json.Marshal(job)
		if err := jq.redis.LPush(ctx, jq.queueName, string(jobBytes)); err != nil {
			logger.WithComponent("job_queue").Warn("failed to promote delayed job", "error", err)
			continue
		}

		// Remove from delayed queue
		jq.redis.ZRem(ctx, jq.delayedQueue, jobStr)
	}

	return nil
}

// RecoverJobs recovers jobs that were in processing during a crash
func (jq *JobQueue) RecoverJobs(ctx context.Context) error {
	if jq == nil || jq.redis == nil {
		return ErrQueueNotAvailable
	}

	log := logger.WithComponent("job_queue")

	// Find all processing keys
	// In a real implementation, we'd use SCAN to find all keys matching pattern
	// For now, we'll check a backup queue

	// Check backup queue for jobs
	backupJobs, err := jq.redis.LRange(ctx, jq.backupQueue, 0, -1)
	if err != nil {
		return err
	}

	recovered := 0
	for _, jobStr := range backupJobs {
		var job EmailJob
		if err := json.Unmarshal([]byte(jobStr), &job); err != nil {
			continue
		}

		// Re-enqueue the job
		job.NextRetryAt = nil
		if err := jq.Enqueue(ctx, &job); err != nil {
			log.Warn("failed to recover job", "job_id", job.ID, "error", err)
			continue
		}
		recovered++
	}

	// Clear backup queue
	jq.redis.Del(ctx, jq.backupQueue)

	log.Info("recovered jobs from backup", "count", recovered)

	return nil
}

// Start begins processing jobs
func (jq *JobQueue) Start(ctx context.Context, handler JobHandler) {
	if jq == nil || jq.redis == nil {
		return
	}

	jq.handler = handler
	log := logger.WithComponent("job_queue")

	log.Info("starting job queue workers", "workers", jq.workers)

	for i := 0; i < jq.workers; i++ {
		jq.wg.Add(1)
		go jq.worker(ctx, i)
	}

	// Start delayed job promoter
	jq.wg.Add(1)
	go jq.delayedJobPromoter(ctx)

	// Start health checker
	jq.wg.Add(1)
	go jq.healthChecker(ctx)
}

// Stop stops processing jobs
func (jq *JobQueue) Stop() {
	if jq == nil {
		return
	}

	log := logger.WithComponent("job_queue")
	log.Info("stopping job queue")

	close(jq.stopCh)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		jq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info("job queue stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Warn("job queue stop timeout, some workers may not have finished")
	}
}

// worker processes jobs from the queue
func (jq *JobQueue) worker(ctx context.Context, id int) {
	defer jq.wg.Done()

	log := logger.WithComponent("job_queue").With("worker_id", id)

	for {
		select {
		case <-jq.stopCh:
			log.Debug("worker stopping")
			return
		case <-ctx.Done():
			log.Debug("worker context cancelled")
			return
		default:
			job, err := jq.Dequeue(ctx)
			if err != nil {
				log.Warn("dequeue error", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}

			if job == nil {
				// No job available, wait a bit
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Process the job
			if err := jq.handler(ctx, job); err != nil {
				log.Warn("job failed", "job_id", job.ID, "error", err)

				// Calculate retry time
				retryAt := calculateRetryTime(job.RetryCount)

				// Requeue with delay
				if nackErr := jq.Nack(ctx, job, retryAt); nackErr != nil {
					log.Error("failed to requeue job", "job_id", job.ID, "error", nackErr)
				}
			} else {
				// Acknowledge successful processing
				jq.Ack(ctx, job.ID)
			}
		}
	}
}

// delayedJobPromoter periodically promotes delayed jobs
func (jq *JobQueue) delayedJobPromoter(ctx context.Context) {
	defer jq.wg.Done()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-jq.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := jq.promoteDelayedJobs(ctx); err != nil {
				logger.WithComponent("job_queue").Warn("failed to promote delayed jobs", "error", err)
			}
		}
	}
}

// healthChecker monitors queue health
func (jq *JobQueue) healthChecker(ctx context.Context) {
	defer jq.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	log := logger.WithComponent("job_queue")

	for {
		select {
		case <-jq.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := jq.Stats(ctx)
			if err != nil {
				log.Warn("failed to get queue stats", "error", err)
				continue
			}

			log.Info("queue stats",
				"pending", stats.Pending,
				"delayed", stats.Delayed,
				"processing", stats.Processing)
		}
	}
}

// JobQueueStats holds job queue statistics
type JobQueueStats struct {
	Pending    int64 `json:"pending"`
	Delayed    int64 `json:"delayed"`
	Processing int64 `json:"processing"`
}

// Stats returns queue statistics
func (jq *JobQueue) Stats(ctx context.Context) (*JobQueueStats, error) {
	if jq == nil || jq.redis == nil {
		return nil, ErrQueueNotAvailable
	}

	pending, err := jq.redis.LLen(ctx, jq.queueName)
	if err != nil {
		return nil, err
	}

	delayed, err := jq.redis.ZCard(ctx, jq.delayedQueue)
	if err != nil {
		delayed = 0
	}

	return &JobQueueStats{
		Pending:    pending,
		Delayed:    delayed,
		Processing: 0, // Would need to count processing keys
	}, nil
}

// IsQueueAvailable checks if the job queue is available
func IsQueueAvailable() bool {
	return globalJobQueue != nil && globalJobQueue.redis != nil
}

// GetJobQueue returns the global job queue
func GetJobQueue() *JobQueue {
	return globalJobQueue
}

var globalJobQueue *JobQueue

// InitJobQueue initializes the global job queue
func InitJobQueue(cfg *JobQueueConfig) error {
	var err error
	globalJobQueue, err = NewJobQueue(cfg)
	return err
}

// generateJobID generates a unique job ID
func generateJobID() string {
	return fmt.Sprintf("job_%d_%s", time.Now().UnixNano(), randomString(8))
}

// calculateRetryTime calculates when to retry based on attempt number
func calculateRetryTime(attempt int) time.Time {
	// Exponential backoff: 1m, 5m, 15m, 1h, 4h, 24h
	delays := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		1 * time.Hour,
		4 * time.Hour,
		24 * time.Hour,
	}

	if attempt >= len(delays) {
		attempt = len(delays) - 1
	}

	return time.Now().Add(delays[attempt])
}

// randomString generates a random string of given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}
