package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ScheduleRepo is the GORM-backed schedule repository.
type ScheduleRepo struct{ db *gorm.DB }

// NewScheduleRepo creates a new ScheduleRepo.
func NewScheduleRepo(db *gorm.DB) *ScheduleRepo { return &ScheduleRepo{db: db} }

func (r *ScheduleRepo) Create(ctx context.Context, s *model.Schedule) error {
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *ScheduleRepo) GetByID(ctx context.Context, id string) (*model.Schedule, error) {
	var s model.Schedule
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ScheduleRepo) Update(ctx context.Context, s *model.Schedule) error {
	return r.db.WithContext(ctx).Save(s).Error
}

func (r *ScheduleRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Schedule{}, "id = ?", id).Error
}

func (r *ScheduleRepo) List(ctx context.Context, filter port.ScheduleFilter, page port.Page) (*port.PageResult[model.Schedule], error) {
	var items []model.Schedule
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Schedule{})
	if filter.OperationType != "" {
		q = q.Where("operation_type = ?", filter.OperationType)
	}
	if filter.TargetType != "" {
		q = q.Where("target_type = ?", filter.TargetType)
	}
	if filter.Enabled != nil {
		q = q.Where("enabled = ?", *filter.Enabled)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.Schedule]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *ScheduleRepo) ListDue(ctx context.Context, now time.Time) ([]model.Schedule, error) {
	var schedules []model.Schedule
	err := r.db.WithContext(ctx).
		Where("enabled = true AND status = ? AND next_run_at <= ?", model.ScheduleStatusActive, now).
		Order("next_run_at ASC").
		Find(&schedules).Error
	return schedules, err
}

func (r *ScheduleRepo) UpdateAfterRun(ctx context.Context, id string, update port.ScheduleRunUpdate) error {
	updates := map[string]interface{}{
		"last_run_at":     update.LastRunAt,
		"next_run_at":     update.NextRunAt,
		"last_run_status": update.LastRunStatus,
		"run_count":       update.RunCount,
		"failure_count":   update.FailureCount,
		"status":          update.Status,
	}
	if update.LastTaskID != nil {
		updates["last_task_id"] = update.LastTaskID
	}
	return r.db.WithContext(ctx).
		Model(&model.Schedule{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ─────────────────────────────────────────────────────────────────────────────
// ScheduleExecutionRepo
// ─────────────────────────────────────────────────────────────────────────────

// ScheduleExecutionRepo is the GORM-backed schedule execution repository.
type ScheduleExecutionRepo struct{ db *gorm.DB }

// NewScheduleExecutionRepo creates a new ScheduleExecutionRepo.
func NewScheduleExecutionRepo(db *gorm.DB) *ScheduleExecutionRepo {
	return &ScheduleExecutionRepo{db: db}
}

func (r *ScheduleExecutionRepo) Create(ctx context.Context, e *model.ScheduleExecution) error {
	return r.db.WithContext(ctx).Create(e).Error
}

func (r *ScheduleExecutionRepo) List(ctx context.Context, scheduleID string, page port.Page) (*port.PageResult[model.ScheduleExecution], error) {
	var items []model.ScheduleExecution
	var total int64

	q := r.db.WithContext(ctx).Model(&model.ScheduleExecution{}).Where("schedule_id = ?", scheduleID)
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.ScheduleExecution]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
