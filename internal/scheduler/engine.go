// Package scheduler provides the cron-based schedule execution engine.
// It polls the database for due schedules, fires the corresponding task engine
// operations, and updates next_run_at for recurring schedules.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/pkg/logger"
)

// EnqueueFunc is the task engine's Enqueue method signature.
type EnqueueFunc func(ctx context.Context, taskID string, priority int) error

// SchedulerOps bundles the service-layer operations the scheduler needs.
// Using function types instead of the full service container breaks the
// service → scheduler → service import cycle.
type SchedulerOps struct {
	SyncInventory  func(ctx context.Context, hypervisorID string) (string, error)
	VMPowerOn      func(ctx context.Context, vmID string) (string, error)
	VMPowerOff     func(ctx context.Context, vmID string) (string, error)
	VMReboot       func(ctx context.Context, vmID string) (string, error)
	VMSnapshot     func(ctx context.Context, vmID string, spec port.SnapshotSpec) (string, error)
	VMBulkPowerOn  func(ctx context.Context, vmIDs []string) (string, error)
	VMBulkPowerOff func(ctx context.Context, vmIDs []string) (string, error)
	VMBulkReboot   func(ctx context.Context, vmIDs []string) (string, error)
	VMBulkSnapshot func(ctx context.Context, vmIDs []string, spec port.SnapshotSpec) (string, error)
}

// EngineDeps carries all dependencies for the scheduler engine.
type EngineDeps struct {
	Schedules          port.ScheduleRepository
	ScheduleExecutions port.ScheduleExecutionRepository
	Ops                SchedulerOps
	Enqueue            EnqueueFunc
	Log                logger.Logger
	PollInterval       time.Duration
}

// Engine is the scheduler execution engine.
type Engine struct {
	deps   EngineDeps
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewEngine creates a new scheduler Engine.
func NewEngine(deps EngineDeps) *Engine {
	if deps.PollInterval <= 0 {
		deps.PollInterval = 30 * time.Second
	}
	return &Engine{
		deps:   deps,
		stopCh: make(chan struct{}),
	}
}

// Start launches the scheduler polling loop.
func (e *Engine) Start(ctx context.Context) {
	e.wg.Add(1)
	go e.loop(ctx)
	e.deps.Log.Info("scheduler engine started",
		logger.String("poll_interval", e.deps.PollInterval.String()),
	)
}

// Stop signals the scheduler to stop and waits for it to finish.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
	e.deps.Log.Info("scheduler engine stopped")
}

func (e *Engine) loop(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.deps.PollInterval)
	defer ticker.Stop()

	e.tick(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) tick(ctx context.Context) {
	now := time.Now().UTC()
	schedules, err := e.deps.Schedules.ListDue(ctx, now)
	if err != nil {
		e.deps.Log.Error("scheduler: failed to list due schedules", logger.Error(err))
		return
	}
	for i := range schedules {
		s := &schedules[i]
		go e.fire(ctx, s, now)
	}
}

func (e *Engine) fire(ctx context.Context, s *model.Schedule, firedAt time.Time) {
	log := e.deps.Log

	nextRun, err := e.computeNextRun(s, firedAt)
	if err != nil {
		log.Error("scheduler: failed to compute next run",
			logger.String("schedule_id", s.ID.String()),
			logger.Error(err),
		)
		return
	}

	newRunCount := s.RunCount + 1
	newStatus := s.Status
	if s.MaxRuns > 0 && newRunCount >= s.MaxRuns {
		newStatus = model.ScheduleStatusExpired
	}
	if s.ExpiresAt != nil && time.Now().UTC().After(*s.ExpiresAt) {
		newStatus = model.ScheduleStatusExpired
	}

	taskID, dispatchErr := e.dispatch(ctx, s)

	execStatus := "triggered"
	errMsg := ""
	if dispatchErr != nil {
		execStatus = "failed"
		errMsg = dispatchErr.Error()
		log.Error("scheduler: dispatch failed",
			logger.String("schedule_id", s.ID.String()),
			logger.String("operation", string(s.OperationType)),
			logger.Error(dispatchErr),
		)
	}

	exec := &model.ScheduleExecution{
		ID:           uuid.New(),
		CreatedAt:    firedAt,
		ScheduleID:   s.ID,
		Status:       execStatus,
		ErrorMessage: errMsg,
		TriggeredAt:  firedAt,
	}
	if taskID != "" {
		tid, _ := uuid.Parse(taskID)
		exec.TaskID = &tid
	}
	if createErr := e.deps.ScheduleExecutions.Create(ctx, exec); createErr != nil {
		log.Warn("scheduler: failed to record execution",
			logger.String("schedule_id", s.ID.String()),
			logger.Error(createErr),
		)
	}

	newFailureCount := s.FailureCount
	lastRunStatus := "success"
	if dispatchErr != nil {
		newFailureCount++
		lastRunStatus = "failed"
	}

	var lastTaskID *uuid.UUID
	if taskID != "" {
		tid, _ := uuid.Parse(taskID)
		lastTaskID = &tid
	}

	update := port.ScheduleRunUpdate{
		LastRunAt:     firedAt,
		NextRunAt:     nextRun,
		LastTaskID:    lastTaskID,
		LastRunStatus: lastRunStatus,
		RunCount:      newRunCount,
		FailureCount:  newFailureCount,
		Status:        newStatus,
	}
	if updateErr := e.deps.Schedules.UpdateAfterRun(ctx, s.ID.String(), update); updateErr != nil {
		log.Error("scheduler: failed to update schedule after run",
			logger.String("schedule_id", s.ID.String()),
			logger.Error(updateErr),
		)
	}

	if dispatchErr == nil {
		log.Info("scheduler: schedule fired",
			logger.String("schedule_id", s.ID.String()),
			logger.String("name", s.Name),
			logger.String("operation", string(s.OperationType)),
			logger.String("task_id", taskID),
		)
	}
}

// DispatchSchedule is a public entry point for the service layer to trigger
// a schedule immediately (bypassing the cron timing check).
func (e *Engine) DispatchSchedule(ctx context.Context, s *model.Schedule) (string, error) {
	return e.dispatch(ctx, s)
}

func (e *Engine) dispatch(ctx context.Context, s *model.Schedule) (string, error) {
	ops := e.deps.Ops
	switch s.OperationType {
	case model.ScheduleOpInventorySync:
		if len(s.TargetIDs) == 0 {
			return "", fmt.Errorf("inventory sync schedule has no target hypervisor IDs")
		}
		var firstTaskID string
		for _, hypervisorID := range s.TargetIDs {
			taskID, err := ops.SyncInventory(ctx, hypervisorID)
			if err != nil {
				return "", fmt.Errorf("sync hypervisor %s: %w", hypervisorID, err)
			}
			_ = e.deps.Enqueue(ctx, taskID, 5)
			if firstTaskID == "" {
				firstTaskID = taskID
			}
		}
		return firstTaskID, nil

	case model.ScheduleOpVMPowerOn:
		return e.dispatchVMOp(ctx, s, ops.VMPowerOn)
	case model.ScheduleOpVMPowerOff:
		return e.dispatchVMOp(ctx, s, ops.VMPowerOff)
	case model.ScheduleOpVMReboot:
		return e.dispatchVMOp(ctx, s, ops.VMReboot)

	case model.ScheduleOpVMSnapshot:
		if len(s.TargetIDs) == 0 {
			return "", fmt.Errorf("snapshot schedule has no target VM IDs")
		}
		snapshotName := fmt.Sprintf("scheduled-%s", time.Now().UTC().Format("2006-01-02T15-04"))
		if s.Payload != nil {
			if n, ok := s.Payload["snapshot_name"].(string); ok && n != "" {
				snapshotName = n
			}
		}
		var firstTaskID string
		for _, vmID := range s.TargetIDs {
			taskID, err := ops.VMSnapshot(ctx, vmID, port.SnapshotSpec{
				Name:        snapshotName,
				Description: fmt.Sprintf("Scheduled snapshot by schedule %s", s.Name),
			})
			if err != nil {
				return "", fmt.Errorf("snapshot VM %s: %w", vmID, err)
			}
			_ = e.deps.Enqueue(ctx, taskID, 5)
			if firstTaskID == "" {
				firstTaskID = taskID
			}
		}
		return firstTaskID, nil

	case model.ScheduleOpVMBulkPowerOn:
		return e.dispatchBulkOp(ctx, s, ops.VMBulkPowerOn)
	case model.ScheduleOpVMBulkPowerOff:
		return e.dispatchBulkOp(ctx, s, ops.VMBulkPowerOff)
	case model.ScheduleOpVMBulkReboot:
		return e.dispatchBulkOp(ctx, s, ops.VMBulkReboot)

	case model.ScheduleOpVMBulkSnapshot:
		if len(s.TargetIDs) == 0 {
			return "", fmt.Errorf("bulk snapshot schedule has no target VM IDs")
		}
		snapshotName := fmt.Sprintf("scheduled-%s", time.Now().UTC().Format("2006-01-02T15-04"))
		if s.Payload != nil {
			if n, ok := s.Payload["snapshot_name"].(string); ok && n != "" {
				snapshotName = n
			}
		}
		taskID, err := ops.VMBulkSnapshot(ctx, s.TargetIDs, port.SnapshotSpec{
			Name:        snapshotName,
			Description: fmt.Sprintf("Scheduled bulk snapshot by schedule %s", s.Name),
		})
		if err != nil {
			return "", err
		}
		_ = e.deps.Enqueue(ctx, taskID, 5)
		return taskID, nil

	default:
		return "", fmt.Errorf("unsupported schedule operation: %s", s.OperationType)
	}
}

func (e *Engine) dispatchVMOp(ctx context.Context, s *model.Schedule, op func(context.Context, string) (string, error)) (string, error) {
	if len(s.TargetIDs) == 0 {
		return "", fmt.Errorf("VM operation schedule has no target VM IDs")
	}
	var firstTaskID string
	for _, vmID := range s.TargetIDs {
		taskID, err := op(ctx, vmID)
		if err != nil {
			return "", fmt.Errorf("VM op on %s: %w", vmID, err)
		}
		_ = e.deps.Enqueue(ctx, taskID, 5)
		if firstTaskID == "" {
			firstTaskID = taskID
		}
	}
	return firstTaskID, nil
}

func (e *Engine) dispatchBulkOp(ctx context.Context, s *model.Schedule, op func(context.Context, []string) (string, error)) (string, error) {
	if len(s.TargetIDs) == 0 {
		return "", fmt.Errorf("bulk operation schedule has no target VM IDs")
	}
	taskID, err := op(ctx, s.TargetIDs)
	if err != nil {
		return "", err
	}
	_ = e.deps.Enqueue(ctx, taskID, 5)
	return taskID, nil
}

func (e *Engine) computeNextRun(s *model.Schedule, after time.Time) (*time.Time, error) {
	if s.ScheduleType == model.ScheduleTypeOnce {
		return nil, nil
	}
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		loc = time.UTC
	}
	cs, err := ParseCron(s.CronExpression)
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression %q: %w", s.CronExpression, err)
	}
	next := cs.Next(after, loc)
	if next.IsZero() {
		return nil, nil
	}
	nextUTC := next.UTC()
	return &nextUTC, nil
}
