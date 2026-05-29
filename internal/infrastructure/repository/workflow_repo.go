package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// WorkflowRepo is the GORM-backed workflow repository.
type WorkflowRepo struct{ db *gorm.DB }

// NewWorkflowRepo creates a new WorkflowRepo.
func NewWorkflowRepo(db *gorm.DB) *WorkflowRepo { return &WorkflowRepo{db: db} }

func (r *WorkflowRepo) Create(ctx context.Context, w *model.Workflow) error {
	return r.db.WithContext(ctx).Create(w).Error
}

func (r *WorkflowRepo) GetByID(ctx context.Context, id string) (*model.Workflow, error) {
	var w model.Workflow
	if err := r.db.WithContext(ctx).
		Preload("Actions", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		First(&w, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

func (r *WorkflowRepo) Update(ctx context.Context, w *model.Workflow) error {
	return r.db.WithContext(ctx).Save(w).Error
}

func (r *WorkflowRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Workflow{}, "id = ?", id).Error
}

func (r *WorkflowRepo) List(ctx context.Context, filter port.WorkflowFilter, page port.Page) (*port.PageResult[model.Workflow], error) {
	var items []model.Workflow
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Workflow{})
	if filter.TriggerType != "" {
		q = q.Where("trigger_type = ?", filter.TriggerType)
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
	if err := q.
		Preload("Actions", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Order("created_at DESC").
		Offset(offset).Limit(page.Size).
		Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.Workflow]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *WorkflowRepo) ListByTrigger(ctx context.Context, triggerType model.WorkflowTriggerType) ([]model.Workflow, error) {
	var workflows []model.Workflow
	err := r.db.WithContext(ctx).
		Where("enabled = true AND status = ? AND trigger_type = ?", model.WorkflowStatusActive, triggerType).
		Preload("Actions", func(db *gorm.DB) *gorm.DB {
			return db.Order("\"order\" ASC")
		}).
		Find(&workflows).Error
	return workflows, err
}

func (r *WorkflowRepo) UpdateAfterRun(ctx context.Context, id string, update port.WorkflowRunUpdate) error {
	return r.db.WithContext(ctx).
		Model(&model.Workflow{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_run_at":     update.LastRunAt,
			"last_run_status": update.LastRunStatus,
			"run_count":       update.RunCount,
			"failure_count":   update.FailureCount,
		}).Error
}

// ─────────────────────────────────────────────────────────────────────────────
// WorkflowRunRepo
// ─────────────────────────────────────────────────────────────────────────────

// WorkflowRunRepo is the GORM-backed workflow run repository.
type WorkflowRunRepo struct{ db *gorm.DB }

// NewWorkflowRunRepo creates a new WorkflowRunRepo.
func NewWorkflowRunRepo(db *gorm.DB) *WorkflowRunRepo { return &WorkflowRunRepo{db: db} }

func (r *WorkflowRunRepo) Create(ctx context.Context, run *model.WorkflowRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *WorkflowRunRepo) GetByID(ctx context.Context, id string) (*model.WorkflowRun, error) {
	var run model.WorkflowRun
	if err := r.db.WithContext(ctx).First(&run, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

func (r *WorkflowRunRepo) Update(ctx context.Context, run *model.WorkflowRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

func (r *WorkflowRunRepo) List(ctx context.Context, workflowID string, page port.Page) (*port.PageResult[model.WorkflowRun], error) {
	var items []model.WorkflowRun
	var total int64

	q := r.db.WithContext(ctx).Model(&model.WorkflowRun{}).Where("workflow_id = ?", workflowID)
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

	return &port.PageResult[model.WorkflowRun]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *WorkflowRunRepo) CountActive(ctx context.Context, workflowID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WorkflowRun{}).
		Where("workflow_id = ? AND status IN ?", workflowID, []string{
			string(model.WorkflowRunStatusPending),
			string(model.WorkflowRunStatusRunning),
		}).
		Count(&count).Error
	return count, err
}
