package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// ─────────────────────────────────────────────────────────────────────────────
// PolicyRepo
// ─────────────────────────────────────────────────────────────────────────────

type PolicyRepo struct{ db *gorm.DB }

func NewPolicyRepo(db *gorm.DB) *PolicyRepo { return &PolicyRepo{db: db} }

func (r *PolicyRepo) Create(ctx context.Context, p *model.Policy) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *PolicyRepo) GetByID(ctx context.Context, id string) (*model.Policy, error) {
	var p model.Policy
	if err := r.db.WithContext(ctx).
		Preload("Conditions").
		Preload("Assignments").
		First(&p, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}
	return &p, nil
}

func (r *PolicyRepo) Update(ctx context.Context, p *model.Policy) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *PolicyRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.Policy{}, "id = ?", id).Error
}

func (r *PolicyRepo) List(ctx context.Context, filter port.PolicyFilter, page port.Page) (*port.PageResult[model.Policy], error) {
	var policies []model.Policy
	var total int64

	q := r.db.WithContext(ctx).Model(&model.Policy{})
	if filter.Type != "" {
		q = q.Where("type = ?", filter.Type)
	}
	if filter.Effect != "" {
		q = q.Where("effect = ?", filter.Effect)
	}
	if filter.Enabled != nil {
		q = q.Where("enabled = ?", *filter.Enabled)
	}
	if filter.Search != "" {
		q = q.Where("name ILIKE ? OR description ILIKE ?", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Preload("Conditions").Order("priority ASC, created_at ASC").
		Offset(offset).Limit(page.Size).Find(&policies).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.Policy]{
		Items:      policies,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}

func (r *PolicyRepo) ListEnabled(ctx context.Context) ([]model.Policy, error) {
	var policies []model.Policy
	if err := r.db.WithContext(ctx).
		Where("enabled = true").
		Preload("Conditions").
		Preload("Assignments").
		Order("priority ASC").
		Find(&policies).Error; err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *PolicyRepo) ReplaceConditions(ctx context.Context, policyID string, conditions []model.PolicyCondition) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing conditions
		if err := tx.Where("policy_id = ?", policyID).Delete(&model.PolicyCondition{}).Error; err != nil {
			return err
		}
		// Insert new conditions
		if len(conditions) > 0 {
			return tx.Create(&conditions).Error
		}
		return nil
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// PolicyAssignmentRepo
// ─────────────────────────────────────────────────────────────────────────────

type PolicyAssignmentRepo struct{ db *gorm.DB }

func NewPolicyAssignmentRepo(db *gorm.DB) *PolicyAssignmentRepo {
	return &PolicyAssignmentRepo{db: db}
}

func (r *PolicyAssignmentRepo) Create(ctx context.Context, a *model.PolicyAssignment) error {
	return r.db.WithContext(ctx).Create(a).Error
}

func (r *PolicyAssignmentRepo) GetByID(ctx context.Context, id string) (*model.PolicyAssignment, error) {
	var a model.PolicyAssignment
	if err := r.db.WithContext(ctx).First(&a, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("assignment not found: %w", err)
	}
	return &a, nil
}

func (r *PolicyAssignmentRepo) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&model.PolicyAssignment{}, "id = ?", id).Error
}

func (r *PolicyAssignmentRepo) ListByPolicy(ctx context.Context, policyID string) ([]model.PolicyAssignment, error) {
	var assignments []model.PolicyAssignment
	if err := r.db.WithContext(ctx).
		Where("policy_id = ?", policyID).
		Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

func (r *PolicyAssignmentRepo) ListForContext(ctx context.Context, targetIDs []string) ([]model.PolicyAssignment, error) {
	var assignments []model.PolicyAssignment
	q := r.db.WithContext(ctx).
		Where("target_type = 'global'")
	if len(targetIDs) > 0 {
		q = q.Or("target_id IN ?", targetIDs)
	}
	if err := q.Find(&assignments).Error; err != nil {
		return nil, err
	}
	return assignments, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// PolicyViolationRepo
// ─────────────────────────────────────────────────────────────────────────────

type PolicyViolationRepo struct{ db *gorm.DB }

func NewPolicyViolationRepo(db *gorm.DB) *PolicyViolationRepo {
	return &PolicyViolationRepo{db: db}
}

func (r *PolicyViolationRepo) Create(ctx context.Context, v *model.PolicyViolation) error {
	return r.db.WithContext(ctx).Create(v).Error
}

func (r *PolicyViolationRepo) GetByID(ctx context.Context, id string) (*model.PolicyViolation, error) {
	var v model.PolicyViolation
	if err := r.db.WithContext(ctx).First(&v, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("violation not found: %w", err)
	}
	return &v, nil
}

func (r *PolicyViolationRepo) List(ctx context.Context, filter port.PolicyViolationFilter, page port.Page) (*port.PageResult[model.PolicyViolation], error) {
	var violations []model.PolicyViolation
	var total int64

	q := r.db.WithContext(ctx).Model(&model.PolicyViolation{})
	if filter.PolicyID != nil {
		q = q.Where("policy_id = ?", filter.PolicyID)
	}
	if filter.UserID != nil {
		q = q.Where("user_id = ?", filter.UserID)
	}
	if filter.Operation != "" {
		q = q.Where("operation = ?", filter.Operation)
	}
	if filter.ResourceType != "" {
		q = q.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		q = q.Where("resource_id = ?", filter.ResourceID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", filter.Until)
	}

	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}

	offset := (page.Number - 1) * page.Size
	if err := q.Order("created_at DESC").Offset(offset).Limit(page.Size).Find(&violations).Error; err != nil {
		return nil, err
	}

	totalPages := int(total) / page.Size
	if int(total)%page.Size != 0 {
		totalPages++
	}

	return &port.PageResult[model.PolicyViolation]{
		Items:      violations,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
