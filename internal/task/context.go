package task

import (
	"context"
	"time"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
)

// ─────────────────────────────────────────────────────────────────────────────
// TaskContext
// ─────────────────────────────────────────────────────────────────────────────

// TaskContext is the execution context passed to every task handler.
// It embeds context.Context so handlers can use it directly with any
// context-aware library, and adds task-specific helpers for progress
// reporting, structured logging, and cooperative cancellation checks.
//
// All methods are safe to call from the handler goroutine only — they are
// not designed for concurrent use from multiple goroutines.
type TaskContext struct {
	context.Context

	taskID   string
	queue    *RedisQueue
	eventBus messaging.EventBus
	stateTTL time.Duration
}

// newTaskContext wraps a cancellable context with task-specific helpers.
func newTaskContext(
	ctx context.Context,
	taskID string,
	queue *RedisQueue,
	eventBus messaging.EventBus,
	stateTTL time.Duration,
) *TaskContext {
	return &TaskContext{
		Context:  ctx,
		taskID:   taskID,
		queue:    queue,
		eventBus: eventBus,
		stateTTL: stateTTL,
	}
}

// TaskID returns the ID of the task being executed.
func (tc *TaskContext) TaskID() string { return tc.taskID }

// ── Progress ─────────────────────────────────────────────────────────────────

// Progress reports execution progress (0–100) with an optional human-readable
// message. It updates the Redis state hash and publishes a WebSocket event.
// Errors are silently swallowed — progress reporting must never fail a task.
func (tc *TaskContext) Progress(pct int, message string) {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}

	_ = tc.queue.SetState(tc.Context, tc.taskID, map[string]interface{}{
		"progress":         pct,
		"progress_message": message,
		"updated_at":       time.Now().UTC().Format(time.RFC3339),
	}, tc.stateTTL)

	tc.eventBus.Publish(tc.Context, messaging.Event{
		Type: messaging.EventTaskProgress,
		Payload: map[string]interface{}{
			"task_id":  tc.taskID,
			"progress": pct,
			"message":  message,
		},
	})
}

// ── Structured logging ────────────────────────────────────────────────────────

// Log appends a structured log entry to the task's Redis log list and
// publishes a WebSocket event to the per-task room. It also persists the
// entry to Postgres asynchronously via the event bus (the engine subscribes
// and writes to DB in the background).
//
// fields is an optional flat map of extra key-value pairs.
func (tc *TaskContext) Log(level model.TaskLogLevel, message string, fields ...map[string]interface{}) {
	var f map[string]interface{}
	if len(fields) > 0 {
		f = fields[0]
	}

	_ = tc.queue.AppendLog(tc.Context, tc.taskID, level, message, f)

	payload := map[string]interface{}{
		"task_id": tc.taskID,
		"level":   string(level),
		"message": message,
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
	}
	if f != nil {
		payload["fields"] = f
	}

	tc.eventBus.Publish(tc.Context, messaging.Event{
		Type:    messaging.EventTaskLogAppended,
		Payload: payload,
	})
}

// Convenience wrappers.

func (tc *TaskContext) LogDebug(message string, fields ...map[string]interface{}) {
	tc.Log(model.TaskLogLevelDebug, message, fields...)
}

func (tc *TaskContext) LogInfo(message string, fields ...map[string]interface{}) {
	tc.Log(model.TaskLogLevelInfo, message, fields...)
}

func (tc *TaskContext) LogWarn(message string, fields ...map[string]interface{}) {
	tc.Log(model.TaskLogLevelWarn, message, fields...)
}

func (tc *TaskContext) LogError(message string, fields ...map[string]interface{}) {
	tc.Log(model.TaskLogLevelError, message, fields...)
}

// ── Cooperative cancellation ──────────────────────────────────────────────────

// IsCancelled returns true if a cancellation signal has been set for this task
// in Redis, OR if the underlying context has been cancelled.
// Handlers should call this at natural checkpoints (between API calls, after
// processing each item in a batch) and return ErrTaskCancelled if true.
func (tc *TaskContext) IsCancelled() bool {
	if tc.Context.Err() != nil {
		return true
	}
	return tc.queue.IsCancelled(tc.Context, tc.taskID)
}
