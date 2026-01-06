package core

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/redis/go-redis/v9"
)

// DelayedQueue provides O(log N) delayed action scheduling using Redis Sorted Sets
// This replaces the inefficient SQL polling approach for large-scale automation
type DelayedQueue struct {
	client *redis.Client
	key    string
	ctx    context.Context
}

// Singleton instance
var delayedQueue *DelayedQueue

// GetDelayedQueue returns the singleton delayed queue
// Returns nil if Redis is not configured
func GetDelayedQueue() *DelayedQueue {
	return delayedQueue
}

// InitDelayedQueue initializes the Redis delayed queue
// Call this at application startup if Redis is configured
func InitDelayedQueue() error {
	cfg := config.Get()

	// Redis is optional - if not configured, fall back to SQL polling
	if cfg.RedisAddr == "" {
		logger.Info("Redis not configured, using SQL polling for delayed actions")
		return nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Warn("Redis connection failed, falling back to SQL polling", "error", err)
		return nil // Non-fatal, we fall back to SQL
	}

	delayedQueue = &DelayedQueue{
		client: client,
		key:    "sampmail:delayed_actions",
		ctx:    ctx,
	}

	logger.Info("Redis delayed queue initialized", "addr", cfg.RedisAddr)
	return nil
}

// Schedule adds an automation run to be executed at a future time
// The score is the Unix timestamp when the action should execute
func (q *DelayedQueue) Schedule(runID uint, executeAt time.Time) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("delayed queue not initialized")
	}

	score := float64(executeAt.Unix())
	member := strconv.FormatUint(uint64(runID), 10)

	err := q.client.ZAdd(q.ctx, q.key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()

	if err != nil {
		logger.Error("Failed to schedule delayed action", "run_id", runID, "error", err)
		return err
	}

	logger.Debug("Scheduled delayed action", "run_id", runID, "execute_at", executeAt)
	return nil
}

// GetDue returns all run IDs whose scheduled time has passed
// This is O(log N + M) where M is the number of due items - much faster than SQL scan
func (q *DelayedQueue) GetDue(limit int) ([]uint, error) {
	if q == nil || q.client == nil {
		return nil, nil
	}

	now := float64(time.Now().Unix())

	// ZRANGEBYSCORE returns items with score between -inf and now
	members, err := q.client.ZRangeByScore(q.ctx, q.key, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", now),
		Count: int64(limit),
	}).Result()

	if err != nil {
		logger.Error("Failed to get due actions", "error", err)
		return nil, err
	}

	runIDs := make([]uint, 0, len(members))
	for _, m := range members {
		if id, err := strconv.ParseUint(m, 10, 64); err == nil {
			runIDs = append(runIDs, uint(id))
		}
	}

	return runIDs, nil
}

// Remove removes a run from the delayed queue after processing
func (q *DelayedQueue) Remove(runID uint) error {
	if q == nil || q.client == nil {
		return nil
	}

	member := strconv.FormatUint(uint64(runID), 10)
	return q.client.ZRem(q.ctx, q.key, member).Err()
}

// RemoveBatch removes multiple runs from the queue atomically
func (q *DelayedQueue) RemoveBatch(runIDs []uint) error {
	if q == nil || q.client == nil || len(runIDs) == 0 {
		return nil
	}

	members := make([]interface{}, len(runIDs))
	for i, id := range runIDs {
		members[i] = strconv.FormatUint(uint64(id), 10)
	}

	return q.client.ZRem(q.ctx, q.key, members...).Err()
}

// Reschedule updates the execution time for a run
func (q *DelayedQueue) Reschedule(runID uint, newTime time.Time) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("delayed queue not initialized")
	}

	// Remove and re-add with new score
	if err := q.Remove(runID); err != nil {
		return err
	}
	return q.Schedule(runID, newTime)
}

// Count returns the total number of scheduled items
func (q *DelayedQueue) Count() (int64, error) {
	if q == nil || q.client == nil {
		return 0, nil
	}

	return q.client.ZCard(q.ctx, q.key).Result()
}

// CountDue returns the number of items that are ready to execute
func (q *DelayedQueue) CountDue() (int64, error) {
	if q == nil || q.client == nil {
		return 0, nil
	}

	now := fmt.Sprintf("%f", float64(time.Now().Unix()))
	return q.client.ZCount(q.ctx, q.key, "-inf", now).Result()
}

// Close closes the Redis connection
func (q *DelayedQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

// IsAvailable returns true if Redis delayed queue is configured and connected
func IsDelayedQueueAvailable() bool {
	return delayedQueue != nil && delayedQueue.client != nil
}
