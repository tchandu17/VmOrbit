package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// TaskRepo is the GORM-backed task repository.
type TaskRepo struct{ db *gorm.DB }

// NewTaskRepo creates a new TaskRepo.
func NewTaskRepo(db *gorm.DB) *TaskRepo { return &TaskRepo{db: db} }

func (r *TaskRepo) Create(ctx context.Context, t *model.Task) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *TaskRepo) GetByID(ctx context.Context, id string) (*model.Task, error) {
	var t model.Task
	if err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("task not found: %w", err)
	}
	return &t, nil
}

func (r *TaskRepo) Update(ctx context.Context, t *model.Task) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *TaskRepo) List(ctx context.Context, page port.Page) (*port.PageResult[model.Task], error) {
	var items []model.Task
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Task{})
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

	return &port.PageResult[model.Task]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// ListByVMID returns paginated tasks scoped to a specific VM, ordered newest-first.
func (r *TaskRepo) ListByVMID(ctx context.Context, vmID string, page port.Page) (*port.PageResult[model.Task], error) {
	var items []model.Task
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Task{}).Where("vm_id = ?", vmID)
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

	return &port.PageResult[model.Task]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

// ListByParentID returns all child tasks for a given parent task ID.
func (r *TaskRepo) ListByParentID(ctx context.Context, parentID string) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.WithContext(ctx).
		Where("parent_task_id = ?", parentID).
		Order("created_at ASC").
		Find(&tasks).Error
	return tasks, err
}

// ListPending returns tasks in pending state only, ordered by priority
// then creation time. Used by the DB fallback poller to recover tasks
// that were created but never enqueued (e.g. server crashed before Enqueue).
// Queued and retrying tasks are intentionally excluded — they are already
// in Redis or will be re-enqueued by the retry backoff timer.
func (r *TaskRepo) ListPending(ctx context.Context, limit int) ([]model.Task, error) {
	var tasks []model.Task
	err := r.db.WithContext(ctx).
		Where("status = ?", model.TaskStatusPending).
		Order("priority ASC, created_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (r *TaskRepo) UpdateStatus(ctx context.Context, id string, status model.TaskStatus, result model.JSONMap, errMsg string) error {
	updates := map[string]interface{}{
		"status":        status,
		"error_message": errMsg,
	}
	if result != nil {
		updates["result"] = result
	}
	return r.db.WithContext(ctx).
		Model(&model.Task{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateProgress sets the progress percentage (0–100) for a task.
func (r *TaskRepo) UpdateProgress(ctx context.Context, id string, progress int) error {
	return r.db.WithContext(ctx).
		Model(&model.Task{}).
		Where("id = ?", id).
		Update("progress", progress).Error
}

// AppendLog inserts a single TaskLog row.
func (r *TaskRepo) AppendLog(ctx context.Context, entry *model.TaskLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

// GetLogs returns paginated log entries for a task, ordered oldest-first.
func (r *TaskRepo) GetLogs(ctx context.Context, taskID string, page port.Page) (*port.PageResult[model.TaskLog], error) {
	var items []model.TaskLog
	var total int64

	q := r.db.WithContext(ctx).Model(&model.TaskLog{}).Where("task_id = ?", taskID)
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at ASC").Offset(offset).Limit(page.Size).Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.TaskLog]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
