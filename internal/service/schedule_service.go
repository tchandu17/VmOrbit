package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
	"github.com/vmOrbit/backend/internal/scheduler"
	"github.com/vmOrbit/backend/pkg/logger"
)

type scheduleService struct {
	repo     port.ScheduleRepository
	execRepo port.ScheduleExecutionRepository
	engine   *scheduler.Engine // injected after wiring
	log      logger.Logger
}

// NewScheduleService creates a new schedule service.
func NewScheduleService(
	repo port.ScheduleRepository,
	execRepo port.ScheduleExecutionRepository,
	log logger.Logger,
) *scheduleService {
	return &scheduleService{
		repo:     repo,
		execRepo: execRepo,
		log:      log,
	}
}

// SetEngine injects the scheduler engine (called from bootstrap after wiring).
func (s *scheduleService) SetEngine(eng *scheduler.Engine) {
	s.engine = eng
}

func (s *scheduleService) Create(ctx context.Context, req port.CreateScheduleRequest) (*model.Schedule, error) {
	cronExpr, err := normaliseCron(req.ScheduleType, req.CronExpression)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}

	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", tz, err)
	}

	var nextRun *time.Time
	if req.ScheduleType != model.ScheduleTypeOnce {
		cs, err := scheduler.ParseCron(cronExpr)
		if err != nil {
			return nil, fmt.Errorf("cron parse error: %w", err)
		}
		next := cs.Next(time.Now().UTC(), loc)
		if !next.IsZero() {
			nextUTC := next.UTC()
			nextRun = &nextUTC
		}
	} else if req.Payload != nil {
		if runAt, ok := req.Payload["run_at"].(string); ok {
			t, err := time.Parse(time.RFC3339, runAt)
			if err == nil {
				nextRun = &t
			}
		}
	}

	sched := &model.Schedule{
		Name:           req.Name,
		Description:    req.Description,
		OperationType:  req.OperationType,
		TargetType:     req.TargetType,
		TargetIDs:      model.StringArray(req.TargetIDs),
		ScheduleType:   req.ScheduleType,
		CronExpression: cronExpr,
		Timezone:       tz,
		Enabled:        req.Enabled,
		Status:         model.ScheduleStatusActive,
		NextRunAt:      nextRun,
		MaxRuns:        req.MaxRuns,
		ExpiresAt:      req.ExpiresAt,
		Payload:        req.Payload,
		CreatedBy:      callerUUID(ctx),
	}
	if !req.Enabled {
		sched.Status = model.ScheduleStatusPaused
	}

	if err := s.repo.Create(ctx, sched); err != nil {
		return nil, fmt.Errorf("creating schedule: %w", err)
	}
	return sched, nil
}

func (s *scheduleService) GetByID(ctx context.Context, id string) (*model.Schedule, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *scheduleService) Update(ctx context.Context, id string, req port.UpdateScheduleRequest) (*model.Schedule, error) {
	sched, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		sched.Name = *req.Name
	}
	if req.Description != nil {
		sched.Description = *req.Description
	}
	if req.OperationType != nil {
		sched.OperationType = *req.OperationType
	}
	if req.TargetType != nil {
		sched.TargetType = *req.TargetType
	}
	if req.TargetIDs != nil {
		sched.TargetIDs = model.StringArray(req.TargetIDs)
	}
	if req.Enabled != nil {
		sched.Enabled = *req.Enabled
		if *req.Enabled {
			sched.Status = model.ScheduleStatusActive
		} else {
			sched.Status = model.ScheduleStatusPaused
		}
	}
	if req.MaxRuns != nil {
		sched.MaxRuns = *req.MaxRuns
	}
	if req.ExpiresAt != nil {
		sched.ExpiresAt = req.ExpiresAt
	}
	if req.Payload != nil {
		sched.Payload = req.Payload
	}

	schedType := sched.ScheduleType
	cronExpr := sched.CronExpression
	if req.ScheduleType != nil {
		schedType = *req.ScheduleType
	}
	if req.CronExpression != nil {
		cronExpr = *req.CronExpression
	}
	if req.ScheduleType != nil || req.CronExpression != nil {
		newCron, err := normaliseCron(schedType, cronExpr)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule: %w", err)
		}
		sched.ScheduleType = schedType
		sched.CronExpression = newCron

		tz := sched.Timezone
		if req.Timezone != nil {
			tz = *req.Timezone
		}
		loc, _ := time.LoadLocation(tz)
		if loc == nil {
			loc = time.UTC
		}
		if schedType != model.ScheduleTypeOnce {
			cs, err := scheduler.ParseCron(newCron)
			if err == nil {
				next := cs.Next(time.Now().UTC(), loc)
				if !next.IsZero() {
					nextUTC := next.UTC()
					sched.NextRunAt = &nextUTC
				}
			}
		}
	}
	if req.Timezone != nil {
		sched.Timezone = *req.Timezone
	}

	if err := s.repo.Update(ctx, sched); err != nil {
		return nil, fmt.Errorf("updating schedule: %w", err)
	}
	return sched, nil
}

func (s *scheduleService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *scheduleService) List(ctx context.Context, filter port.ScheduleFilter, page port.Page) (*port.PageResult[model.Schedule], error) {
	return s.repo.List(ctx, filter, page)
}

func (s *scheduleService) Enable(ctx context.Context, id string) error {
	sched, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sched.Enabled = true
	sched.Status = model.ScheduleStatusActive
	return s.repo.Update(ctx, sched)
}

func (s *scheduleService) Disable(ctx context.Context, id string) error {
	sched, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	sched.Enabled = false
	sched.Status = model.ScheduleStatusPaused
	return s.repo.Update(ctx, sched)
}

func (s *scheduleService) TriggerNow(ctx context.Context, id string) (string, error) {
	sched, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("schedule not found: %w", err)
	}
	if s.engine == nil {
		return "", fmt.Errorf("scheduler engine not available")
	}

	taskID, err := s.engine.DispatchSchedule(ctx, sched)
	if err != nil {
		return "", fmt.Errorf("trigger schedule: %w", err)
	}

	exec := &model.ScheduleExecution{
		ID:          uuid.New(),
		CreatedAt:   time.Now().UTC(),
		ScheduleID:  sched.ID,
		Status:      "triggered",
		TriggeredAt: time.Now().UTC(),
	}
	if taskID != "" {
		tid, _ := uuid.Parse(taskID)
		exec.TaskID = &tid
	}
	_ = s.execRepo.Create(ctx, exec)

	return taskID, nil
}

func (s *scheduleService) ListExecutions(ctx context.Context, scheduleID string, page port.Page) (*port.PageResult[model.ScheduleExecution], error) {
	return s.execRepo.List(ctx, scheduleID, page)
}

// normaliseCron converts convenience schedule types to cron expressions.
func normaliseCron(schedType model.ScheduleType, expr string) (string, error) {
	switch schedType {
	case model.ScheduleTypeCron:
		if _, err := scheduler.ParseCron(expr); err != nil {
			return "", fmt.Errorf("invalid cron expression %q: %w", expr, err)
		}
		return expr, nil
	case model.ScheduleTypeOnce:
		if expr == "" {
			return "0 0 1 1 *", nil
		}
		return expr, nil
	case model.ScheduleTypeDaily, model.ScheduleTypeWeekly, model.ScheduleTypeMonthly:
		if expr == "" {
			return "", fmt.Errorf("cron_expression is required for %s schedules", schedType)
		}
		if _, err := scheduler.ParseCron(expr); err != nil {
			return "", fmt.Errorf("invalid cron expression %q: %w", expr, err)
		}
		return expr, nil
	default:
		return "", fmt.Errorf("unsupported schedule type: %s", schedType)
	}
}
