package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/vmOrbit/backend/internal/domain/model"
	"github.com/vmOrbit/backend/internal/domain/port"
)

// AuditRepo is the GORM-backed audit log repository.
// Audit logs are append-only: Create is the only write operation.
type AuditRepo struct{ db *gorm.DB }

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *gorm.DB) *AuditRepo { return &AuditRepo{db: db} }

func (r *AuditRepo) Create(ctx context.Context, entry *model.AuditLog) error {
	return r.db.WithContext(ctx).Create(entry).Error
}

func (r *AuditRepo) List(ctx context.Context, filter port.AuditFilter, page port.Page) (*port.PageResult[model.AuditLog], error) {
	var items []model.AuditLog
	var total int64

	q := r.db.WithContext(ctx).Model(&model.AuditLog{})

	if filter.UserID != nil {
		q = q.Where("user_id = ?", *filter.UserID)
	}
	if filter.HypervisorID != nil {
		q = q.Where("hypervisor_id = ?", *filter.HypervisorID)
	}
	if filter.Resource != "" {
		q = q.Where("resource = ?", filter.Resource)
	}
	if filter.ResourceID != nil {
		q = q.Where("resource_id = ?", *filter.ResourceID)
	}
	if filter.Action != "" {
		q = q.Where("action = ?", filter.Action)
	}
	if filter.Since != nil {
		q = q.Where("created_at >= ?", *filter.Since)
	}
	if filter.Until != nil {
		q = q.Where("created_at <= ?", *filter.Until)
	}
	if filter.SuccessOnly != nil {
		q = q.Where("success = ?", *filter.SuccessOnly)
	}
	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		q = q.Where("(username ILIKE ? OR description ILIKE ?)", like, like)
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

	return &port.PageResult[model.AuditLog]{
		Items:      items,
		TotalItems: total,
		TotalPages: totalPages,
		Page:       page.Number,
		PageSize:   page.Size,
	}, nil
}
