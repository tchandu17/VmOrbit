package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/vmOrbit/backend/internal/config"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/infrastructure/messaging"
	"github.com/vmOrbit/backend/internal/provider"
	"github.com/vmOrbit/backend/internal/service"
	"github.com/vmOrbit/backend/pkg/logger"
)

// ErrTaskCancelled is returned by a handler when it detects a cancellation
// signal via TaskContext.IsCancelled(). The engine treats this as a clean
// cancellation, not a failure, and will not retry.
var ErrTaskCancelled = errors.New("task cancelled")

// ErrTaskPermanentFailure is returned by a handler when the failure is
// permanent and retrying would never succeed (e.g. unsupported operation).
// The engine marks the task as failed immediately without scheduling a retry.
var ErrTaskPermanentFailure = errors.New("task permanent failure")

// ─────────────────────────────────────────────────────────────────────────────
// Handler signature
// ─────────────────────────────────────────────────────────────────────────────

// Handler is a function that executes a specific task type.
type Handler func(tc *TaskContext, task *model.Task) error

// ─────────────────────────────────────────────────────────────────────────────
// EngineDeps
// ─────────────────────────────────────────────────────────────────────────────

// EngineDeps carries all dependencies for the task engine.
type EngineDeps struct {
	TaskRepo         port.TaskRepository
	VMRepo           port.VMRepository
	SnapshotRepo     port.SnapshotRepository
	HypervisorRepo   port.HypervisorRepository
	ProvisioningRepo port.ProvisioningJobRepository
	Registry         *provider.Registry
	RedisClient      *redis.Client
	Services         *service.Services
	EventBus         messaging.EventBus
	Log              logger.Logger
	Config           config.TaskEngineConfig
}

// ─────────────────────────────────────────────────────────────────────────────
// Engine
// ─────────────────────────────────────────────────────────────────────────────

// Engine is the async task processing engine.
//
// Architecture:
//   - Workers call RedisQueue.Dequeue (BRPOP) — blocking, no busy-wait.
//   - A DB fallback poller re-enqueues tasks that were created while Redis
//     was unavailable (e.g. after a restart).
//   - Each worker acquires a Redis NX lock before executing to prevent
//     double-execution if the same task ID appears twice in the queue.
//   - Handlers receive a TaskContext for progress/log/cancel support.
//   - Failed tasks are retried with exponential backoff via EnqueueDelayed.
//   - Per-provider semaphores prevent overloading a single hypervisor.
//   - Backpressure guard rejects enqueues when the queue is near-full.
type Engine struct {
	deps         EngineDeps
	queue        *RedisQueue
	backoff      BackoffStrategy
	handlers     map[model.TaskType]Handler
	wg           sync.WaitGroup
	stopCh       chan struct{}
	mu           sync.RWMutex
	provSem      *providerSemaphore  // per-hypervisor concurrency limiter
	dedup        *deduplicator       // prevents duplicate task enqueues
	backpressure *backpressureGuard  // rejects enqueues when queue is full
}

// NewEngine creates a new task engine and registers built-in handlers.
func NewEngine(deps EngineDeps) *Engine {
	if deps.Config.LockTTL <= 0 {
		deps.Config.LockTTL = 5 * time.Minute
	}
	if deps.Config.DefaultTimeout <= 0 {
		deps.Config.DefaultTimeout = 10 * time.Minute
	}
	if deps.Config.RetryBaseDelay <= 0 {
		deps.Config.RetryBaseDelay = 5 * time.Second
	}
	if deps.Config.MaxLogEntries <= 0 {
		deps.Config.MaxLogEntries = 500
	}
	if deps.Config.PollInterval <= 0 {
		deps.Config.PollInterval = 5 * time.Second
	}
	if deps.Config.QueueSize <= 0 {
		deps.Config.QueueSize = 1000
	}

	e := &Engine{
		deps:     deps,
		queue:    NewRedisQueue(deps.RedisClient, deps.Config.MaxLogEntries),
		backoff:  DefaultBackoff(deps.Config.RetryBaseDelay),
		handlers: make(map[model.TaskType]Handler),
		stopCh:   make(chan struct{}),
		// Allow at most 3 concurrent tasks per hypervisor to avoid API saturation.
		provSem: newProviderSemaphore(3),
		// Deduplicate inventory syncs within a 30s window.
		dedup: newDeduplicator(30 * time.Second),
		// Reject new enqueues when queue depth exceeds 80% of configured queue_size.
		backpressure: newBackpressureGuard(deps.Config.QueueSize * 8 / 10),
	}

	// Register built-in handlers.
	e.Register(model.TaskTypeVMPowerOn, e.handleVMPowerOn)
	e.Register(model.TaskTypeVMPowerOff, e.handleVMPowerOff)
	e.Register(model.TaskTypeVMReboot, e.handleVMReboot)
	e.Register(model.TaskTypeVMSuspend, e.handleVMSuspend)
	e.Register(model.TaskTypeVMDelete, e.handleVMDelete)
	e.Register(model.TaskTypeVMSnapshot, e.handleVMSnapshot)
	e.Register(model.TaskTypeVMSnapshotDelete, e.handleVMSnapshotDelete)
	e.Register(model.TaskTypeVMRestore, e.handleVMRestore)
	e.Register(model.TaskTypeInventorySync, e.handleInventorySync)
	e.Register(model.TaskTypeHypervisorSync, e.handleInventorySync) // alias

	// Bulk operation parent handlers.
	e.Register(model.TaskTypeVMBulkPowerOn, e.handleVMBulkPowerOn)
	e.Register(model.TaskTypeVMBulkPowerOff, e.handleVMBulkPowerOff)
	e.Register(model.TaskTypeVMBulkReboot, e.handleVMBulkReboot)
	e.Register(model.TaskTypeVMBulkSnapshot, e.handleVMBulkSnapshot)

	// Template & provisioning handlers.
	e.Register(model.TaskTypeTemplateSync, e.handleTemplateSync)
	e.Register(model.TaskTypeVMCloneOp, e.handleVMClone)
	e.Register(model.TaskTypeVMProvision, e.handleVMProvision)

	// Environment orchestration handlers.
	e.Register(model.TaskTypeEnvStart, e.handleEnvStart)
	e.Register(model.TaskTypeEnvStop, e.handleEnvStop)
	e.Register(model.TaskTypeEnvRestart, e.handleEnvRestart)
	e.Register(model.TaskTypeEnvSnapshot, e.handleEnvSnapshot)
	e.Register(model.TaskTypeEnvClone, e.handleEnvClone)

	return e
}

// Register adds a handler for a task type.
func (e *Engine) Register(taskType model.TaskType, handler Handler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[taskType] = handler
}

// Enqueue pushes a task onto the Redis queue and updates its DB status.
// Enforces backpressure — rejects when the queue is near capacity.
func (e *Engine) Enqueue(ctx context.Context, taskID string, priority int) error {
	if !e.backpressure.Allow() {
		return fmt.Errorf("task queue is at capacity (%d/%d) — try again later",
			e.backpressure.QueueDepth(), e.deps.Config.QueueSize)
	}
	if err := e.queue.Enqueue(ctx, taskID, priority); err != nil {
		return fmt.Errorf("enqueue task %s: %w", taskID, err)
	}
	return e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusQueued, nil, "")
}

// Start launches the worker pool, DB fallback poller, and queue depth monitor.
func (e *Engine) Start(ctx context.Context) {
	e.deps.Log.Info("task engine starting",
		logger.Int("workers", e.deps.Config.WorkerCount),
	)
	for i := 0; i < e.deps.Config.WorkerCount; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}
	e.wg.Add(1)
	go e.dbFallbackPoller(ctx)
	e.wg.Add(1)
	go e.queueDepthMonitor(ctx)
}

// Stop signals all workers to finish and waits for them.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.deps.Log.Info("task engine stopped")
}

// ─────────────────────────────────────────────────────────────────────────────
// Worker loop
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) worker(ctx context.Context, id int) {
	defer e.wg.Done()
	e.deps.Log.Debug("task worker started", logger.Int("worker_id", id))

	for {
		select {
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		taskID, err := e.queue.Dequeue(ctx)
		if err != nil {
			e.deps.Log.Error("worker dequeue error",
				logger.Int("worker_id", id),
				logger.Error(err),
			)
			select {
			case <-time.After(time.Second):
			case <-e.stopCh:
				return
			case <-ctx.Done():
				return
			}
			continue
		}
		if taskID == "" {
			return
		}
		e.processTask(ctx, taskID)
	}
}

// processTask loads, locks, and executes a single task.
func (e *Engine) processTask(ctx context.Context, taskID string) {
	// ── Acquire worker lock (at-most-once) ───────────────────────────────────
	acquired, err := e.queue.AcquireLock(ctx, taskID, e.deps.Config.LockTTL)
	if err != nil {
		e.deps.Log.Error("failed to acquire task lock",
			logger.String("task_id", taskID),
			logger.Error(err),
		)
		return
	}
	if !acquired {
		e.deps.Log.Debug("task already locked, skipping",
			logger.String("task_id", taskID),
		)
		return
	}
	defer e.queue.ReleaseLock(ctx, taskID) //nolint:errcheck

	// ── Load task from DB ────────────────────────────────────────────────────
	t, err := e.deps.TaskRepo.GetByID(ctx, taskID)
	if err != nil {
		e.deps.Log.Error("failed to load task",
			logger.String("task_id", taskID),
			logger.Error(err),
		)
		return
	}

	// ── Check for pre-execution cancellation ─────────────────────────────────
	if t.Status == model.TaskStatusCancelled || e.queue.IsCancelled(ctx, taskID) {
		_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusCancelled, nil, "cancelled before execution")
		e.publishEvent(ctx, messaging.EventTaskCancelled, t)
		_ = e.queue.ClearCancel(ctx, taskID)
		return
	}

	// ── Acquire per-provider concurrency slot ─────────────────────────────────
	// Prevents multiple workers from hammering the same hypervisor simultaneously.
	hypervisorID := ""
	if t.HypervisorID != nil {
		hypervisorID = t.HypervisorID.String()
	}
	releaseSlot, slotErr := e.provSem.Acquire(ctx, hypervisorID)
	if slotErr != nil {
		// Context cancelled while waiting — re-enqueue for later.
		e.deps.Log.Warn("provider slot acquire cancelled, re-enqueuing task",
			logger.String("task_id", taskID),
			logger.String("hypervisor_id", hypervisorID),
		)
		_ = e.queue.Enqueue(ctx, taskID, t.Priority)
		return
	}
	defer releaseSlot()

	// ── Mark running ─────────────────────────────────────────────────────────
	now := time.Now().UTC()
	t.StartedAt = &now
	_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusRunning, nil, "")
	_ = e.queue.SetState(ctx, taskID, map[string]interface{}{
		"status":     string(model.TaskStatusRunning),
		"started_at": now.Format(time.RFC3339),
		"progress":   0,
	}, e.deps.Config.TaskTTL)
	e.publishStatusEvent(ctx, t, model.TaskStatusRunning)

	// ── Build execution context with timeout ─────────────────────────────────
	execCtx, cancel := context.WithTimeout(ctx, e.deps.Config.DefaultTimeout)
	defer cancel()

	stopRefresh := e.startLockRefresher(execCtx, taskID)
	defer stopRefresh()

	tc := newTaskContext(execCtx, taskID, e.queue, e.deps.EventBus, e.deps.Config.TaskTTL)

	// ── Execute ───────────────────────────────────────────────────────────────
	e.mu.RLock()
	handler, ok := e.handlers[t.Type]
	e.mu.RUnlock()

	if !ok {
		msg := fmt.Sprintf("no handler registered for task type %q", t.Type)
		e.deps.Log.Warn(msg, logger.String("task_id", taskID))
		_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, msg)
		e.publishStatusEvent(ctx, t, model.TaskStatusFailed)
		return
	}

	handlerErr := handler(tc, t)

	// ── Post-execution ────────────────────────────────────────────────────────
	completedAt := time.Now().UTC()
	t.CompletedAt = &completedAt

	switch {
	case handlerErr == nil:
		_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusCompleted, nil, "")
		_ = e.deps.TaskRepo.UpdateProgress(ctx, taskID, 100)
		_ = e.queue.SetState(ctx, taskID, map[string]interface{}{
			"status":       string(model.TaskStatusCompleted),
			"progress":     100,
			"completed_at": completedAt.Format(time.RFC3339),
		}, e.deps.Config.TaskTTL)
		e.publishStatusEvent(ctx, t, model.TaskStatusCompleted)
		e.deps.Log.Info("task completed",
			logger.String("task_id", taskID),
			logger.String("type", string(t.Type)),
		)

	case errors.Is(handlerErr, ErrTaskCancelled):
		_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusCancelled, nil, "cancelled during execution")
		_ = e.queue.ClearCancel(ctx, taskID)
		e.publishEvent(ctx, messaging.EventTaskCancelled, t)
		e.deps.Log.Info("task cancelled", logger.String("task_id", taskID))

	case errors.Is(handlerErr, ErrTaskPermanentFailure):
		// Permanent failure — do not retry regardless of MaxRetries.
		msg := handlerErr.Error()
		_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, msg)
		e.publishStatusEvent(ctx, t, model.TaskStatusFailed)
		e.deps.Log.Error("task failed permanently (unsupported operation)",
			logger.String("task_id", taskID),
			logger.String("type", string(t.Type)),
			logger.Error(handlerErr),
		)

	default:
		t.RetryCount++
		if t.RetryCount < t.MaxRetries {
			delay := e.backoff.Delay(t.RetryCount)
			_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusRetrying, nil, handlerErr.Error())
			e.publishStatusEvent(ctx, t, model.TaskStatusRetrying)
			e.deps.Log.Warn("task failed, scheduling retry",
				logger.String("task_id", taskID),
				logger.Int("attempt", t.RetryCount),
				logger.String("retry_in", delay.String()),
				logger.Error(handlerErr),
			)
			e.queue.EnqueueDelayed(taskID, t.Priority, delay)
		} else {
			_ = e.deps.TaskRepo.UpdateStatus(ctx, taskID, model.TaskStatusFailed, nil, handlerErr.Error())
			e.publishStatusEvent(ctx, t, model.TaskStatusFailed)
			e.deps.Log.Error("task failed permanently",
				logger.String("task_id", taskID),
				logger.Int("attempts", t.RetryCount),
				logger.Error(handlerErr),
			)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Lock refresher
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) startLockRefresher(ctx context.Context, taskID string) func() {
	ticker := time.NewTicker(e.deps.Config.LockTTL / 2)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = e.queue.RefreshLock(ctx, taskID, e.deps.Config.LockTTL)
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// ─────────────────────────────────────────────────────────────────────────────
// DB fallback poller
// ─────────────────────────────────────────────────────────────────────────────

// dbFallbackPoller periodically scans the DB for tasks stuck in pending state
// and re-enqueues them. This is a crash-recovery mechanism only.
func (e *Engine) dbFallbackPoller(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(e.deps.Config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.rehydrateFromDB(ctx)
		}
	}
}

func (e *Engine) rehydrateFromDB(ctx context.Context) {
	tasks, err := e.deps.TaskRepo.ListPending(ctx, e.deps.Config.WorkerCount*2)
	if err != nil {
		e.deps.Log.Error("db fallback poller: failed to fetch tasks", logger.Error(err))
		return
	}
	for i := range tasks {
		t := &tasks[i]
		if t.Status == model.TaskStatusQueued || t.Status == model.TaskStatusRetrying {
			continue
		}
		acquired, err := e.queue.AcquireLock(ctx, t.ID.String(), e.deps.Config.LockTTL)
		if err != nil || !acquired {
			continue
		}
		_ = e.queue.ReleaseLock(ctx, t.ID.String())

		if err := e.queue.Enqueue(ctx, t.ID.String(), t.Priority); err != nil {
			e.deps.Log.Error("db fallback: failed to re-enqueue task",
				logger.String("task_id", t.ID.String()),
				logger.Error(err),
			)
			continue
		}
		_ = e.deps.TaskRepo.UpdateStatus(ctx, t.ID.String(), model.TaskStatusQueued, nil, "")
		e.deps.Log.Info("db fallback: re-enqueued stuck pending task",
			logger.String("task_id", t.ID.String()),
			logger.String("type", string(t.Type)),
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Queue depth monitor
// ─────────────────────────────────────────────────────────────────────────────

// queueDepthMonitor measures total Redis queue depth every 5s and updates
// the backpressure guard so Enqueue can reject requests when near-full.
func (e *Engine) queueDepthMonitor(ctx context.Context) {
	defer e.wg.Done()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			total := 0
			for i := 1; i <= 10; i++ {
				key := fmt.Sprintf("task:queue:%d", i)
				n, err := e.deps.RedisClient.LLen(ctx, key).Result()
				if err == nil {
					total += int(n)
				}
			}
			e.backpressure.SetQueueSize(total)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Event helpers
// ─────────────────────────────────────────────────────────────────────────────

func (e *Engine) publishStatusEvent(ctx context.Context, t *model.Task, status model.TaskStatus) {
	e.deps.EventBus.Publish(ctx, messaging.Event{
		Type: messaging.EventTaskStatusChanged,
		Payload: map[string]interface{}{
			"task_id":  t.ID.String(),
			"type":     string(t.Type),
			"status":   string(status),
			"progress": t.Progress,
		},
	})
}

func (e *Engine) publishEvent(ctx context.Context, eventType messaging.EventType, t *model.Task) {
	e.deps.EventBus.Publish(ctx, messaging.Event{
		Type: eventType,
		Payload: map[string]interface{}{
			"task_id": t.ID.String(),
			"type":    string(t.Type),
		},
	})
}
