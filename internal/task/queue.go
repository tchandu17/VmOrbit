package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/internal/domain/model"
)

// ─────────────────────────────────────────────────────────────────────────────
// Redis key helpers
// ─────────────────────────────────────────────────────────────────────────────

const (
	// keyQueue is a sorted set per priority level.
	// We use one list per priority so BRPOP can honour priority order.
	keyQueueFmt  = "task:queue:%d"   // task:queue:1 … task:queue:10
	keyLockFmt   = "task:lock:%s"    // task:lock:{task_id}
	keyCancelFmt = "task:cancel:%s"  // task:cancel:{task_id}
	keyStateFmt  = "task:state:%s"   // task:state:{task_id}  (HASH)
	keyLogsFmt   = "task:logs:%s"    // task:logs:{task_id}   (LIST)
)

// priorityQueues returns the Redis list keys in priority order (1 = highest).
// BRPOP checks keys left-to-right, so listing them in ascending priority order
// means higher-priority tasks are always dequeued first.
func priorityQueues() []string {
	keys := make([]string, 10)
	for i := 0; i < 10; i++ {
		keys[i] = fmt.Sprintf(keyQueueFmt, i+1)
	}
	return keys
}

// ─────────────────────────────────────────────────────────────────────────────
// RedisQueue
// ─────────────────────────────────────────────────────────────────────────────

// RedisQueue wraps a Redis client and exposes queue, lock, cancel, progress,
// and log operations used by the task engine.
type RedisQueue struct {
	client      *redis.Client
	maxLogLines int
}

// NewRedisQueue creates a RedisQueue.
func NewRedisQueue(client *redis.Client, maxLogLines int) *RedisQueue {
	return &RedisQueue{client: client, maxLogLines: maxLogLines}
}

// ── Enqueue ──────────────────────────────────────────────────────────────────

// Enqueue pushes a task ID onto the appropriate priority list.
// Priority is clamped to [1, 10].
func (q *RedisQueue) Enqueue(ctx context.Context, taskID string, priority int) error {
	if priority < 1 {
		priority = 1
	}
	if priority > 10 {
		priority = 10
	}
	key := fmt.Sprintf(keyQueueFmt, priority)
	return q.client.LPush(ctx, key, taskID).Err()
}

// EnqueueDelayed schedules a task to be pushed onto the queue after delay.
// It uses a non-blocking goroutine with time.AfterFunc — suitable for retry
// backoff. The context is intentionally not propagated so a shutdown does not
// silently drop a retry.
func (q *RedisQueue) EnqueueDelayed(taskID string, priority int, delay time.Duration) {
	time.AfterFunc(delay, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = q.Enqueue(ctx, taskID, priority)
	})
}

// ── Dequeue ──────────────────────────────────────────────────────────────────

// Dequeue blocks until a task ID is available across all priority queues.
// It returns ("", nil) when the context is cancelled.
func (q *RedisQueue) Dequeue(ctx context.Context) (string, error) {
	keys := priorityQueues()
	result, err := q.client.BRPop(ctx, 0, keys...).Result()
	if err != nil {
		if err == context.Canceled || err == context.DeadlineExceeded || err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("queue dequeue: %w", err)
	}
	// BRPop returns [key, value]
	if len(result) < 2 {
		return "", nil
	}
	return result[1], nil
}

// ── Worker lock (at-most-once execution) ─────────────────────────────────────

// AcquireLock tries to set a NX lock for the given task.
// Returns true if the lock was acquired (this worker owns the task).
func (q *RedisQueue) AcquireLock(ctx context.Context, taskID string, ttl time.Duration) (bool, error) {
	key := fmt.Sprintf(keyLockFmt, taskID)
	ok, err := q.client.SetNX(ctx, key, "1", ttl).Result()
	return ok, err
}

// ReleaseLock removes the worker lock for a task.
func (q *RedisQueue) ReleaseLock(ctx context.Context, taskID string) error {
	return q.client.Del(ctx, fmt.Sprintf(keyLockFmt, taskID)).Err()
}

// RefreshLock extends the lock TTL while a long-running task is still active.
func (q *RedisQueue) RefreshLock(ctx context.Context, taskID string, ttl time.Duration) error {
	return q.client.Expire(ctx, fmt.Sprintf(keyLockFmt, taskID), ttl).Err()
}

// ── Cancellation ─────────────────────────────────────────────────────────────

// SignalCancel sets the cancellation flag for a task.
// The TTL should be at least as long as the task's maximum execution time.
func (q *RedisQueue) SignalCancel(ctx context.Context, taskID string, ttl time.Duration) error {
	return q.client.Set(ctx, fmt.Sprintf(keyCancelFmt, taskID), "1", ttl).Err()
}

// IsCancelled returns true if a cancellation signal exists for the task.
func (q *RedisQueue) IsCancelled(ctx context.Context, taskID string) bool {
	n, err := q.client.Exists(ctx, fmt.Sprintf(keyCancelFmt, taskID)).Result()
	return err == nil && n > 0
}

// ClearCancel removes the cancellation flag (called after the task is done).
func (q *RedisQueue) ClearCancel(ctx context.Context, taskID string) error {
	return q.client.Del(ctx, fmt.Sprintf(keyCancelFmt, taskID)).Err()
}

// ── Live state (HASH) ─────────────────────────────────────────────────────────

// SetState writes a live snapshot of task state to a Redis HASH.
// This is the fast-path read for the GET /tasks/{id} endpoint — no DB hit.
func (q *RedisQueue) SetState(ctx context.Context, taskID string, fields map[string]interface{}, ttl time.Duration) error {
	key := fmt.Sprintf(keyStateFmt, taskID)
	pipe := q.client.Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetState reads the live state HASH for a task.
func (q *RedisQueue) GetState(ctx context.Context, taskID string) (map[string]string, error) {
	return q.client.HGetAll(ctx, fmt.Sprintf(keyStateFmt, taskID)).Result()
}

// ── Task logs (LIST) ──────────────────────────────────────────────────────────

// logEntry is the JSON structure stored in the Redis log list.
type logEntry struct {
	Timestamp time.Time            `json:"ts"`
	Level     model.TaskLogLevel   `json:"level"`
	Message   string               `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// AppendLog pushes a log entry to the task's Redis list and trims to maxLogLines.
func (q *RedisQueue) AppendLog(ctx context.Context, taskID string, level model.TaskLogLevel, message string, fields map[string]interface{}) error {
	entry := logEntry{
		Timestamp: time.Now().UTC(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal log entry: %w", err)
	}
	key := fmt.Sprintf(keyLogsFmt, taskID)
	pipe := q.client.Pipeline()
	pipe.RPush(ctx, key, string(b))
	// Keep only the most recent maxLogLines entries.
	pipe.LTrim(ctx, key, int64(-q.maxLogLines), -1)
	_, err = pipe.Exec(ctx)
	return err
}

// GetLogs returns all log entries currently in Redis for a task.
func (q *RedisQueue) GetLogs(ctx context.Context, taskID string) ([]logEntry, error) {
	raw, err := q.client.LRange(ctx, fmt.Sprintf(keyLogsFmt, taskID), 0, -1).Result()
	if err != nil {
		return nil, err
	}
	entries := make([]logEntry, 0, len(raw))
	for _, s := range raw {
		var e logEntry
		if err := json.Unmarshal([]byte(s), &e); err == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// ClearLogs removes the log list for a task (called on cleanup).
func (q *RedisQueue) ClearLogs(ctx context.Context, taskID string) error {
	return q.client.Del(ctx, fmt.Sprintf(keyLogsFmt, taskID)).Err()
}
