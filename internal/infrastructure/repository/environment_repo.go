package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// EnvironmentRepo
// ─────────────────────────────────────────────────────────────────────────────

type EnvironmentRepo struct{ db *gorm.DB }

func NewEnvironmentRepo(db *gorm.DB) *EnvironmentRepo { return &EnvironmentRepo{db: db} }

func (r *EnvironmentRepo) Create(ctx context.Context, env *model.Environment) error {
	return r.db.WithContext(ctx).Create(env).Error
}

func (r *EnvironmentRepo) GetByID(ctx context.Context, id string) (*model.Environment, error) {
	var env model.Environment
	err := r.db.WithContext(ctx).
		Preload("VMs.VM.Hypervisor").
		Preload("EnvTags.Tag").
		First(&env, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("environment not found: %w", err)
	}
	return &env, nil
}

func (r *EnvironmentRepo) Update(ctx context.Context, env *model.Environment) error {
	return r.db.WithContext(ctx).Save(env).Error
}

func (r *EnvironmentRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove orchestration steps → runs → dependencies → env VMs → env tags → environment
		if err := tx.Exec(
			`DELETE FROM orchestration_steps WHERE run_id IN (SELECT id FROM orchestration_runs WHERE environment_id = ?)`, id,
		).Error; err != nil {
			return fmt.Errorf("deleting orchestration steps: %w", err)
		}
		if err := tx.Unscoped().Where("environment_id = ?", id).Delete(&model.OrchestrationRun{}).Error; err != nil {
			return fmt.Errorf("deleting orchestration runs: %w", err)
		}
		if err := tx.Unscoped().Where("environment_id = ?", id).Delete(&model.VMDependency{}).Error; err != nil {
			return fmt.Errorf("deleting vm dependencies: %w", err)
		}
		if err := tx.Unscoped().Where("environment_id = ?", id).Delete(&model.EnvironmentVM{}).Error; err != nil {
			return fmt.Errorf("deleting environment vms: %w", err)
		}
		if err := tx.Where("environment_id = ?", id).Delete(&model.EnvironmentTag{}).Error; err != nil {
			return fmt.Errorf("deleting environment tags: %w", err)
		}
		if err := tx.Unscoped().Where("id = ?", id).Delete(&model.Environment{}).Error; err != nil {
			return fmt.Errorf("deleting environment: %w", err)
		}
		return nil
	})
}

func (r *EnvironmentRepo) List(ctx context.Context, filter port.EnvironmentFilter, page port.Page) (*port.PageResult[model.Environment], error) {
	var items []model.Environment
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Environment{})
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.OwnerID != "" {
		q = q.Where("owner_id = ?", filter.OwnerID)
	}
	if filter.Search != "" {
		q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Offset(offset).Limit(page.Size).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.Environment]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *EnvironmentRepo) UpdateStatus(ctx context.Context, id string, status model.EnvironmentStatus) error {
	return r.db.WithContext(ctx).
		Model(&model.Environment{}).
		Where("id = ?", id).
		Update("status", status).Error
}

// ─────────────────────────────────────────────────────────────────────────────
// EnvironmentVMRepo
// ─────────────────────────────────────────────────────────────────────────────

type EnvironmentVMRepo struct{ db *gorm.DB }

func NewEnvironmentVMRepo(db *gorm.DB) *EnvironmentVMRepo { return &EnvironmentVMRepo{db: db} }

func (r *EnvironmentVMRepo) AddVM(ctx context.Context, ev *model.EnvironmentVM) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "environment_id"}, {Name: "vm_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"start_order", "stop_order", "role", "notes", "updated_at"}),
		}).
		Create(ev).Error
}

func (r *EnvironmentVMRepo) RemoveVM(ctx context.Context, environmentID, vmID string) error {
	return r.db.WithContext(ctx).
		Unscoped().
		Where("environment_id = ? AND vm_id = ?", environmentID, vmID).
		Delete(&model.EnvironmentVM{}).Error
}

func (r *EnvironmentVMRepo) ListByEnvironment(ctx context.Context, environmentID string) ([]model.EnvironmentVM, error) {
	var items []model.EnvironmentVM
	err := r.db.WithContext(ctx).
		Preload("VM.Hypervisor").
		Where("environment_id = ?", environmentID).
		Order("start_order ASC, created_at ASC").
		Find(&items).Error
	return items, err
}

func (r *EnvironmentVMRepo) UpdateOrdering(ctx context.Context, environmentID, vmID string, startOrder, stopOrder int, role string) error {
	return r.db.WithContext(ctx).
		Model(&model.EnvironmentVM{}).
		Where("environment_id = ? AND vm_id = ?", environmentID, vmID).
		Updates(map[string]interface{}{
			"start_order": startOrder,
			"stop_order":  stopOrder,
			"role":        role,
		}).Error
}

func (r *EnvironmentVMRepo) GetByEnvironmentAndVM(ctx context.Context, environmentID, vmID string) (*model.EnvironmentVM, error) {
	var ev model.EnvironmentVM
	err := r.db.WithContext(ctx).
		Where("environment_id = ? AND vm_id = ?", environmentID, vmID).
		First(&ev).Error
	if err != nil {
		return nil, fmt.Errorf("environment vm not found: %w", err)
	}
	return &ev, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// VMDependencyRepo
// ─────────────────────────────────────────────────────────────────────────────

type VMDependencyRepo struct{ db *gorm.DB }

func NewVMDependencyRepo(db *gorm.DB) *VMDependencyRepo { return &VMDependencyRepo{db: db} }

func (r *VMDependencyRepo) Create(ctx context.Context, dep *model.VMDependency) error {
	return r.db.WithContext(ctx).Create(dep).Error
}

func (r *VMDependencyRepo) GetByID(ctx context.Context, id string) (*model.VMDependency, error) {
	var dep model.VMDependency
	err := r.db.WithContext(ctx).
		Preload("SourceVM").
		Preload("TargetVM").
		First(&dep, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("dependency not found: %w", err)
	}
	return &dep, nil
}

func (r *VMDependencyRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&model.VMDependency{}).Error
}

func (r *VMDependencyRepo) ListByEnvironment(ctx context.Context, environmentID string) ([]model.VMDependency, error) {
	var items []model.VMDependency
	err := r.db.WithContext(ctx).
		Preload("SourceVM").
		Preload("TargetVM").
		Where("environment_id = ?", environmentID).
		Find(&items).Error
	return items, err
}

func (r *VMDependencyRepo) ListDependenciesOf(ctx context.Context, environmentID, vmID string) ([]model.VMDependency, error) {
	var items []model.VMDependency
	err := r.db.WithContext(ctx).
		Where("environment_id = ? AND source_vm_id = ?", environmentID, vmID).
		Find(&items).Error
	return items, err
}

func (r *VMDependencyRepo) ListDependentsOf(ctx context.Context, environmentID, vmID string) ([]model.VMDependency, error) {
	var items []model.VMDependency
	err := r.db.WithContext(ctx).
		Where("environment_id = ? AND target_vm_id = ?", environmentID, vmID).
		Find(&items).Error
	return items, err
}

// ─────────────────────────────────────────────────────────────────────────────
// OrchestrationRunRepo
// ─────────────────────────────────────────────────────────────────────────────

type OrchestrationRunRepo struct{ db *gorm.DB }

func NewOrchestrationRunRepo(db *gorm.DB) *OrchestrationRunRepo {
	return &OrchestrationRunRepo{db: db}
}

func (r *OrchestrationRunRepo) Create(ctx context.Context, run *model.OrchestrationRun) error {
	return r.db.WithContext(ctx).Create(run).Error
}

func (r *OrchestrationRunRepo) GetByID(ctx context.Context, id string) (*model.OrchestrationRun, error) {
	var run model.OrchestrationRun
	err := r.db.WithContext(ctx).
		Preload("Steps.VM").
		First(&run, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("orchestration run not found: %w", err)
	}
	return &run, nil
}

func (r *OrchestrationRunRepo) Update(ctx context.Context, run *model.OrchestrationRun) error {
	return r.db.WithContext(ctx).Save(run).Error
}

func (r *OrchestrationRunRepo) List(ctx context.Context, environmentID string, page port.Page) (*port.PageResult[model.OrchestrationRun], error) {
	var items []model.OrchestrationRun
	var total int64

	q := r.db.WithContext(ctx).Model(&model.OrchestrationRun{}).Where("environment_id = ?", environmentID)

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Offset(offset).Limit(page.Size).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.OrchestrationRun]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *OrchestrationRunRepo) UpdateProgress(ctx context.Context, id string, progress, completed, failed, skipped int) error {
	return r.db.WithContext(ctx).
		Model(&model.OrchestrationRun{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"progress":      progress,
			"completed_vms": completed,
			"failed_vms":    failed,
			"skipped_vms":   skipped,
		}).Error
}

func (r *OrchestrationRunRepo) UpdateStatus(ctx context.Context, id string, status model.OrchestrationRunStatus, errMsg string) error {
	updates := map[string]interface{}{"status": status}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	return r.db.WithContext(ctx).
		Model(&model.OrchestrationRun{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// ─────────────────────────────────────────────────────────────────────────────
// OrchestrationStepRepo
// ─────────────────────────────────────────────────────────────────────────────

type OrchestrationStepRepo struct{ db *gorm.DB }

func NewOrchestrationStepRepo(db *gorm.DB) *OrchestrationStepRepo {
	return &OrchestrationStepRepo{db: db}
}

func (r *OrchestrationStepRepo) CreateBatch(ctx context.Context, steps []model.OrchestrationStep) error {
	if len(steps) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&steps).Error
}

func (r *OrchestrationStepRepo) GetByID(ctx context.Context, id string) (*model.OrchestrationStep, error) {
	var step model.OrchestrationStep
	err := r.db.WithContext(ctx).Preload("VM").First(&step, "id = ?", id).Error
	if err != nil {
		return nil, fmt.Errorf("orchestration step not found: %w", err)
	}
	return &step, nil
}

func (r *OrchestrationStepRepo) ListByRun(ctx context.Context, runID string) ([]model.OrchestrationStep, error) {
	var items []model.OrchestrationStep
	err := r.db.WithContext(ctx).
		Preload("VM.Hypervisor").
		Where("run_id = ?", runID).
		Order("execution_order ASC").
		Find(&items).Error
	return items, err
}

func (r *OrchestrationStepRepo) UpdateStatus(ctx context.Context, id string, status model.OrchestrationStepStatus, taskID *string, errMsg string) error {
	updates := map[string]interface{}{"status": status}
	if taskID != nil {
		updates["task_id"] = *taskID
	}
	if errMsg != "" {
		updates["error_message"] = errMsg
	}
	return r.db.WithContext(ctx).
		Model(&model.OrchestrationStep{}).
		Where("id = ?", id).
		Updates(updates).Error
}
